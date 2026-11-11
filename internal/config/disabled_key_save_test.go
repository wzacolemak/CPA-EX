package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The disabled flag has to survive a save, and have to be removed again once the
// operator switches the key back on: the writer merges into the existing file, so a
// flag left behind would silently keep the credential disabled.
func TestSaveConfigPreserveComments_DisabledKeyFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	initial := "port: 8317\n" +
		"claude-api-key:\n" +
		"  - api-key: \"sk-ant-1\"\n" +
		"    base-url: \"https://api.anthropic.com\"\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.ClaudeKey) != 1 {
		t.Fatalf("claude keys = %d, want 1", len(cfg.ClaudeKey))
	}

	cfg.ClaudeKey[0].Disabled = true
	if err = SaveConfigPreserveComments(path, cfg); err != nil {
		t.Fatalf("save disabled key: %v", err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "disabled: true") {
		t.Fatalf("disabled flag missing after save:\n%s", saved)
	}

	reloaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.ClaudeKey) != 1 || !reloaded.ClaudeKey[0].Disabled {
		t.Fatalf("disabled flag did not round-trip: %#v", reloaded.ClaudeKey)
	}

	reloaded.ClaudeKey[0].Disabled = false
	if err = SaveConfigPreserveComments(path, reloaded); err != nil {
		t.Fatalf("save enabled key: %v", err)
	}
	saved, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "disabled:") {
		t.Fatalf("stale disabled flag kept after enabling:\n%s", saved)
	}
}
