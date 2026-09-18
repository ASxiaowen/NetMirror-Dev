package auth

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

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
//
// 前四条永远放行（靠 Guard 里的 isAlwaysOpen），否则登录页无法调用；
// credentials 两条则要求登录用户（靠 isUserOnly），临时链接令牌一律拒绝。
func RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/auth/config", handlePublicConfig)
	g.POST("/auth/login", handleLogin)
	g.GET("/auth/verify", handleVerify)
	g.POST("/auth/logout", handleLogout)

	g.GET("/auth/credentials", handleGetCredentials)
	g.POST("/auth/credentials", handleUpdateCredentials)
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

	// 取「当前生效」凭据：环境变量给初值，管理页可运行时覆盖（见 credentials.go）。
	// 每次登录都取一次而不是用包级 Cfg，这样任意一个进程改完密码，
	// 另一个进程（panel 与 agent 是独立进程）的下一次登录就能用上新值。
	curUser, curPass, _ := CredentialsInfo()

	// 账号与密码都用定长比较，避免时序侧信道；两个比较都执行完再判断，
	// 不因账号错就短路返回，否则响应耗时会泄漏「账号是否存在」。
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(curUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(curPass)) == 1
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
		Sub:  curUser,
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
		"username":  curUser,
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
		// nodes 是权威字段（多节点）；nodeId/nodeUrl/nodeName 保留给旧客户端与旧令牌，
		// 这样前端升级前后都能正确渲染。
		nodes := cl.NodesOrDefault()
		h["nodes"] = nodes
		h["nodeCount"] = len(nodes)
		h["nodeId"] = cl.NodeID
		h["nodeUrl"] = cl.NodeURL
		// 刷新页面时外壳走的是 verify 这条路径（而不是 redeem），
		// 少了 nodeName 受限模式的节点名会退化成 id。
		h["nodeName"] = cl.NodesSummary()
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

// handleGetCredentials 返回当前生效的账号与密码，供管理页展示与修改。
//
// 为什么把密码回显给前端：能走到这里的请求已经过了登录门（而本接口又被
// isUserOnly 限制为「仅登录用户」），且服务器上的 EnvironmentFile 本身就是
// 明文、管理员随时能 cat。不回显反而让他无法确认「当前密码到底是什么」，
// 改密时容易写错。代价是密码会经网络传输，因此：
//   - 加 Cache-Control: no-store，禁止浏览器与中间层缓存；
//   - 部署侧要求走可信网络或 HTTPS（见交付文档「已知限制」）。
func handleGetCredentials(c *gin.Context) {
	if !Cfg.Enabled {
		c.JSON(http.StatusOK, gin.H{"success": true, "enabled": false})
		return
	}

	user, pass, source := CredentialsInfo()

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"enabled":     true,
		"username":    user,
		"password":    pass,
		"source":      source, // env | runtime
		"minPassword": MinPasswordLen,
		"filePath":    CredPath(),
	})
}

type credUpdateRequest struct {
	// CurrentPassword 当前密码，改任何一项都必须提供
	CurrentPassword string `json:"currentPassword"`
	// Username 留空表示不改账号
	Username string `json:"username"`
	// Password 留空表示不改密码
	Password string `json:"password"`
}

// handleUpdateCredentials 修改账号 / 密码，改完立即生效，无需重启服务。
//
// 要求提供「当前密码」而不是只看令牌：万一令牌被人拿到（或管理员忘了锁屏），
// 也改不了密码。这一层是改密操作的最后一道闸。
func handleUpdateCredentials(c *gin.Context) {
	if !Cfg.Enabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "AUTH_DISABLED",
			"error":   "登录门未启用，无需修改凭据",
		})
		return
	}

	var req credUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "BAD_REQUEST",
			"error":   "请求格式错误",
		})
		return
	}

	curUser, curPass, _ := CredentialsInfo()

	if subtle.ConstantTimeCompare([]byte(req.CurrentPassword), []byte(curPass)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"code":    CodeAuthRequired,
			"error":   "当前密码不正确",
		})
		return
	}

	newUser := strings.TrimSpace(req.Username)
	newPass := req.Password

	if newUser == "" && newPass == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "NO_CHANGE",
			"error":   "没有要修改的内容",
		})
		return
	}

	// 用 RuneCount 而不是 len：中文密码按「字数」而不是「字节数」判断长度
	if newPass != "" && utf8.RuneCountInString(newPass) < MinPasswordLen {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "WEAK_PASSWORD",
			"error":   fmt.Sprintf("新密码至少 %d 位", MinPasswordLen),
		})
		return
	}

	target := curUser
	if newUser != "" {
		target = newUser
	}
	pass := curPass
	if newPass != "" {
		pass = newPass
	}

	changed := make([]string, 0, 2)
	if target != curUser {
		changed = append(changed, "username")
	}
	if pass != curPass {
		changed = append(changed, "password")
	}
	if len(changed) == 0 {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"username": target,
			"changed":  []string{},
			"message":  "内容与当前一致，未做修改",
		})
		return
	}

	if err := SetCredentials(target, pass, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    "SAVE_FAILED",
			"error":   "保存失败：" + err.Error(),
		})
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"username": target,
		"changed":  changed,
		"message":  "已生效。已签发的登录令牌在到期前仍然有效。",
	})
}
