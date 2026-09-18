// Package cors 是 NetMirror 二次开发的 CORS 扩展模块。
//
// 背景（为什么需要这个模块）
//
//	上游 backend/als/route.go 的全局 CORS 中间件在预检响应中返回的白名单是
//	"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization,
//	 accept, origin, Cache-Control, X-Requested-With, session, X-Api-Key"，
//	缺少 Content-Encoding。
//
//	而前端上行测速时，ui/public/speedtest_worker.js:521 会对 POST 显式设置
//	`Content-Encoding: identity`。该请求头不属于 CORS 安全列表请求头，必然
//	触发 OPTIONS 预检；预检被拒后浏览器根本不会发出 POST。
//
//	上行速度由 totLoaded 推算，而 totLoaded 只在 xhr.upload.onprogress 中累加
//	（speedtest_worker.js:491-503）。预检失败 → 无 progress 事件 → totLoaded 恒为 0
//	→ ulStatus 恒为 "0.00"；又因为同文件 xhr_ignoreErrors=1 会静默重启流而不置
//	failed（speedtest_worker.js:57/511-516），界面上显示的是 "0.00" 而不是 "Fail"。
//
//	该问题只在「面板与节点不同源」时暴露 —— 远程节点必然跨域；同源部署不产生
//	CORS，因此不会出现。下行是简单 GET，不触发预检，所以下行始终正常。
//
// 做法（最小化侵入）
//
//	不修改上游 CORS 中间件本体，而是在其之前注册本中间件提前接管 OPTIONS：
//	预检请求在 gin 中会落到 NoRoute 分支，全局中间件按注册顺序执行，因此本模块
//	只要先于上游注册，就能在它 Set/Abort 之前完成处理，不会被它的白名单覆盖。
//	非 OPTIONS 请求直接 c.Next() 放行，行为与上游完全一致，零行为差异。
//
// 上游同步提示
//
//	若上游后续自行补全了该白名单，本模块可整体删除，并移除
//	backend/als/route.go 中那一行注册语句即可，无需其它改动。
package cors

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AllowHeaders 是放行给浏览器的请求头白名单。
//
// 注意: 必须与上游 backend/als/route.go 中 "Access-Control-Allow-Headers" 的值
// 保持一致，仅额外追加 Content-Encoding。上游若新增请求头，请同步到这里。
const AllowHeaders = "Content-Type, Content-Length, Accept-Encoding, Content-Encoding, " +
	"X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, " +
	"session, X-Api-Key"

// AllowedMethods 与上游保持一致。
const AllowedMethods = "POST, OPTIONS, GET, PUT, DELETE"

// PreflightHandler 接管所有 OPTIONS 预检请求。
//
// 必须在 gin 引擎上注册于上游 CORS 中间件之前才能生效。
func PreflightHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 非预检请求完全交给上游处理
		if c.Request.Method != http.MethodOptions {
			c.Next()
			return
		}

		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Set("Access-Control-Allow-Headers", AllowHeaders)
		h.Set("Access-Control-Allow-Methods", AllowedMethods)

		c.AbortWithStatus(http.StatusNoContent)
	}
}
