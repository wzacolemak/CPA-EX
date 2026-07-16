package management

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// Version the browser cookie so stale cookies created before the shared
	// /v0/ scope cannot shadow a newly issued embed capability.
	usageKeeperEmbedCookie = "cpa_management_embed_v2"
	usageKeeperEmbedTTL    = 8 * time.Hour
	usageKeeperBaseURL     = "http://cpa-usage-keeper:8080/v0/usage-keeper"
	modelTesterBaseURL     = "http://cpa-model-tester:8090"
)

// CreateUsageKeeperEmbedSession creates a browser-scoped capability after the
// normal management middleware has authenticated the operator. The capability
// is intentionally short lived and never exposes either management or Keeper
// credentials to the browser.
func (h *Handler) CreateUsageKeeperEmbedSession(c *gin.Context) {
	h.createEmbedSession(c, "/v0/usage-keeper/?embed=cpamc&token=")
}

// CreateModelTesterEmbedSession uses the same CPA-authenticated capability
// model as the Usage Keeper dashboard.
func (h *Handler) CreateModelTesterEmbedSession(c *gin.Context) {
	h.createEmbedSession(c, "/v0/model-tester/?token=")
}

func (h *Handler) createEmbedSession(c *gin.Context, prefix string) {
	if h == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to create usage keeper session"})
		return
	}
	token := hex.EncodeToString(buf)
	now := time.Now()
	h.usageKeeperEmbedMu.Lock()
	for value, expiresAt := range h.usageKeeperEmbedTokens {
		if !expiresAt.After(now) {
			delete(h.usageKeeperEmbedTokens, value)
		}
	}
	h.usageKeeperEmbedTokens[token] = now.Add(usageKeeperEmbedTTL)
	h.usageKeeperEmbedMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"url": prefix + token})
}

// ServeUsageKeeperEmbed proxies the embedded dashboard after validating the
// capability issued above. It deliberately sits outside the management group:
// iframe navigations cannot attach an Authorization header.
func (h *Handler) ServeUsageKeeperEmbed(c *gin.Context) {
	cookie, _ := c.Cookie(usageKeeperEmbedCookie)
	if h == nil || !h.validUsageKeeperEmbedToken(c.Query("token"), cookie) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "usage keeper embed session required"})
		return
	}
	if token := c.Query("token"); token != "" {
		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie(usageKeeperEmbedCookie, token, int(usageKeeperEmbedTTL.Seconds()), "/v0/", "", false, true)
		query := c.Request.URL.Query()
		query.Del("token")
		cleanURL := c.Request.URL.Path
		if encoded := query.Encode(); encoded != "" {
			cleanURL += "?" + encoded
		}
		c.Redirect(http.StatusFound, cleanURL)
		return
	}

	targetURL, err := h.usageKeeperTargetURL()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "usage keeper is not configured"})
		return
	}
	h.serveEmbeddedProxy(c, targetURL, true)
}

// ServeModelTesterEmbed exposes the tester only behind the same short-lived
// CPA management capability. The tester itself remains private to cpa-net.
func (h *Handler) ServeModelTesterEmbed(c *gin.Context) {
	cookie, _ := c.Cookie(usageKeeperEmbedCookie)
	if h == nil || !h.validUsageKeeperEmbedToken(c.Query("token"), cookie) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "model tester embed session required"})
		return
	}
	if token := c.Query("token"); token != "" {
		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie(usageKeeperEmbedCookie, token, int(usageKeeperEmbedTTL.Seconds()), "/v0/", "", false, true)
		c.Redirect(http.StatusFound, "/v0/model-tester/")
		return
	}
	targetURL, _ := url.Parse(modelTesterBaseURL)
	h.serveEmbeddedProxy(c, targetURL, false)
}

func (h *Handler) serveEmbeddedProxy(c *gin.Context, targetURL *url.URL, preservePrefix bool) {
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		if preservePrefix {
			// Keeper is configured with APP_BASE_PATH=/v0/usage-keeper.
			req.URL.Path = c.Request.URL.Path
		} else {
			req.URL.Path = strings.TrimPrefix(c.Request.URL.Path, "/v0/model-tester")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}
		req.URL.RawPath = ""
		req.Host = targetURL.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"usage keeper is unavailable"}`))
	}
	proxy.ServeHTTP(c.Writer, c.Request)
	c.Abort()
}

func (h *Handler) validUsageKeeperEmbedToken(values ...string) bool {
	now := time.Now()
	h.usageKeeperEmbedMu.Lock()
	defer h.usageKeeperEmbedMu.Unlock()
	for value, expiresAt := range h.usageKeeperEmbedTokens {
		if !expiresAt.After(now) {
			delete(h.usageKeeperEmbedTokens, value)
		}
	}
	for _, value := range values {
		if value != "" && h.usageKeeperEmbedTokens[value].After(now) {
			return true
		}
	}
	return false
}

func (h *Handler) usageKeeperTargetURL() (*url.URL, error) {
	baseURL := usageKeeperBaseURL
	if h != nil && h.cfg != nil && strings.TrimSpace(h.cfg.DailyCostQuota.KeeperBaseURL) != "" {
		baseURL = strings.TrimSpace(h.cfg.DailyCostQuota.KeeperBaseURL)
	}
	targetURL, err := url.Parse(baseURL)
	if err != nil || (targetURL.Scheme != "http" && targetURL.Scheme != "https") || targetURL.Host == "" {
		if err == nil {
			err = fmt.Errorf("invalid Usage Keeper URL")
		}
		return nil, err
	}
	return targetURL, nil
}
