package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

// randHex 生成 n 字节随机数的十六进制串，用于令牌 id
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(b)
}

// NewJti 生成一个令牌唯一 id（带前缀便于区分类型）
func NewJti(prefix string) string {
	return prefix + randHex(8)
}

type loginRequest struct {
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
// 只暴露非敏感项，口令与密钥永不出现在响应里。
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

	// 定长比较，避免时序侧信道泄漏口令长度与内容
	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(Cfg.Password)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"code":    CodeAuthRequired,
			"error":   "口令不正确",
		})
		return
	}

	claims := &Claims{
		Kind: KindUser,
		Sub:  "panel",
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
	}
	if cl.Kind == KindShare {
		h["nodeId"] = cl.NodeID
		h["nodeUrl"] = cl.NodeURL
		h["tools"] = cl.Tools
		h["jti"] = cl.Jti
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
