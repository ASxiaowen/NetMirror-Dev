package share

import (
	"fmt"
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
//   - /custom/share        → 仅登录用户（增删查），见 auth.isUserOnly
//   - /custom/sharelink/*  → 永远放行，供链接持有者查看信息并兑换临时密码
//
// 路由命名刻意让 /share 与 /sharelink 成为同级的不同静态段，
// 避免 gin 路由树里「静态段 vs 参数段」在兄弟节点上的冲突。
func RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/share", handleList)
	g.POST("/share", handleCreate)
	g.DELETE("/share/:id", handleRevoke)

	g.GET("/sharelink/info", handleLinkInfo)
	g.POST("/sharelink/redeem", handleRedeem)
}

type createRequest struct {
	Note string `json:"note"`
	// Nodes 多节点绑定。单节点也走这条路（数组长度为 1），
	// 这样后端只有一种分支，不会出现「单/多节点行为不一致」。
	Nodes []auth.NodeRef `json:"nodes"`
	// NodeID / NodeName / NodeURL 是单节点绑定的旧字段，
	// 保留是为了让尚未更新的前端仍能签发（老前端只传这三个）。
	NodeID     string   `json:"nodeId"`
	NodeName   string   `json:"nodeName"`
	NodeURL    string   `json:"nodeUrl"`
	Tools      []string `json:"tools"`
	TTLSeconds int      `json:"ttlSeconds"`
}

// maxNodes 一条链接最多绑定的节点数。
//
// 不是技术限制，而是防呆：绑定范围越大，事后越难说清这条链接到底
// 把哪些机器交给了谁。真要覆盖很多台，应该拆成几条链接分别发放。
const maxNodes = 20

// normalizeNodes 归并「多节点数组」与「单节点三字段」两种入参，去重后校验。
//
// 保证返回的每一项都有可用的 http(s) 地址 —— 请求归属判断与前端建连都靠它。
// 顺序保持入参顺序（前端按勾选顺序提交，展示时也按这个顺序，不做二次排序）。
func normalizeNodes(req createRequest) ([]auth.NodeRef, string) {
	in := make([]auth.NodeRef, 0, len(req.Nodes)+1)
	in = append(in, req.Nodes...)
	if len(in) == 0 {
		// 兼容尚未更新的前端：只传了单节点三字段
		if url := strings.TrimSpace(req.NodeURL); url != "" {
			in = append(in, auth.NodeRef{
				ID:   strings.TrimSpace(req.NodeID),
				Name: strings.TrimSpace(req.NodeName),
				URL:  url,
			})
		}
	}

	out := make([]auth.NodeRef, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, n := range in {
		url := strings.TrimRight(strings.TrimSpace(n.URL), "/")
		if url == "" {
			continue
		}
		// 前端要靠它拼请求地址，没有 scheme 会拼出坏 URL；
		// 顺手挡住手误与意外注入。
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return nil, "节点地址必须以 http:// 或 https:// 开头：" + url
		}
		key := strings.ToLower(url)
		if seen[key] {
			continue // 同一台机器勾了两次（或 id 不同但地址相同）：按一台算
		}
		seen[key] = true

		name := strings.TrimSpace(n.Name)
		if name == "" {
			name = strings.TrimSpace(n.ID)
		}
		out = append(out, auth.NodeRef{
			ID:       strings.TrimSpace(n.ID),
			Name:     name,
			URL:      url,
			Location: strings.TrimSpace(n.Location),
		})
	}

	if len(out) == 0 {
		return nil, "请先选择要分享的节点"
	}
	if len(out) > maxNodes {
		return nil, fmt.Sprintf("一次最多绑定 %d 个节点，请拆分后再签发", maxNodes)
	}
	return out, ""
}

type redeemRequest struct {
	ID       string `json:"id"`
	Password string `json:"password"`
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

// linkPath 由记录 id 拼出链接路径
func linkPath(id string) string { return "/t/" + id }

// groupPassword 把 16 位十六进制临时密码按 4 位分组，便于人工抄写与输入。
// 纯展示形式，校验时会先剥掉分隔符。
func groupPassword(raw string) string {
	if len(raw) != 16 {
		return raw
	}
	return raw[0:4] + "-" + raw[4:8] + "-" + raw[8:12] + "-" + raw[12:16]
}

// normalizePassword 去掉用户输入里的分隔符/空白并统一小写，
// 让「带不带横杠」「大小写」都不影响兑换。
func normalizePassword(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// recordView 把记录转成给管理端的响应体 —— 白名单式列出字段，
// PassHash 永远不会被带出去（不是靠记得删，而是根本没写进来）。
func recordView(c *gin.Context, r *Record) gin.H {
	nodes := r.EffectiveNodes()
	view := gin.H{
		"id":        r.ID,
		"note":      r.Note,
		"nodes":     nodes,
		"nodeCount": len(nodes),
		// nodeName 给「列表里一行怎么称呼这批机器」用；完整列表在 nodes 里。
		"nodeName":  r.NodeSummary(),
		"tools":     r.Tools,
		"createdAt": r.CreatedAt,
		"expiresAt": r.ExpiresAt,
		"createdIp": r.CreatedIP,
		"revoked":   r.Revoked,
		"useCount":  r.UseCount,
		"lastUseAt": r.LastUseAt,
		"status":    r.Status(),
		"path":      linkPath(r.ID),
		"url":       baseURL(c) + linkPath(r.ID),
		"leftSecs":  r.LeftSeconds(),
	}
	// 旧字段：始终指向首个节点，让尚未更新的客户端仍能按单节点渲染，
	// 而不是因为缺少 nodeUrl 就显示成空。
	if len(nodes) > 0 {
		view["nodeId"] = nodes[0].ID
		view["nodeUrl"] = nodes[0].URL
	}
	return view
}

func handleCreate(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "BAD_REQUEST", "error": "请求格式错误"})
		return
	}

	// 节点：至少一台，每台都要有可用的 http(s) 地址
	nodes, nodeErr := normalizeNodes(req)
	if nodeErr != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "BAD_REQUEST", "error": nodeErr})
		return
	}

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

	// 生成两件凭证：链接 id（非秘密）与临时密码（秘密）
	linkID := auth.NewJti("t_")
	rawPwd := auth.RandHex(8) // 8 字节 = 16 位十六进制 = 64 bit 熵
	if linkID == "" || rawPwd == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false, "code": "RAND_FAILED",
			"error": "随机源不可用，已拒绝签发（不使用可预测的密码）",
		})
		return
	}

	now := time.Now()
	exp := now.Add(time.Duration(ttl) * time.Second).Unix()

	rec := &Record{
		ID:        linkID,
		Note:      strings.TrimSpace(req.Note),
		Nodes:     nodes,
		PassHash:  auth.HashSecret(linkID, rawPwd),
		Tools:     tools,
		CreatedAt: now.Unix(),
		ExpiresAt: exp,
		CreatedBy: "panel",
		CreatedIP: c.ClientIP(),
	}
	if err := db.Add(rec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "STORE_FAILED", "error": "保存记录失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"record":  recordView(c, rec),
		// 密码明文只在这一次响应里出现，之后任何接口都不再回显。
		// 前端必须提示管理员立刻复制保存。
		"password":  groupPassword(rawPwd),
		"path":      linkPath(linkID),
		"url":       baseURL(c) + linkPath(linkID),
		"expiresAt": exp,
	})
}

func handleList(c *gin.Context) {
	list := db.List()
	out := make([]gin.H, 0, len(list))
	for _, r := range list {
		out = append(out, recordView(c, r))
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

// handleLinkInfo 供链接持有者在输入密码前查看「这条链接给了我什么」。
//
// 永远放行。只回非秘密信息：不含密码派生值、不含签名令牌。
// 回显节点与工具是刻意的 —— 对方能在输密码前确认这条链接是不是给自己的、
// 能做什么；而这些信息即使泄露也不构成访问能力。
func handleLinkInfo(c *gin.Context) {
	id := strings.TrimSpace(c.Query("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "BAD_REQUEST", "error": "缺少链接标识"})
		return
	}
	rec, ok := db.FindByID(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false, "valid": false, "code": "LINK_NOT_FOUND",
			"error": "链接不存在，请向发送方确认地址是否完整",
		})
		return
	}
	if rec.Revoked {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false, "valid": false, "code": "LINK_REVOKED",
			"error": "该链接已被发送方吊销",
		})
		return
	}
	if rec.Expired() {
		c.JSON(http.StatusGone, gin.H{
			"success": false, "valid": false, "code": "LINK_EXPIRED",
			"error": "该链接已过期，请向发送方索取新的链接",
		})
		return
	}

	nodes := rec.EffectiveNodes()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"valid":   true,
		"info": gin.H{
			// nodes 是权威字段；nodeName 是给「一行文案」用的概括
			// （单节点即其名字，多节点形如「HKG1-246 等 3 个节点」）。
			"nodes":      nodes,
			"nodeCount":  len(nodes),
			"nodeName":   firstNonEmpty(rec.NodeSummary(), rec.NodeID, rec.NodeURL),
			"note":       rec.Note,
			"tools":      rec.Tools,
			"expiresAt":  rec.ExpiresAt,
			"leftSecs":   rec.LeftSeconds(),
			"needPasswd": true,
		},
	})
}

// handleRedeem 用「链接标识 + 临时密码」换取受限作用域的令牌。
//
// 这是访客唯一的入口，因此：
//   - 密码用定长比较（auth.CompareSecret）；
//   - 连续失败达阈值后按记录锁定一段时间（内存态，重启清零）；
//   - 令牌的 exp 取记录的剩余有效期，不会因 TokenTTLHours 而超期。
func handleRedeem(c *gin.Context) {
	var req redeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "BAD_REQUEST", "error": "请求格式错误"})
		return
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "BAD_REQUEST", "error": "缺少链接标识"})
		return
	}

	if ok, left := allowAttempt(id); !ok {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false, "code": "RATE_LIMITED",
			"error": "临时密码错误次数过多，请在 " + humanDuration(left) + "后重试",
		})
		return
	}

	rec, ok := db.FindByID(id)
	if !ok {
		// 链接标识是 64 bit 随机串，不担心被枚举；直接说明原因对使用者更友好
		c.JSON(http.StatusNotFound, gin.H{
			"success": false, "code": "LINK_NOT_FOUND",
			"error": "链接不存在，请向发送方确认地址是否完整",
		})
		return
	}
	if rec.Revoked {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false, "code": "LINK_REVOKED",
			"error": "该链接已被发送方吊销",
		})
		return
	}
	if rec.Expired() {
		c.JSON(http.StatusGone, gin.H{
			"success": false, "code": "LINK_EXPIRED",
			"error": "该链接已过期，请向发送方索取新的链接",
		})
		return
	}

	if !auth.CompareSecret(rec.ID, normalizePassword(req.Password), rec.PassHash) {
		locked, left := noteFailure(id)
		msg := "临时密码不正确"
		if locked {
			msg = "临时密码连续错误次数过多，已锁定 " + humanDuration(left)
		}
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false, "code": "BAD_PASSWORD",
			"error": msg,
		})
		return
	}
	clearAttempts(id)

	// 令牌的 jti 直接取记录 id：守卫的吊销回调按它查库，一处 id 贯穿到底
	nodes := rec.EffectiveNodes()
	claims := &auth.Claims{
		Kind:  auth.KindShare,
		Sub:   "share",
		Jti:   rec.ID,
		Nodes: nodes,
		Tools: rec.Tools,
		Note:  rec.Note,
		Iat:   time.Now().Unix(),
		Exp:   rec.ExpiresAt,
	}
	// 首个节点同时写进旧字段 NodeURL。理由见 auth.Claims.NodeURL 的注释：
	// 若某个节点还跑着不认识 Nodes 的旧版本，它只读 NodeURL ——
	// 留空会被它当成「未绑定节点」而放行任意 Host（权限放大），
	// 填首个节点则退化成「只认一台」（权限收敛）。
	if len(nodes) > 0 {
		claims.NodeID = nodes[0].ID
		claims.NodeURL = nodes[0].URL
	}
	token, err := auth.Sign(claims)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false, "code": "SIGN_FAILED", "error": "签发失败: " + err.Error(),
		})
		return
	}

	db.BumpUse(rec.ID)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"token":     token,
		"expiresAt": rec.ExpiresAt,
		"scope": gin.H{
			"kind":      auth.KindShare,
			"jti":       rec.ID,
			"note":      rec.Note,
			"nodes":     nodes,
			"nodeCount": len(nodes),
			// 概括文案，前端顶部提示条用它；逐台展示读 nodes
			"nodeName": firstNonEmpty(rec.NodeSummary(), rec.NodeID, rec.NodeURL),
			"tools":    rec.Tools,
			"exp":      rec.ExpiresAt,
			"leftSecs": rec.LeftSeconds(),
		},
	})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// humanDuration 把时长说成人话（用于限流提示）
func humanDuration(d time.Duration) string {
	if d <= 0 {
		return "片刻"
	}
	m := int(d.Minutes())
	if m <= 0 {
		return "不到 1 分钟"
	}
	if m < 60 {
		return strconv.Itoa(m) + " 分钟"
	}
	return strconv.Itoa(m/60) + " 小时"
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
