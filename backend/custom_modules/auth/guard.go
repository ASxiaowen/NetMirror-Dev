package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 前端据此区分错误类型并给出不同引导（规范里的「错误与状态统一」）
const (
	// CodeAuthRequired 未登录 / 未携带令牌
	CodeAuthRequired = "AUTH_REQUIRED"
	// CodeTokenExpired 令牌过期，需重新登录或重新获取链接
	CodeTokenExpired = "TOKEN_EXPIRED"
	// CodeTokenInvalid 令牌非法
	CodeTokenInvalid = "TOKEN_INVALID"
	// CodeForbidden 已登录但权限不足（临时链接越权）
	CodeForbidden = "FORBIDDEN"
)

// ctxClaims gin.Context 中存放令牌载荷的键
const ctxClaims = "customAuthClaims"

// RevocationChecker 由上层业务模块注入，用于判断某个令牌是否已被吊销。
//
// 之所以用注入而不是直接依赖具体模块：auth 是通用鉴权层，不应认识「临时链接」
// 这类业务语义；同时可避免 auth → share 的反向 import 造成循环依赖。
//
// 返回 (revoked, known)：
//   - known=false 表示本机没有该令牌的记录（例如请求打到的是只持有签名密钥、
//     不持有签发方记录表的远端节点），此时交由令牌自身的 exp 兜底；
//   - known=true 且 revoked=true 才拒绝。
var RevocationChecker func(jti string) (revoked bool, known bool)

func abortAuth(c *gin.Context, status int, code, msg string) {
	c.AbortWithStatusJSON(status, gin.H{
		"success": false,
		"code":    code,
		"error":   msg,
	})
}

// ClaimsFrom 取出中间件写入的令牌载荷
func ClaimsFrom(c *gin.Context) (*Claims, bool) {
	v, ok := c.Get(ctxClaims)
	if !ok {
		return nil, false
	}
	cl, ok := v.(*Claims)
	return cl, ok
}

// Guard 按路径前缀保护上游的核心接口。
//
// 为什么不逐个路由挂中间件：上游的 /session、/method/:x 是在 SetupHttpRoute 里注册的，
// 我们的模块无法给它们单独挂中间件；而全局中间件按路径判断等价、且能让 route.go
// 保持「只有一行注册」，符合规范第 1、2 条。
func Guard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !Cfg.Enabled {
			c.Next()
			return
		}
		path := c.Request.URL.Path

		// 1) 永远放行：登录接口、临时链接校验、分享落地页、静态资源
		if isAlwaysOpen(path) {
			c.Next()
			return
		}

		// 2) 仅登录用户：临时链接管理
		if isUserOnly(path) {
			claims, ok := authorize(c)
			if !ok {
				return
			}
			if claims.Kind != KindUser {
				abortAuth(c, http.StatusForbidden, CodeForbidden, "该功能需要登录后使用")
				return
			}
			c.Set(ctxClaims, claims)
			c.Next()
			return
		}

		// 3) 受保护的核心接口
		if !isProtected(path) {
			c.Next()
			return
		}
		claims, ok := authorize(c)
		if !ok {
			return
		}

		// 4) 临时链接：再叠加作用域校验（吊销 + 节点归属 + 工具白名单）
		if claims.Kind == KindShare {
			if RevocationChecker != nil {
				if revoked, known := RevocationChecker(claims.Jti); known && revoked {
					abortAuth(c, http.StatusForbidden, CodeForbidden, "该临时链接已被吊销或已过期")
					return
				}
			}

			tool, ok := ToolOfPath(path)
			if !ok {
				// 认不出来的接口一律拒绝，避免白名单漏配变成全权限
				abortAuth(c, http.StatusForbidden, CodeForbidden, "该临时链接无权访问此接口")
				return
			}

			// 裸 /session 不属于任何工具（ToolOfPath 返回空串）：它只返回该节点自身的
			// 公开元信息（配置/功能开关/内存占用），不含任何可执行能力，前端启动与
			// 节点建连都必须经过它。若在此处也套用工具白名单，页面会连不上节点、
			// 整页停在“正在连接”，实际权限收口交给下面有工具标识的接口。
			if tool == "" {
				c.Set(ctxClaims, claims)
				c.Next()
				return
			}

			if !claims.HostAllowed(c.Request.Host) {
				abortAuth(c, http.StatusForbidden, CodeForbidden, "该临时链接仅限绑定节点使用")
				return
			}
			if !claims.AllowTool(tool) {
				abortAuth(c, http.StatusForbidden, CodeForbidden, "该临时链接未授权「"+tool+"」功能")
				return
			}
		}

		c.Set(ctxClaims, claims)
		c.Next()
	}
}

// authorize 从请求中取令牌并校验，失败时已写好响应
func authorize(c *gin.Context) (*Claims, bool) {
	token := ExtractToken(
		c.GetHeader("Authorization"),
		c.GetHeader("X-Auth-Token"),
		c.Query("token"),
	)
	claims, err := Verify(token)
	if err == nil {
		return claims, true
	}
	switch {
	case errors.Is(err, ErrTokenExpired):
		abortAuth(c, http.StatusUnauthorized, CodeTokenExpired, err.Error())
	case errors.Is(err, ErrTokenMissing):
		abortAuth(c, http.StatusUnauthorized, CodeAuthRequired, err.Error())
	default:
		abortAuth(c, http.StatusUnauthorized, CodeTokenInvalid, err.Error())
	}
	return nil, false
}

func isAlwaysOpen(path string) bool {
	return strings.HasPrefix(path, "/custom/auth/") ||
		strings.HasPrefix(path, "/custom/link/") ||
		strings.HasPrefix(path, "/t/")
}

func isUserOnly(path string) bool {
	return strings.HasPrefix(path, "/custom/share")
}

func isProtected(path string) bool {
	if path == "/session" || strings.HasPrefix(path, "/session/") {
		return true
	}
	if path == "/method" || strings.HasPrefix(path, "/method/") {
		return true
	}
	if Cfg.ProtectNodes && (path == "/nodes" || strings.HasPrefix(path, "/nodes/")) {
		return true
	}
	return false
}

// methodToTool 把上游的 /method/<x> 段映射成前端同名的工具 id。
// 前端 id 见 ui/src/components/Utilities.vue 与 ui/src/custom_components/toolIcons.js。
var methodToTool = map[string]string{
	"ping":              "ping",
	"ping6":             "ping6",
	"mtr":               "mtr",
	"mtr6":              "mtr6",
	"traceroute":        "traceroute",
	"traceroute6":       "traceroute6",
	"iperf3":            "iperf3",
	"speedtest_dot_net": "speedtest-net",
	"cache":             "traffic",
}

// ToolOfPath 判断一个受保护路径对应哪个工具，第二个返回值为 false 表示无法识别（应拒绝）。
//
// 覆盖三类：
//   - /method/<tool>              普通工具
//   - /session/<id>/speedtest/... LibreSpeed 测速（含 download/upload/file）
//   - /session/<id>/shell         WebShell
//
// 裸的 /session（建会话/SSE 事件流）不属于任何工具，返回 ("", true)：
// 临时链接必须能建会话，具体能做什么由后续的工具接口把关。
func ToolOfPath(path string) (string, bool) {
	if strings.HasPrefix(path, "/method/") {
		seg := strings.SplitN(strings.TrimPrefix(path, "/method/"), "/", 2)[0]
		if tool, ok := methodToTool[seg]; ok {
			return tool, true
		}
		return seg, false
	}
	if path == "/session" {
		return "", true
	}
	if strings.HasPrefix(path, "/session/") {
		rest := strings.TrimPrefix(path, "/session/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) < 2 {
			return "", true // /session/<id>
		}
		sub := strings.SplitN(parts[1], "/", 2)[0]
		switch sub {
		case "speedtest":
			return "speedtest", true
		case "shell":
			return "shell", true
		default:
			return sub, false
		}
	}
	if path == "/nodes" || strings.HasPrefix(path, "/nodes/") {
		return "", true
	}
	return "", false
}
