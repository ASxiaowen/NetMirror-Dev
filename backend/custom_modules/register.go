// Package custom 是 NetMirror 二次开发后端模块的**唯一装配入口**。
//
// 规范第 2 条的「入口隔离」在这里落地：backend/als/route.go 只允许出现
// 一行 custom.Register(e)，所有子模块（cors / auth / share）的挂载、
// 中间件顺序、路由注册都在本文件内完成。原文件因此几乎不会被上游更新波及。
package custom

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/X-Zero-L/als/config"
	"github.com/X-Zero-L/als/custom_modules/auth"
	"github.com/X-Zero-L/als/custom_modules/cors"
	"github.com/X-Zero-L/als/custom_modules/share"
	iEmbed "github.com/X-Zero-L/als/embed"
	"github.com/gin-gonic/gin"
)

// Register 装配全部二次开发模块。
//
// 必须在 gin.Engine 上最先调用：gin 的全局中间件按注册顺序执行，
// 而 cors.PreflightHandler 需要抢在上游 CORS 中间件之前接管 OPTIONS。
func Register(e *gin.Engine) {
	// 1) CORS：先确保所有响应都带上 CORS 头（含后续被守卫中止的 401/403），
	//    再接管 OPTIONS 预检。两步都必须最先执行，见 custom_modules/cors 的说明。
	e.Use(cors.EnsureHeaders())
	e.Use(cors.PreflightHandler())

	// 2) 装配临时链接模块：把吊销判定注入 auth 守卫
	share.Init()

	// 3) 鉴权守卫：按路径保护上游的 /session、/method/* 等核心接口
	e.Use(auth.Guard())

	// 4) 自定义接口
	g := e.Group("/custom")
	auth.RegisterRoutes(g)  // /custom/auth/{config,login,verify,logout}
	share.RegisterRoutes(g) // /custom/share[...]（需登录）、/custom/sharelink/*（放行）

	// 5) 临时链接落地页：/t/<id> 直接返回前端入口页。
	//    上游只给 "/" 注册了 index.html，而临时链接是 /t/xxx 这样的路径，
	//    不在前端路由内（本项目是单页无 router），必须由后端把入口页吐出来。
	e.GET("/t/:token", serveIndexHTML)
}

// serveIndexHTML 从内嵌前端产物里取出 index.html 返回。
//
// 与上游 handleIndexHTML 的区别：上游那个是包内私有函数（package als），
// 我们的模块无法调用；这里按同样的思路重新实现一遍，读取的仍是同一份
// 内嵌资源（iEmbed.UIStaticFiles），不复制任何文件。
//
// 注意：这里**刻意不校验链接 id**，任何 /t/<任意串> 都返回入口页。
// 理由：返回的只是一份与 "/" 完全相同的 SPA 外壳，不含任何业务数据，
// 真正的校验发生在 API 层（/custom/sharelink/info 与 redeem）。
// 若在这里对无效 id 直接返回 401，浏览器会渲染一段裸 JSON，
// 而保留 200 能让前端把「链接已过期 / 已被吊销」渲染成一张正常的中文提示卡片。
func serveIndexHTML(c *gin.Context) {
	sub, err := fs.Sub(iEmbed.UIStaticFiles, "ui")
	if err != nil {
		c.String(http.StatusNotFound, "Not found")
		return
	}
	raw, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	html := string(raw)
	// 与上游保持一致：用 LOCATION 覆盖默认标题，让分享出去的页面也有站点标识
	if config.Config.Location != "" {
		html = strings.Replace(html, "<title>Looking glass server</title>",
			"<title>"+config.Config.Location+"</title>", 1)
	}

	// 关键：前端产物用的是相对路径（vite base: "./"，如 ./js/index-xxx.js）。
	// 页面在 /t/<token> 下打开时，相对路径会解析成 /t/js/... 而 404。
	// 注入 <base href="/"> 把基准路径钉回根目录，这样 /t/<token> 与 / 完全等价，
	// 既保留了好看的分享链接，又不需要在 nginx 上做任何额外的 rewrite。
	if !strings.Contains(html, "<base ") {
		html = strings.Replace(html, "<head>", "<head>\n    <base href=\"/\">", 1)
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	// 入口页不缓存，避免换令牌后仍拿到旧页
	c.Header("Cache-Control", "no-store")
	c.String(http.StatusOK, html)
}
