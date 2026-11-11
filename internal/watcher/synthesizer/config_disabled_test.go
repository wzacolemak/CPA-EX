package synthesizer

import (
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
)

func newDisabledTestContext(cfg *config.Config) *SynthesisContext {
	return &SynthesisContext{
		Config:      cfg,
		Now:         time.Now(),
		IDGenerator: NewStableIDGenerator(),
	}
}

// disabledAuthsByProvider indexes synthesized auths so each case can assert on the
// entry it disabled without depending on synthesis order.
func disabledAuthsByProvider(auths []*coreauth.Auth) map[string][]*coreauth.Auth {
	out := make(map[string][]*coreauth.Auth, len(auths))
	for _, auth := range auths {
		out[auth.Provider] = append(out[auth.Provider], auth)
	}
	return out
}

// A disabled config API key stays configured and listed, but out of rotation.
func TestConfigSynthesizer_DisabledKeysStayListed(t *testing.T) {
	cfg := &config.Config{
		ClaudeKey: []config.ClaudeKey{
			{APIKey: "claude-disabled", BaseURL: "https://api.anthropic.com", Disabled: true},
			{APIKey: "claude-active", BaseURL: "https://api.anthropic.com"},
		},
		GeminiKey: []config.GeminiKey{
			{APIKey: "gemini-disabled", Disabled: true},
			{APIKey: "gemini-active"},
		},
		CodexKey: []config.CodexKey{
			{APIKey: "codex-disabled", Disabled: true},
		},
		XAIKey: []config.XAIKey{
			{APIKey: "xai-disabled", Disabled: true},
		},
		MetaKey: []config.MetaKey{
			{APIKey: "meta-disabled", Disabled: true},
		},
		VertexCompatAPIKey: []config.VertexCompatKey{
			{APIKey: "vertex-disabled", Disabled: true},
		},
		OpenAICompatibility: []config.OpenAICompatibility{
			{
				Name:    "compat",
				BaseURL: "https://compat.example.com/v1",
				APIKeyEntries: []config.OpenAICompatibilityAPIKey{
					{APIKey: "compat-disabled", Disabled: true},
				},
			},
		},
	}

	auths, err := NewConfigSynthesizer().Synthesize(newDisabledTestContext(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byProvider := disabledAuthsByProvider(auths)
	counts := map[string]int{
		"claude": 2,
		"gemini": 2,
		"codex":  1,
		"xai":    1,
		"meta":   1,
		"vertex": 1,
	}
	for provider, want := range counts {
		if got := len(byProvider[provider]); got != want {
			t.Fatalf("%s auths = %d, want %d", provider, got, want)
		}
	}
	assertDisabled := func(provider, apiKey string, want bool) {
		t.Helper()
		for _, auth := range auths {
			if auth.Attributes["api_key"] != apiKey {
				continue
			}
			if auth.Disabled != want {
				t.Fatalf("%s/%s Disabled = %v, want %v", provider, apiKey, auth.Disabled, want)
			}
			status := auth.Status
			if want && status != coreauth.StatusDisabled {
				t.Fatalf("%s/%s status = %v, want disabled", provider, apiKey, status)
			}
			if !want && status != coreauth.StatusActive {
				t.Fatalf("%s/%s status = %v, want active", provider, apiKey, status)
			}
			return
		}
		t.Fatalf("%s/%s not synthesized", provider, apiKey)
	}

	assertDisabled("claude", "claude-disabled", true)
	assertDisabled("claude", "claude-active", false)
	assertDisabled("gemini", "gemini-disabled", true)
	assertDisabled("gemini", "gemini-active", false)
	assertDisabled("codex", "codex-disabled", true)
	assertDisabled("xai", "xai-disabled", true)
	assertDisabled("meta", "meta-disabled", true)
	assertDisabled("vertex", "vertex-disabled", true)
	assertDisabled("compat", "compat-disabled", true)
}
