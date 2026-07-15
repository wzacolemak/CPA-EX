package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

type failOnReadBody struct{ reads int }

func (b *failOnReadBody) Read([]byte) (int, error) {
	b.reads++
	return 0, errors.New("body must not be read")
}

func (b *failOnReadBody) Close() error { return nil }

func TestModelACLUnrestrictedKeySkipsBodyInspection(t *testing.T) {
	cfg := &config.Config{SDKConfig: config.SDKConfig{
		APIKeyPolicies: []config.APIKeyPolicy{{Key: "restricted", AllowedModels: []string{"gpt-*"}}},
	}}
	body := &failOnReadBody{}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", body)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Set("userApiKey", "unrestricted")

	ModelACLMiddleware(func() *config.Config { return cfg })(ctx)
	if body.reads != 0 {
		t.Fatalf("unrestricted request body was read %d times", body.reads)
	}
	if ctx.IsAborted() {
		t.Fatal("unrestricted request was unexpectedly aborted")
	}
}

func TestModelACLCapComesFromConfig(t *testing.T) {
	cfg := &config.Config{SDKConfig: config.SDKConfig{ModelACLMaxBodySizeMB: 32}}
	if got, want := modelACLBodyCap(cfg), int64(32*1024*1024); got != want {
		t.Fatalf("body cap = %d, want %d", got, want)
	}
	cfg.ModelACLMaxBodySizeMB = 1
	if got, want := modelACLBodyCap(cfg), int64(1024*1024); got != want {
		t.Fatalf("hot-reloaded body cap = %d, want %d", got, want)
	}
}

func TestModelACLRestrictedKeyUsesConfiguredCap(t *testing.T) {
	cfg := &config.Config{SDKConfig: config.SDKConfig{
		ModelACLMaxBodySizeMB: 1,
		APIKeyPolicies:        []config.APIKeyPolicy{{Key: "restricted", AllowedModels: []string{"gpt-*"}}},
	}}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req.ContentLength = 2 * 1024 * 1024
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Set("userApiKey", "restricted")

	ModelACLMiddleware(func() *config.Config { return cfg })(ctx)
	if got, want := rec.Code, http.StatusRequestEntityTooLarge; got != want {
		t.Fatalf("status = %d, want %d; body=%s", got, want, rec.Body.String())
	}
}
