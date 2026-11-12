package management

import (
	"fmt"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/watcher/synthesizer"
	coreauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
)

// configAPIKeyLegacyDisablePattern is the "*" entry earlier versions appended to
// excluded-models to disable a config API key. Toggling a key now migrates that
// state onto the entry's own disabled flag.
const configAPIKeyLegacyDisablePattern = "*"

// withoutLegacyDisablePattern drops a bare "*" left behind by the previous
// excluded-models based disable mechanism. Real model IDs are preserved.
func withoutLegacyDisablePattern(models []string) []string {
	filtered := make([]string, 0, len(models))
	for _, item := range models {
		if strings.TrimSpace(item) == configAPIKeyLegacyDisablePattern {
			continue
		}
		filtered = append(filtered, item)
	}
	return config.NormalizeExcludedModels(filtered)
}

// applyConfigAPIKeyDisabled flips one config API key entry between in-service and
// disabled. Disabled entries stay in the config file, so settings can show them and
// an operator can switch them back on without retyping the key.
func applyConfigAPIKeyDisabled(entryDisabled *bool, entryExcluded *[]string, disabled bool) {
	*entryDisabled = disabled
	*entryExcluded = withoutLegacyDisablePattern(*entryExcluded)
}

// toggleConfigAPIKeyDisabled finds the config entry behind a synthesized API key auth
// and toggles its disabled flag. It returns false when no entry matches the auth ID.
func toggleConfigAPIKeyDisabled(cfg *config.Config, auth *coreauth.Auth, disable bool) (bool, error) {
	if cfg == nil || auth == nil || !coreauth.IsConfigAPIKeyAuth(auth) {
		return false, nil
	}
	authID := strings.TrimSpace(auth.ID)
	if authID == "" {
		return false, fmt.Errorf("auth id is empty")
	}

	idGen := synthesizer.NewStableIDGenerator()

	for i := range cfg.GeminiKey {
		entry := &cfg.GeminiKey[i]
		key := strings.TrimSpace(entry.APIKey)
		base := strings.TrimSpace(entry.BaseURL)
		proxyURL := strings.TrimSpace(entry.ProxyURL)
		prefix := strings.TrimSpace(entry.Prefix)
		if key == "" && base == "" {
			continue
		}
		idParts := []string{key, base, proxyURL, prefix, config.FormatSortedHeaders(entry.Headers)}
		if poolID := config.FormatConnectionPoolDedupID(entry.ConnectionPool); poolID != "" {
			idParts = append(idParts, poolID)
		}
		id, _ := idGen.Next("gemini:apikey", idParts...)
		if id == authID {
			applyConfigAPIKeyDisabled(&entry.Disabled, &entry.ExcludedModels, disable)
			return true, nil
		}
	}
	for i := range cfg.InteractionsKey {
		entry := &cfg.InteractionsKey[i]
		key := strings.TrimSpace(entry.APIKey)
		base := strings.TrimSpace(entry.BaseURL)
		proxyURL := strings.TrimSpace(entry.ProxyURL)
		prefix := strings.TrimSpace(entry.Prefix)
		if key == "" && base == "" {
			continue
		}
		idParts := []string{key, base, proxyURL, prefix, config.FormatSortedHeaders(entry.Headers)}
		if poolID := config.FormatConnectionPoolDedupID(entry.ConnectionPool); poolID != "" {
			idParts = append(idParts, poolID)
		}
		id, _ := idGen.Next("gemini-interactions:apikey", idParts...)
		if id == authID {
			applyConfigAPIKeyDisabled(&entry.Disabled, &entry.ExcludedModels, disable)
			return true, nil
		}
	}
	for i := range cfg.ClaudeKey {
		entry := &cfg.ClaudeKey[i]
		key := strings.TrimSpace(entry.APIKey)
		base := strings.TrimSpace(entry.BaseURL)
		proxyURL := strings.TrimSpace(entry.ProxyURL)
		prefix := strings.TrimSpace(entry.Prefix)
		if key == "" && base == "" {
			continue
		}
		idParts := []string{key, base, proxyURL, prefix, config.FormatSortedHeaders(entry.Headers)}
		if poolID := config.FormatConnectionPoolDedupID(entry.ConnectionPool); poolID != "" {
			idParts = append(idParts, poolID)
		}
		id, _ := idGen.Next("claude:apikey", idParts...)
		if id == authID {
			applyConfigAPIKeyDisabled(&entry.Disabled, &entry.ExcludedModels, disable)
			return true, nil
		}
	}
	for i := range cfg.CodexKey {
		entry := &cfg.CodexKey[i]
		key := strings.TrimSpace(entry.APIKey)
		base := strings.TrimSpace(entry.BaseURL)
		proxyURL := strings.TrimSpace(entry.ProxyURL)
		prefix := strings.TrimSpace(entry.Prefix)
		if key == "" && base == "" {
			continue
		}
		idParts := []string{key, base, proxyURL, prefix, config.FormatSortedHeaders(entry.Headers)}
		if poolID := config.FormatConnectionPoolDedupID(entry.ConnectionPool); poolID != "" {
			idParts = append(idParts, poolID)
		}
		id, _ := idGen.Next("codex:apikey", idParts...)
		if id == authID {
			applyConfigAPIKeyDisabled(&entry.Disabled, &entry.ExcludedModels, disable)
			return true, nil
		}
	}
	for i := range cfg.XAIKey {
		entry := &cfg.XAIKey[i]
		key := strings.TrimSpace(entry.APIKey)
		base := strings.TrimSpace(entry.BaseURL)
		proxyURL := strings.TrimSpace(entry.ProxyURL)
		prefix := strings.TrimSpace(entry.Prefix)
		if key == "" && base == "" {
			continue
		}
		idParts := []string{key, base, proxyURL, prefix, config.FormatSortedHeaders(entry.Headers)}
		if poolID := config.FormatConnectionPoolDedupID(entry.ConnectionPool); poolID != "" {
			idParts = append(idParts, poolID)
		}
		id, _ := idGen.Next("xai:apikey", idParts...)
		if id == authID {
			applyConfigAPIKeyDisabled(&entry.Disabled, &entry.ExcludedModels, disable)
			return true, nil
		}
	}
	for i := range cfg.MetaKey {
		entry := &cfg.MetaKey[i]
		key := strings.TrimSpace(entry.APIKey)
		base := strings.TrimSpace(entry.BaseURL)
		proxyURL := strings.TrimSpace(entry.ProxyURL)
		prefix := strings.TrimSpace(entry.Prefix)
		if key == "" && base == "" {
			continue
		}
		idParts := []string{key, base, proxyURL, prefix, config.FormatSortedHeaders(entry.Headers)}
		if poolID := config.FormatConnectionPoolDedupID(entry.ConnectionPool); poolID != "" {
			idParts = append(idParts, poolID)
		}
		id, _ := idGen.Next("meta:apikey", idParts...)
		if id == authID {
			applyConfigAPIKeyDisabled(&entry.Disabled, &entry.ExcludedModels, disable)
			return true, nil
		}
	}
	for i := range cfg.VertexCompatAPIKey {
		entry := &cfg.VertexCompatAPIKey[i]
		key := strings.TrimSpace(entry.APIKey)
		base := strings.TrimSpace(entry.BaseURL)
		proxy := strings.TrimSpace(entry.ProxyURL)
		idParts := []string{key, base, proxy}
		if poolID := config.FormatConnectionPoolDedupID(entry.ConnectionPool); poolID != "" {
			idParts = append(idParts, poolID)
		}
		id, _ := idGen.Next("vertex:apikey", idParts...)
		if id == authID {
			applyConfigAPIKeyDisabled(&entry.Disabled, &entry.ExcludedModels, disable)
			return true, nil
		}
	}
	if toggleOpenAICompatibilityKeyDisabled(cfg, idGen, authID, disable) {
		return true, nil
	}
	return false, nil
}

// openAICompatPoolForID mirrors the synthesizer's per-key pool resolution so this
// package can rebuild an entry's auth ID. It must stay in step with
// synthesizer.openAICompatConnectionPool, which owns the real routing behavior.
func openAICompatPoolForID(compat *config.OpenAICompatibility, entry *config.OpenAICompatibilityAPIKey) *config.ProxyConnectionPoolConfig {
	var group *config.ProxyConnectionPoolConfig
	if compat != nil {
		group = compat.ConnectionPool
	}
	if entry == nil || entry.ConnectionPool == nil {
		return group
	}
	merged := *entry.ConnectionPool
	if group != nil {
		if merged.Enabled == nil {
			merged.Enabled = group.Enabled
		}
		if strings.TrimSpace(merged.IdleConnTimeout) == "" {
			merged.IdleConnTimeout = group.IdleConnTimeout
		}
		if merged.MaxIdleConnsPerHost == nil {
			merged.MaxIdleConnsPerHost = group.MaxIdleConnsPerHost
		}
	}
	return &merged
}

// toggleOpenAICompatibilityKeyDisabled toggles one key entry inside an
// openai-compatibility provider. Those keys are synthesized as config API key
// auths too, so without this branch the settings panel could list them but
// never switch one off. Only a key entry carries the flag: the key-less
// fallback auth of a provider is not an API key auth at all, so it never
// reaches this function.
func toggleOpenAICompatibilityKeyDisabled(cfg *config.Config, idGen *synthesizer.StableIDGenerator, authID string, disable bool) bool {
	for i := range cfg.OpenAICompatibility {
		compat := &cfg.OpenAICompatibility[i]
		providerName := strings.ToLower(strings.TrimSpace(compat.Name))
		if providerName == "" {
			providerName = "openai-compatibility"
		}
		idKind := "openai-compatibility:" + providerName
		base := strings.TrimSpace(compat.BaseURL)

		for j := range compat.APIKeyEntries {
			entry := &compat.APIKeyEntries[j]
			effectivePool := openAICompatPoolForID(compat, entry)
			idParts := []string{strings.TrimSpace(entry.APIKey), base, strings.TrimSpace(entry.ProxyURL)}
			if poolID := config.FormatConnectionPoolDedupID(effectivePool); poolID != "" {
				idParts = append(idParts, poolID)
			}
			id, _ := idGen.Next(idKind, idParts...)
			if id == authID {
				entry.Disabled = disable
				return true
			}
		}
	}
	return false
}

// configAPIKeyEntryID rebuilds the id the synthesizer assigns to a config API key
// entry so a management call can address it. parts must match the synthesizer's own
// Next call for that provider exactly; entries with neither key nor base URL are
// skipped there, so they never match here.
func configAPIKeyEntryID(idGen *synthesizer.StableIDGenerator, kind, apiKey, baseURL string, parts ...string) string {
	key := strings.TrimSpace(apiKey)
	base := strings.TrimSpace(baseURL)
	if key == "" && base == "" {
		return ""
	}
	id, _ := idGen.Next(kind, append([]string{key, base}, parts...)...)
	return id
}
