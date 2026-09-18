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
| `authState.js` | 令牌与临时链接作用域的唯一真相源。登录令牌存 localStorage；临时链接令牌按链接 id 存 **sessionStorage**（关掉标签页即消失，且多条链接互不覆盖） | apiClient.js, Librespeed.vue, useShare.js, RootShell.vue |
| `apiClient.js` | **统一 API 层**：axios 实例 + EventSource 工厂 + 身份探活（`verifyIdentity`）+ 错误分类（`ApiError` / `ErrCode`） | stores/app.js, stores/nodes.js, useNodeSession.js, useAuth.js, useShare.js |
| `useAuth.js` | 登录态编排：探测登录门、账号+密码登录/登出、校验已有令牌 | RootShell.vue, LoginView.vue |
| `useShare.js` | 临时链接编排：管理侧（创建/列表/吊销）与访客侧（查信息/兑换）；`restrictedNode()`、`shareToolAllowed()` | RootShell.vue, SharePasswordView.vue, ShareAdminDialog.vue, App.vue, Utilities.vue, Speedtest.vue, stores/nodes.js, stores/app.js |
| `LoginView.vue` | 登录页（账号 + 密码，沿用 `.lg-card` 视觉，深浅主题跟随） | RootShell.vue |
| `SharePasswordView.vue` | **临时密码页**：先展示「这条链接给了什么」（节点/备注/可用功能/剩余有效），再要求输密码 | RootShell.vue |
| `ShareBanner.vue` | 受限模式顶部提示条（含剩余有效期倒计时，归零时上报过期） | RootShell.vue |
| `SharePanel.vue` | **可嵌入的临时链接面板**（生成表单 + 已生成列表）。弹窗与管理页两处共用同一实现，避免逻辑漂移 | ShareAdminDialog.vue, AdminExtras.vue |
| `ShareAdminDialog.vue` | 临时链接管理**弹窗壳**（遮罩 + 关闭 + `<SharePanel>`），主面板右下角入口用 | RootShell.vue |
| `AdminExtras.vue` | **管理页扩展**：①登录凭据（改账号/密码）②临时链接（内嵌 SharePanel）。挂在 Admin.vue 的已认证区域 | components/Admin.vue |
| `useCredentials.js` | 凭据编排：读取当前账号/来源标记、保存新账号密码（带当前密码确认） | AdminExtras.vue |
| `RootShell.vue` | **多态应用外壳**：loading / 登录页 / 临时密码页 / 失效卡片 / 原 `App.vue`（含受限模式） | main.js（挂载它而非 App.vue） |

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
| 13 | `src/components/Admin.vue` | 已认证区域插入一行 `<AdminExtras />` + 一行 import（**本轮唯一上游改动，+12 行 / 0 删除**） |

`tailwind.config.js` **未修改**（规则 5），`shadow-card/soft/lift/glow` 改为在 theme.css 里以纯 CSS 定义。

## 受限模式（临时链接）的作用范围

访客打开 `/t/<id>` 时先看到**临时密码页**（展示这条链接绑了哪台机器、能做哪些事、
还剩多久），输入对方单独发来的密码后 `RootShell` 进入受限模式，由四条独立机制共同收敛权限：

1. **区块级**（`App.vue`）—— 未授权的区块整块不渲染，避免留下空卡片。
2. **工具级**（`Utilities.vue` / `Speedtest.vue`）—— 未授权的工具不进网格。
3. **入口级**（`theme.css` 的 `.nm-restricted-mode`）—— 用 CSS 覆盖隐藏管理悬浮按钮，
   不改上游组件结构（规则 5）。
4. **节点级**（`useShare.restrictedNode()` + `stores/nodes.js`）—— 节点列表被替换为
   唯一的绑定节点，SSE 与工具请求全部只指向它，访客无法切换到别的节点。

> 前端只是「不给入口」，**真正的权限判定在后端** `custom_modules/auth` 的守卫里
> （吊销名单 + 节点归属 + 工具白名单）。前端过滤被绕过也不影响安全。

## 两条身份链路（互不干扰）

```
自己人   账号 + 密码        → user  令牌（localStorage，整站可用）
访客     链接 + 临时密码     → share 令牌（sessionStorage，按链接 id 分键，仅限绑定节点与授权工具）
```

打开分享链接时**临时链接优先于既有登录态** —— 否则访客会以上一个管理员的身份看到整站，
权限模型就失效了。

## 管理页扩展（AdminExtras）

Node Management 管理页（`components/Admin.vue`）里插了两块控制，**上游文件只多一行
`<AdminExtras />` 与一行 import**：

| 区块 | 内容 | 后端 |
| --- | --- | --- |
| ① 登录凭据 | 改面板账号 / 密码，改完**立即生效**、无需重启；密码默认打码，可点「显示」查看 | `GET/POST /custom/auth/credentials`（**仅登录用户**，share 令牌一律 403） |
| ② 临时测试链接 | 直接内嵌 `SharePanel`，与主面板右下角弹窗**共用同一份实现** | 同 `/custom/share` |

设计要点：

- **一份实现、两处入口**：`SharePanel.vue` 是可嵌入体，`ShareAdminDialog.vue`（主面板弹窗）
  与 `AdminExtras.vue`（管理页内嵌）都只是外壳。避免两处各写一遍后逐渐漂移。
- **保持上游的双重认证**：管理页本身仍要 Admin API Key，登录门是它之外的第一道；
  两者互不影响，也不互相替代。
- **改密码需要当前密码确认**：避免拿到一个已登录的浏览器就能直接改掉凭据。
- 凭据来源会在界面上标注（环境变量 / 文件），并给出「删掉该文件即恢复初始值」的提示。

> 测试侧提醒：`SharePanel` 的**页签与提交按钮文本完全相同**（都是「生成链接与密码」）。
> 用「按文本找第一个 button」的方式点提交会命中页签，表现是「点了没反应、生成结果不出现」。
> 提交按钮的稳定特征是 class 里的 `bg-gradient-to-b`（页签是 `flex-1 rounded-md`）。
> 这个坑在 CDP 自动化里踩过两次。

## 两个容易踩的时序点

1. **`/custom/auth/verify` 属于「永远放行」路径**，守卫不会为它做作用域校验。
   因此被吊销的临时链接必须在 verify 内部再查一次吊销名单，否则令牌在自然到期前
   始终「有效」，前端刷新页面会直接进受限模式，表现为一直连不上节点。
2. **SSE 的 `onerror` 拿不到 HTTP 状态码**，无法区分「节点离线」与「令牌已失效」。
   `useNodeSession` 在建连失败后先调一次 `verifyIdentity()`；身份已失效就跳出重试，
   由外壳给出「链接已被吊销 / 已过期」的明确提示，而不是无限重试。

## 冲突演练

```
git fetch upstream
git merge upstream/main
```

预期冲突点只会落在上表 12 个文件的 CUSTOM 标记区间内；`custom_components/` 是纯新增目录，不会与上游冲突。
