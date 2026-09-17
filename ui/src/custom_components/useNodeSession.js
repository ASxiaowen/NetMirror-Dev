/**
 * NetMirror 二次开发 · 节点 SSE 会话状态机
 * 目录: ui/src/custom_components/useNodeSession.js
 *
 * 背景：上游 stores/nodes.js 的 selectNode() 在 SSE 建连失败 / 超时时会把
 *       selectedNode 置空，页面所有区块依赖 selectedNode.config 渲染 → 整页空白；
 *       而且建连期间没有任何状态反馈，用户只看到“白屏”。
 *
 * 这里把「连接超时 + 自动重试 + 竞态令牌 + 状态机 + 上一次配置兜底」整体搬到本文件，
 * stores/nodes.js 只负责调用与落地结果，把对原文件的侵入压到最小。
 *
 * 状态: idle → connecting → ready | error
 */
import { ref } from 'vue'
import { uiConfig } from './ui.config.js'

export function useNodeSession() {
  /** idle | connecting | ready | error */
  const sessionStatus = ref('idle')
  /** 失败原因（用于页面提示） */
  const sessionError = ref('')
  /** 上一次成功拿到的配置，切换节点期间用作兜底，避免整块内容消失 */
  const lastConfig = ref(null)

  /**
   * 竞态令牌：快速连点切换节点时，旧请求晚到不能覆盖新请求的结果
   */
  let selectSeq = 0
  const nextToken = () => ++selectSeq
  const isStale = (token) => token !== selectSeq

  /**
   * 单次建连（带超时）
   * @param {{name:string, url:string}} node
   * @returns {Promise<{sessionId:string, source:EventSource, config:object}>}
   */
  const connectOnce = (node) => {
    return new Promise((resolve, reject) => {
      const eventSource = new EventSource(`${node.url}/session`)
      let sessionId = null
      let nodeConfig = null
      let settled = false

      const done = (fn, value) => {
        if (settled) return
        settled = true
        clearTimeout(timeout)
        fn(value)
      }

      const timeout = setTimeout(() => {
        done(() => {
          eventSource.close()
          reject(new Error('连接节点超时'))
        })
      }, uiConfig.session.connectTimeout)

      eventSource.addEventListener('SessionId', (e) => {
        sessionId = e.data
      })

      eventSource.addEventListener('Config', (e) => {
        nodeConfig = JSON.parse(e.data)
        // 只有当我们有了 sessionId 和 config 才算完成
        if (sessionId && nodeConfig) {
          done(() =>
            resolve({
              sessionId: sessionId,
              source: eventSource,
              config: nodeConfig
            })
          )
        }
      })

      eventSource.onerror = () => {
        done(() => {
          eventSource.close()
          reject(new Error('无法连接到节点 ' + node.name))
        })
      }
    })
  }

  /**
   * 建连，失败按 uiConfig.session.maxRetry 自动重试
   */
  const establishNodeSession = async (node) => {
    if (!node) return null
    let attempt = 0
    let lastError = null
    while (attempt <= uiConfig.session.maxRetry) {
      try {
        return await connectOnce(node)
      } catch (err) {
        lastError = err
        attempt++
        if (attempt > uiConfig.session.maxRetry) break
        await new Promise((r) => setTimeout(r, uiConfig.session.retryDelay))
      }
    }
    throw lastError
  }

  return {
    sessionStatus,
    sessionError,
    lastConfig,
    nextToken,
    isStale,
    establishNodeSession
  }
}

export default useNodeSession
