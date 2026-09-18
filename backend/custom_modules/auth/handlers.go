package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// NewJti 生成一个令牌唯一 id（带前缀便于区分类型）
func NewJti(prefix string) string {
	return prefix + RandHex(8)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterRoutes 挂载登录相关路由。
// 注意：这些路由本身永远放行（靠 Guard 里的 isAlwaysOpen），否则登录页无法调用。
func RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/auth/config", handlePublicConfig)
	g.POST("/auth/login", handleLogin)
	g.GET("/auth/verify", handleVerify)
	g.POST("/auth/logout", handleLogout)
}

// handlePublicConfig 给登录页用的公开信息：是否启用、令牌有效期。
// 只暴露非敏感项，账号、密码与密钥永不出现在响应里。
func handlePublicConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"enabled":       Cfg.Enabled,
		"tokenTtlHours": Cfg.TokenTTLHours,
	})
}

func handleLogin(c *gin.Context) {
	if !Cfg.Enabled {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"enabled": false,
			"message": "登录门未启用",
		})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "BAD_REQUEST",
			"error":   "请求格式错误",
		})
		return
	}

	user := strings.TrimSpace(req.Username)

	// 账号与密码都用定长比较，避免时序侧信道；两个比较都执行完再判断，
	// 不因账号错就短路返回，否则响应耗时会泄漏「账号是否存在」。
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(Cfg.Username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(Cfg.Password)) == 1
	if !userOK || !passOK {
		// 刻意不区分「账号不存在」与「密码错误」—— 避免被用来枚举账号
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"code":    CodeAuthRequired,
			"error":   "账号或密码不正确",
		})
		return
	}

	claims := &Claims{
		Kind: KindUser,
		Sub:  Cfg.Username,
		Jti:  NewJti("u_"),
	}
	token, err := Sign(claims)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    "SIGN_FAILED",
			"error":   "签发令牌失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"token":     token,
		"kind":      KindUser,
		"username":  Cfg.Username,
		"expiresAt": claims.Exp,
		"clientIP":  c.ClientIP(),
	})
}

// tokenSummary 把载荷转成给前端看的安全摘要（不含任何密钥）
func tokenSummary(cl *Claims) gin.H {
	h := gin.H{
		"kind": cl.Kind,
		"exp":  cl.Exp,
		"note": cl.Note,
		"sub":  cl.Sub,
	}
	if cl.Kind == KindShare {
		h["nodeId"] = cl.NodeID
		h["nodeUrl"] = cl.NodeURL
		// nodeName 与 redeem 的 scope 保持一致：刷新页面时外壳走的是 verify 这条路径，
		// 少了它受限模式的节点名会退化成 id。
		h["nodeName"] = cl.NodeID
		h["tools"] = cl.Tools
		h["jti"] = cl.Jti
		if cl.Exp > 0 {
			if left := cl.Exp - time.Now().Unix(); left > 0 {
				h["leftSecs"] = left
			} else {
				h["leftSecs"] = 0
			}
		}
	} else {
		h["username"] = cl.Sub
	}
	return h
}

func handleVerify(c *gin.Context) {
	if !Cfg.Enabled {
		// 未启用时一律视为已登录，前端直接进主应用
		c.JSON(http.StatusOK, gin.H{"success": true, "valid": true, "enabled": false, "kind": KindUser})
		return
	}

	token := ExtractToken(c.GetHeader("Authorization"), c.GetHeader("X-Auth-Token"), c.Query("token"))
	cl, err := Verify(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"valid":   false,
			"error":   err.Error(),
		})
		return
	}

	// 临时链接令牌还要问一次吊销名单。
	//
	// 为什么非得在这里查：/custom/auth/verify 属于「永远放行」路径，Guard 不会
	// 为它做作用域校验。若这里只看签名与 exp，被吊销的链接在令牌自然到期前
	// 仍会被判为有效 —— 前端刷新页面时就会直接进入受限模式，随后每个真实请求
	// 才陆续 403，表现为「页面进来了但一直在连接节点」。
	//
	// 返回 200 + valid:false（而不是 401）：本接口的语义是「这个令牌还能用吗」，
	// 用正常报文作答更方便前端区分「令牌格式坏」与「被吊销」。
	if cl.Kind == KindShare && RevocationChecker != nil {
		if revoked, known := RevocationChecker(cl.Jti); known && revoked {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"valid":   false,
				"enabled": true,
				"code":    CodeForbidden,
				"error":   "该临时链接已被吊销或已过期",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"valid":   true,
		"enabled": true,
		"scope":   tokenSummary(cl),
	})
}

// handleLogout 令牌是无状态的（HMAC 签名），服务端无可撤销状态，
// 登出由前端丢弃本地令牌完成；这里保持接口存在以便未来切换成可吊销模式。
func handleLogout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}
