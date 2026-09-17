/*
 * NetMirror 二次开发 · 前端定制层
 * 目录: ui/src/custom_components/
 *
 * 约定（对应《NetMirror 二次开发协作与架构规范》）：
 *   规则 2  所有新增文件只放本目录，不散落到原项目的 components/ stores/ composables/
 *   规则 3  万不得已要改原文件时，改动处必须成对出现 CUSTOM START / CUSTOM END 标记
 *   规则 4  可调参数集中在 ui.config.js，不hardcode 在原文件里
 *   规则 5  样式走 CSS 覆盖（theme.css），不改 tailwind.config.js 等基础配置
 *   规则 6  每周同步上游：git fetch upstream → git merge，冲突只会出现在带标记的位置
 *
 * 上游仓库（只读，不同步推送）：
 *   git remote add upstream https://github.com/X-Zero-L/als.git
 */

## 文件清单

| 文件 | 作用 | 被哪些原文件引用 |
| --- | --- | --- |
| `ui.config.js` | 定制参数集中配置（超时/重试/动画节奏/视觉主题） | useNodeSession.js, App.vue |
| `theme.css` | 视觉覆盖层：`.lg-card` / `.bg-grid` / `.lg-rise` / `shadow-*` | main.js（一行 import） |
| `SectionTitle.vue` | 区块标题组件（渐变竖条 + 15px 半粗标题） | App.vue |
| `LanguageSelectorCustom.vue` | 语言下拉修复版（Teleport + fixed 定位） | components/LanguageSelector.vue（转发壳） |
| `useNodeSession.js` | 节点 SSE 会话状态机（connecting / ready / error） | stores/nodes.js |
| `ToolGrid.vue` | 工具按钮网格（图标 + hover 抬升） | components/Utilities.vue |
| `toolIcons.js` | 工具图标路径映射 | components/Utilities.vue |

## 已改动的上游原文件（均带 CUSTOM 标记）

1. `src/main.js` — 多一行 `import '@/custom_components/theme.css'`
2. `src/App.vue` — 引用 SectionTitle、info chips、连接状态提示、foot er 避让
3. `src/stores/nodes.js` — 引入 useNodeSession 状态机、effectiveConfig 兜底
4. `src/composables/useNodeTool.js` — 透出 sessionStatus / sessionError / effectiveConfig
5. `src/components/LanguageSelector.vue` — 改为转发壳，实现迁到本目录
6. `src/components/Utilities.vue` — 图标由 toolIcons.js 注入、按钮网格换成 ToolGrid
7. `src/components/Utilities/NodeList.vue` — 按钮/状态条视觉
8. `src/components/Speedtest.vue` — 分段控件 + 连接中提示
9. `src/components/Speedtest/Librespeed.vue` — 结果卡视觉 + 坐标轴格式
10. `src/components/TrafficDisplay.vue` — 接口卡视觉
11. `src/components/Loading.vue` — 连接卡视觉

`tailwind.config.js` **未修改**（规则 5），`shadow-card/soft/lift/glow` 改为在 theme.css 里以纯 CSS 定义。

## 冲突演练

```
git fetch upstream
git merge upstream/main
```
预期冲突点只会落在上面 11 个文件的 CUSTOM 标记区间内；`custom_components/` 是纯新增目录，不会与上游冲突。
