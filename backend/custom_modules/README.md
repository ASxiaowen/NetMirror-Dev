# backend/custom_modules —— NetMirror 二次开发后端专属目录

依据《NetMirror 二次开发协作与架构规范》第 2 条，所有二次开发新增的后端逻辑、
API 接口与模型都集中放在本目录，**禁止散落在上游目录中**。

## 约定

1. **一个功能一个子包**，例如 `cors/`、`auth/`，包内自带注释说明「为什么需要它」。
2. **上游文件只允许加一行导入/注册**，并用统一标记包裹：

   ```go
   // === CUSTOM START: [功能名称] - By [开发者姓名] ===
   // 理由: ...
   // === CUSTOM END: [功能名称] ===
   ```

3. **不修改上游的默认配置文件**（`.env.example` 等）。私有参数走环境变量或独立覆盖文件。
4. 每个子包在注释里写清**「上游若自行修复，本模块可整体删除」**的条件，便于未来同步。
5. **统一入口**：上游只认识 `register.go` 里的 `Register(e *gin.Engine)` 一个函数，
   新增子模块时改本目录内部即可，不必再动上游文件。

## 现有模块

| 子包 | 作用 | 上游修复后可否移除 |
|---|---|---|
| `cors` | 补齐 CORS 预检白名单中的 `Content-Encoding`，修复跨域部署下 librespeed 上行恒为 `0.00`；并为提前中止的鉴权响应补上 CORS 头 | 部分可（条件：上游白名单已含 `Content-Encoding` 且鉴权在中止时也写 CORS 头） |
| `auth` | 整站访问口令登录：HMAC-SHA256 无状态令牌、按路径前缀的鉴权守卫、登录/校验/登出接口 | 不可（属自研功能，上游无对应能力） |
| `share` | 临时测试链接：签发/列表/吊销/解析，绑定单节点 + 工具白名单 + 有效期，可随时吊销 | 不可（属自研功能，上游无对应能力） |

## 注册顺序（关键，改动前务必先读）

`register.go` 里的注册顺序是**有语义的**，不能随手调整：

1. `cors.EnsureHeaders()` —— 最先。提前写好 CORS 响应头，这样后面任何中间件
   返回 401/403 时响应仍带 CORS 头。否则跨域部署下浏览器会把「未登录」显示成
   CORS 错误，前端无法区分 `AUTH_REQUIRED` 与网络故障。
2. `cors.PreflightHandler()` —— 也要先于上游 CORS 中间件。上游对 OPTIONS 是
   `Set(...)` 后立刻 `AbortWithStatus(204)`，排在它后面的中间件对预检不会执行。
3. `auth.Guard()` —— 鉴权守卫，在各业务路由之前。
4. 业务路由：`/custom/auth/*`、`/custom/share/*`、`/t/:token`。

## 环境变量

| 变量 | 必填 | 说明 |
|---|---|---|
| `PANEL_PASSWORD` | 否 | 访问口令。**不设置则登录门整体不启用**（保持上游开放行为），启动时打警告。 |
| `AUTH_SECRET` | 否 | 令牌签名密钥。未设置时进程启动随机生成 —— 多进程部署（如 panel + agent 各跑一份）必须显式设置同一个值，否则两边令牌互不认。 |
| `AUTH_TOKEN_TTL_HOURS` | 否 | 登录态有效期，默认 `168`（7 天）。 |

配置走**环境变量**而非配置文件，符合规范第 4 条。推荐用 systemd drop-in
（`/etc/systemd/system/<unit>.service.d/*.conf`）注入，不落仓库、不动原单元文件。

## 构建提示

`backend/embed/ui/` 是前端产物，被 `.gitignore` 忽略，构建前需由 `ui/dist` 生成：

```bash
cd ui && npm run build
rm -rf ../backend/embed/ui && cp -r dist ../backend/embed/ui
cd ../backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o netmirror .
```

前端对应的自定义目录是 `ui/src/custom_components/`，两个目录的职能一一对应。
