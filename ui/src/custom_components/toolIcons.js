/**
 * NetMirror 二次开发 · 工具图标映射
 * 目录: ui/src/custom_components/toolIcons.js
 *
 * 作用：上游 components/Utilities.vue 的工具定义里没有 icon 字段，
 *      与其在 9 个工具对象里逐个插入 icon 行（9 处侵入），
 *      不如把图标集中在这里，由 Utilities.vue 用一个 CUSTOM 块统一注入。
 */
export const DEFAULT_TOOL_ICON = 'M13 10V3L4 14h7v7l9-11h-7z'

export const TOOL_ICONS = {
  ping: DEFAULT_TOOL_ICON,
  ping6: DEFAULT_TOOL_ICON,
  mtr: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
  mtr6: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
  traceroute: 'M13 7h8m0 0v8m0-8l-8 8-4-4-6 6',
  traceroute6: 'M13 7h8m0 0v8m0-8l-8 8-4-4-6 6',
  iperf3: 'M7 12l3-3 3 3 4-4M8 21l4-4 4 4M3 4h18M4 4h16v12a1 1 0 01-1 1H5a1 1 0 01-1-1V4z',
  'speedtest-net': DEFAULT_TOOL_ICON,
  shell: 'M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z'
}

export const toolIcon = (id) => TOOL_ICONS[id] || DEFAULT_TOOL_ICON

export default TOOL_ICONS
