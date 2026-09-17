/**
 * NetMirror 二次开发 · 定制参数集中配置
 * 目录: ui/src/custom_components/ui.config.js
 *
 * 规则 4（配置隔离）：所有可调参数写在这里，原文件只引用不硬编码。
 * 改这里不会影响上游合并，因为上游文件里没有这些常量。
 */

export const uiConfig = {
  /** 节点 SSE 会话 */
  session: {
    /** 单次建连超时(ms) */
    connectTimeout: 15000,
    /** 首次失败后的重试等待(ms) */
    retryDelay: 800,
    /** 最多重试次数（0 = 不重试） */
    maxRetry: 1
  },

  /** 首屏区块入场动画节奏(s)，按节点/工具/测速/流量顺序 */
  animation: {
    sectionDelays: [0.02, 0.08, 0.14, 0.2]
  },

  /** 视觉主题（theme.css 里的同名值以本文件为唯一来源说明） */
  theme: {
    shadow: {
      card: '0 1px 2px 0 rgba(16,24,40,.04), 0 1px 3px 0 rgba(16,24,40,.05)',
      soft: '0 8px 24px -10px rgba(15,23,42,.18), 0 2px 6px -2px rgba(15,23,42,.06)',
      lift: '0 18px 40px -18px rgba(15,23,42,.28), 0 2px 8px -4px rgba(15,23,42,.08)',
      glow: '0 0 0 1px rgba(14,165,233,.18), 0 8px 24px -10px rgba(14,165,233,.45)'
    }
  }
}

export default uiConfig
