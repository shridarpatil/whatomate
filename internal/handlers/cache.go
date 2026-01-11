package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/gorm"
)

const (
	// Cache TTLs - 6 hours since these rarely change (invalidated on update anyway)
	settingsCacheTTL        = 6 * time.Hour
	flowsCacheTTL           = 6 * time.Hour
	keywordRulesCacheTTL    = 6 * time.Hour
	whatsappAccountCacheTTL = 6 * time.Hour
	webhooksCacheTTL        = 6 * time.Hour
	slaSettingsCacheTTL     = 6 * time.Hour
	aiContextsCacheTTL      = 6 * time.Hour

	// Cache key prefixes
	settingsCachePrefix        = "chatbot:settings:"
	flowsCachePrefix           = "chatbot:flows:"
	keywordRulesCachePrefix    = "chatbot:keywords:"
	whatsappAccountCachePrefix = "whatsapp:account:"
	webhooksCachePrefix        = "webhooks:"
	slaSettingsCacheKey        = "chatbot:sla_enabled_settings"
	aiContextsCachePrefix      = "chatbot:ai_contexts:"
)

// getChatbotSettingsCached retrieves chatbot settings from cache or database
func (a *App) getChatbotSettingsCached(orgID uuid.UUID, whatsAppAccount string) (*models.ChatbotSettings, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("%s%s:%s", settingsCachePrefix, orgID.String(), whatsAppAccount)

	// Try cache first
	cached, err := a.Redis.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var settings models.ChatbotSettings
		if err := json.Unmarshal([]byte(cached), &settings); err == nil {
			return &settings, nil
		}
	}

	// Cache miss - fetch from database
	var settings models.ChatbotSettings
	result := a.DB.Where("organization_id = ? AND (whats_app_account = ? OR whats_app_account = '')",
		orgID, whatsAppAccount).
		Order("CASE WHEN whats_app_account = '' THEN 1 ELSE 0 END"). // Prefer account-specific settings
		First(&settings)

	if result.Error != nil {
		return nil, result.Error
	}

	// Cache the result
	if data, err := json.Marshal(settings); err == nil {
		a.Redis.Set(ctx, cacheKey, data, settingsCacheTTL)
	}

	return &settings, nil
}

// getChatbotFlowsCached retrieves all enabled flows with steps from cache or database
func (a *App) getChatbotFlowsCached(orgID uuid.UUID) ([]models.ChatbotFlow, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("%s%s", flowsCachePrefix, orgID.String())

	// Try cache first
	cached, err := a.Redis.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var flows []models.ChatbotFlow
		if err := json.Unmarshal([]byte(cached), &flows); err == nil {
			return flows, nil
		}
	}

	// Cache miss - fetch from database
	var flows []models.ChatbotFlow
	if err := a.DB.Where("organization_id = ? AND is_enabled = true", orgID).
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_order ASC")
		}).
		Find(&flows).Error; err != nil {
		return nil, err
	}

	// Cache the result
	if data, err := json.Marshal(flows); err == nil {
		a.Redis.Set(ctx, cacheKey, data, flowsCacheTTL)
	}

	return flows, nil
}

// getChatbotFlowByIDCached retrieves a specific flow by ID from the cached flows list
func (a *App) getChatbotFlowByIDCached(orgID uuid.UUID, flowID uuid.UUID) (*models.ChatbotFlow, error) {
	flows, err := a.getChatbotFlowsCached(orgID)
	if err != nil {
		return nil, err
	}

	for i := range flows {
		if flows[i].ID == flowID {
			return &flows[i], nil
		}
	}

	return nil, gorm.ErrRecordNotFound
}

// getKeywordRulesCached retrieves keyword rules from cache or database
func (a *App) getKeywordRulesCached(orgID uuid.UUID, whatsAppAccount string) ([]models.KeywordRule, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("%s%s:%s", keywordRulesCachePrefix, orgID.String(), whatsAppAccount)

	// Try cache first
	cached, err := a.Redis.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var rules []models.KeywordRule
		if err := json.Unmarshal([]byte(cached), &rules); err == nil {
			return rules, nil
		}
	}

	// Cache miss - fetch from database (account-specific + global)
	var rules []models.KeywordRule

	// Get account-specific rules
	var accountRules []models.KeywordRule
	if err := a.DB.Where("organization_id = ? AND whats_app_account = ? AND is_enabled = true",
		orgID, whatsAppAccount).
		Order("priority DESC").
		Find(&accountRules).Error; err != nil {
		a.Log.Error("Failed to fetch account keyword rules", "error", err, "org_id", orgID)
	}

	// Get global rules (whats_app_account = '')
	var globalRules []models.KeywordRule
	if err := a.DB.Where("organization_id = ? AND whats_app_account = '' AND is_enabled = true",
		orgID).
		Order("priority DESC").
		Find(&globalRules).Error; err != nil {
		a.Log.Error("Failed to fetch global keyword rules", "error", err, "org_id", orgID)
	}

	// Merge: account-specific first, then global
	rules = append(accountRules, globalRules...)

	// Cache the result
	if data, err := json.Marshal(rules); err == nil {
		a.Redis.Set(ctx, cacheKey, data, keywordRulesCacheTTL)
	}

	return rules, nil
}

// InvalidateChatbotSettingsCache invalidates the settings cache for an organization
func (a *App) InvalidateChatbotSettingsCache(orgID uuid.UUID) {
	ctx := context.Background()
	pattern := fmt.Sprintf("%s%s:*", settingsCachePrefix, orgID.String())
	a.deleteKeysByPattern(ctx, pattern)
}

// InvalidateChatbotFlowsCache invalidates the flows cache for an organization
func (a *App) InvalidateChatbotFlowsCache(orgID uuid.UUID) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("%s%s", flowsCachePrefix, orgID.String())
	a.Redis.Del(ctx, cacheKey)
}

// InvalidateKeywordRulesCache invalidates the keyword rules cache for an organization
func (a *App) InvalidateKeywordRulesCache(orgID uuid.UUID) {
	ctx := context.Background()
	pattern := fmt.Sprintf("%s%s:*", keywordRulesCachePrefix, orgID.String())
	a.deleteKeysByPattern(ctx, pattern)
}

// deleteKeysByPattern deletes all keys matching a pattern
func (a *App) deleteKeysByPattern(ctx context.Context, pattern string) {
	iter := a.Redis.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		a.Redis.Del(ctx, iter.Val())
	}
}

// whatsAppAccountCache is used for caching since AccessToken has json:"-" tag
type whatsAppAccountCache struct {
	models.WhatsAppAccount
	AccessToken string `json:"access_token"`
}

// getWhatsAppAccountCached retrieves WhatsApp account by phone_id from cache or database
func (a *App) getWhatsAppAccountCached(phoneID string) (*models.WhatsAppAccount, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("%s%s", whatsappAccountCachePrefix, phoneID)

	// Try cache first
	cached, err := a.Redis.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var cacheData whatsAppAccountCache
		if err := json.Unmarshal([]byte(cached), &cacheData); err == nil {
			cacheData.WhatsAppAccount.AccessToken = cacheData.AccessToken
			return &cacheData.WhatsAppAccount, nil
		}
	}

	// Cache miss - fetch from database
	var account models.WhatsAppAccount
	if err := a.DB.Where("phone_id = ?", phoneID).First(&account).Error; err != nil {
		return nil, err
	}

	// Cache the result (include AccessToken explicitly)
	cacheData := whatsAppAccountCache{
		WhatsAppAccount: account,
		AccessToken:     account.AccessToken,
	}
	if data, err := json.Marshal(cacheData); err == nil {
		a.Redis.Set(ctx, cacheKey, data, whatsappAccountCacheTTL)
	}

	return &account, nil
}

// InvalidateWhatsAppAccountCache invalidates the WhatsApp account cache
func (a *App) InvalidateWhatsAppAccountCache(phoneID string) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("%s%s", whatsappAccountCachePrefix, phoneID)
	a.Redis.Del(ctx, cacheKey)
}

// getWebhooksCached retrieves active webhooks for an organization from cache or database
func (a *App) getWebhooksCached(orgID uuid.UUID) ([]models.Webhook, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("%s%s", webhooksCachePrefix, orgID.String())

	// Try cache first
	cached, err := a.Redis.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var webhooks []models.Webhook
		if err := json.Unmarshal([]byte(cached), &webhooks); err == nil {
			return webhooks, nil
		}
	}

	// Cache miss - fetch from database
	var webhooks []models.Webhook
	if err := a.DB.Where("organization_id = ? AND is_active = ?", orgID, true).Find(&webhooks).Error; err != nil {
		return nil, err
	}

	// Cache the result
	if data, err := json.Marshal(webhooks); err == nil {
		a.Redis.Set(ctx, cacheKey, data, webhooksCacheTTL)
	}

	return webhooks, nil
}

// InvalidateWebhooksCache invalidates the webhooks cache for an organization
func (a *App) InvalidateWebhooksCache(orgID uuid.UUID) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("%s%s", webhooksCachePrefix, orgID.String())
	a.Redis.Del(ctx, cacheKey)
}

// getSLAEnabledSettingsCached retrieves all SLA-enabled chatbot settings from cache or database
func (a *App) getSLAEnabledSettingsCached() ([]models.ChatbotSettings, error) {
	ctx := context.Background()

	// Try cache first
	cached, err := a.Redis.Get(ctx, slaSettingsCacheKey).Result()
	if err == nil && cached != "" {
		var settings []models.ChatbotSettings
		if err := json.Unmarshal([]byte(cached), &settings); err == nil {
			return settings, nil
		}
	}

	// Cache miss - fetch from database
	var settings []models.ChatbotSettings
	if err := a.DB.Where("sla_enabled = ?", true).Find(&settings).Error; err != nil {
		return nil, err
	}

	// Cache the result
	if data, err := json.Marshal(settings); err == nil {
		a.Redis.Set(ctx, slaSettingsCacheKey, data, slaSettingsCacheTTL)
	}

	return settings, nil
}

// InvalidateSLASettingsCache invalidates the SLA settings cache
func (a *App) InvalidateSLASettingsCache() {
	ctx := context.Background()
	a.Redis.Del(ctx, slaSettingsCacheKey)
}

// getAIContextsCached retrieves AI contexts from cache or database
func (a *App) getAIContextsCached(orgID uuid.UUID, whatsAppAccount string) ([]models.AIContext, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("%s%s:%s", aiContextsCachePrefix, orgID.String(), whatsAppAccount)

	// Try cache first
	cached, err := a.Redis.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var contexts []models.AIContext
		if err := json.Unmarshal([]byte(cached), &contexts); err == nil {
			return contexts, nil
		}
	}

	// Cache miss - fetch from database (account-specific + global)
	var contexts []models.AIContext

	// Get account-specific contexts
	var accountContexts []models.AIContext
	if err := a.DB.Where("organization_id = ? AND whats_app_account = ? AND is_enabled = true",
		orgID, whatsAppAccount).
		Order("priority DESC").
		Find(&accountContexts).Error; err != nil {
		a.Log.Error("Failed to fetch account AI contexts", "error", err, "org_id", orgID)
	}

	// Get global contexts (whats_app_account = '')
	var globalContexts []models.AIContext
	if err := a.DB.Where("organization_id = ? AND whats_app_account = '' AND is_enabled = true",
		orgID).
		Order("priority DESC").
		Find(&globalContexts).Error; err != nil {
		a.Log.Error("Failed to fetch global AI contexts", "error", err, "org_id", orgID)
	}

	// Merge: account-specific first, then global
	contexts = append(accountContexts, globalContexts...)

	// Cache the result
	if data, err := json.Marshal(contexts); err == nil {
		a.Redis.Set(ctx, cacheKey, data, aiContextsCacheTTL)
	}

	return contexts, nil
}

// InvalidateAIContextsCache invalidates the AI contexts cache for an organization
func (a *App) InvalidateAIContextsCache(orgID uuid.UUID) {
	ctx := context.Background()
	pattern := fmt.Sprintf("%s%s:*", aiContextsCachePrefix, orgID.String())
	a.deleteKeysByPattern(ctx, pattern)
}
