// Package share 是 NetMirror 二次开发的临时链接模块。
//
// 用途：把一个节点 + 一组工具 + 一个有效期打包成一条链接 `/t/<token>`，
// 发给客户或同事自测，无需把面板口令交出去，到期自动失效，也可随时吊销。
//
// 与 auth 的分工：
//   - auth 负责「令牌签名与通用守卫」，它不认识临时链接的业务语义；
//   - share 负责签发、列表、吊销，并通过 auth.RevocationChecker 把吊销结果
//     注入守卫（反向依赖，避免 auth 反过来 import share 造成循环）。
//
// 存储：DATA_DIR（缺省 ./data）下的 custom_shares.json，与上游 nodes.json /
// tokens.json 同目录同风格，不引入新的存储依赖。
package share

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/X-Zero-L/als/custom_modules/auth"
)

// Record 是一条临时链接的元数据。令牌本身不落库（无状态），
// 这里只存吊销与审计所需的最小信息。
type Record struct {
	ID        string   `json:"id"`
	Jti       string   `json:"jti"` // 与令牌载荷中的 jti 对应，用于吊销判定
	Note      string   `json:"note"`
	NodeID    string   `json:"nodeId"`
	NodeURL   string   `json:"nodeUrl"`
	NodeName  string   `json:"nodeName"`
	Tools     []string `json:"tools"`
	CreatedAt int64    `json:"createdAt"`
	ExpiresAt int64    `json:"expiresAt"`
	CreatedBy string   `json:"createdBy"`
	CreatedIP string   `json:"createdIp"`
	Revoked   bool     `json:"revoked"`
	RevokedAt int64    `json:"revokedAt,omitempty"`
	UseCount  int      `json:"useCount"`
}

// Expired 是否已过期
func (r *Record) Expired() bool {
	return r.ExpiresAt > 0 && time.Now().Unix() > r.ExpiresAt
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

func (s *store) load() {
	if s.loaded {
		return
	}
	s.loaded = true

	p := s.filePath()
	raw, err := os.ReadFile(p)
	if err != nil {
		return // 首次运行文件不存在，属正常
	}
	var list []*Record
	if err := json.Unmarshal(raw, &list); err != nil {
		return // 文件损坏时不阻塞服务，按空库处理
	}
	s.records = list
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
	return os.Rename(tmp, s.filePath())
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

// Revoke 按业务 id 吊销
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

// Lookup 按 jti 查记录，返回 (记录, 是否存在)
func (s *store) Lookup(jti string) (*Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for _, r := range s.records {
		if r.Jti == jti {
			return r, true
		}
	}
	return nil, false
}

// BumpUse 记录一次使用（用于「查看谁在用」的审计），失败不影响请求
func (s *store) BumpUse(jti string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for _, r := range s.records {
		if r.Jti == jti {
			r.UseCount++
			_ = s.persist()
			return
		}
	}
}

// Init 装配模块：把吊销判定注入 auth 的守卫。
//
// 之所以注入而不是让 auth 直接读库：auth 是通用鉴权层，不该认识临时链接的业务语义；
// 同时这样能避免 auth → share 的反向 import 形成循环依赖。
//
// 「known=false」表示本机没有这条记录 —— 常见于请求打到的是远端节点，
// 它只持有签名密钥、不持有签发方的记录表。此时按令牌自带的 exp 生效，
// 吊销立即生效的范围限于共享同一 DATA_DIR 的进程。
func Init() {
	auth.RevocationChecker = func(jti string) (revoked bool, known bool) {
		if jti == "" {
			return false, false
		}
		r, ok := db.Lookup(jti)
		if !ok {
			return false, false
		}
		return r.Revoked || r.Expired(), true
	}
}
