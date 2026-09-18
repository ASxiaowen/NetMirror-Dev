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

/**
 * 从作用域里取出被授权的节点列表。
 *
 * 兼容两种载荷：
 *   - 新（多节点）：scope.nodes = [{id,name,url,location}, ...]
 *   - 旧（单节点）：scope.nodeUrl / nodeId / nodeName
 * 合成同一个数组返回，调用方不需要区分。
 */
export function lockedNodesFromScope() {
  const s = authState.scope
  if (!s) return []

  let raw = []
  if (Array.isArray(s.nodes) && s.nodes.length) {
    raw = s.nodes
  } else if (s.nodeUrl) {
    raw = [{ id: s.nodeId, name: s.nodeName, url: s.nodeUrl }]
  }

  return raw
    .map((n) => ({
      id: n.id || '',
      name: n.name || n.id || 'Shared node',
      location: n.location || '',
      url: String(n.url || '').replace(/\/+$/, '')
    }))
    .filter((n) => !!n.url)
}

/**
 * 兼容入口：只需要「一个目标」的调用点取列表首项。
 * 多节点下它不再代表「唯一被授权的节点」，别用它做权限判断。
 */
export function lockedNodeFromScope() {
  return lockedNodesFromScope()[0] || null
}

/** 受限模式下被授权的节点列表（非受限模式为空数组） */
export function restrictedNodes() {
  return isShareMode() ? lockedNodesFromScope() : []
}

/**
 * 当前应当承载 API 请求的节点。
 *
 * 非受限模式返回 null（= 用「当前页面所在源」的相对路径，保持原行为）；
 * 受限模式返回首个被授权节点，兜住那些没显式传节点的调用点。
 * 多节点场景下，调用方应当显式传自己想访问的那个节点。
 */
export function restrictedNode() {
  return isShareMode() ? lockedNodeFromScope() : null
}

/**
 * 某个节点是否在当前临时链接的授权范围内。
 *
 * 仅用于前端「不给入口」。真正的判定在后端守卫（按请求 Host 比对授权节点），
 * 所以这里被绕过也只是多显示一个切换项，数据仍拿不到。
 */
export function shareNodeAllowed(node) {
  if (!isShareMode()) return true
  const list = lockedNodesFromScope()
  if (!list.length) return true
  const url = String(node?.url || '').replace(/\/+$/, '')
  if (!url) return false
  return list.some((n) => n.url === url)
}

/** 剩余秒数（响应式，随 scope.exp 计算） */
export const shareLeftSeconds = computed(() => {
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
  /**
   * 查询链接信息（访客侧，无需登录、无需密码）。
   * 用于在输入密码前展示「这条链接给了我什么」，让对方确认链接是不是给自己的。
   */
  const linkInfo = async (id) => {
    const d = await request('/custom/sharelink/info', {
      params: { id },
      timeout: 15000
    })
    if (!d?.valid || !d?.info) throw new Error(d?.error || '该临时链接不可用')
    return d.info
  }

  /**
   * 用「链接标识 + 临时密码」兑换受限作用域的令牌（访客侧，无需登录）。
   * @returns {Promise<{token:string, scope:object, expiresAt:number}>}
   */
  const redeemLink = async (id, password) => {
    const d = await request('/custom/sharelink/redeem', {
      method: 'POST',
      data: { id, password },
      timeout: 15000
    })
    if (!d?.success || !d?.token) throw new Error(d?.error || '兑换失败')
    return { token: d.token, scope: d.scope || {}, expiresAt: d.expiresAt || 0 }
  }

  /** 创建临时链接（需登录）。响应里带一次性展示的临时密码。 */
  const createShare = async (payload) => {
    const d = await request('/custom/share', {
      method: 'POST',
      data: payload,
      timeout: 20000
    })
    if (!d?.success) throw new Error(d?.error || '生成失败')
    return d
  }

  /** 列出全部临时链接（需登录）。列表不含密码，只有可再次复制的链接。 */
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

  return { linkInfo, redeemLink, createShare, listShares, revokeShare }
}

export default useShare
