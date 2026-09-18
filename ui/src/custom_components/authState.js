/**
 * NetMirror 二次开发 · 认证状态（单例）
 * 目录: ui/src/custom_components/authState.js
 *
 * 为什么用「模块级 reactive 单例」而不是 pinia store：
 *   apiClient.js 需要在任何组件之外（甚至在 pinia 安装之前）读取令牌，
 *   如果做成 store 就会引入 app.use(pinia) 的时序依赖与潜在的循环 import。
 *   这里只放状态与纯函数，不依赖任何上层模块。
 *
 * 两类令牌：
 *   - user  登录令牌（账号+密码换取）：持久化到 localStorage，用于整站访问。
 *   - share 临时链接令牌（链接 id + 临时密码换取）：持久化到 sessionStorage，
 *     按链接 id 分键。用 session 而不是 local 是因为它是「这一次测试」的凭据，
 *     关掉标签页就该消失；分键则让同一个人同时开多条不同链接互不覆盖。
 *
 * 注意：URL 里的 /t/<id> 只是**链接标识**，不是凭据 —— 必须再用临时密码
 * 兑换出共享令牌才能访问。所以这里对 id 与令牌是分开处理的。
 */

import { reactive } from 'vue'

const USER_TOKEN_KEY = 'nm_custom_user_token'
const USER_EXP_KEY = 'nm_custom_user_exp'
const SHARE_KEY_PREFIX = 'nm_custom_share_'

export const authState = reactive({
  /** 当前令牌原文 */
  token: '',
  /** '' | 'user' | 'share' */
  kind: '',
  /** kind=share 时的作用域（节点、工具白名单、有效期） */
  scope: null,
  /** 过期时间（秒级 Unix 时间戳），0 表示未知 */
  expiresAt: 0,
  /** 后端是否启用了登录门（由 /custom/auth/config 得到） */
  enabled: false,
  /** 是否已完成一次启动期校验 */
  checked: false,
  /** 当前登录账号名（kind=user 时由后端回显，仅用于界面展示） */
  username: '',
  /** kind=share 时对应的链接标识，用于清理 sessionStorage 与展示 */
  linkId: ''
})

export const isShareMode = () => authState.kind === 'share'
export const isLoggedIn = () => authState.kind === 'user'

/** 写入登录令牌并持久化 */
export function setUserToken(token, expiresAt) {
  authState.token = token || ''
  authState.kind = token ? 'user' : ''
  authState.scope = null
  authState.expiresAt = expiresAt || 0
  authState.linkId = ''
  try {
    if (token) {
      localStorage.setItem(USER_TOKEN_KEY, token)
      localStorage.setItem(USER_EXP_KEY, String(expiresAt || 0))
    } else {
      localStorage.removeItem(USER_TOKEN_KEY)
      localStorage.removeItem(USER_EXP_KEY)
    }
  } catch (e) {
    // 隐私模式 / 禁用存储时 localStorage 会抛异常，不影响本次会话可用
  }
}

/** 写入临时链接令牌（按链接 id 存进 sessionStorage） */
export function setShareToken(token, scope, linkId) {
  authState.token = token || ''
  authState.kind = token ? 'share' : ''
  authState.scope = scope || null
  authState.expiresAt = scope?.exp || 0
  authState.linkId = linkId || ''
  if (token && linkId) {
    try {
      sessionStorage.setItem(SHARE_KEY_PREFIX + linkId, token)
    } catch (e) {
      // 存不下也不影响本次访问，只是刷新后要重新输一次密码
    }
  }
}

/** 读取某条链接已兑换的令牌（刷新页面时免去重新输密码） */
export function loadStoredShareToken(linkId) {
  if (!linkId) return ''
  try {
    return sessionStorage.getItem(SHARE_KEY_PREFIX + linkId) || ''
  } catch (e) {
    return ''
  }
}

/** 丢弃某条链接的令牌 */
export function clearStoredShareToken(linkId) {
  if (!linkId) return
  try {
    sessionStorage.removeItem(SHARE_KEY_PREFIX + linkId)
  } catch (e) {
    /* 同上 */
  }
}

/** 清空认证状态 */
export function clearAuth() {
  const prevLink = authState.linkId
  authState.token = ''
  authState.kind = ''
  authState.scope = null
  authState.expiresAt = 0
  authState.linkId = ''
  clearStoredShareToken(prevLink)
  try {
    localStorage.removeItem(USER_TOKEN_KEY)
    localStorage.removeItem(USER_EXP_KEY)
  } catch (e) {
    /* 同上 */
  }
}

/** 读取本地持久化的登录令牌，已过期则自动清理 */
export function loadStoredUserToken() {
  try {
    const t = localStorage.getItem(USER_TOKEN_KEY)
    if (!t) return ''
    const exp = parseInt(localStorage.getItem(USER_EXP_KEY) || '0', 10)
    if (exp && Date.now() / 1000 > exp) {
      clearAuth()
      return ''
    }
    return t
  } catch (e) {
    return ''
  }
}

/** 供 axios 使用的认证请求头 */
export function authHeaders() {
  return authState.token ? { Authorization: 'Bearer ' + authState.token } : {}
}

/**
 * 给 URL 附加令牌查询参数。
 *
 * 为什么必须支持 query 传令牌：浏览器的 EventSource 无法自定义请求头，
 * SSE（/session 的会话与事件流）只能靠查询参数携带身份。
 */
export function appendToken(url, params) {
  const u = new URL(url, window.location.origin)
  if (authState.token) u.searchParams.set('token', authState.token)
  if (params) {
    Object.keys(params).forEach((k) => {
      const v = params[k]
      if (v !== undefined && v !== null && v !== '') u.searchParams.set(k, v)
    })
  }
  // 同源时去掉 origin，保持相对路径（便于反向代理下的部署）
  if (u.origin === window.location.origin) return u.pathname + u.search + u.hash
  return u.toString()
}

/**
 * 从当前页面 URL 解析临时链接标识。
 *
 * 返回的是**链接标识**（/t/<id> 里的 id），不是可用凭据 ——
 * 还需要用临时密码去 /custom/sharelink/redeem 兑换令牌。
 *
 * 支持两种形态：/t/<id> 与 ?t=<id>（后者用于 nginx 静态托管的部署）。
 */
export function parseShareLinkFromUrl(href) {
  try {
    const u = new URL(href || window.location.href)
    const q = u.searchParams.get('t') || ''
    if (q) return q.trim()
    const m = u.pathname.match(/^\/t\/([^/]+)\/?$/)
    if (m) return decodeURIComponent(m[1]).trim()
    return ''
  } catch (e) {
    return ''
  }
}

/** 兼容旧名，语义同上 */
export const parseShareTokenFromUrl = parseShareLinkFromUrl
