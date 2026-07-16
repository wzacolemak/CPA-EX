package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

const (
	defaultUsageKeeperURL      = "http://cpa-usage-keeper:8080"
	defaultQuotaRefreshSeconds = 15
	usageKeeperFetchHeader     = "X-CPA-Usage-Keeper-Request"
)

// DailyCostQuotaMiddleware rejects billable client requests once Usage Keeper
// reports that the calling key has reached its configured rolling 24-hour cost
// limit. Usage is recorded after a request completes, so this is a
// post-accounting cutoff: already in-flight requests can cause a small
// concurrent overshoot.
func DailyCostQuotaMiddleware(cfgFn func() *config.Config) gin.HandlerFunc {
	checker := &dailyCostQuotaChecker{}
	return func(c *gin.Context) {
		if !isBillableRequest(c.Request) {
			c.Next()
			return
		}
		cfg := cfgFn()
		if cfg == nil || !cfg.DailyCostQuota.Enabled {
			c.Next()
			return
		}
		rawKey, exists := c.Get("userApiKey")
		apiKey, ok := rawKey.(string)
		if !exists || !ok || strings.TrimSpace(apiKey) == "" {
			c.Next()
			return
		}

		limited, used, limit, err := checker.check(c.Request.Context(), cfg.DailyCostQuota, apiKey)
		if err != nil {
			if cfg.DailyCostQuota.FailOpen {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
				"type":    "usage_quota_unavailable",
				"message": "daily cost quota could not be verified",
			}})
			return
		}
		if limited && used >= limit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": gin.H{
				"type":    "daily_cost_quota_exceeded",
				"message": "daily cost quota exhausted for this API key",
				"used":    used,
				"limit":   limit,
			}})
			return
		}
		c.Next()
	}
}

func isBillableRequest(req *http.Request) bool {
	if req == nil {
		return false
	}
	switch req.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

type dailyCostQuotaChecker struct {
	mu       sync.Mutex
	client   *http.Client
	baseURL  string
	password string
	keyIDs   map[string]string
	costs    map[string]dailyCostCacheEntry
}

type dailyCostCacheEntry struct {
	used      float64
	refreshed time.Time
}

type usageKeeperSettingsResponse struct {
	Items []struct {
		ID     string `json:"id"`
		APIKey string `json:"apiKey"`
	} `json:"items"`
}

type usageKeeperOverviewResponse struct {
	Summary struct {
		TotalCost     float64 `json:"total_cost"`
		CostAvailable bool    `json:"cost_available"`
	} `json:"summary"`
}

func (q *dailyCostQuotaChecker) check(ctx context.Context, quota config.DailyCostQuotaConfig, apiKey string) (bool, float64, float64, error) {
	limit, configured := quotaLimitForKey(quota, apiKey)
	if !configured {
		return false, 0, 0, nil
	}

	refresh := time.Duration(quota.RefreshSeconds) * time.Second
	if refresh <= 0 {
		refresh = defaultQuotaRefreshSeconds * time.Second
	}
	baseURL := strings.TrimRight(strings.TrimSpace(quota.KeeperBaseURL), "/")
	if baseURL == "" {
		baseURL = defaultUsageKeeperURL
	}
	password := strings.TrimSpace(quota.KeeperPassword)
	if password == "" && quota.KeeperPasswordEnv != "" {
		password = strings.TrimSpace(os.Getenv(quota.KeeperPasswordEnv))
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.resetIfConfigChanged(baseURL, password)
	if cached, ok := q.costs[apiKey]; ok && time.Since(cached.refreshed) < refresh {
		return true, cached.used, limit, nil
	}

	// A Keeper deployed behind CPA's embedded management proxy has no separate
	// login. Retain the optional password flow for compatible external Keepers.
	if password != "" {
		if err := q.login(ctx); err != nil {
			return true, 0, limit, err
		}
	}
	keyID, err := q.keyID(ctx, apiKey)
	if err != nil {
		return true, 0, limit, err
	}
	used, err := q.rolling24HourCost(ctx, keyID)
	if err != nil {
		return true, 0, limit, err
	}
	q.costs[apiKey] = dailyCostCacheEntry{used: used, refreshed: time.Now()}
	return true, used, limit, nil
}

func quotaLimitForKey(quota config.DailyCostQuotaConfig, apiKey string) (float64, bool) {
	apiKey = strings.TrimSpace(apiKey)
	for _, item := range quota.Limits {
		if strings.TrimSpace(item.Key) == apiKey && item.Limit >= 0 {
			return item.Limit, true
		}
	}
	return 0, false
}

func (q *dailyCostQuotaChecker) resetIfConfigChanged(baseURL, password string) {
	if q.client != nil && q.baseURL == baseURL && q.password == password {
		return
	}
	jar, _ := cookiejar.New(nil)
	q.client = &http.Client{Timeout: 10 * time.Second, Jar: jar}
	q.baseURL = baseURL
	q.password = password
	q.keyIDs = make(map[string]string)
	q.costs = make(map[string]dailyCostCacheEntry)
}

func (q *dailyCostQuotaChecker) login(ctx context.Context) error {
	payload, err := json.Marshal(map[string]string{"password": q.password})
	if err != nil {
		return err
	}
	resp, err := q.request(ctx, http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return responseError(resp)
	}
	return nil
}

func (q *dailyCostQuotaChecker) keyID(ctx context.Context, apiKey string) (string, error) {
	if id := q.keyIDs[apiKey]; id != "" {
		return id, nil
	}
	resp, err := q.request(ctx, http.MethodGet, "/api/v1/usage/api-keys/settings", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		if err := q.login(ctx); err != nil {
			return "", err
		}
		return q.keyID(ctx, apiKey)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", responseError(resp)
	}
	var settings usageKeeperSettingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		return "", err
	}
	for _, item := range settings.Items {
		if item.APIKey != "" && item.ID != "" {
			q.keyIDs[item.APIKey] = item.ID
		}
	}
	if id := q.keyIDs[apiKey]; id != "" {
		return id, nil
	}
	return "", fmt.Errorf("API key is not registered in Usage Keeper")
}

func (q *dailyCostQuotaChecker) rolling24HourCost(ctx context.Context, keyID string) (float64, error) {
	path := "/api/v1/usage/overview?range=24h&api_key_id=" + url.QueryEscape(keyID)
	resp, err := q.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		if err := q.login(ctx); err != nil {
			return 0, err
		}
		return q.rolling24HourCost(ctx, keyID)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, responseError(resp)
	}
	var overview usageKeeperOverviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&overview); err != nil {
		return 0, err
	}
	if !overview.Summary.CostAvailable {
		return 0, fmt.Errorf("Usage Keeper has no cost data for this API key")
	}
	return overview.Summary.TotalCost, nil
}

func (q *dailyCostQuotaChecker) request(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, q.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(usageKeeperFetchHeader, "fetch")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return q.client.Do(req)
}

func responseError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("Usage Keeper returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}
