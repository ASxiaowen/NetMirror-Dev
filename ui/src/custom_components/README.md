/*
 * NetMirror 二次开发 · 前端定制层
 * 目录: ui/src/custom_components/
 *
 * 约定（对应《NetMirror 二次开发协作与架构规范》）：
 *   规则 1  最小化侵入：原文件只把 fetch / new EventSource / axios 换成这里的调用，
 *           业务逻辑（重试、状态机、UI）仍留在原文件
 *   规则 2  所有新增文件只放本目录，不散落到原项目的 components/ stores/ composables/
 *   规则 3  万不得已要改原文件时，改动处必须成对出现 CUSTOM START / CUSTOM END 标记
 *   规则 4  可调参数集中在 ui.config.js，不从原文件里 hardcode
 *   规则 5  样式走 CSS 覆盖（theme.css），不改 tailwind.config.js 等基础配置
 *   规则 6  每周同步上游：git fetch upstream → git merge，冲突只会出现在带标记的位置
 *
 * 上游仓库（只读，不同步推送）：
 *   git remote add upstream https://github.com/X-Zero-L/als.git
 */

## 文件清单

### 一、认证与访问控制（登录门 / 临时链接）

| 文件 | 作用 | 被哪些原文件引用 |
| --- | --- | --- |
| `authState.js` | 令牌与临时链接作用域的唯一真相源（localStorage 持久化、`authHeaders()`、`appendToken()`） | apiClient.js, Librespeed.vue, useShare.js |
| `apiClient.js` | **统一 API 层**：axios 实例 + EventSource 工厂 + 错误分类（`ApiError` / `ErrCode`） | stores/app.js, stores/nodes.js, useNodeSession.js |
| `useAuth.js` | 登录态编排：探测后端是否启用登录门、登录/登出、口令错误提示 | RootShell.vue, LoginView.vue |
| `useShare.js` | 临时链接编排：创建/列表/吊销/解析、`restrictedNode()`、`shareToolAllowed()` | RootShell.vue, ShareAdminDialog.vue, App.vue, Utilities.vue, Speedtest.vue, stores/nodes.js, stores/app.js |
| `LoginView.vue` | 登录页（沿用 `.lg-card` 视觉，深浅主题跟随） | RootShell.vue |
| `ShareBanner.vue` | 受限模式顶部提示条（含剩余有效期倒计时） | RootShell.vue |
| `ShareAdminDialog.vue` | 临时链接管理弹窗（选节点、勾工具、设有效期、复制、吊销） | RootShell.vue |
| `RootShell.vue` | **三态应用外壳**：登录页 / 临时链接受限模式 / 原 `App.vue` | main.js（挂载它而非 App.vue） |

### 二、既有定制（第一轮重构）

| 文件 | 作用 | 被哪些原文件引用 |
| --- | --- | --- |
| `ui.config.js` | 定制参数集中配置（超时/重试/动画节奏/视觉主题） | useNodeSession.js, App.vue |
| `theme.css` | 视觉覆盖层：`.lg-card` / `.bg-grid` / `.lg-rise` / `shadow-*` / `.nm-restricted-mode` | main.js（一行 import） |
| `SectionTitle.vue` | 区块标题组件（渐变竖条 + 15px 半粗标题） | App.vue |
| `LanguageSelectorCustom.vue` | 语言下拉修复版（Teleport + fixed 定位） | components/LanguageSelector.vue（转发壳） |
| `useNodeSession.js` | 节点 SSE 会话状态机（connecting / ready / error） | stores/nodes.js |
| `ToolGrid.vue` | 工具按钮网格（图标 + hover 抬升） | components/Utilities.vue |
| `toolIcons.js` | 工具图标路径映射 | components/Utilities.vue |

## 已改动的上游原文件（均带 CUSTOM 标记）

| # | 文件 | 改动性质 |
| --- | --- | --- |
| 1 | `src/main.js` | import theme.css；挂载 `RootShell` 而非 `App.vue` |
| 2 | `src/App.vue` | 引用 SectionTitle、info chips、连接状态提示、footer 避让；三个区块加 `v-if` 白名单 |
| 3 | `src/stores/app.js` | 会话 SSE 与工具请求改走 apiClient |
| 4 | `src/stores/nodes.js` | 引入 useNodeSession 状态机、`effectiveConfig` 兜底；`/nodes`、延迟探测、节点请求改走 apiClient |
| 5 | `src/composables/useNodeTool.js` | 透出 sessionStatus / sessionError / effectiveConfig |
| 6 | `src/components/LanguageSelector.vue` | 改为转发壳，实现迁到本目录 |
| 7 | `src/components/Utilities.vue` | 图标由 toolIcons.js 注入、按钮网格换成 ToolGrid；工具按临时链接白名单过滤 |
| 8 | `src/components/Utilities/NodeList.vue` | 按钮/状态条视觉 |
| 9 | `src/components/Speedtest.vue` | 分段控件 + 连接中提示；测速形式按临时链接白名单过滤 |
| 10 | `src/components/Speedtest/Librespeed.vue` | 结果卡视觉 + 坐标轴格式；测速 URL 附加令牌查询参数 |
| 11 | `src/components/TrafficDisplay.vue` | 接口卡视觉 |
| 12 | `src/components/Loading.vue` | 连接卡视觉 |

`tailwind.config.js` **未修改**（规则 5），`shadow-card/soft/lift/glow` 改为在 theme.css 里以纯 CSS 定义。

## 受限模式（临时链接）的作用范围

访客打开 `/t/<token>` 时，`RootShell` 进入受限模式，由三条独立机制共同收敛权限：

1. **区块级**（`App.vue`）—— 未授权的区块整块不渲染，避免留下空卡片。
2. **工具级**（`Utilities.vue` / `Speedtest.vue`）—— 未授权的工具不进网格。
3. **入口级**（`theme.css` 的 `.nm-restricted-mode`）—— 用 CSS 覆盖隐藏管理悬浮按钮，
   不改上游组件结构（规则 5）。
4. **节点级**（`useShare.restrictedNode()` + `stores/nodes.js`）—— 节点列表被替换为
   唯一的绑定节点，SSE 与工具请求全部只指向它，访客无法切换到别的节点。

> 前端只是「不给入口」，**真正的权限判定在后端** `custom_modules/auth` 的守卫里
> （节点归属 + 工具白名单）。前端过滤被绕过也不影响安全。

## 冲突演练

```
git fetch upstream
git merge upstream/main
```

预期冲突点只会落在上表 12 个文件的 CUSTOM 标记区间内；`custom_components/` 是纯新增目录，不会与上游冲突。
