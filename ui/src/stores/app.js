import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import { formatBytes } from '@/helper/unit'
// === CUSTOM START: 统一 API 层 - By ASxiaowen ===
// 理由: 会话 SSE 与工具请求原先直接 new EventSource / axios.create，散落在三个文件里。
//       收敛到 custom_components/apiClient.js：统一注入认证令牌、统一错误分类；
//       临时链接受限模式下还会把请求指向被绑定的节点。业务逻辑（重连、状态）不变。
import { createEventSource, request } from '@/custom_components/apiClient'
import { restrictedNode } from '@/custom_components/useShare'
// === CUSTOM END: 统一 API 层 ===

export const useAppStore = defineStore('app', () => {
  const source = ref()
  const sessionId = ref()
  const connecting = ref(true)
  const config = ref()
  const drawerWidth = ref()
  const memoryUsage = ref()
  
  // Toast management
  const toasts = ref([])
  let toastIdCounter = 0
  
  // Theme and language settings with persistence
  const theme = ref(localStorage.getItem('theme') || 'light')
  const language = ref(localStorage.getItem('language') || 'en-US')
  
  // Watch theme changes and persist
  const setTheme = (newTheme) => {
    theme.value = newTheme
    localStorage.setItem('theme', newTheme)
    document.documentElement.classList.toggle('dark', newTheme === 'dark')
  }
  
  // Watch language changes and persist
  const setLanguage = (newLang) => {
    language.value = newLang
    localStorage.setItem('language', newLang)
  }
  
  // Initialize theme on load
  if (theme.value === 'dark') {
    document.documentElement.classList.add('dark')
  }
  
  let timer = ''

  const handleResize = () => {
    let width = window.innerWidth
    if (width > 800) {
      drawerWidth.value = 800
    } else {
      drawerWidth.value = width
    }
  }
  window.addEventListener('resize', handleResize)
  handleResize()

  let initializePromise = null

  const reconnectEventSource = () => {
    clearTimeout(timer)
    setTimeout(() => {
      // Reset promise to allow re-initialization on reconnect
      initializePromise = null
      initialize()
    }, 1000)
  }

  const setupEventSource = () => {
    return new Promise((resolve, reject) => {
      connecting.value = true
      // === CUSTOM START: 会话 SSE 走统一 API 层 - By ASxiaowen ===
      // 理由: EventSource 无法设置请求头，令牌只能由 apiClient 以 ?token= 附加；
      //       受限模式下指向被绑定节点，使临时链接的流量全部落在同一个节点上。
      const eventSource = createEventSource('/session', { node: restrictedNode() })
      // === CUSTOM END: 会话 SSE 走统一 API 层 ===

      eventSource.addEventListener('SessionId', (e) => {
        sessionId.value = e.data
        console.log('session', e.data)
        resolve()
      })

      eventSource.addEventListener('Config', (e) => {
        config.value = JSON.parse(e.data)
        connecting.value = false
      })

      eventSource.addEventListener('MemoryUsage', (e) => {
        memoryUsage.value = formatBytes(e.data)
      })

      eventSource.onerror = function (e) {
        eventSource.close()
        connecting.value = true
        console.log('SSE disconnected')
        reconnectEventSource()
        reject(new Error('SSE connection failed'))
      }
      source.value = eventSource
    })
  }

  const initialize = () => {
    if (!initializePromise) {
      initializePromise = setupEventSource()
    }
    return initializePromise
  }

  const requestMethod = (method, data = {}, signal = null) => {
    // === CUSTOM START: 工具请求走统一 API 层 - By ASxiaowen ===
    // 理由: 传输层换成 custom_components/apiClient（自动携带令牌、统一超时与错误分类），
    //       原有的 resolve/reject 语义与 400 文案提示保持不变。
    return request('/method/' + method, {
      params: data,
      session: sessionId.value,
      timeout: 1000 * 120, // 请求超时时间
      signal: signal || undefined,
      node: restrictedNode()
    })
      .then((payload) => {
        if (payload && payload.success) return payload
        return Promise.reject(payload)
      })
      .catch((error) => {
        if (error && error.canceled) return Promise.reject(error)

        // Handle 400 Bad Request errors
        if (error && error.status === 400) {
          showToast('Bad Request, please check your input', 'error')
        }

        console.error(error)
        return Promise.reject(error)
      })
    // === CUSTOM END: 工具请求走统一 API 层 ===
  }

  // Toast methods
  const showToast = (message, type = 'info', duration = 5000) => {
    const id = ++toastIdCounter
    const toast = {
      id,
      message,
      type,
      timestamp: Date.now()
    }
    
    toasts.value.push(toast)
    
    // Auto remove after duration
    if (duration > 0) {
      setTimeout(() => {
        removeToast(id)
      }, duration)
    }
    
    return id
  }

  const removeToast = (id) => {
    const index = toasts.value.findIndex(toast => toast.id === id)
    if (index > -1) {
      toasts.value.splice(index, 1)
    }
  }

  const clearToasts = () => {
    toasts.value = []
  }

  return {
    //vars
    source,
    sessionId,
    connecting,
    config,
    drawerWidth,
    memoryUsage,
    theme,
    language,
    toasts,

    //methods
    initialize,
    requestMethod,
    setTheme,
    setLanguage,
    showToast,
    removeToast,
    clearToasts
  }
})
