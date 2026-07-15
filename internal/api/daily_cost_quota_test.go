package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestDailyCostQuotaMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const clientKey = "quota-test-key"

	keeper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(usageKeeperFetchHeader) != "fetch" {
			t.Fatal("Usage Keeper request header is missing")
		}
		switch r.URL.Path {
		case "/api/v1/auth/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "ok"})
			w.WriteHeader(http.StatusOK)
		case "/api/v1/usage/api-keys/settings":
			if _, err := w.Write([]byte(`{"items":[{"id":"key-id","apiKey":"quota-test-key"}]}`)); err != nil {
				t.Fatal(err)
			}
		case "/api/v1/usage/overview":
			if r.URL.Query().Get("range") != "24h" || r.URL.Query().Get("api_key_id") != "key-id" {
				t.Fatalf("unexpected overview query: %s", r.URL.RawQuery)
			}
			if err := json.NewEncoder(w).Encode(map[string]any{
				"summary": map[string]any{"total_cost": 12.5, "cost_available": true},
			}); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("unexpected Usage Keeper path: %s", r.URL.Path)
		}
	}))
	defer keeper.Close()

	for _, tc := range []struct {
		name       string
		limit      float64
		wantStatus int
	}{
		{name: "below quota proceeds", limit: 13, wantStatus: http.StatusNoContent},
		{name: "at quota is rejected", limit: 12.5, wantStatus: http.StatusTooManyRequests},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{SDKConfig: config.SDKConfig{DailyCostQuota: config.DailyCostQuotaConfig{
				Enabled:        true,
				KeeperBaseURL:  keeper.URL,
				KeeperPassword: "test-password",
				Limits: []config.DailyCostQuotaLimit{{
					Key: clientKey, Limit: tc.limit,
				}},
			}}}
			engine := gin.New()
			engine.Use(func(c *gin.Context) {
				c.Set("userApiKey", clientKey)
			})
			engine.Use(DailyCostQuotaMiddleware(func() *config.Config { return cfg }))
			engine.POST("/v1/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			engine.ServeHTTP(recorder, request)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
		})
	}
}

func TestDailyCostQuotaMiddlewareFailOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{SDKConfig: config.SDKConfig{DailyCostQuota: config.DailyCostQuotaConfig{
		Enabled:        true,
		KeeperBaseURL:  "http://127.0.0.1:1",
		KeeperPassword: "test-password",
		FailOpen:       true,
		Limits:         []config.DailyCostQuotaLimit{{Key: "quota-test-key", Limit: 1}},
	}}}
	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Set("userApiKey", "quota-test-key") })
	engine.Use(DailyCostQuotaMiddleware(func() *config.Config { return cfg }))
	engine.POST("/v1/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/responses", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}
