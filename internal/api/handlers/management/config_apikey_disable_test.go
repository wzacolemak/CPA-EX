package management

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/watcher/synthesizer"
	coreauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
)

func TestWithoutLegacyDisablePattern(t *testing.T) {
	got := withoutLegacyDisablePattern([]string{"gpt-5", "*"})
	if len(got) != 1 || got[0] != "gpt-5" {
		t.Fatalf("legacy pattern not stripped: %#v", got)
	}
	got = withoutLegacyDisablePattern([]string{"gpt-5"})
	if len(got) != 1 || got[0] != "gpt-5" {
		t.Fatalf("model ids must be preserved: %#v", got)
	}
}

func TestToggleConfigAPIKeyDisabled_XAI(t *testing.T) {
	cfg := &config.Config{
		XAIKey: []config.XAIKey{{
			APIKey:  "xai-test",
			BaseURL: "https://api.x.ai/v1",
		}},
	}
	idGen := synthesizer.NewStableIDGenerator()
	authID, _ := idGen.Next("xai:apikey", "xai-test", "https://api.x.ai/v1", "", "", "")
	auth := &coreauth.Auth{
		ID:       authID,
		Provider: "xai",
		Attributes: map[string]string{
			"api_key":  "xai-test",
			"base_url": "https://api.x.ai/v1",
			"source":   "config:xai[abc]",
		},
	}

	handled, errToggle := toggleConfigAPIKeyDisabled(cfg, auth, true)
	if errToggle != nil || !handled {
		t.Fatalf("toggle disable: handled=%v err=%v", handled, errToggle)
	}
	if !cfg.XAIKey[0].Disabled {
		t.Fatal("xai key should be disabled")
	}
	if len(cfg.XAIKey[0].ExcludedModels) != 0 {
		t.Fatalf("excluded-models must stay untouched, got %#v", cfg.XAIKey[0].ExcludedModels)
	}
}

func TestToggleConfigAPIKeyDisabled_Meta(t *testing.T) {
	cfg := &config.Config{
		MetaKey: []config.MetaKey{{
			APIKey:  "meta-test",
			BaseURL: "https://api.meta.ai/v1",
		}},
	}
	idGen := synthesizer.NewStableIDGenerator()
	authID, _ := idGen.Next("meta:apikey", "meta-test", "https://api.meta.ai/v1", "", "", "")
	auth := &coreauth.Auth{
		ID:       authID,
		Provider: "meta",
		Attributes: map[string]string{
			"api_key":  "meta-test",
			"base_url": "https://api.meta.ai/v1",
			"source":   "config:meta[abc]",
		},
	}

	handled, errToggle := toggleConfigAPIKeyDisabled(cfg, auth, true)
	if errToggle != nil || !handled {
		t.Fatalf("toggle disable: handled=%v err=%v", handled, errToggle)
	}
	if !cfg.MetaKey[0].Disabled {
		t.Fatal("meta key should be disabled")
	}
}

// Codex keeps a real excluded model while disabling, and enabling clears the legacy
// "*" that older versions wrote instead of the disabled flag.
func TestToggleConfigAPIKeyDisabled_Codex(t *testing.T) {
	cfg := &config.Config{
		CodexKey: []config.CodexKey{{
			APIKey:         "sk-test",
			BaseURL:        "https://example.com/v1",
			ExcludedModels: []string{"gpt-4o"},
		}},
	}
	idGen := synthesizer.NewStableIDGenerator()
	authID, _ := idGen.Next("codex:apikey", "sk-test", "https://example.com/v1", "", "", "")
	auth := &coreauth.Auth{
		ID:       authID,
		Provider: "codex",
		Attributes: map[string]string{
			"api_key":  "sk-test",
			"base_url": "https://example.com/v1",
			"source":   "config:codex[abc]",
		},
	}

	handled, err := toggleConfigAPIKeyDisabled(cfg, auth, true)
	if err != nil || !handled {
		t.Fatalf("toggle disable: handled=%v err=%v", handled, err)
	}
	if !cfg.CodexKey[0].Disabled {
		t.Fatal("codex key should be disabled")
	}
	if len(cfg.CodexKey[0].ExcludedModels) != 1 || cfg.CodexKey[0].ExcludedModels[0] != "gpt-4o" {
		t.Fatalf("excluded-models = %#v, want [gpt-4o]", cfg.CodexKey[0].ExcludedModels)
	}

	cfg.CodexKey[0].ExcludedModels = []string{"gpt-4o", "*"}
	handled, err = toggleConfigAPIKeyDisabled(cfg, auth, false)
	if err != nil || !handled {
		t.Fatalf("toggle enable: handled=%v err=%v", handled, err)
	}
	if cfg.CodexKey[0].Disabled {
		t.Fatal("codex key should be enabled")
	}
	if len(cfg.CodexKey[0].ExcludedModels) != 1 || cfg.CodexKey[0].ExcludedModels[0] != "gpt-4o" {
		t.Fatalf("legacy pattern not migrated, excluded-models = %#v", cfg.CodexKey[0].ExcludedModels)
	}
}

func TestToggleConfigAPIKeyDisabled_Vertex_NoBaseURL(t *testing.T) {
	cfg := &config.Config{
		VertexCompatAPIKey: []config.VertexCompatKey{{
			APIKey: "vertex-key-only",
		}},
	}
	idGen := synthesizer.NewStableIDGenerator()
	authID, _ := idGen.Next("vertex:apikey", "vertex-key-only", "", "")
	auth := &coreauth.Auth{
		ID:       authID,
		Provider: "vertex",
		Attributes: map[string]string{
			"auth_kind": "apikey",
			"api_key":   "vertex-key-only",
			"source":    "config:vertex[xyz]",
		},
	}

	handled, errToggle := toggleConfigAPIKeyDisabled(cfg, auth, true)
	if errToggle != nil || !handled {
		t.Fatalf("toggle disable: handled=%v err=%v", handled, errToggle)
	}
	if !cfg.VertexCompatAPIKey[0].Disabled {
		t.Fatal("vertex key should be disabled")
	}
}

func TestToggleConfigAPIKeyDisabled_EmptyKeyWithBaseURL(t *testing.T) {
	cfg := &config.Config{
		ClaudeKey: []config.ClaudeKey{{
			APIKey:  "",
			BaseURL: "https://custom-claude.example.com",
		}},
		GeminiKey: []config.GeminiKey{{
			APIKey:  "   ",
			BaseURL: "https://custom-gemini.example.com",
		}},
	}
	idGen := synthesizer.NewStableIDGenerator()
	claudeID, _ := idGen.Next("claude:apikey", "", "https://custom-claude.example.com", "", "", "")
	geminiID, _ := idGen.Next("gemini:apikey", "", "https://custom-gemini.example.com", "", "", "")

	claudeAuth := &coreauth.Auth{
		ID:       claudeID,
		Provider: "claude",
		Attributes: map[string]string{
			"auth_kind": "apikey",
			"base_url":  "https://custom-claude.example.com",
			"source":    "config:claude[abc]",
		},
	}
	geminiAuth := &coreauth.Auth{
		ID:       geminiID,
		Provider: "gemini",
		Attributes: map[string]string{
			"auth_kind": "apikey",
			"base_url":  "https://custom-gemini.example.com",
			"source":    "config:gemini[def]",
		},
	}

	handled, err := toggleConfigAPIKeyDisabled(cfg, claudeAuth, true)
	if err != nil || !handled {
		t.Fatalf("toggle claude: handled=%v err=%v", handled, err)
	}
	if !cfg.ClaudeKey[0].Disabled {
		t.Fatal("claude key should be disabled")
	}

	handled, err = toggleConfigAPIKeyDisabled(cfg, geminiAuth, true)
	if err != nil || !handled {
		t.Fatalf("toggle gemini: handled=%v err=%v", handled, err)
	}
	if !cfg.GeminiKey[0].Disabled {
		t.Fatal("gemini key should be disabled")
	}
}

// An openai-compatibility key is synthesized as config API key auth too, so the
// settings panel can list it. Toggling must reach that one key entry and leave
// its siblings in the same provider in rotation.
func TestToggleConfigAPIKeyDisabled_OpenAICompatibilityKey(t *testing.T) {
	cfg := &config.Config{
		OpenAICompatibility: []config.OpenAICompatibility{{
			Name:    "opencode_go",
			BaseURL: "https://opencode.ai/zen/go/v1",
			APIKeyEntries: []config.OpenAICompatibilityAPIKey{
				{APIKey: "sk-first"},
				{APIKey: "sk-second"},
			},
		}},
	}
	idGen := synthesizer.NewStableIDGenerator()
	authID, _ := idGen.Next("openai-compatibility:opencode_go", "sk-second", "https://opencode.ai/zen/go/v1", "")
	auth := &coreauth.Auth{
		ID:       authID,
		Provider: "openai-compatibility:opencode_go",
		Attributes: map[string]string{
			"api_key":     "sk-second",
			"base_url":    "https://opencode.ai/zen/go/v1",
			"source":      "config:opencode_go[abc]",
			"compat_name": "opencode_go",
		},
	}

	handled, errToggle := toggleConfigAPIKeyDisabled(cfg, auth, true)
	if errToggle != nil || !handled {
		t.Fatalf("toggle disable: handled=%v err=%v", handled, errToggle)
	}
	if cfg.OpenAICompatibility[0].APIKeyEntries[0].Disabled {
		t.Fatal("sibling key must stay in rotation")
	}
	if !cfg.OpenAICompatibility[0].APIKeyEntries[1].Disabled {
		t.Fatal("target key should be disabled")
	}
	if cfg.OpenAICompatibility[0].Disabled {
		t.Fatal("provider group must stay enabled")
	}

	handled, errToggle = toggleConfigAPIKeyDisabled(cfg, auth, false)
	if errToggle != nil || !handled {
		t.Fatalf("toggle enable: handled=%v err=%v", handled, errToggle)
	}
	if cfg.OpenAICompatibility[0].APIKeyEntries[1].Disabled {
		t.Fatal("target key should be back in rotation")
	}
}
