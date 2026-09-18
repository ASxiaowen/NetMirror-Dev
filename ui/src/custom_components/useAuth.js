/**
 * NetMirror 二次开发 · 登录态
 * 目录: ui/src/custom_components/useAuth.js
 *
 * 把「配置探测 → 校验已有令牌 → 登录 → 登出」这条链路收敛到一处，
 * RootShell 与 LoginView 只消费这里的状态，不直接碰接口。
 *
 * 临时链接的兑换不在本文件：它属于「访客侧」的流程，见 useShare.js
 * 的 linkInfo / redeemLink。这样登录门与临时链接两条链路各自独立，
 * 任何一条改动都不会牵动另一条。
 */

import { ref } from 'vue'
import { request, ErrCode } from './apiClient'
import { authState, setUserToken, clearAuth, loadStoredUserToken } from './authState'

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
        const scope = d.scope || null
        if (scope?.username) authState.username = scope.username
        return { valid: true, kind: d.kind || scope?.kind || 'user', scope }
      }
      return { valid: false, error: d?.error || '令牌无效' }
    } catch (e) {
      return { valid: false, error: e?.message || '令牌校验失败' }
    }
  }

  /**
   * 账号 + 密码登录。
   * @param {string} username
   * @param {string} password
   * @returns {Promise<boolean>} 是否成功（失败时 error 已填好文案）
   */
  const login = async (username, password) => {
    loading.value = true
    error.value = ''
    try {
      const d = await request('/custom/auth/login', {
        method: 'POST',
        data: { username, password },
        timeout: 20000
      })
      if (!d?.success || !d?.token) {
        error.value = d?.error || '登录失败'
        return false
      }
      authState.username = d.username || username
      setUserToken(d.token, d.expiresAt)
      return true
    } catch (e) {
      // 后端对「账号错」与「密码错」返回同一文案，前端不做二次区分
      error.value =
        e?.code === ErrCode.AUTH_REQUIRED || e?.status === 401
          ? '账号或密码不正确'
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
    authState.username = ''
    clearAuth()
  }

  /**
   * 启动期解析身份：仅处理本地持久化的登录令牌。
   * 临时链接的令牌由 RootShell 走「查链接信息 → 输密码 → 兑换」流程建立。
   * @returns {Promise<'user'|'none'>}
   */
  const bootstrapIdentity = async () => {
    const stored = loadStoredUserToken()
    if (!stored) return 'none'
    setUserToken(stored, 0)
    const r = await verify()
    if (r.valid) return 'user'
    clearAuth()
    return 'none'
  }

  return { config, loading, error, fetchConfig, verify, login, logout, bootstrapIdentity }
}

export default useAuth
