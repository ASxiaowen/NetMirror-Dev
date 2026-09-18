/**
 * NetMirror 二次开发 · 统一 API 代理层
 * 目录: ui/src/custom_components/apiClient.js
 *
 * 解决的问题（上游的现状）：
 *   /session、/method/*、/nodes 的调用散落在 stores/app.js、stores/nodes.js、
 *   Speedtest/Librespeed.vue 里，各自直接 fetch / new EventSource / axios：
 *     · 没有统一位置注入认证令牌；
 *     · 错误处理各写各的，401/超时/节点离线在前端无法区分，只能白屏或英文报错；
 *     · 「当前页面源」与「节点地址」的拼接规则重复三遍，改一处要动三个文件。
 *
 * 本文件把这些收敛成一层，上游文件只把 fetch/EventSource 换成这里的调用，
 * 业务逻辑（重试、状态机、UI）仍留在原文件，符合规范第 1 条「最小化侵入」。
 */

import axios from 'axios'
import { authState, authHeaders, appendToken } from './authState'

/** 统一的错误码。后端返回同名 code，前端据此给出不同引导文案。 */
export const ErrCode = {
  /** 未登录 / 未携带令牌 */
  AUTH_REQUIRED: 'AUTH_REQUIRED',
  /** 令牌过期，需重新登录 */
  TOKEN_EXPIRED: 'TOKEN_EXPIRED',
  /** 令牌非法（被篡改、签名不符） */
  TOKEN_INVALID: 'TOKEN_INVALID',
  /** 已认证但无权限（临时链接越权 / 功能未授权） */
  FORBIDDEN: 'FORBIDDEN',
  /** 请求超时 */
  TIMEOUT: 'TIMEOUT',
  /** 被限流 */
  RATE_LIMITED: 'RATE_LIMITED',
  /** 节点不可达 */
  NODE_OFFLINE: 'NODE_OFFLINE',
  /** 服务端错误 */
  SERVER_ERROR: 'SERVER_ERROR',
  /** 其它未知错误 */
  UNKNOWN: 'UNKNOWN'
}

/** 各错误码对应的中文提示，集中维护，避免文案散落 */
export const ErrMessage = {
  AUTH_REQUIRED: '请先登录后再使用',
  TOKEN_EXPIRED: '登录状态已过期，请重新登录',
  TOKEN_INVALID: '访问令牌无效',
  FORBIDDEN: '当前权限不足，无法执行该操作',
  TIMEOUT: '请求超时，请检查网络或稍后重试',
  RATE_LIMITED: '请求过于频繁，请稍后重试',
  NODE_OFFLINE: '节点无法连接，可能已离线',
  SERVER_ERROR: '服务端异常，请稍后重试',
  UNKNOWN: '请求失败'
}

/** 统一的 ApiError，调用方可按 code 分支处理 */
export class ApiError extends Error {
  constructor(code, message, status = 0, payload = null, raw = null) {
    super(message || ErrMessage[code] || ErrMessage.UNKNOWN)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.payload = payload
    this.raw = raw
  }
}

/**
 * 令牌失效回调。RootShell 注册它来把用户弹回登录页；
 * 用回调而不是直接 import 组件，避免 apiClient → 组件 → store → apiClient 的循环依赖。
 */
let authExpiredHandler = null
export function onAuthExpired(fn) {
  authExpiredHandler = fn
}
function notifyAuthExpired(code) {
  if (typeof authExpiredHandler === 'function') {
    try {
      authExpiredHandler(code)
    } catch (e) {
      console.error('[apiClient] onAuthExpired 回调异常', e)
    }
  }
}

/** node 缺省表示「当前页面所在源」；给了 node 则用节点的绝对地址。 */
function resolveUrl(path, node) {
  if (!node || !node.url) return path
  return String(node.url).replace(/\/+$/, '') + path
}

/** 把 HTTP 响应归类成 ApiError */
function classifyResponse(res, node) {
  const status = res.status
  const body = res.data && typeof res.data === 'object' ? res.data : null
  const serverCode = body?.code || ''
  const serverMsg = body?.error || ''

  // 服务端已给出明确 code（auth 模块的 401/403），优先采用
  if (serverCode && ErrMessage[serverCode]) {
    if (serverCode === ErrCode.TOKEN_EXPIRED || serverCode === ErrCode.AUTH_REQUIRED) {
      notifyAuthExpired(serverCode)
    }
    return new ApiError(serverCode, serverMsg, status, body, res)
  }
  if (status === 401) {
    notifyAuthExpired(ErrCode.AUTH_REQUIRED)
    return new ApiError(ErrCode.AUTH_REQUIRED, serverMsg, status, body, res)
  }
  if (status === 403) return new ApiError(ErrCode.FORBIDDEN, serverMsg, status, body, res)
  if (status === 429) return new ApiError(ErrCode.RATE_LIMITED, serverMsg, status, body, res)
  if (status >= 500) return new ApiError(ErrCode.SERVER_ERROR, serverMsg, status, body, res)
  return new ApiError(ErrCode.UNKNOWN, serverMsg || `HTTP ${status}`, status, body, res)
}

/** 把网络层异常归类成 ApiError */
function classifyException(err, node) {
  if (err instanceof ApiError) return err
  if (axios.isCancel?.(err) || err?.code === 'ERR_CANCELED') {
    const e = new ApiError(ErrCode.UNKNOWN, '请求已取消', 0, null, err)
    e.canceled = true
    return e
  }
  if (err?.code === 'ECONNABORTED' || err?.code === 'ETIMEDOUT') {
    return new ApiError(ErrCode.TIMEOUT, '', 0, null, err)
  }
  if (err?.code === 'ERR_NETWORK') {
    // 有 node 说明是节点侧不可达；没有则多半是本站后端挂了
    return new ApiError(node ? ErrCode.NODE_OFFLINE : ErrCode.SERVER_ERROR, '', 0, null, err)
  }
  return new ApiError(ErrCode.UNKNOWN, err?.message || '', 0, null, err)
}

/**
 * 统一请求。
 *
 * @param {string} path            以 / 开头的路径，如 '/custom/auth/login'
 * @param {object} [opts]
 * @param {string} [opts.method]   HTTP 方法，默认 GET
 * @param {object} [opts.params]   查询参数
 * @param {object} [opts.data]     请求体（自动 JSON）
 * @param {object} [opts.node]     目标节点 {url}；省略 = 当前页面所在源
 * @param {string} [opts.session]  节点会话 id，写入 session 请求头
 * @param {number} [opts.timeout]  超时毫秒，默认 120s
 * @param {AbortSignal} [opts.signal]
 * @param {boolean} [opts.raw]     true 返回完整 axios 响应，false 只返回 data
 * @returns {Promise<any>}         解析后的响应体
 */
export async function request(path, opts = {}) {
  const {
    method = 'GET',
    params,
    data,
    node,
    session,
    timeout = 120000,
    signal,
    raw = false
  } = opts

  const headers = { ...authHeaders() }
  if (session) headers.session = session
  if (data !== undefined) headers['Content-Type'] = 'application/json'

  try {
    const res = await axios({
      url: resolveUrl(path, node),
      method,
      params,
      data,
      headers,
      timeout,
      signal,
      // 自己判状态码，才能把 401/403 归类成带 code 的错误而不是抛裸异常
      validateStatus: () => true
    })
    if (res.status >= 200 && res.status < 300) {
      return raw ? res : res.data
    }
    throw classifyResponse(res, node)
  } catch (err) {
    throw classifyException(err, node)
  }
}

/**
 * 创建一个带认证的 EventSource。
 *
 * EventSource 不能设置请求头，所以令牌只能走查询参数（后端 ExtractToken 支持）。
 */
export function createEventSource(path, opts = {}) {
  const { node, params } = opts
  const url = appendToken(resolveUrl(path, node), params)
  return new EventSource(url)
}

/** 构造带令牌的资源 URL（供 speedtest worker 这类无法加请求头的场景） */
export function authedUrl(path, node, params) {
  return appendToken(resolveUrl(path, node), params)
}

/** 目标是否与当前页面同源（同源时不必构造绝对地址，便于反代部署） */
export function isSameOrigin(node) {
  if (!node || !node.url) return true
  try {
    return new URL(node.url, window.location.origin).origin === window.location.origin
  } catch (e) {
    return false
  }
}

/** 后端是否启用了登录门 */
export function authEnabled() {
  return !!authState.enabled
}

export default { request, createEventSource, authedUrl, isSameOrigin, ErrCode, ErrMessage, ApiError, onAuthExpired, authEnabled }
