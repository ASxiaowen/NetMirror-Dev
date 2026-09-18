package share

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/X-Zero-L/als/custom_modules/auth"
	"github.com/gin-gonic/gin"
)

const (
	minTTLSeconds = 60
	maxTTLSeconds = 30 * 24 * 3600 // 30 天
	defTTLSeconds = 24 * 3600      // 1 天
)

// RegisterRoutes 挂载临时链接路由（挂载点为 /custom）。
//
// 权限分工（由 auth.Guard 按路径判断）：
//   - /custom/share → 仅登录用户（增删查）
//   - /custom/link/:token → 永远放行，供链接持有者换取作用域
//
// 校验接口刻意不放在 /custom/share/resolve/:token —— 那会与 /custom/share/:id
// 在同一层形成「静态段 vs 参数段」的兄弟节点，gin 的路由树不允许这种冲突。
func RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/share", handleList)
	g.POST("/share", handleCreate)
	g.DELETE("/share/:id", handleRevoke)
	g.GET("/link/:token", handleResolve)
}

type createRequest struct {
	Note       string   `json:"note"`
	NodeID     string   `json:"nodeId"`
	NodeName   string   `json:"nodeName"`
	NodeURL    string   `json:"nodeUrl"`
	Tools      []string `json:"tools"`
	TTLSeconds int      `json:"ttlSeconds"`
}

// baseURL 依据请求推断对外可访问的地址，用于拼出可直接分享的链接
func baseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if p := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); p != "" {
		scheme = strings.Split(p, ",")[0]
	}
	return scheme + "://" + c.Request.Host
}

func handleCreate(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "BAD_REQUEST", "error": "请求格式错误"})
		return
	}

	// 节点：至少要有 url，工具与测速都靠它定位
	nodeURL := strings.TrimSpace(req.NodeURL)
	if nodeURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "BAD_REQUEST", "error": "请先选择要分享的节点"})
		return
	}
	nodeURL = strings.TrimRight(nodeURL, "/")

	// 工具白名单：逐个校验，未知 id 直接拒绝（避免签发端手误放大权限）
	tools := make([]string, 0, len(req.Tools))
	for _, t := range req.Tools {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !IsKnownTool(t) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"code":    "BAD_REQUEST",
				"error":   "未知的工具标识: " + t,
			})
			return
		}
		tools = append(tools, t)
	}
	if len(tools) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "BAD_REQUEST", "error": "请至少勾选一个允许使用的功能"})
		return
	}

	ttl := req.TTLSeconds
	if ttl <= 0 {
		ttl = defTTLSeconds
	}
	if ttl < minTTLSeconds {
		ttl = minTTLSeconds
	}
	if ttl > maxTTLSeconds {
		ttl = maxTTLSeconds
	}

	now := time.Now()
	jti := auth.NewJti("s_")

	claims := &auth.Claims{
		Kind:    auth.KindShare,
		Sub:     "share",
		Jti:     jti,
		NodeID:  strings.TrimSpace(req.NodeID),
		NodeURL: nodeURL,
		Tools:   tools,
		Note:    strings.TrimSpace(req.Note),
		Iat:     now.Unix(),
		Exp:     now.Add(time.Duration(ttl) * time.Second).Unix(),
	}

	token, err := auth.Sign(claims)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "SIGN_FAILED", "error": "签发失败: " + err.Error()})
		return
	}

	rec := &Record{
		ID:        auth.NewJti("rec_"),
		Jti:       jti,
		Note:      claims.Note,
		NodeID:    claims.NodeID,
		NodeURL:   nodeURL,
		NodeName:  strings.TrimSpace(req.NodeName),
		Tools:     tools,
		CreatedAt: now.Unix(),
		ExpiresAt: claims.Exp,
		CreatedBy: "panel",
		CreatedIP: c.ClientIP(),
	}
	if err := db.Add(rec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "STORE_FAILED", "error": "保存记录失败: " + err.Error()})
		return
	}

	path := "/t/" + token
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"record":  rec,
		"token":   token,
		"path":    path,
		"url":     baseURL(c) + path,
	})
}

func handleList(c *gin.Context) {
	list := db.List()
	// 补一个状态字段，前端不用自己算时间
	out := make([]gin.H, 0, len(list))
	for _, r := range list {
		out = append(out, gin.H{
			"id":        r.ID,
			"note":      r.Note,
			"nodeId":    r.NodeID,
			"nodeName":  r.NodeName,
			"nodeUrl":   r.NodeURL,
			"tools":     r.Tools,
			"createdAt": r.CreatedAt,
			"expiresAt": r.ExpiresAt,
			"createdIp": r.CreatedIP,
			"revoked":   r.Revoked,
			"useCount":  r.UseCount,
			"status":    r.Status(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "shares": out})
}

func handleRevoke(c *gin.Context) {
	id := c.Param("id")
	if !db.Revoke(id) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "NOT_FOUND", "error": "记录不存在或已被吊销"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleResolve 供链接持有者（未登录）换取作用域。
// 令牌本身已由签名 + exp 保证有效，这里额外查一次吊销状态与使用计数。
func handleResolve(c *gin.Context) {
	token := c.Param("token")
	claims, err := auth.Verify(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "valid": false, "error": err.Error()})
		return
	}
	if claims.Kind != auth.KindShare {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "valid": false, "error": "该令牌不是临时链接"})
		return
	}

	// 有记录才校验吊销；没有记录说明本机不是签发方，交给 exp 兜底
	if rec, ok := db.Lookup(claims.Jti); ok {
		if rec.Revoked {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "valid": false, "error": "该临时链接已被吊销"})
			return
		}
		if rec.Expired() {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "valid": false, "error": "该临时链接已过期"})
			return
		}
		db.BumpUse(claims.Jti)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"valid":   true,
		"scope": gin.H{
			"kind":     claims.Kind,
			"jti":      claims.Jti,
			"note":     claims.Note,
			"nodeId":   claims.NodeID,
			"nodeUrl":  claims.NodeURL,
			"tools":    claims.Tools,
			"exp":      claims.Exp,
			"leftSecs": maxInt64(0, claims.Exp-time.Now().Unix()),
		},
	})
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// ParseTTL 把 "1h" / "30m" / "7d" / "3600" 这类输入解析成秒；解析失败回退到默认值。
// 保留给未来的表单使用，当前前端直接传秒。
func ParseTTL(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return defTTLSeconds
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n <= 0 {
		return defTTLSeconds
	}
	switch unit {
	case 'm', 'M':
		return n * 60
	case 'h', 'H':
		return n * 3600
	case 'd', 'D':
		return n * 86400
	default:
		return defTTLSeconds
	}
}
