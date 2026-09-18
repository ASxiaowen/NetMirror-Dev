// Package auth · 运行时凭据（可在管理页修改）
//
// 为什么需要这一层：
//   - 环境变量（systemd EnvironmentFile）是**部署期初值**：改它要登服务器、
//     要重启服务，不适合当日常操作。
//   - 管理页需要「改完立即生效、不用重启」。
//
// 于是引入 DATA_DIR/custom_auth.json 作为运行时覆盖文件：
//   - 文件不存在 → 完全沿用环境变量（已有部署行为零变化）
//   - 文件存在   → 账号 / 密码以文件为准，环境变量退化为「出厂默认」
//   - 删掉文件   → 立即回到环境变量值（等于一键恢复默认）
//
// 多进程一致性：panel(:8081) 与 agent(:3000) 是两个独立进程。任一方改密后
// 另一方必须立刻看到新值，否则会出现「在面板改了密码、节点上却还在用旧的」。
// 这里用「按 mtime 惰性重载」解决：每次取凭据前 stat 一次文件，变了才重新解析。
// 登录不是热路径，这点开销可忽略。
//
// 权限：文件以 0600 落盘，仅属主可读 —— 与 /etc/netmirror/custom-auth.env 同级。
package auth

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// errInvalidCred 账号或密码为空
var errInvalidCred = errors.New("账号与密码均不能为空")

// 凭据来源，供界面展示「当前值是环境变量给的还是页面改的」
const (
	// CredSourceEnv 来自环境变量（尚未在页面改过，或运行时文件已被删除）
	CredSourceEnv = "env"
	// CredSourceRuntime 来自运行时文件
	CredSourceRuntime = "runtime"
)

// MinPasswordLen 新密码的最小长度。
// 面板暴露在公网时可被暴力尝试，太短的密码撑不住。
const MinPasswordLen = 8

type credFile struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
	UpdatedIP string `json:"updatedIp,omitempty"`
}

var (
	credMu   sync.Mutex
	credUser string
	credPass string
	// credMT 已加载文件的 mtime，零值表示「当前用的是环境变量」
	credMT   time.Time
	credInit bool
)

// CredPath 运行时凭据文件路径，取值方式与上游 DATA_DIR 一致
func CredPath() string {
	dir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dir == "" {
		dir = "./data"
	}
	return filepath.Join(dir, "custom_auth.json")
}

// resolveCreds 返回当前生效的账号、密码与来源。
//
// 调用前会按 mtime 判断是否需要重载，因此多进程部署下任一方改密都能立即生效。
func resolveCreds() (user, pass, source string) {
	credMu.Lock()
	defer credMu.Unlock()

	if !credInit {
		credUser, credPass = Cfg.Username, Cfg.Password
		credInit = true
	}

	path := CredPath()
	st, err := os.Stat(path)
	if err != nil {
		// 文件不存在：回到环境变量值。
		// 覆盖「曾经有、后来被删掉」的场景 —— 删除即恢复默认。
		if !credMT.IsZero() {
			credUser, credPass = Cfg.Username, Cfg.Password
			credMT = time.Time{}
			log.Printf("[custom/auth] 运行时凭据文件已移除，账号密码回退为环境变量值（账号 %q）。\n", credUser)
		}
		return credUser, credPass, CredSourceEnv
	}

	if st.ModTime().Equal(credMT) {
		return credUser, credPass, CredSourceRuntime
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[custom/auth] 运行时凭据文件读取失败，继续使用当前凭据: %v\n", err)
		return credUser, credPass, currentSource()
	}

	var f credFile
	if err := json.Unmarshal(raw, &f); err != nil {
		// 文件损坏不阻塞服务：继续用现有凭据，避免把管理员锁在门外
		log.Printf("[custom/auth] 运行时凭据文件解析失败，继续使用当前凭据: %v\n", err)
		return credUser, credPass, currentSource()
	}

	u := strings.TrimSpace(f.Username)
	if u == "" || f.Password == "" {
		log.Println("[custom/auth] 运行时凭据文件字段不完整（账号或密码为空），已忽略。")
		return credUser, credPass, currentSource()
	}

	credUser, credPass = u, f.Password
	credMT = st.ModTime()
	log.Printf("[custom/auth] 已加载运行时凭据（账号 %q，更新时间 %s）。\n",
		credUser, st.ModTime().Format("2006-01-02 15:04:05"))
	return credUser, credPass, CredSourceRuntime
}

// currentSource 仅用于「回退分支」里报告来源，调用方必须已持有 credMu
func currentSource() string {
	if credMT.IsZero() {
		return CredSourceEnv
	}
	return CredSourceRuntime
}

// CurrentCredentials 供守卫等外部调用，返回当前生效的账号与密码
func CurrentCredentials() (string, string) {
	u, p, _ := resolveCreds()
	return u, p
}

// CredentialsInfo 供管理接口读取（含来源标记）
func CredentialsInfo() (user, pass, source string) {
	return resolveCreds()
}

// SetCredentials 落盘并立即生效。
//
// 写入是「先写临时文件再 rename」：rename 在同一文件系统内是原子的，
// 避免出现「文件被截断到一半时恰好有请求来读」的窗口。
func SetCredentials(username, password, ip string) error {
	u := strings.TrimSpace(username)
	if u == "" || password == "" {
		return errInvalidCred
	}

	f := credFile{
		Username:  u,
		Password:  password,
		UpdatedAt: time.Now().Unix(),
		UpdatedIP: ip,
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}

	path := CredPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	// WriteFile 对已存在的文件不改权限，显式收紧一次
	if err := os.Chmod(tmp, 0600); err != nil {
		log.Printf("[custom/auth] 设置凭据文件权限失败（仍继续）: %v\n", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	// 立即刷新内存，别等下一次 stat（否则同一进程紧接着的校验会用旧值）
	credMu.Lock()
	credUser, credPass = u, password
	credInit = true
	credMu.Unlock()

	if st, err := os.Stat(path); err == nil {
		credMu.Lock()
		credMT = st.ModTime()
		credMu.Unlock()
	}

	log.Printf("[custom/auth] 登录凭据已更新（账号 %q，来自 %s）。\n", u, ip)
	return nil
}

// ResetCredentials 删除运行时文件，回到环境变量值
func ResetCredentials() error {
	path := CredPath()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	credMu.Lock()
	credUser, credPass = Cfg.Username, Cfg.Password
	credMT = time.Time{}
	credInit = true
	credMu.Unlock()

	log.Printf("[custom/auth] 运行时凭据已重置为环境变量值（账号 %q）。\n", Cfg.Username)
	return nil
}
