// Package auth 是 NetMirror 二次开发的登录与鉴权模块。
//
// 目录归属见 backend/custom_modules/README.md（规范第 2 条）。
//
// 设计要点
//
//   - **无状态令牌**：HMAC-SHA256 签名，形如 base64url(payload).base64url(sig)。
//     不在服务端存会话，进程重启不掉线，多机共享同一 AUTH_SECRET 即可互认。
//   - **账号密码来自环境变量**：PANEL_USER / PANEL_PASSWORD 由部署方注入，
//     不写进任何配置文件（规范第 4 条）。默认账号 admin。
//   - **降级为开放**：未配置 PANEL_PASSWORD 时整个登录门自动关闭并打警告，
//     这样任何已有部署升级后都不会被锁在门外。
//   - **跨主机可用**：令牌里带 scope.nodeUrl，任意节点只要用同一个 AUTH_SECRET，
//     就能独立校验并判断「这次请求是不是冲我来的」，无需中心化鉴权。
package auth

import (
	"crypto/sha256"
	"log"
	"os"
	"strconv"
	"strings"
)

// Config 登录与鉴权配置，全部来自环境变量。
type Config struct {
	// Enabled 是否启用登录门。由 PANEL_PASSWORD 是否存在决定，可被 AUTH_ENABLED 强制关闭。
	Enabled bool
	// Username 登录账号（环境变量 PANEL_USER），缺省 admin。
	Username string
	// Password 登录密码（环境变量 PANEL_PASSWORD）。
	Password string
	// Secret 令牌签名密钥。来自 AUTH_SECRET；缺省时由口令派生（可用但不推荐）。
	Secret []byte
	// TokenTTLHours 登录态有效期（小时），环境变量 AUTH_TOKEN_TTL_HOURS，默认 168（7 天）。
	TokenTTLHours int
	// ProtectNodes 是否连 /nodes 节点发现接口一起保护。
	// 默认 false —— 节点发现属于元数据，且多节点聚合时中心面板需要读取。
	ProtectNodes bool
}

// DefUsername 未配置 PANEL_USER 时使用的账号名。
const DefUsername = "admin"

// Cfg 进程级配置，包初始化时读取一次。
var Cfg = loadConfig()

func envStr(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func envBool(key string, def bool) bool {
	v := envStr(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt(key string, def int) int {
	v := envStr(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func loadConfig() *Config {
	pwd := envStr("PANEL_PASSWORD")
	user := envStr("PANEL_USER")
	if user == "" {
		user = DefUsername
	}

	// 有密码才谈得上启用；AUTH_ENABLED 只能把它关掉，不能凭空打开
	enabled := pwd != "" && envBool("AUTH_ENABLED", true)

	ttl := envInt("AUTH_TOKEN_TTL_HOURS", 168)

	c := &Config{
		Enabled:       enabled,
		Username:      user,
		Password:      pwd,
		TokenTTLHours: ttl,
		ProtectNodes:  envBool("AUTH_PROTECT_NODES", false),
	}

	if sec := envStr("AUTH_SECRET"); sec != "" {
		sum := sha256.Sum256([]byte(sec))
		c.Secret = sum[:]
	} else if pwd != "" {
		// 由密码派生，保证同一密码的多台机器能互认；换密码即全端下线
		sum := sha256.Sum256([]byte("als-custom-auth::" + pwd))
		c.Secret = sum[:]
	}

	if c.Enabled {
		if envStr("AUTH_SECRET") == "" {
			log.Printf("[custom/auth] 登录门已启用（账号 %q，密码来自 PANEL_PASSWORD）。未设置 AUTH_SECRET，"+
				"当前由密码派生签名密钥；跨机部署请显式设置相同的 AUTH_SECRET。\n", c.Username)
		} else {
			log.Printf("[custom/auth] 登录门已启用（账号 %q + PANEL_PASSWORD + AUTH_SECRET）。\n", c.Username)
		}
	} else {
		log.Println("[custom/auth] 登录门未启用：未配置 PANEL_PASSWORD 或 AUTH_ENABLED=false，" +
			"站点按原行为对外开放。")
	}
	return c
}
