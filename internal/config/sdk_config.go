// Package config provides configuration management for the CLI Proxy API server.
// It handles loading and parsing YAML configuration files, and provides structured
// access to application settings including server port, authentication directory,
// debug settings, proxy configuration, and API keys.
package config

import (
	"path"
	"strings"
)

const (
	// APIKeyDefaultPolicyAllowAll permits every model for keys without an explicit
	// model policy. It is the backward-compatible default.
	APIKeyDefaultPolicyAllowAll = "allow-all"

	// APIKeyDefaultPolicyDenyAll denies every model for keys without an explicit
	// model policy.
	APIKeyDefaultPolicyDenyAll = "deny-all"
)

// APIKeyPolicy limits a client key to the listed path.Match-style model globs.
type APIKeyPolicy struct {
	Key           string   `yaml:"key" json:"key"`
	Name          string   `yaml:"name,omitempty" json:"name,omitempty"`
	AllowedModels []string `yaml:"allowed-models,omitempty" json:"allowedModels,omitempty"`
}

// DailyCostQuotaConfig controls daily billing limits backed by Usage Keeper's
// calculated rolling-24-hour summary.total_cost for each client API key.
type DailyCostQuotaConfig struct {
	Enabled           bool                  `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	KeeperBaseURL     string                `yaml:"keeper-base-url,omitempty" json:"keeper-base-url,omitempty"`
	KeeperPassword    string                `yaml:"keeper-password,omitempty" json:"-"`
	KeeperPasswordEnv string                `yaml:"keeper-password-env,omitempty" json:"-"`
	RefreshSeconds    int                   `yaml:"refresh-seconds,omitempty" json:"refresh-seconds,omitempty"`
	FailOpen          bool                  `yaml:"fail-open,omitempty" json:"fail-open,omitempty"`
	Limits            []DailyCostQuotaLimit `yaml:"limits,omitempty" json:"limits,omitempty"`
}

// DailyCostQuotaLimit gives one client key a rolling-24-hour maximum amount in
// the same currency/unit used by Usage Keeper's summary.total_cost.
type DailyCostQuotaLimit struct {
	Key   string  `yaml:"key" json:"key"`
	Limit float64 `yaml:"limit" json:"limit"`
}

// SDKConfig represents the application's configuration, loaded from a YAML file.
type SDKConfig struct {
	// ProxyURL is the URL of an optional proxy server to use for outbound requests.
	ProxyURL string `yaml:"proxy-url" json:"proxy-url"`

	// DisableImageGeneration controls whether the built-in image_generation tool is injected/allowed.
	//
	// Supported values:
	//   - false (default): image_generation is enabled everywhere (normal behavior).
	//   - true: image_generation is disabled everywhere. The server stops injecting it, removes it from request payloads,
	//     and returns 404 for /v1/images/generations and /v1/images/edits.
	//   - "chat": disable image_generation injection for all non-images endpoints (e.g. /v1/responses, /v1/chat/completions),
	//     while keeping /v1/images/generations and /v1/images/edits enabled and preserving image_generation there.
	//   - "passthrough": do not modify the tool list on non-images endpoints — keep image_generation if the client
	//     sent it and do not inject it otherwise; on /v1/images/generations and /v1/images/edits behave like "chat".
	DisableImageGeneration DisableImageGenerationMode `yaml:"disable-image-generation" json:"disable-image-generation"`

	// GPTImage2BaseModel sets the base (mainline) model used by the legacy hosted
	// image_generation tool path when a Codex image request is not proxied directly
	// through the Image API.
	//
	// The value must start with "gpt-" (case-insensitive). If empty or invalid, the
	// default base model ("gpt-5.4-mini") is used.
	GPTImage2BaseModel string `yaml:"gpt-image-2-base-model,omitempty" json:"gpt-image-2-base-model,omitempty"`

	// VideoResultAuthCacheTTL controls how long video IDs stay pinned to the credential
	// that created them. Accepts duration strings like "30m" or "3h".
	// Empty or invalid values use the default 3h.
	VideoResultAuthCacheTTL string `yaml:"video-result-auth-cache-ttl,omitempty" json:"video-result-auth-cache-ttl,omitempty"`

	// ForceModelPrefix requires explicit model prefixes (e.g., "teamA/gemini-3-pro-preview")
	// to target prefixed credentials. When false, unprefixed model requests may use prefixed
	// credentials as well.
	ForceModelPrefix bool `yaml:"force-model-prefix" json:"force-model-prefix"`

	// RequestLog enables or disables detailed request logging functionality.
	RequestLog bool `yaml:"request-log" json:"request-log"`

	// CodexOptimizeMultiAgentV2 mirrors the provider-wide runtime setting for API handlers.
	CodexOptimizeMultiAgentV2 bool `yaml:"-" json:"-"`

	// CodexOrphanDelegationCompatibility mirrors the provider-wide runtime setting for API handlers.
	CodexOrphanDelegationCompatibility bool `yaml:"-" json:"-"`

	// ClaudeCode configures Claude Code compatibility behavior.
	ClaudeCode ClaudeCodeConfig `yaml:"claude-code" json:"claude-code"`

	// APIKeys is a list of keys for authenticating clients to this proxy server.
	APIKeys []string `yaml:"api-keys" json:"api-keys"`

	// APIKeyPolicies optionally limits individual client keys to model globs.
	APIKeyPolicies []APIKeyPolicy `yaml:"api-key-policies,omitempty" json:"api-key-policies,omitempty"`

	// APIKeyDefaultPolicy controls keys without a non-empty policy. Empty and
	// "allow-all" are equivalent; "deny-all" requires an explicit allowlist.
	APIKeyDefaultPolicy string `yaml:"api-key-default-policy,omitempty" json:"api-key-default-policy,omitempty"`

	// ModelACLMaxBodySizeMB limits the body buffered to identify a model for a
	// restricted key. Zero uses the secure 10 MiB default.
	ModelACLMaxBodySizeMB int `yaml:"model-acl-max-body-size-mb,omitempty" json:"model-acl-max-body-size-mb,omitempty"`

	// DailyCostQuota enables per-client-key rolling-24-hour limits using CPA
	// Usage Keeper cost data.
	DailyCostQuota DailyCostQuotaConfig `yaml:"daily-cost-quota,omitempty" json:"daily-cost-quota,omitempty"`

	// PassthroughHeaders controls whether upstream response headers are forwarded to downstream clients.
	// Default is false (disabled).
	PassthroughHeaders bool `yaml:"passthrough-headers" json:"passthrough-headers"`

	// Streaming configures server-side streaming behavior (keep-alives and safe bootstrap retries).
	Streaming StreamingConfig `yaml:"streaming" json:"streaming"`

	// NonStreamKeepAliveInterval controls how often blank lines are emitted for non-streaming responses.
	// <= 0 disables keep-alives. Value is in seconds.
	NonStreamKeepAliveInterval int `yaml:"nonstream-keepalive-interval,omitempty" json:"nonstream-keepalive-interval,omitempty"`
}

// IsModelAllowedForKey reports whether key may access model according to its
// configured allowlist and default policy.
func (c *SDKConfig) IsModelAllowedForKey(key, model string) bool {
	if c == nil {
		return true
	}
	policy, hasPolicy := c.APIKeyPolicyForKey(key)
	if !hasPolicy || len(policy.AllowedModels) == 0 {
		return !strings.EqualFold(strings.TrimSpace(c.APIKeyDefaultPolicy), APIKeyDefaultPolicyDenyAll)
	}

	candidate := model
	if idx := strings.Index(candidate, "/"); idx >= 0 && idx < len(candidate)-1 {
		candidate = candidate[idx+1:]
	}
	for _, pattern := range policy.AllowedModels {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if pattern == candidate {
			return true
		}
		if matched, err := path.Match(pattern, candidate); err == nil && matched {
			return true
		}
		if matched, err := path.Match(pattern, model); err == nil && matched {
			return true
		}
	}
	return false
}

// APIKeyPolicyForKey returns the configured policy for key.
func (c *SDKConfig) APIKeyPolicyForKey(key string) (APIKeyPolicy, bool) {
	if c == nil {
		return APIKeyPolicy{}, false
	}
	key = strings.TrimSpace(key)
	for _, policy := range c.APIKeyPolicies {
		if strings.TrimSpace(policy.Key) == key {
			return policy, true
		}
	}
	return APIKeyPolicy{}, false
}

// ClaudeCodeConfig configures Claude Code compatibility behavior.
type ClaudeCodeConfig struct {
	// DisableCloakingModelList disables model ID cloaking in Anthropic model list responses.
	DisableCloakingModelList bool `yaml:"disable-cloaking-model-list" json:"disable-cloaking-model-list"`
}

// StreamingConfig holds server streaming behavior configuration.
type StreamingConfig struct {
	// KeepAliveSeconds controls how often the server emits SSE heartbeats (": keep-alive\n\n")
	// or WebSocket Ping control frames.
	// <= 0 disables keep-alives. Default is 0.
	KeepAliveSeconds int `yaml:"keepalive-seconds,omitempty" json:"keepalive-seconds,omitempty"`

	// BootstrapRetries controls how many times the server may retry a streaming request before any bytes are sent,
	// to allow auth rotation / transient recovery.
	// <= 0 disables bootstrap retries. Default is 0.
	BootstrapRetries int `yaml:"bootstrap-retries,omitempty" json:"bootstrap-retries,omitempty"`
}
