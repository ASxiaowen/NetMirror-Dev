import { ref, computed, watch } from 'vue'
import { defineStore } from 'pinia'
import axios from 'axios'
import { useAppStore } from '@/stores/app'

// === CUSTOM START: 节点 Session 状态机 - By ASxiaowen ===
// 理由: 上游 selectNode() 建连失败会把 selectedNode 置空 → 整页空白，且过程无任何反馈。
//       状态机（超时/重试/竞态令牌/配置兜底）整体放在 custom_components/useNodeSession.js，
//       本文件只取状态与调用，避免在原文件里堆逻辑。
import { useNodeSession } from '@/custom_components/useNodeSession'
// === CUSTOM END: 节点 Session 状态机 ===

export const useNodesStore = defineStore('nodes', () => {
  // 节点相关状态
  const nodes = ref([])
  const selectedNode = ref(null)
  const currentNode = ref(null)
  const latencies = ref({})
  const loading = ref(false)
  const pingStates = ref({})

  // Session管理
  const selectedNodeSession = ref(null)
  const selectedNodeSource = ref(null)

  // === CUSTOM START: 节点 Session 状态机 - By ASxiaowen ===
  // 理由: sessionStatus / sessionError / lastConfig / 竞态令牌由 custom_components/useNodeSession.js 提供
  const {
    sessionStatus,
    sessionError,
    lastConfig,
    nextToken,
    isStale,
    establishNodeSession
  } = useNodeSession()
  // === CUSTOM END: 节点 Session 状态机 ===

  // 生成节点唯一键
  const getNodeKey = (node) => {
    if (!node) return ''
    return `${node.name}_${node.url.replace(/[^a-zA-Z0-9]/g, '_')}`
  }

  // 获取当前页面URL
  const getCurrentURL = () => {
    const protocol = window.location.protocol
    const host = window.location.host
    const port = window.location.port
    
    let baseURL = `${protocol}//${window.location.hostname}`
    
    if (port && ((protocol === 'http:' && port !== '80') || (protocol === 'https:' && port !== '443'))) {
      baseURL += `:${port}`
    }
    
    return baseURL
  }

  // 检查是否为当前节点
  const isCurrentNode = (node) => {
    if (!node) return false
    const currentURL = getCurrentURL()
    return node.url.replace(/\/$/, '') === currentURL.replace(/\/$/, '')
  }

  // 根据延迟获取状态
  const getStatusByLatency = (latency) => {
    if (latency < 200) return 'good'
    if (latency < 500) return 'medium'
    return 'high'
  }

  // 获取状态文本
  const getStatusText = (status) => {
    switch (status) {
      case 'good': return 'Excellent'
      case 'medium': return 'Good'
      case 'high': return 'Slow'
      case 'error': return 'Offline'
      default: return 'Unknown'
    }
  }

  // 清理选定节点的Session
  const cleanupNodeSession = () => {
    if (selectedNodeSource.value) {
      selectedNodeSource.value.close()
    }
    selectedNodeSession.value = null
    selectedNodeSource.value = null
  }

  // 获取节点列表
  const fetchNodes = async () => {
    loading.value = true
    try {
      const response = await fetch('/nodes')
      const data = await response.json()
      if (data.success) {
        nodes.value = data.nodes || []
        console.log('Fetched nodes:', nodes.value)
        
        // 设置当前节点
        const current = nodes.value.find(node => isCurrentNode(node))
        if (current) {
          currentNode.value = current
          if (!selectedNode.value) {
            await selectNode(current)
          }
        }
        
        // 立即测试延迟
        await testAllLatencies()
      }
    } catch (error) {
      console.error('Failed to fetch nodes:', error)
    } finally {
      loading.value = false
    }
  }

  // 测试单个节点延迟
  const testNodeLatency = async (node) => {
    if (!node) return
    
    try {
      const timestamp = Date.now()
      const targetUrl = isCurrentNode(node) ? '/nodes/latency' : `${node.url}/nodes/latency`
      
      const response = await fetch(`${targetUrl}?timestamp=${timestamp}`, {
        method: 'GET',
        mode: 'cors',
        cache: 'no-cache',
        signal: AbortSignal.timeout(5000)
      })
      
      if (response.ok) {
        const data = await response.json()
        const latency = Date.now() - timestamp
        console.log(`Latency response for ${node.name}:`, data)
        if (data.success) {
          const nodeKey = getNodeKey(node)
          latencies.value[nodeKey] = {
            latency: latency,
            status: getStatusByLatency(latency)
          }
          console.log(`Updated latencies for ${node.name} (${nodeKey}):`, latencies.value[nodeKey])
        } else {
          throw new Error('Server returned error')
        }
      } else {
        throw new Error('Server not responding properly')
      }
    } catch (error) {
      console.error('Failed to test latency for', node.name, error)
      const nodeKey = getNodeKey(node)
      latencies.value[nodeKey] = {
        latency: -1,
        status: 'error'
      }
    }
  }

  // 测试所有节点延迟
  const testAllLatencies = async () => {
    if (nodes.value.length === 0) return
    
    loading.value = true
    try {
      for (const node of nodes.value) {
        await testNodeLatency(node)
        await new Promise(resolve => setTimeout(resolve, 100))
      }
    } finally {
      loading.value = false
    }
  }

  // 单独ping节点
  const pingSingleNode = async (node) => {
    if (!node) return
    
    const nodeKey = getNodeKey(node)
    pingStates.value[nodeKey] = { isPinging: true }
    await testNodeLatency(node)
    pingStates.value[nodeKey] = { isPinging: false }
  }

  // === CUSTOM START: 选择节点时不清空页面 - By ASxiaowen ===
  // 理由: 上游在 catch 里把 selectedNode 置空，导致切换/重连失败时整页空白且无法恢复。
  //       这里改为：失败也保留 selectedNode，只把状态置为 error 并给出文案（页面顶部展示 + 重试）。
  const selectNode = async (node) => {
    if (!node || !nodes.value.includes(node)) return

    // 本次切换的令牌，避免快速连点时旧请求的结果覆盖新请求
    const token = nextToken()

    // 清理之前的Session
    cleanupNodeSession()

    // 设置新的选定节点（不清空 config：新配置到达前继续用旧配置渲染，避免页面整块空白）
    selectedNode.value = node
    sessionStatus.value = 'connecting'
    sessionError.value = ''

    try {
      const session = await establishNodeSession(node)

      // 已经切到别的节点了，丢弃这次的结果
      if (isStale(token)) {
        session.source.close()
        return
      }

      selectedNodeSession.value = session.sessionId
      selectedNodeSource.value = session.source

      // 将配置信息存储到节点对象中
      if (session.config) {
        selectedNode.value.config = session.config
        lastConfig.value = session.config
      }

      sessionStatus.value = 'ready'
    } catch (error) {
      if (isStale(token)) return
      // 失败时保留 selectedNode，页面不会变空白，只提示错误
      selectedNodeSession.value = null
      selectedNodeSource.value = null
      sessionStatus.value = 'error'
      sessionError.value = error?.message || '节点连接失败'
    }
  }
  // === CUSTOM END: 选择节点时不清空页面 ===

  // 为选定节点创建API请求
  const createNodeRequest = async (method, data = {}, signal = null) => {
    if (!selectedNode.value || !selectedNodeSession.value) {
      throw new Error('No node session available')
    }

    const targetNode = selectedNode.value
    const sessionId = selectedNodeSession.value

    // 所有节点都使用独立的session ID
    const baseURL = targetNode.url

    let axiosConfig = {
      timeout: 1000 * 120,
      headers: {
        'session': sessionId,
        'Content-Type': 'application/json'
      }
    }

    if (signal != null) {
      axiosConfig.signal = signal
    }

    const _axios = axios.create(axiosConfig)

    return new Promise((resolve, reject) => {
      _axios
        .get(`${baseURL}/method/${method}`, { params: data })
        .then((response) => {
          if (response.data && response.data.success) {
            resolve(response.data)
            return
          }
          reject(response)
        })
        .catch((error) => {
          console.error('Node request error:', error)
          reject(error)
        })
    })
  }

  // 获取选定节点的EventSource
  const getNodeEventSource = () => {
    if (!selectedNode.value || !selectedNodeSource.value) {
      throw new Error('No node session available')
    }
    return selectedNodeSource.value
  }

  // computed properties
  const availableNodes = computed(() => nodes.value)
  const hasSelectedNode = computed(() => selectedNode.value !== null)
  const selectedNodeName = computed(() => selectedNode.value?.name || '')
  const selectedNodeLocation = computed(() => selectedNode.value?.location || '')
  const hasNodeSession = computed(() => selectedNodeSession.value !== null)

  // === CUSTOM START: 生效配置兜底 - By ASxiaowen ===
  // 理由: 连接中/失败时 selectedNode.config 还没到或已失效，各区块靠 effectiveConfig
  //       回落到上一次成功的配置，避免整块消失。
  const effectiveConfig = computed(() => {
    if (selectedNode.value && selectedNode.value.config) return selectedNode.value.config
    return lastConfig.value
  })
  // === CUSTOM END: 生效配置兜底 ===

  return {
    // 状态
    nodes,
    selectedNode,
    currentNode,
    latencies,
    loading,
    pingStates,
    selectedNodeSession,
    // === CUSTOM START: 会话状态透出 - By ASxiaowen ===
    sessionStatus,
    sessionError,
    // === CUSTOM END: 会话状态透出 ===

    // computed
    availableNodes,
    hasSelectedNode,
    selectedNodeName,
    selectedNodeLocation,
    hasNodeSession,
    // === CUSTOM START: 生效配置透出 - By ASxiaowen ===
    effectiveConfig,
    // === CUSTOM END: 生效配置透出 ===

    // 方法
    getNodeKey,
    isCurrentNode,
    getStatusByLatency,
    getStatusText,
    fetchNodes,
    testNodeLatency,
    testAllLatencies,
    pingSingleNode,
    selectNode,
    createNodeRequest,
    getNodeEventSource,
    cleanupNodeSession
  }
})