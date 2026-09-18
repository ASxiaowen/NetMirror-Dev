// Package share 是 NetMirror 二次开发的临时链接模块。
//
// 用途：把「一组节点 + 一组工具 + 一个有效期」打包成一条链接 `/t/<id>`，
// 再配一个**一次性展示**的临时密码。链接与密码分开发送，对方打开链接后
// 输入密码才能进入受限模式 —— 链接被转发出去也没用。
//
// 绑定的节点可以是多台：访客在这些节点之间自由切换，工具白名单对
// 所有节点**共用一套**（不按节点区分权限）。这是刻意的取舍 ——
// 「节点×工具」的二维矩阵容易配错，而实际场景（让客户测这几台机器的这几项）
// 一维就够了。
//
// 两件凭证的分工：
//   - **链接 id 不是秘密**，它只用来定位记录（类似用户名）。所以它可以明文落库，
//     管理员事后也能从列表里重新复制链接。
//   - **临时密码才是秘密**，只存 HMAC 派生值（见 auth.HashSecret），
//     创建时返回一次明文，之后任何接口都不再回显。
//
// 与 auth 的分工：
//   - auth 负责「令牌签名、密码派生与通用守卫」，它不认识临时链接的业务语义；
//   - share 负责签发、列表、吊销、密码校验，并通过 auth.RevocationChecker
//     把吊销结果注入守卫（反向依赖，避免 auth 反过来 import share 造成循环）。
//
// 存储：DATA_DIR（缺省 ./data）下的 custom_shares.json，与上游 nodes.json /
// tokens.json 同目录同风格，不引入新的存储依赖。
package share

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/X-Zero-L/als/custom_modules/auth"
)

// Record 是一条临时链接的记录。
type Record struct {
	// ID 同时充当三处标识：URL 里的 /t/<ID>、令牌载荷的 jti（吊销用）、
	// 以及密码派生时的绑定串。合成一个可以省掉「id 与 jti 两套值互相对照」的负担。
	ID   string `json:"id"`
	Note string `json:"note"`

	// Nodes 绑定的节点集合（多节点：访客可在这些节点间自由切换）。
	// 工具白名单对所有节点共用一套，不按节点区分权限。
	Nodes []auth.NodeRef `json:"nodes,omitempty"`

	// 以下是早期「单节点绑定」的字段。保留只为**向后读取**：
	// 升级前落库的记录只有它们，EffectiveNodes 会合成一条返回，
	// 因此本次升级不需要迁移 JSON、旧链接继续可用。
	NodeID   string `json:"nodeId,omitempty"`
	NodeURL  string `json:"nodeUrl,omitempty"`
	NodeName string `json:"nodeName,omitempty"`

	// PassHash 临时密码的派生值（HMAC(secret, id::明文)）。永不出现在响应里。
	PassHash  string   `json:"passHash"`
	Tools     []string `json:"tools"`
	CreatedAt int64    `json:"createdAt"`
	ExpiresAt int64    `json:"expiresAt"`
	CreatedBy string   `json:"createdBy"`
	CreatedIP string   `json:"createdIp"`
	Revoked   bool     `json:"revoked"`
	RevokedAt int64    `json:"revokedAt,omitempty"`
	UseCount  int      `json:"useCount"`
	LastUseAt int64    `json:"lastUseAt,omitempty"`
}

// EffectiveNodes 返回该记录绑定的节点列表。
//
// 新记录读 Nodes；早期记录只有单值字段，这里合成一条 —— 于是调用方
// （签发令牌、渲染列表、展示详情）都不需要关心记录是新是旧。
func (r *Record) EffectiveNodes() []auth.NodeRef {
	if len(r.Nodes) > 0 {
		return r.Nodes
	}
	if r.NodeURL == "" && r.NodeID == "" && r.NodeName == "" {
		return nil
	}
	name := firstNonEmpty(r.NodeName, r.NodeID, r.NodeURL)
	return []auth.NodeRef{{ID: r.NodeID, Name: name, URL: r.NodeURL}}
}

// NodeSummary 把绑定的节点概括成一句话（列表与详情页展示用）。
func (r *Record) NodeSummary() string {
	nodes := r.EffectiveNodes()
	switch len(nodes) {
	case 0:
		return ""
	case 1:
		return nodes[0].Label()
	default:
		return nodes[0].Label() + " 等 " + strconv.Itoa(len(nodes)) + " 个节点"
	}
}

// Expired 是否已过期
func (r *Record) Expired() bool {
	return r.ExpiresAt > 0 && time.Now().Unix() > r.ExpiresAt
}

// Usable 是否仍可用于兑换（未吊销、未过期）
func (r *Record) Usable() bool {
	return !r.Revoked && !r.Expired()
}

// Status 计算展示用状态
func (r *Record) Status() string {
	if r.Revoked {
		return "revoked"
	}
	if r.Expired() {
		return "expired"
	}
	return "active"
}

// LeftSeconds 剩余有效秒数（已失效返回 0）
func (r *Record) LeftSeconds() int64 {
	if !r.Usable() {
		return 0
	}
	left := r.ExpiresAt - time.Now().Unix()
	if left < 0 {
		return 0
	}
	return left
}

// KnownTools 允许被授权的工具集合，与前端 id 对齐
// （见 ui/src/components/Utilities.vue 与 ui/src/custom_components/toolIcons.js）。
var KnownTools = []string{
	"ping", "ping6",
	"mtr", "mtr6",
	"traceroute", "traceroute6",
	"iperf3", "speedtest-net",
	"shell", "traffic",
	"speedtest",
}

// IsKnownTool 校验工具 id 是否合法，防止签发端传入手误的白名单
func IsKnownTool(t string) bool {
	t = strings.TrimSpace(t)
	if t == "*" {
		return true
	}
	for _, k := range KnownTools {
		if k == t {
			return true
		}
	}
	return false
}

type store struct {
	mu      sync.RWMutex
	path    string
	records []*Record
	loaded  bool
	modTime time.Time // 上次成功加载时文件的修改时间，用于跨进程感知变更
}

var db = &store{}

// dataDir 与上游 tokens/storage.go 的取值方式保持一致
func dataDir() string {
	dir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dir == "" {
		dir = "./data"
	}
	return dir
}

func (s *store) filePath() string {
	if s.path == "" {
		s.path = filepath.Join(dataDir(), "custom_shares.json")
	}
	return s.path
}

// load 保证 s.records 是磁盘上最新的一份。
//
// 为什么不能「只加载一次」：panel 与 agent（或多个节点）常常是**共用同一份 JSON 文件
// 的不同进程**。若各自在启动时读一次就永久缓存，那么 A 进程吊销后、B 进程的内存里
// 仍是旧快照，临时链接在 B 上继续可用 —— 吊销形同虚设。
//
// 这里按文件修改时间做增量重载：stat 很便宜，只有 mtime 变新时才真正读盘解析。
// 既保证跨进程吊销在下一次请求就生效，又不会让每个请求都付一遍 JSON 解析的代价。
func (s *store) load() {
	p := s.filePath()
	st, err := os.Stat(p)
	if err != nil {
		return // 首次运行文件不存在，属正常；保持现有内存状态
	}

	if s.loaded && !st.ModTime().After(s.modTime) {
		return // 文件没动过，继续用内存快照
	}

	raw, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var list []*Record
	if err := json.Unmarshal(raw, &list); err != nil {
		return // 文件损坏时不阻塞服务，保留上一次的可用快照
	}
	s.records = list
	s.loaded = true
	s.modTime = st.ModTime()
}

func (s *store) persist() error {
	if err := os.MkdirAll(dataDir(), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.records, "", "  ")
	if err != nil {
		return err
	}
	// 先写临时文件再改名，避免写到一半进程退出留下半个 JSON
	tmp := s.filePath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.filePath()); err != nil {
		return err
	}
	// 记下自己刚写出的时间：否则下一次 load 会看到 mtime 变新，
	// 把刚刚亲手写进去的内容再读一遍（读出来的完全一样，纯属浪费）。
	if st, err := os.Stat(s.filePath()); err == nil {
		s.modTime = st.ModTime()
	}
	return nil
}

// List 返回全部记录（新→旧）
func (s *store) List() []*Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()

	out := make([]*Record, len(s.records))
	copy(out, s.records)
	return out
}

// Add 新增一条记录
func (s *store) Add(r *Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	s.records = append([]*Record{r}, s.records...)
	return s.persist()
}

// Revoke 按记录 id 吊销
func (s *store) Revoke(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for _, r := range s.records {
		if r.ID == id && !r.Revoked {
			r.Revoked = true
			r.RevokedAt = time.Now().Unix()
			_ = s.persist()
			return true
		}
	}
	return false
}

// FindByID 按记录 id 查，返回 (记录, 是否存在)
func (s *store) FindByID(id string) (*Record, bool) {
	if id == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for _, r := range s.records {
		if r.ID == id {
			return r, true
		}
	}
	return nil, false
}

// BumpUse 记录一次成功兑换（用于「查看谁在用」的审计），失败不影响请求
func (s *store) BumpUse(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for _, r := range s.records {
		if r.ID == id {
			r.UseCount++
			r.LastUseAt = time.Now().Unix()
			_ = s.persist()
			return
		}
	}
}

// ============================ 兑换限流 ============================
//
// 临时密码是 64 bit 随机串，在线爆破本就不现实；限流的意义在于挡住
// 「拿一条已知链接反复试」的脚本，同时避免无脑请求把日志刷爆。
//
// 刻意放在内存里、不落库：重启即清零，代价是重启后限流计数丢失 ——
// 对一次性测试链接来说这个取舍可以接受，换来的是不污染 JSON 记录结构。

const (
	maxAttempts   = 8                // 一个窗口内允许的失败次数
	attemptWindow = 15 * time.Minute // 超过次数后锁定的时长
)

type attempt struct {
	fails int
	until time.Time
}

var (
	attemptMu sync.Mutex
	attempts  = map[string]*attempt{}
)

// allowAttempt 判断某条记录当前是否还允许尝试兑换
func allowAttempt(id string) (bool, time.Duration) {
	attemptMu.Lock()
	defer attemptMu.Unlock()
	a := attempts[id]
	if a == nil {
		return true, 0
	}
	if a.until.After(time.Now()) {
		return false, time.Until(a.until)
	}
	return true, 0
}

// noteFailure 记一次失败；返回是否已触发锁定与实际锁定剩余时长
func noteFailure(id string) (locked bool, left time.Duration) {
	attemptMu.Lock()
	defer attemptMu.Unlock()
	a := attempts[id]
	if a == nil {
		a = &attempt{}
		attempts[id] = a
	}
	// 窗口已过则重新计数
	if a.until.Before(time.Now()) && a.fails >= maxAttempts {
		a.fails = 0
		a.until = time.Time{}
	}
	a.fails++
	if a.fails >= maxAttempts {
		a.until = time.Now().Add(attemptWindow)
		return true, time.Until(a.until)
	}
	return false, 0
}

// clearAttempts 兑换成功后清空该记录的失败计数
func clearAttempts(id string) {
	attemptMu.Lock()
	defer attemptMu.Unlock()
	delete(attempts, id)
}

// Init 装配模块：把吊销判定注入 auth 的守卫。
//
// 之所以注入而不是让 auth 直接读库：auth 是通用鉴权层，不该认识临时链接的业务语义；
// 同时这样能避免 auth → share 的反向 import 形成循环依赖。
//
// 「known=false」表示本机看不到这条记录 —— 即请求打到的是**不共享 DATA_DIR 的远端节点**：
// 它只持有签名密钥、没有签发方的记录表。此时按令牌自带的 exp 生效，吊销无法立刻传播。
//
// 共享同一份 DATA_DIR 的进程（如同一台机上的 panel 与 agent）则能立即感知吊销 ——
// load() 按文件 mtime 增量重载，任一侧写盘后另一侧在下一个请求就能读到。
func Init() {
	auth.RevocationChecker = func(jti string) (revoked bool, known bool) {
		if jti == "" {
			return false, false
		}
		r, ok := db.FindByID(jti)
		if !ok {
			return false, false
		}
		return !r.Usable(), true
	}
}
