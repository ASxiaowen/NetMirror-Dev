# backend/custom_modules —— NetMirror 二次开发后端专属目录

依据《NetMirror 二次开发协作与架构规范》第 2 条，所有二次开发新增的后端逻辑、
API 接口与模型都集中放在本目录，**禁止散落在上游目录中**。

## 约定

1. **一个功能一个子包**，例如 `cors/`、`xxx/`，包内自带注释说明「为什么需要它」。
2. **上游文件只允许加一行导入/注册**，并用统一标记包裹：

   ```go
   // === CUSTOM START: [功能名称] - By [开发者姓名] ===
   // 理由: ...
   // === CUSTOM END: [功能名称] ===
   ```

3. **不修改上游的默认配置文件**（`.env.example` 等）。私有参数走环境变量或独立覆盖文件。
4. 每个子包在注释里写清**「上游若自行修复，本模块可整体删除」**的条件，便于未来同步。

## 现有模块

| 子包 | 作用 | 注册位置 | 上游修复后可否移除 |
|---|---|---|---|
| `cors` | 补齐 CORS 预检白名单中的 `Content-Encoding`，修复跨域部署下 librespeed 上行恒为 `0.00` | `backend/als/route.go` `SetupHttpRoute()` 首行 | 可以（条件：上游白名单已含 `Content-Encoding`） |

## 构建提示

`backend/embed/ui/` 是前端产物，被 `.gitignore` 忽略，构建前需由 `ui/dist` 生成：

```bash
cd ui && npm run build
rm -rf ../backend/embed/ui && cp -r dist ../backend/embed/ui
cd ../backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o netmirror .
```
