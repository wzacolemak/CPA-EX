package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/tidwall/gjson"
)

const (
	defaultModelACLMaxBodyBytes int64 = 10 * 1024 * 1024
	maximumModelACLMaxBodyBytes int64 = 256 * 1024 * 1024
	modelACLPeekBytes           int64 = 16 * 1024
)

var errModelACLBodyTooLarge = errors.New("model_acl: request body exceeds cap")

// ModelACLMiddleware enforces per-key model allowlists. It intentionally skips
// unrestricted keys before reading their request bodies: there is no policy to
// enforce, so buffering a potentially large prompt only creates a false limit.
// cfgFn is evaluated for every request so config-file hot reloads apply without
// restarting the process.
func ModelACLMiddleware(cfgFn func() *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := cfgFn()
		if cfg == nil || (len(cfg.APIKeyPolicies) == 0 && !isDefaultDenyAll(cfg)) {
			c.Next()
			return
		}

		rawKey, exists := c.Get("userApiKey")
		apiKey, ok := rawKey.(string)
		if !exists || !ok || strings.TrimSpace(apiKey) == "" {
			c.Next()
			return
		}

		if !keyHasModelRestriction(cfg, apiKey) {
			c.Next()
			return
		}

		if isWebsocketUpgradeRequest(c.Request) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{
				"type":    "websocket_not_allowed_for_restricted_key",
				"message": "model-restricted api keys cannot use websocket upgrade routes; model selection happens in frames the ACL cannot inspect",
			}})
			return
		}

		model, found, err := extractRequestedModel(c, modelACLBodyCap(cfg))
		if errors.Is(err, errModelACLBodyTooLarge) {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{
				"type": "request_too_large", "message": "request body exceeds the model-ACL inspection cap",
			}})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{
				"type": "invalid_request_body", "message": "could not read request body for model ACL enforcement",
			}})
			return
		}
		if !found || cfg.IsModelAllowedForKey(apiKey, model) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{
			"type": "model_not_allowed_for_key", "message": "this api key is not permitted to use the requested model", "model": model,
		}})
	}
}

func modelACLBodyCap(cfg *config.Config) int64 {
	if cfg == nil || cfg.ModelACLMaxBodySizeMB <= 0 {
		return defaultModelACLMaxBodyBytes
	}
	cap := int64(cfg.ModelACLMaxBodySizeMB) * 1024 * 1024
	if cap > maximumModelACLMaxBodyBytes {
		return maximumModelACLMaxBodyBytes
	}
	return cap
}

func isDefaultDenyAll(cfg *config.Config) bool {
	return cfg != nil && strings.EqualFold(strings.TrimSpace(cfg.APIKeyDefaultPolicy), config.APIKeyDefaultPolicyDenyAll)
}

func keyHasModelRestriction(cfg *config.Config, key string) bool {
	if cfg == nil {
		return false
	}
	policy, found := cfg.APIKeyPolicyForKey(key)
	if found {
		return len(policy.AllowedModels) > 0 || isDefaultDenyAll(cfg)
	}
	return isDefaultDenyAll(cfg)
}

func isWebsocketUpgradeRequest(req *http.Request) bool {
	return req != nil && strings.EqualFold(strings.TrimSpace(req.Header.Get("Upgrade")), "websocket")
}

func extractRequestedModel(c *gin.Context, cap int64) (string, bool, error) {
	if c == nil || c.Request == nil {
		return "", false, nil
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1beta/models/") {
		model := strings.TrimPrefix(c.Request.URL.Path, "/v1beta/models/")
		if idx := strings.IndexByte(model, ':'); idx >= 0 {
			model = model[:idx]
		}
		if model = strings.TrimSpace(model); model != "" {
			return model, true, nil
		}
	}
	if c.Request.Method != http.MethodPost && c.Request.Method != http.MethodPut && c.Request.Method != http.MethodPatch {
		return "", false, nil
	}
	return extractModelFromJSONBody(c.Request, cap)
}

func extractModelFromJSONBody(req *http.Request, cap int64) (string, bool, error) {
	if req.Body == nil {
		return "", false, nil
	}
	if req.ContentLength > cap {
		return "", false, errModelACLBodyTooLarge
	}

	peek := make([]byte, modelACLPeekBytes)
	n, readErr := io.ReadFull(req.Body, peek)
	peek = peek[:n]
	fullyRead := readErr == io.EOF || readErr == io.ErrUnexpectedEOF
	if readErr != nil && !fullyRead {
		return "", false, readErr
	}
	if fullyRead {
		req.Body = io.NopCloser(bytes.NewReader(peek))
		model, found := extractModelFromBytes(peek)
		return model, found, nil
	}
	if model, found := extractModelFromBytes(peek); found {
		req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(peek), req.Body))
		return model, true, nil
	}

	remaining := cap - int64(len(peek))
	if remaining <= 0 {
		return "", false, errModelACLBodyTooLarge
	}
	rest, err := io.ReadAll(io.LimitReader(req.Body, remaining+1))
	if err != nil {
		return "", false, err
	}
	if int64(len(rest)) > remaining {
		return "", false, errModelACLBodyTooLarge
	}
	body := append(peek, rest...)
	req.Body = io.NopCloser(bytes.NewReader(body))
	model, found := extractModelFromBytes(body)
	return model, found, nil
}

func extractModelFromBytes(body []byte) (string, bool) {
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	return model, model != ""
}
