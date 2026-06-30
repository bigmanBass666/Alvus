package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alvus/internal/cmd"
	"alvus/internal/config"
)

// resetConfigEnv clears all config-related env vars to prevent interference.
func resetConfigEnv() {
	resetAllEnv()
	for _, k := range []string{
		"BACKOFF_CAP_SEC", "BACKOFF_MULTIPLIER", "CB_RESET_SEC",
		"UPSTREAM_CB_THRESHOLD", "KEYS_FILE",
		"HEALTH_CHECK_INTERVAL_SEC", "HEALTH_CHECK_PATH", "HEALTH_CHECK_TIMEOUT_SEC",
		"KEYS_ENCRYPTION_KEY",
	} {
		os.Unsetenv(k)
	}
}

// TestConfigInit_CreatesFile verifies that "alvus config init -p <path>"
// creates a valid TOML config file at the specified path.
func TestConfigInit_CreatesFile(t *testing.T) {
	resetConfigEnv()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"alvus", "config", "init", "-p", configPath}

	err := cmd.Execute("")
	if err != nil {
		t.Fatalf("config init failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config.toml was not created")
	}

	// Verify file is loadable as valid TOML config
	cfg, err := config.LoadToml(configPath)
	if err != nil {
		t.Fatalf("created config.toml is not valid: %v", err)
	}
	// Generated config should have example placeholder providers
	if cfg.TargetBase != "https://api.example-a.com/v1" {
		t.Errorf("TargetBase should be set to example-a target, got %q", cfg.TargetBase)
	}
}

// TestConfigView_ShowsConfig verifies that "alvus config view" prints
// the current configuration from a .env file.
func TestConfigView_ShowsConfig(t *testing.T) {
	resetConfigEnv()
	// Ensure XDG config does not exist, otherwise it would take priority
	xdgPath, _ := config.XDGConfigPath()
	if _, err := os.Stat(xdgPath); err == nil {
		t.Skipf("XDG config exists at %s, would interfere with config view test", xdgPath)
	}

	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(oldDir) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create a .env file in the temp directory
	envContent := `TARGET_BASE_URL=https://api.example.com
GENAI_BASE_URL=https://ai.example.com
API_KEYS=nvapi-key1-abcdef
`
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"alvus", "config", "view"}

	err := cmd.Execute("")
	w.Close()
	os.Stdout = oldStdout

	out, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("config view failed: %v", err)
	}

	output := string(out)

	// Verify output contains expected fields
	if !strings.Contains(output, "Configuration source:") {
		t.Error("config view output missing 'Configuration source:'")
	}
	if !strings.Contains(output, "https://api.example.com") {
		t.Error("config view output missing target URL")
	}
	if !strings.Contains(output, "nvap...cdef") {
		t.Error("config view output missing masked API key, got:", output)
	}
}

// TestConfigInit_DetectsEnvFile verifies that "alvus config init" prints
// a migration hint when a .env file exists in the current directory.
func TestConfigInit_DetectsEnvFile(t *testing.T) {
	resetConfigEnv()
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(oldDir) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create .env in the temp directory
	envContent := `TARGET_BASE_URL=https://example.com
GENAI_BASE_URL=https://ai.example.com
API_KEYS=nvapi-key1
`
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(tmpDir, "config.toml")

	// Capture stdout
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"alvus", "config", "init", "-p", configPath}

	err := cmd.Execute("")
	w.Close()
	os.Stdout = oldStdout

	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("config init failed: %v", err)
	}

	// Check file was created
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		t.Error("config.toml was not created")
	}

	// Check for migration hint about .env
	output := buf.String()
	if !strings.Contains(output, ".env") {
		t.Error("config init output should mention .env migration hint")
	}
}