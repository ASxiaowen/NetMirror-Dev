/**
 * NetMirror 二次开发 · 登录态
 * 目录: ui/src/custom_components/useAuth.js
 *
 * 把「配置探测 → 校验已有令牌 → 登录 → 登出」这条链路收敛到一处，
 * RootShell 与 LoginView 只消费这里的状态，不直接碰接口。
 */

import { ref } from 'vue'
import { request, ErrCode } from './apiClient'
import {
  authState,
  setUserToken,
  setShareToken,
  clearAuth,
  loadStoredUserToken
} from './authState'

export function useAuth() {
  /** 后端登录门的公开配置 */
  const config = ref({ enabled: false, tokenTtlHours: 168 })
  /** 是否正在处理（登录 / 校验） */
  const loading = ref(false)
  /** 最近一次错误文案 */
  const error = ref('')

  /** 读取后端配置（无需认证）。顺带把 enabled 落到 authState 供 apiClient 判断。 */
  const fetchConfig = async () => {
    try {
      const d = await request('/custom/auth/config')
      config.value = {
        enabled: !!d?.enabled,
        tokenTtlHours: d?.tokenTtlHours || 168
      }
    } catch (e) {
      // 后端不通时不阻塞页面：按「未启用登录门」处理，让原界面照常显示
      config.value = { enabled: false, tokenTtlHours: 168 }
    }
    authState.enabled = config.value.enabled
    return config.value
  }

  /**
   * 校验当前令牌是否仍然有效。
   * @returns {Promise<{valid:boolean, kind?:string, scope?:object, error?:string}>}
   */
  const verify = async () => {
    try {
      const d = await request('/custom/auth/verify')
      if (d?.valid) {
        return { valid: true, kind: d.kind || d.scope?.kind || 'user', scope: d.scope || null }
      }
      return { valid: false, error: d?.error || '令牌无效' }
    } catch (e) {
      return { valid: false, error: e?.message || '令牌校验失败' }
    }
  }

  /**
   * 口令登录。
   * @param {string} password
   * @returns {Promise<boolean>} 是否成功（失败时 error 已填好文案）
   */
  const login = async (password) => {
    loading.value = true
    error.value = ''
    try {
      const d = await request('/custom/auth/login', {
        method: 'POST',
        data: { password },
        timeout: 20000
      })
      if (!d?.success || !d?.token) {
        error.value = d?.error || '登录失败'
        return false
      }
      setUserToken(d.token, d.expiresAt)
      return true
    } catch (e) {
      error.value =
        e?.code === ErrCode.AUTH_REQUIRED || e?.status === 401
          ? '口令不正确'
          : e?.message || '登录失败'
      return false
    } finally {
      loading.value = false
    }
  }

  /** 登出：无状态令牌由前端丢弃即可 */
  const logout = async () => {
    try {
      await request('/custom/auth/logout', { method: 'POST', timeout: 8000 })
    } catch (e) {
      // 后端不可达也要让本地登出成功
    }
    clearAuth()
  }

  /**
   * 启动期解析身份。优先处理 URL 上的临时链接令牌，其次用本地持久化的登录令牌。
   * @returns {Promise<'share'|'user'|'none'|'share-invalid'>}
   */
  const bootstrapIdentity = async (shareToken) => {
    if (shareToken) {
      try {
        const d = await request(`/custom/link/${encodeURIComponent(shareToken)}`, { timeout: 15000 })
        if (d?.valid && d?.scope) {
          setShareToken(shareToken, d.scope)
          return 'share'
        }
      } catch (e) {
        // 失效原因（过期 / 吊销）写进 error 供落地页展示
        error.value = e?.payload?.error || e?.message || '该临时链接不可用'
        return 'share-invalid'
      }
      error.value = '该临时链接不可用'
      return 'share-invalid'
    }

    const stored = loadStoredUserToken()
    if (stored) {
      setUserToken(stored, 0)
      const r = await verify()
      if (r.valid) return 'user'
      // 令牌已失效：verify 内部已通过 onAuthExpired 之外的方式失败，这里显式清掉
      clearAuth()
      return 'none'
    }
    return 'none'
  }

  return { config, loading, error, fetchConfig, verify, login, logout, bootstrapIdentity }
}

export default useAuth
