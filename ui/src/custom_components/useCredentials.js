/**
 * NetMirror 二次开发 · 登录凭据（账号密码）管理
 * 目录: ui/src/custom_components/useCredentials.js
 *
 * 与 useAuth 的分工：
 *   - useAuth       管「当前这次浏览是不是登录了」，面向登录页；
 *   - useCredentials 管「账号密码本身是什么、怎么改」，面向管理页。
 *
 * 后端契约（backend/custom_modules/auth/handlers.go）：
 *   GET  /custom/auth/credentials → { enabled, username, password, source, minPassword, filePath }
 *   POST /custom/auth/credentials ← { currentPassword, username?, password? }
 *
 * 两项都是「仅登录用户可访问」，临时链接令牌会被后端 403 挡住。
 */

import { ref } from 'vue'
import { request } from './apiClient'

export function useCredentials() {
  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')

  const info = ref({
    enabled: false,
    username: '',
    password: '',
    /** 'env'（来自环境变量） | 'runtime'（已在页面改过） */
    source: 'env',
    minPassword: 8,
    filePath: ''
  })

  /** 读取当前账号密码 */
  const load = async () => {
    loading.value = true
    error.value = ''
    try {
      const d = await request('/custom/auth/credentials', { timeout: 15000 })
      info.value = {
        enabled: !!d?.enabled,
        username: d?.username || '',
        password: d?.password || '',
        source: d?.source || 'env',
        minPassword: d?.minPassword || 8,
        filePath: d?.filePath || ''
      }
      return info.value
    } catch (e) {
      error.value = e?.message || '读取凭据失败'
      throw e
    } finally {
      loading.value = false
    }
  }

  /**
   * 保存账号 / 密码。留空表示该项不改。
   * @param {{currentPassword:string, username?:string, password?:string}} payload
   */
  const save = async (payload) => {
    saving.value = true
    error.value = ''
    try {
      const d = await request('/custom/auth/credentials', {
        method: 'POST',
        data: {
          currentPassword: payload.currentPassword || '',
          username: payload.username || '',
          password: payload.password || ''
        },
        timeout: 20000
      })
      if (d?.username) {
        info.value.username = d.username
        info.value.source = 'runtime'
      }
      return d
    } catch (e) {
      error.value = e?.message || '保存失败'
      throw e
    } finally {
      saving.value = false
    }
  }

  return { loading, saving, error, info, load, save }
}

export default useCredentials
