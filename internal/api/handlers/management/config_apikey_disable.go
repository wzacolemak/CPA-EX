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
	return false, nil
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
