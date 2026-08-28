package management

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestGetOpenAICompatIncludesDisableCooling(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")

	requestRetry := 0
	disableCooling := true
	h := NewHandlerWithoutConfigFilePath(&config.Config{
		OpenAICompatibility: []config.OpenAICompatibility{
			{
				Name:    "Mimo CN",
				BaseURL: "https://token-plan-cn.xiaomimimo.com/v1",
				APIKeyEntries: []config.OpenAICompatibilityAPIKey{
					{APIKey: "test-key"},
				},
				Models: []config.OpenAICompatibilityModel{
					{Name: "mimo-v2.5", Alias: ""},
				},
				SupportPromptCacheKey: true,
				DisableCooling:        &disableCooling,
				RequestRetry:          &requestRetry,
			},
		},
	}, nil)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/openai-compatibility", nil)
	h.GetOpenAICompat(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var body struct {
		OpenAICompatibility []struct {
			SupportPromptCacheKey *bool `json:"support-prompt-cache-key"`
			DisableCooling        *bool `json:"disable-cooling"`
			RequestRetry          *int  `json:"request-retry"`
		} `json:"openai-compatibility"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.OpenAICompatibility) != 1 {
		t.Fatalf("expected 1 openai-compatibility entry, got %d", len(body.OpenAICompatibility))
	}
	if body.OpenAICompatibility[0].SupportPromptCacheKey == nil || !*body.OpenAICompatibility[0].SupportPromptCacheKey {
		t.Fatalf("expected support-prompt-cache-key to be present and true, got %#v", body.OpenAICompatibility[0].SupportPromptCacheKey)
	}
	if body.OpenAICompatibility[0].DisableCooling == nil || !*body.OpenAICompatibility[0].DisableCooling {
		t.Fatalf("expected disable-cooling to be present and true, got %#v", body.OpenAICompatibility[0].DisableCooling)
	}
	if body.OpenAICompatibility[0].RequestRetry == nil || *body.OpenAICompatibility[0].RequestRetry != 0 {
		t.Fatalf("expected request-retry to be present and 0, got %#v", body.OpenAICompatibility[0].RequestRetry)
	}
}

func TestGetOpenAICompatKeepsAuthIndexForDisabledProviders(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")

	const (
		baseURL = "https://api.duckcoding.ai/v1"
		apiKey  = "test-key"
	)
	h := NewHandlerWithoutConfigFilePath(&config.Config{
		OpenAICompatibility: []config.OpenAICompatibility{
			{
				Name:     "duck",
				BaseURL:  baseURL,
				Disabled: true,
				APIKeyEntries: []config.OpenAICompatibilityAPIKey{
					{APIKey: apiKey},
				},
			},
			{
				Name:     "nokey",
				BaseURL:  baseURL,
				Disabled: true,
			},
		},
	}, nil)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/openai-compatibility", nil)
	h.GetOpenAICompat(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var body struct {
		OpenAICompatibility []struct {
			Name          string `json:"name"`
			Disabled      bool   `json:"disabled"`
			AuthIndex     string `json:"auth-index"`
			APIKeyEntries []struct {
				AuthIndex string `json:"auth-index"`
			} `json:"api-key-entries"`
		} `json:"openai-compatibility"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.OpenAICompatibility) != 2 {
		t.Fatalf("expected 2 openai-compatibility entries, got %d", len(body.OpenAICompatibility))
	}

	duck := body.OpenAICompatibility[0]
	if !duck.Disabled {
		t.Fatalf("expected duck provider to stay disabled, got %#v", duck)
	}
	if len(duck.APIKeyEntries) != 1 || duck.APIKeyEntries[0].AuthIndex == "" {
		t.Fatalf("expected disabled provider api-key entry to keep auth-index, got %#v", duck.APIKeyEntries)
	}

	// The fallback must match the auth manager index: sha256("openai-compatibility:"+baseURL+"+"+apiKey)[:8] hex.
	seed := "openai-compatibility:" + baseURL + "+" + apiKey
	sum := sha256.Sum256([]byte(seed))
	want := hex.EncodeToString(sum[:8])
	if got := duck.APIKeyEntries[0].AuthIndex; got != want {
		t.Fatalf("auth-index = %q, want %q", got, want)
	}

	noKey := body.OpenAICompatibility[1]
	if noKey.AuthIndex != "" {
		t.Fatalf("expected no auth-index for entry without api key, got %q", noKey.AuthIndex)
	}
}
