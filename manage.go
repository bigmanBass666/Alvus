package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ── Config ─────────────────────────────────────

type ManageConfig struct {
	Providers []ProviderDef `json:"providers"`
}

type ProviderDef struct {
	Name     string `json:"name"`
	Dir      string `json:"dir"`
	Port     int    `json:"port"`
	Disabled bool   `json:"disabled,omitempty"`
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
	// Verify .env exists
	if _, err := os.Stat(filepath.Join(absDir, ".env")); os.IsNotExist(err) {
		log.Printf("⚠️ [%s] no .env in %s", m.Name, absDir)
	}
	cmd := exec.Command(binary, "-local")
	cmd.Dir = absDir
	// Capture stderr for diagnostics
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

	// Read child's stderr and log it
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
}

func LoadManagerConfig(path string) (ManageConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ManageConfig{}, fmt.Errorf("read %s: %v", path, err)
	}
	var cfg ManageConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ManageConfig{}, fmt.Errorf("parse %s: %v", path, err)
	}
	for i, p := range cfg.Providers {
		if p.Name == "" {
			return ManageConfig{}, fmt.Errorf("provider[%d]: name is required", i)
		}
		if p.Dir == "" {
			p.Dir = filepath.Join("proxies", p.Name)
			cfg.Providers[i] = p
		}
	}
	return cfg, nil
}

func NewManager(cfg ManageConfig) *Manager {
	m := &Manager{config: cfg}
	for _, p := range cfg.Providers {
		if p.Disabled {
			continue
		}
		m.instances = append(m.instances, &ManagedInstance{
			Name: p.Name,
			Dir:  p.Dir,
			Port: p.Port,
		})
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
			log.Printf("❌ [%s] %v", inst.Name, err)
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
					log.Printf("🔄 [%s] restarting...", inst.Name)
					if err := inst.Start(self); err != nil {
						log.Printf("❌ [%s] restart failed: %v", inst.Name, err)
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
	log.Printf("🚀 Manager: %d/%d instances started", n, len(mgr.instances))

	go mgr.WatchAndRestart(stop)

	<-stop
	log.Printf("🛑 Manager shutting down...")
	mgr.StopAll()
}
