package calling

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/utils"
)

const (
	ivrFlowCachePrefix    = "ivr_flow:"
	ivrFlowCfgCachePrefix = "ivr_flow:cfg:"
	orgSettingsCachePrefix = "org:calling_settings:"
	callingCacheTTL        = 6 * time.Hour
)

// getIVRFlowCached returns an IVR flow by ID, reading from Redis first.
func (m *Manager) getIVRFlowCached(flowID uuid.UUID) *models.IVRFlow {
	ctx := context.Background()
	key := ivrFlowCachePrefix + flowID.String()

	// Try cache
	if cached, err := m.redis.Get(ctx, key).Result(); err == nil && cached != "" {
		var flow models.IVRFlow
		if json.Unmarshal([]byte(cached), &flow) == nil {
			return &flow
		}
	}

	// Cache miss — DB
	var flow models.IVRFlow
	if err := m.db.First(&flow, flowID).Error; err != nil {
		return nil
	}

	// Store
	if data, err := json.Marshal(flow); err == nil {
		m.redis.Set(ctx, key, string(data), callingCacheTTL)
	}
	return &flow
}

// getIVRFlowByConfigCached finds the IVR flow for a given org+account+config flag.
// configType is "call_start" or "outgoing_end".
func (m *Manager) getIVRFlowByConfigCached(orgID uuid.UUID, accountName, configType string) *models.IVRFlow {
	ctx := context.Background()
	key := fmt.Sprintf("%s%s:%s:%s", ivrFlowCfgCachePrefix, orgID.String(), accountName, configType)

	// Try cache
	if cached, err := m.redis.Get(ctx, key).Result(); err == nil {
		if cached == "null" {
			return nil // cached miss
		}
		var flow models.IVRFlow
		if json.Unmarshal([]byte(cached), &flow) == nil {
			return &flow
		}
	}

	// Cache miss — DB
	var flow models.IVRFlow
	var query string
	switch configType {
	case "call_start":
		query = "organization_id = ? AND whatsapp_account = ? AND is_call_start = ? AND is_active = ? AND deleted_at IS NULL"
	case "outgoing_end":
		query = "organization_id = ? AND whatsapp_account = ? AND is_outgoing_end = ? AND is_active = ?"
	default:
		return nil
	}

	if err := m.db.Where(query, orgID, accountName, true, true).First(&flow).Error; err != nil {
		// Cache the miss so we don't hit DB again
		m.redis.Set(ctx, key, "null", callingCacheTTL)
		return nil
	}

	if data, err := json.Marshal(flow); err == nil {
		m.redis.Set(ctx, key, string(data), callingCacheTTL)
	}
	return &flow
}

// cachedOrgSettings is the JSON-serializable subset of org settings used for calling.
type cachedOrgSettings struct {
	TransferTimeoutSecs int               `json:"transfer_timeout_secs,omitempty"`
	HoldMusicFile       string            `json:"hold_music_file,omitempty"`
	RingbackFile        string            `json:"ringback_file,omitempty"`
	Settings            map[string]any    `json:"settings,omitempty"`
}

// getOrgCallingSettingsCached loads org calling overrides with Redis caching.
func (m *Manager) getOrgCallingSettingsCached(orgID uuid.UUID) orgCallingSettings {
	s := orgCallingSettings{
		TransferTimeoutSecs: m.config.TransferTimeoutSecs,
		HoldMusicFile:       m.config.AudioDir + "/" + m.config.HoldMusicFile,
		RingbackFile:        "",
	}
	if m.config.RingbackFile != "" {
		s.RingbackFile = m.config.AudioDir + "/" + m.config.RingbackFile
	}

	ctx := context.Background()
	key := orgSettingsCachePrefix + orgID.String()

	// Try cache
	if cached, err := m.redis.Get(ctx, key).Result(); err == nil && cached != "" {
		var cos cachedOrgSettings
		if json.Unmarshal([]byte(cached), &cos) == nil {
			m.applyOrgOverrides(&s, cos.Settings)
			return s
		}
	}

	// Cache miss — DB (select only the settings JSONB field)
	var org models.Organization
	if err := m.db.Select("id, settings").Where("id = ?", orgID).First(&org).Error; err != nil || org.Settings == nil {
		// Cache empty settings so we don't query again
		m.redis.Set(ctx, key, "{}", callingCacheTTL)
		return s
	}

	cos := cachedOrgSettings{Settings: org.Settings}
	if data, err := json.Marshal(cos); err == nil {
		m.redis.Set(ctx, key, string(data), callingCacheTTL)
	}

	m.applyOrgOverrides(&s, org.Settings)
	return s
}

// applyOrgOverrides applies org-level JSONB overrides to calling settings.
func (m *Manager) applyOrgOverrides(s *orgCallingSettings, settings map[string]any) {
	if settings == nil {
		return
	}
	if v, ok := settings["mask_phone_numbers"].(bool); ok {
		s.MaskPhoneNumbers = v
	}
	if v, ok := settings["transfer_timeout_secs"].(float64); ok && v > 0 {
		s.TransferTimeoutSecs = int(v)
	}
	if v, ok := settings["hold_music_file"].(string); ok && v != "" {
		s.HoldMusicFile = m.config.AudioDir + "/" + v
	}
	if v, ok := settings["ringback_file"].(string); ok && v != "" {
		s.RingbackFile = m.config.AudioDir + "/" + v
	}
}

// GetIVRFlowByConfig is the public wrapper for getIVRFlowByConfigCached,
// used by handlers that need cached config-flag lookups (e.g., call_webhook).
func (m *Manager) GetIVRFlowByConfig(orgID uuid.UUID, accountName, configType string) *models.IVRFlow {
	return m.getIVRFlowByConfigCached(orgID, accountName, configType)
}

// InvalidateIVRFlowCache removes cached IVR flow data. Called from IVR flow CRUD handlers.
func (m *Manager) InvalidateIVRFlowCache(flowID uuid.UUID, orgID uuid.UUID, accountName string) {
	ctx := context.Background()
	// Invalidate by-ID cache
	m.redis.Del(ctx, ivrFlowCachePrefix+flowID.String())
	// Invalidate config-based caches for this org+account
	m.redis.Del(ctx, fmt.Sprintf("%s%s:%s:call_start", ivrFlowCfgCachePrefix, orgID.String(), accountName))
	m.redis.Del(ctx, fmt.Sprintf("%s%s:%s:outgoing_end", ivrFlowCfgCachePrefix, orgID.String(), accountName))
}

// InvalidateOrgCallingSettingsCache removes cached org calling settings.
func (m *Manager) InvalidateOrgCallingSettingsCache(orgID uuid.UUID) {
	ctx := context.Background()
	m.redis.Del(ctx, orgSettingsCachePrefix+orgID.String())
}

// maybeMaskPhone masks a phone number if masking is enabled for the org.
// Reuses the cached org settings (no extra DB query).
func (m *Manager) maybeMaskPhone(orgID uuid.UUID, phone string) string {
	settings := m.getOrgCallingSettingsCached(orgID)
	if !settings.MaskPhoneNumbers {
		return phone
	}
	return utils.MaskPhoneNumber(phone)
}
