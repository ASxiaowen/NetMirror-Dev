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
| `auth` | 账号 + 密码登录：HMAC-SHA256 无状态令牌、按路径前缀的鉴权守卫、登录/校验/登出接口；并提供临时密码的派生与定长比较原语；账号密码支持**运行时修改**（`DATA_DIR/custom_auth.json`，改完立即生效、多进程经 mtime 惰性重载一致） | 不可（属自研功能，上游无对应能力） |
| `share` | 临时测试链接（**一链一密**）：签发/列表/吊销、链接信息查询、临时密码兑换，绑定单节点 + 工具白名单 + 有效期，可随时吊销；连续输错密码按记录锁定 | 不可（属自研功能，上游无对应能力） |

## 接口一览

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/custom/auth/config` | 公开 | 登录门是否启用、令牌有效期 |
| POST | `/custom/auth/login` | 公开 | 入参 `{username, password}`，出参 `token` |
| GET | `/custom/auth/verify` | 公开 | 校验令牌；临时链接令牌在此**再查一次吊销名单** |
| POST | `/custom/auth/logout` | 公开 | 无状态令牌，前端丢弃即可 |
| GET | `/custom/auth/credentials` | 需登录 | 读当前账号与密码、来源标记（`env` / `runtime`）、最小密码长度 |
| POST | `/custom/auth/credentials` | 需登录 | 改账号/密码，入参 `{currentPassword, username, password}`；**必须带当前密码确认**；改完立即生效，无需重启 |
| GET | `/custom/share` | 需登录 | 列出全部临时链接（不含密码） |
| POST | `/custom/share` | 需登录 | 生成链接 + 一次性临时密码 |
| DELETE | `/custom/share/:id` | 需登录 | 吊销 |
| GET | `/custom/sharelink/info?id=` | 公开 | 访客在输密码前查看「这条链接给了什么」 |
| POST | `/custom/sharelink/redeem` | 公开 | 入参 `{id, password}`，出参受限令牌 + 作用域 |
| GET | `/t/:id` | 公开 | 临时链接落地页（返回内嵌 SPA 外壳） |

> 路由命名上 `/share` 与 `/sharelink` 是**同级的不同静态段**，这是刻意的：
> gin 的路由树不允许同一层出现「静态段 vs 参数段」的兄弟冲突。
> 同理，`auth.isUserOnly` 必须写成 `path == "/custom/share" || HasPrefix("/custom/share/")`，
> 用裸 `HasPrefix("/custom/share")` 会把访客用的 `/custom/sharelink/*` 一起圈进来。
>
> **加路由前必看**：`isAlwaysOpen` **逐条精确列出**公开路径，而不是用
> `HasPrefix("/custom/auth/")`。原因是同一前缀下混着公开与需鉴权的接口 ——
> `/custom/auth/credentials`（改账号密码）必须登录，若用前缀匹配就会被一起放行，
> 等于把改密码接口做成公开接口。**只要新增与既有前缀同族的路径，就要回头审计这两个函数。**
> 本轮已实际踩到过一次（`/custom/sharelink/*` 被 `/custom/share` 吃掉）。

## 注册顺序（关键，改动前务必先读）

`register.go` 里的注册顺序是**有语义的**，不能随手调整：

1. `cors.EnsureHeaders()` —— 最先。提前写好 CORS 响应头，这样后面任何中间件
   返回 401/403 时响应仍带 CORS 头。否则跨域部署下浏览器会把「未登录」显示成
   CORS 错误，前端无法区分 `AUTH_REQUIRED` 与网络故障。
2. `cors.PreflightHandler()` —— 也要先于上游 CORS 中间件。上游对 OPTIONS 是
   `Set(...)` 后立刻 `AbortWithStatus(204)`，排在它后面的中间件对预检不会执行。
3. `auth.Guard()` —— 鉴权守卫，在各业务路由之前。
4. 业务路由：`/custom/auth/*`、`/custom/share/*`、`/custom/sharelink/*`、`/t/:id`。

## 环境变量

| 变量 | 必填 | 说明 |
|---|---|---|
| `PANEL_USER` | 否 | 登录账号，默认 `admin`。 |
| `PANEL_PASSWORD` | 否 | 登录密码。**不设置则登录门整体不启用**（保持上游开放行为），启动时打警告。 |
| `AUTH_SECRET` | 否 | 令牌签名密钥，同时用于临时密码派生。未设置时进程启动随机生成 —— 多进程部署（如 panel + agent 各跑一份）必须显式设置同一个值，否则两边令牌互不认。 |
| `AUTH_TOKEN_TTL_HOURS` | 否 | 登录态有效期，默认 `168`（7 天）。临时链接令牌不受它影响，取记录自身的剩余有效期。 |

配置走**环境变量**而非配置文件，符合规范第 4 条。推荐用 systemd drop-in
（`/etc/systemd/system/<unit>.service.d/*.conf`）+ `EnvironmentFile` 注入，
不落仓库、不动原单元文件。

**升级提示**：关闭登录门只需清掉 `PANEL_PASSWORD`；只要该变量还在，
`PANEL_USER` 缺失时账号为 `admin`，可能与既有部署的预期不符，升级时请显式设置。

### 运行时凭据（`DATA_DIR/custom_auth.json`）

环境变量是**部署期初值**（改它要登服务器 + 重启），管理页提供的是**运行期覆盖**：

| 状态 | 生效值 | 界面上的来源标记 |
|---|---|---|
| 文件不存在 | `PANEL_USER` / `PANEL_PASSWORD` | `env` |
| 文件存在 | 文件里的账号密码 | `runtime` |
| 删掉文件 | 立即回到环境变量值（等于一键恢复默认） | `env` |

- 文件以 **0600** 落盘，先写 `.tmp` 再 `rename`（同文件系统内原子，避免读到半截内容）。
- panel 与 agent 是**两个进程**：任一方改完，另一方靠 **mtime 惰性重载**立刻看到新值，
  否则会出现「面板改了密码、节点上还在用旧的」。登录不是热路径，这点 stat 开销可忽略。
- 改密码**不会**让已签发的登录令牌立即失效 —— 令牌是无状态的，签名密钥不变就仍有效。
  需要立刻踢下线时，重启服务或更换 `AUTH_SECRET`。
- 写入失败 / 文件损坏都**不阻塞服务**，继续用现有凭据，避免把管理员锁在门外。

## 构建提示

`backend/embed/ui/` 是前端产物，被 `.gitignore` 忽略，构建前需由 `ui/dist` 生成：

```bash
cd ui && npm run build
rm -rf ../backend/embed/ui && cp -r dist ../backend/embed/ui
cd ../backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o netmirror .
```

前端对应的自定义目录是 `ui/src/custom_components/`，两个目录的职能一一对应。
