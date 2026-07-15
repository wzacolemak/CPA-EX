package management

import (
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

// GetDailyCostQuota returns the editable daily-cost-quota settings together
// with the currently configured client API keys. The Keeper password itself is
// deliberately never returned or accepted by this API.
func (h *Handler) GetDailyCostQuota(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusOK, gin.H{"daily-cost-quota": config.DailyCostQuotaConfig{}})
		return
	}
	labels := make(map[string]string)
	for _, policy := range h.cfg.APIKeyPolicies {
		if key := strings.TrimSpace(policy.Key); key != "" {
			labels[key] = strings.TrimSpace(policy.Name)
		}
	}
	quota := h.cfg.DailyCostQuota
	quota.KeeperPassword = ""
	c.JSON(http.StatusOK, gin.H{
		"daily-cost-quota": quota,
		"api-keys":         append([]string(nil), h.cfg.APIKeys...),
		"api-key-labels":   labels,
	})
}

// PutDailyCostQuota replaces the editable daily-cost-quota settings. It keeps
// the optional direct Keeper password unchanged so a browser never needs to
// receive or submit that secret.
func (h *Handler) PutDailyCostQuota(c *gin.Context) {
	var quota config.DailyCostQuotaConfig
	if err := c.ShouldBindJSON(&quota); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid daily cost quota configuration"})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if err := validateDailyCostQuota(quota, h.cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	quota.KeeperBaseURL = strings.TrimSpace(quota.KeeperBaseURL)
	quota.KeeperPasswordEnv = strings.TrimSpace(quota.KeeperPasswordEnv)
	for i := range quota.Limits {
		quota.Limits[i].Key = strings.TrimSpace(quota.Limits[i].Key)
	}
	quota.KeeperPassword = h.cfg.DailyCostQuota.KeeperPassword
	if quota.KeeperPasswordEnv == "" {
		quota.KeeperPasswordEnv = h.cfg.DailyCostQuota.KeeperPasswordEnv
	}
	h.cfg.DailyCostQuota = quota
	h.persistLocked(c)
}

func validateDailyCostQuota(quota config.DailyCostQuotaConfig, cfg *config.Config) error {
	if quota.RefreshSeconds < 0 || quota.RefreshSeconds > 3600 {
		return errDailyCostQuota("refresh-seconds must be between 0 and 3600")
	}
	knownKeys := make(map[string]struct{})
	if cfg != nil {
		for _, key := range cfg.APIKeys {
			if key = strings.TrimSpace(key); key != "" {
				knownKeys[key] = struct{}{}
			}
		}
	}
	seen := make(map[string]struct{}, len(quota.Limits))
	for _, item := range quota.Limits {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			return errDailyCostQuota("each quota limit requires an API key")
		}
		if _, ok := knownKeys[key]; !ok {
			return errDailyCostQuota("quota limit references an API key not configured in api-keys")
		}
		if _, duplicate := seen[key]; duplicate {
			return errDailyCostQuota("each API key may have only one quota limit")
		}
		seen[key] = struct{}{}
		if item.Limit < 0 || math.IsNaN(item.Limit) || math.IsInf(item.Limit, 0) {
			return errDailyCostQuota("quota limit must be a non-negative finite number")
		}
	}
	return nil
}

type errDailyCostQuota string

func (e errDailyCostQuota) Error() string { return string(e) }
