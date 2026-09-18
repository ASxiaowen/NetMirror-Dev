package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// 令牌类型
const (
	// KindUser 登录用户：拥有全部权限
	KindUser = "user"
	// KindShare 临时链接：权限由 scope 限定
	KindShare = "share"
)

var (
	// ErrTokenMissing 未提供令牌
	ErrTokenMissing = errors.New("未提供访问令牌")
	// ErrTokenInvalid 令牌签名不合法或格式错误
	ErrTokenInvalid = errors.New("访问令牌无效")
	// ErrTokenExpired 令牌已过期
	ErrTokenExpired = errors.New("访问令牌已过期")
)

// Claims 是令牌载荷。无状态设计：所有鉴权所需信息都在这里。
type Claims struct {
	Kind string `json:"kind"`          // user | share
	Sub  string `json:"sub,omitempty"` // 主体标识（用户名为固定值 panel，临时链接为签发人）
	Jti  string `json:"jti,omitempty"` // 令牌唯一 id，便于吊销与审计
	Iat  int64  `json:"iat"`
	Exp  int64  `json:"exp"`

	// 以下仅 kind=share 有值
	NodeID  string   `json:"nodeId,omitempty"`  // 绑定的节点 id
	NodeURL string   `json:"nodeUrl,omitempty"` // 绑定的节点地址，用于在各节点上判断请求归属
	Tools   []string `json:"tools,omitempty"`   // 允许使用的工具集合
	Note    string   `json:"note,omitempty"`    // 备注（发给谁）
}

// IsExpired 判断令牌是否过期
func (c *Claims) IsExpired() bool {
	return c.Exp > 0 && time.Now().Unix() > c.Exp
}

// AllowTool 判断该令牌是否允许使用某个工具。
// user 类型一律放行；share 类型需在 Tools 列表内（Tools 为空视为不放行，避免签发疏漏变成全权限）。
func (c *Claims) AllowTool(tool string) bool {
	if c.Kind == KindUser {
		return true
	}
	for _, t := range c.Tools {
		if t == "*" || t == tool {
			return true
		}
	}
	return false
}

// HostAllowed 判断请求是否发往该令牌绑定的节点。
// 用请求的 Host 与 scope.NodeURL 的 host:port 比对，因此任意节点只要共享同一
// AUTH_SECRET 就能独立完成校验，不需要中心化鉴权服务。
func (c *Claims) HostAllowed(reqHost string) bool {
	if c.Kind == KindUser {
		return true
	}
	if c.NodeURL == "" {
		return true // 未绑定节点，由工具白名单兜底
	}
	want := hostOf(c.NodeURL)
	if want == "" {
		return true
	}
	return strings.EqualFold(want, reqHost)
}

// hostOf 从 URL 里取出 host:port
func hostOf(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// Sign 签发令牌。secret 取 Cfg.Secret，调用方一般用 SignWithCfg。
func Sign(c *Claims) (string, error) {
	if len(Cfg.Secret) == 0 {
		return "", fmt.Errorf("未配置签名密钥，无法签发令牌")
	}
	if c.Iat == 0 {
		c.Iat = time.Now().Unix()
	}
	if c.Exp == 0 {
		c.Exp = time.Now().Add(time.Duration(Cfg.TokenTTLHours) * time.Hour).Unix()
	}

	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	p := b64(payload)

	mac := hmac.New(sha256.New, Cfg.Secret)
	mac.Write([]byte(p))
	return p + "." + b64(mac.Sum(nil)), nil
}

// Verify 校验令牌并返回载荷
func Verify(token string) (*Claims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrTokenMissing
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, ErrTokenInvalid
	}

	mac := hmac.New(sha256.New, Cfg.Secret)
	mac.Write([]byte(parts[0]))
	want := b64(mac.Sum(nil))
	// 定长比较，避免时序侧信道
	if subtle.ConstantTimeCompare([]byte(want), []byte(parts[1])) != 1 {
		return nil, ErrTokenInvalid
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrTokenInvalid
	}
	var c Claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, ErrTokenInvalid
	}
	if c.IsExpired() {
		return nil, ErrTokenExpired
	}
	if c.Kind == "" {
		return nil, ErrTokenInvalid
	}
	return &c, nil
}

// ExtractToken 从请求里取令牌。顺序: Authorization: Bearer → X-Auth-Token → ?token=
// 之所以支持查询参数，是因为浏览器的 EventSource 无法自定义请求头（SSE 会话、
// 工具事件流都靠它），这一点在跨域场景下是硬约束。
func ExtractToken(authorization, headerToken, queryToken string) string {
	if authorization != "" {
		v := strings.TrimSpace(authorization)
		if len(v) > 7 && strings.EqualFold(v[:7], "bearer ") {
			return strings.TrimSpace(v[7:])
		}
		return v
	}
	if headerToken != "" {
		return strings.TrimSpace(headerToken)
	}
	return strings.TrimSpace(queryToken)
}
