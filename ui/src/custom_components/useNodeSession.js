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
// === CUSTOM START: 会话 SSE 走统一 API 层 - By ASxiaowen ===
// 理由: EventSource 不能自定义请求头，令牌必须由 apiClient 以 ?token= 形式附加，
//       否则在启用登录门/临时链接后，节点会话一律 401。
import { createEventSource, verifyIdentity } from './apiClient'
// === CUSTOM END: 会话 SSE 走统一 API 层 ===

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
      // === CUSTOM START: 会话 SSE 走统一 API 层 - By ASxiaowen ===
      // 理由: 见文件顶部；apiClient 会按当前身份附加令牌查询参数。
      const eventSource = createEventSource('/session', { node })
      // === CUSTOM END: 会话 SSE 走统一 API 层 ===
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

        // 重试前先确认身份还在不在。
        // SSE 的 onerror 拿不到状态码，若这里不查一次，被吊销 / 过期的令牌会让
        // 页面永远停在「正在连接节点」上反复重试，用户看不到任何可行动的提示。
        // 身份已失效时直接跳出重试，由外壳（RootShell）接管并给出对应引导。
        const id = await verifyIdentity({ notify: true })
        if (!id.valid) break

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
