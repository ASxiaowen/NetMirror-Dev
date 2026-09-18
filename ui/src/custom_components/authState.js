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
 *   - user  登录令牌：可持久化到 localStorage，用于整站访问。
 *   - share 临时链接令牌：来自 URL（/t/<token> 或 ?t=<token>），
 *     刻意**不落地**——它只属于本次访问，刷新时从 URL 重新解析即可。
 */

import { reactive } from 'vue'

const USER_TOKEN_KEY = 'nm_custom_user_token'
const USER_EXP_KEY = 'nm_custom_user_exp'

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
  checked: false
})

export const isShareMode = () => authState.kind === 'share'
export const isLoggedIn = () => authState.kind === 'user'

/** 写入登录令牌并持久化 */
export function setUserToken(token, expiresAt) {
  authState.token = token || ''
  authState.kind = token ? 'user' : ''
  authState.scope = null
  authState.expiresAt = expiresAt || 0
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

/** 写入临时链接令牌（不持久化） */
export function setShareToken(token, scope) {
  authState.token = token || ''
  authState.kind = token ? 'share' : ''
  authState.scope = scope || null
  authState.expiresAt = scope?.exp || 0
}

/** 清空认证状态 */
export function clearAuth() {
  authState.token = ''
  authState.kind = ''
  authState.scope = null
  authState.expiresAt = 0
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
 * 从当前页面 URL 解析临时链接令牌。
 * 支持两种形态：/t/<token> 与 ?t=<token>（后者用于 nginx 静态托管的部署）。
 */
export function parseShareTokenFromUrl(href) {
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
