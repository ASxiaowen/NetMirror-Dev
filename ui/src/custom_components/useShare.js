/**
 * NetMirror 二次开发 · 临时链接
 * 目录: ui/src/custom_components/useShare.js
 *
 * 两端职责：
 *   · 管理侧（已登录）：创建 / 列表 / 吊销；
 *   · 访客侧（受限模式）：把作用域变成「锁定单节点 + 过滤工具」的运行环境。
 */

import { computed } from 'vue'
import { request } from './apiClient'
import { authState, isShareMode } from './authState'

/**
 * 可授权工具目录。id 必须与后端 share.KnownTools
 * （backend/custom_modules/share/store.go）以及上游 Utilities.vue 的 id 保持一致。
 */
export const TOOL_CATALOG = [
  { id: 'ping', label: 'Ping', desc: 'IPv4 连通性测试' },
  { id: 'ping6', label: 'Ping IPv6', desc: 'IPv6 连通性测试' },
  { id: 'mtr', label: 'MTR', desc: '网络路径分析' },
  { id: 'mtr6', label: 'MTR IPv6', desc: 'IPv6 路径分析' },
  { id: 'traceroute', label: 'Traceroute', desc: '路由路径发现' },
  { id: 'traceroute6', label: 'Traceroute IPv6', desc: 'IPv6 路由发现' },
  { id: 'iperf3', label: 'IPerf3', desc: '带宽测量' },
  { id: 'speedtest-net', label: 'Speedtest.net', desc: '官方 Speedtest CLI' },
  { id: 'shell', label: 'Shell', desc: '交互式命令行' },
  { id: 'speedtest', label: 'LibreSpeed 测速', desc: '上下行速率测试' },
  { id: 'traffic', label: '带宽流量', desc: '网卡流量曲线' }
]

/** 有效期预设（秒） */
export const TTL_PRESETS = [
  { label: '1 小时', value: 3600 },
  { label: '6 小时', value: 6 * 3600 },
  { label: '1 天', value: 24 * 3600 },
  { label: '3 天', value: 3 * 24 * 3600 },
  { label: '7 天', value: 7 * 24 * 3600 },
  { label: '30 天', value: 30 * 24 * 3600 }
]

/** 默认勾选：只给网络诊断类，不给 Shell / 带宽流量这类敏感能力 */
export const DEFAULT_TOOLS = ['ping', 'ping6', 'mtr', 'mtr6', 'traceroute', 'traceroute6', 'speedtest']

export const isKnownToolId = (id) => TOOL_CATALOG.some((t) => t.id === id)

/** 标签查询，给受限模式的提示条用 */
export const toolLabel = (id) => TOOL_CATALOG.find((t) => t.id === id)?.label || id

/**
 * 当前令牌是否允许使用某个工具。
 * 登录用户一律允许；临时链接按白名单（空白名单视为不允许，避免签发疏漏放大权限）。
 */
export function shareToolAllowed(toolId) {
  if (!isShareMode()) return true
  const tools = authState.scope?.tools || []
  return tools.includes('*') || tools.includes(toolId)
}

/**
 * 是否至少有一个工具被授权。
 * 用于隐藏「整块都是空的」卡片，而不是留一个只有标题的空壳。
 */
export function hasAnyAllowedTool() {
  if (!isShareMode()) return true
  return TOOL_CATALOG.some((t) => shareToolAllowed(t.id))
}

/** 从作用域里取出被锁定的节点（临时链接只允许访问这一个节点） */
export function lockedNodeFromScope() {
  const s = authState.scope
  if (!s || !s.nodeUrl) return null
  return {
    id: s.nodeId || '',
    name: s.nodeId || s.note || 'Shared node',
    location: '',
    url: String(s.nodeUrl).replace(/\/+$/, '')
  }
}

/**
 * 当前应当承载 API 请求的节点。
 *
 * 非受限模式返回 null（= 用「当前页面所在源」的相对路径，保持原行为）；
 * 受限模式返回绑定的那个节点，让同源相对请求（./session 等）也一起走到绑定节点上，
 * 这样临时链接的全部流量都只落在一个节点，后端按 Host 做的归属校验才能一致通过。
 */
export function restrictedNode() {
  return isShareMode() ? lockedNodeFromScope() : null
}

/** 剩余秒数（响应式，随 scope.exp 计算） */export const shareLeftSeconds = computed(() => {
  const exp = authState.scope?.exp || 0
  if (!exp) return 0
  return Math.max(0, exp - Math.floor(Date.now() / 1000))
})

/** 把秒数格式化成「1 天 3 小时」这类文案 */
export function formatDuration(secs) {
  const s = Math.max(0, Math.floor(secs))
  if (s >= 86400) return `${Math.floor(s / 86400)} 天 ${Math.floor((s % 86400) / 3600)} 小时`
  if (s >= 3600) return `${Math.floor(s / 3600)} 小时 ${Math.floor((s % 3600) / 60)} 分`
  if (s >= 60) return `${Math.floor(s / 60)} 分 ${s % 60} 秒`
  return `${s} 秒`
}

export function useShare() {
  /** 换取作用域（访客侧，无需登录） */
  const resolveLink = async (token) => {
    const d = await request(`/custom/link/${encodeURIComponent(token)}`, { timeout: 15000 })
    if (!d?.valid || !d?.scope) throw new Error(d?.error || '临时链接无效')
    return d.scope
  }

  /** 创建临时链接（需登录） */
  const createShare = async (payload) => {
    const d = await request('/custom/share', {
      method: 'POST',
      data: payload,
      timeout: 20000
    })
    if (!d?.success) throw new Error(d?.error || '生成失败')
    return d
  }

  /** 列出全部临时链接（需登录） */
  const listShares = async () => {
    const d = await request('/custom/share', { timeout: 20000 })
    return d?.shares || []
  }

  /** 吊销（需登录） */
  const revokeShare = async (id) => {
    const d = await request(`/custom/share/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      timeout: 20000
    })
    if (!d?.success) throw new Error(d?.error || '吊销失败')
    return true
  }

  return { resolveLink, createShare, listShares, revokeShare }
}

export default useShare
