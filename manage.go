package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ── Config ─────────────────────────────────────

type ManageConfig struct {
	Providers []ProviderDef `json:"providers"`
}

type ProviderDef struct {
	Name      string   `json:"name"`
	TargetURL string   `json:"target_url"`
	GenaiURL  string   `json:"genai_url,omitempty"`
	APIKeys   []string `json:"api_keys"`
	Port      int      `json:"port"`
	Disabled  bool     `json:"disabled,omitempty"`
}

// ── Managed Instance ───────────────────────────

type ManagedInstance struct {
	Name    string
	Dir     string
	Port    int
	Cmd     *exec.Cmd
	Running bool
	mu      sync.Mutex
}

// writeEnvFile generates and writes the .env file for a managed instance.
func (m *ManagedInstance) writeEnvFile(cfg ProviderDef) error {
	// Ensure work dir exists
	if err := os.MkdirAll(m.Dir, 0755); err != nil {
		return fmt.Errorf("create dir %q: %v", m.Dir, err)
	}
	lines := []string{
		fmt.Sprintf("PORT=%d", m.Port),
		fmt.Sprintf("TARGET_BASE_URL=%s", strings.TrimRight(cfg.TargetURL, "/")),
		fmt.Sprintf("API_KEYS=%s", strings.Join(cfg.APIKeys, ",")),
		"COOLDOWN_SEC=60",
	}
	if cfg.GenaiURL != "" {
		lines = append(lines, fmt.Sprintf("GENAI_BASE_URL=%s", strings.TrimRight(cfg.GenaiURL, "/")))
	}
	content := strings.Join(lines, "\n") + "\n"
	envPath := filepath.Join(m.Dir, ".env")
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write .env: %v", err)
	}
	return nil
}

func (m *ManagedInstance) Start(binary string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Running {
		return nil
	}
	absDir, err := filepath.Abs(m.Dir)
	if err != nil {
		return fmt.Errorf("bad dir %q: %v", m.Dir, err)
	}
	// Ensure work dir exists
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return fmt.Errorf("create dir %q: %v", absDir, err)
	}
	// Verify .env exists
	if _, err := os.Stat(filepath.Join(absDir, ".env")); os.IsNotExist(err) {
		return fmt.Errorf(".env not found in %s — writeEnvFile() was not called", absDir)
	}
	cmd := exec.Command(binary, "-local")
	cmd.Dir = absDir
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %v", err)
	}
	cmd.Stdout = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %q: %v", m.Name, err)
	}
	m.Cmd = cmd
	m.Running = true

	go func() {
		stderr, _ := io.ReadAll(stderrPipe)
		if len(stderr) > 0 {
			log.Printf("⚠️ [%s] stderr: %s", m.Name, string(stderr))
		}
	}()

	go func() {
		cmd.Wait()
		m.mu.Lock()
		m.Running = false
		m.mu.Unlock()
		log.Printf("⚠️ [%s] exited (PID %d)", m.Name, cmd.Process.Pid)
	}()

	log.Printf("✅ [%s] started (PID %d, port %d, dir: %s)", m.Name, cmd.Process.Pid, m.Port, absDir)
	return nil
}

func (m *ManagedInstance) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.Running || m.Cmd == nil || m.Cmd.Process == nil {
		return
	}
	pid := m.Cmd.Process.Pid
	log.Printf("🛑 [%s] stopping (PID %d)...", m.Name, pid)
	m.Cmd.Process.Kill()
	m.Running = false
	log.Printf("🛑 [%s] stopped", m.Name)
}

// ── Manager ────────────────────────────────────

type Manager struct {
	instances []*ManagedInstance
	config    ManageConfig
	workBase  string
}

// detectOldFormat checks if the config is in the old format (has "dir" field).
func detectOldConfigFormat(cfg *ManageConfig) bool {
	// We can't detect "dir" from JSON unmarshal since Go discards unknown fields,
	// but we can check the raw JSON for the "dir" key.
	return false // handled via raw check in LoadManagerConfig
}

func LoadManagerConfig(path string) (ManageConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ManageConfig{}, fmt.Errorf("读取 %s 失败: %v", path, err)
	}

	// Detect old format: check if JSON has "dir" field
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err == nil {
		if rawProviders, ok := rawMap["providers"]; ok {
			if providers, ok := rawProviders.([]interface{}); ok {
				for _, p := range providers {
					if pm, ok := p.(map[string]interface{}); ok {
						if _, hasDir := pm["dir"]; hasDir {
							return ManageConfig{}, fmt.Errorf(
								"❌ manage.json 是旧格式，请参考 manage.example.json 更新\n" +
									"   改动说明：配置已合并，不再需要 dir 和 .env 文件\n" +
									"   新格式把 target_url 和 api_keys 直接写在 manage.json 里")
						}
					}
				}
			}
		}
	}

	var cfg ManageConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ManageConfig{}, fmt.Errorf("解析 %s 失败: %v", path, err)
	}

	usedPorts := make(map[int]string)
	for i, p := range cfg.Providers {
		if p.Name == "" {
			return ManageConfig{}, fmt.Errorf("providers[%d]: name 不能为空", i)
		}
		if p.TargetURL == "" {
			return ManageConfig{}, fmt.Errorf("providers[%d] %q: target_url 不能为空", i, p.Name)
		}
		if len(p.APIKeys) == 0 {
			return ManageConfig{}, fmt.Errorf("providers[%d] %q: 至少需要一个 api_key", i, p.Name)
		}
		// Auto-assign port if not set
		if p.Port == 0 {
			p.Port = 4000 + i
			cfg.Providers[i] = p
		}
		// Check port conflict
		if existing, ok := usedPorts[p.Port]; ok {
			return ManageConfig{}, fmt.Errorf(
				"❌ 端口 %d 冲突：%q 和 %q 都用了同一个端口，请修改其中一个的 port", p.Port, existing, p.Name)
		}
		usedPorts[p.Port] = p.Name
	}
	return cfg, nil
}

func NewManager(cfg ManageConfig) *Manager {
	m := &Manager{
		config:   cfg,
		workBase: filepath.Join("manage-work"),
	}
	for _, p := range cfg.Providers {
		if p.Disabled {
			continue
		}
		workDir := filepath.Join(m.workBase, p.Name)
		inst := &ManagedInstance{
			Name: p.Name,
			Dir:  workDir,
			Port: p.Port,
		}
		// Write .env file for this instance
		if err := inst.writeEnvFile(p); err != nil {
			log.Printf("❌ [%s] 创建配置失败: %v", p.Name, err)
			continue
		}
		m.instances = append(m.instances, inst)
	}
	return m
}

func (m *Manager) StartAll() int {
	count := 0
	self, _ := os.Executable()
	if self == "" {
		self = "alvus.exe"
	}
	for _, inst := range m.instances {
		if err := inst.Start(self); err != nil {
			log.Printf("❌ [%s] 启动失败: %v", inst.Name, err)
		} else {
			count++
		}
	}
	return count
}

func (m *Manager) StopAll() {
	for _, inst := range m.instances {
		inst.Stop()
	}
}

func (m *Manager) WatchAndRestart(stop <-chan struct{}) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			self, _ := os.Executable()
			if self == "" {
				self = "alvus.exe"
			}
			for _, inst := range m.instances {
				inst.mu.Lock()
				running := inst.Running
				inst.mu.Unlock()
				if !running {
					log.Printf("🔄 [%s] 重启中...", inst.Name)
					if err := inst.Start(self); err != nil {
						log.Printf("❌ [%s] 重启失败: %v", inst.Name, err)
					}
				}
			}
		}
	}
}

// ── RunMode: Manager ──────────────────────────

func runManager(managePath string, stop <-chan struct{}) {
	cfg, err := LoadManagerConfig(managePath)
	if err != nil {
		log.Fatalf("❌ %v", err)
	}

	mgr := NewManager(cfg)
	n := mgr.StartAll()
	log.Printf("🚀 已启动 %d/%d 个实例", n, len(mgr.instances))

	go mgr.WatchAndRestart(stop)

	<-stop
	log.Printf("🛑 管理器关闭中...")
	mgr.StopAll()

	// Clean up work directories
	workBase := filepath.Join("manage-work")
	if fi, err := os.Stat(workBase); err == nil && fi.IsDir() {
		if err := os.RemoveAll(workBase); err != nil {
			log.Printf("⚠️ 清理工作目录失败: %v", err)
		}
	}
}
