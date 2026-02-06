package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

const (
	// Cache TTLs based on granularity
	metaAnalyticsCacheHalfHourTTL = 1 * time.Hour
	metaAnalyticsCacheDayTTL      = 3 * time.Hour
	metaAnalyticsCacheMonthTTL    = 6 * time.Hour

	// Cache key prefix
	metaAnalyticsCachePrefix = "meta:analytics:"
)

// MetaAnalyticsRequest represents the request parameters for Meta analytics
type MetaAnalyticsRequest struct {
	AccountID     string `json:"account_id"`      // Optional: specific account ID or empty for all
	AnalyticsType string `json:"analytics_type"`  // Required: analytics, pricing_analytics, template_analytics, call_analytics
	Start         string `json:"start"`           // Required: YYYY-MM-DD format
	End           string `json:"end"`             // Required: YYYY-MM-DD format
	Granularity   string `json:"granularity"`     // Optional: HALF_HOUR, DAY, MONTH (default: DAY)
}

// MetaAnalyticsResponse represents the response for Meta analytics
type MetaAnalyticsResponse struct {
	AccountID    string                          `json:"account_id"`
	AccountName  string                          `json:"account_name"`
	Data         *whatsapp.MetaAnalyticsResponse `json:"data"`
	TemplateNames map[string]string              `json:"template_names,omitempty"` // meta_template_id -> template name
}

// GetMetaAnalytics fetches Meta WhatsApp analytics with Redis caching
func (a *App) GetMetaAnalytics(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check permission
	if !a.HasPermission(userID, "analytics", "read", orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Permission denied", nil, "")
	}

	// Parse request parameters
	accountID := string(r.RequestCtx.QueryArgs().Peek("account_id"))
	analyticsType := string(r.RequestCtx.QueryArgs().Peek("analytics_type"))
	startStr := string(r.RequestCtx.QueryArgs().Peek("start"))
	endStr := string(r.RequestCtx.QueryArgs().Peek("end"))
	granularity := string(r.RequestCtx.QueryArgs().Peek("granularity"))

	// Validate required parameters
	if analyticsType == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "analytics_type is required", nil, "")
	}
	if !whatsapp.ValidateAnalyticsType(analyticsType) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid analytics_type. Must be one of: analytics, pricing_analytics, template_analytics, call_analytics", nil, "")
	}
	if startStr == "" || endStr == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "start and end dates are required (YYYY-MM-DD format)", nil, "")
	}

	// Parse dates
	startDate, endDate, errMsg := parseDateRange(startStr, endStr)
	if errMsg != "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, errMsg, nil, "")
	}

	// Validate date range
	if endDate.Before(startDate) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "End date must be after start date", nil, "")
	}

	// Set default granularity (use DAY as standard input, will be normalized per endpoint)
	if granularity == "" {
		granularity = "DAY"
	}
	if !whatsapp.ValidateGranularity(granularity) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid granularity. Must be one of: HALF_HOUR, DAY, MONTH", nil, "")
	}

	// Auto-adjust granularity based on date range to avoid Meta API errors
	daysDiff := int(endDate.Sub(startDate).Hours() / 24)
	originalGranularity := granularity

	// MONTHLY requires at least 30 days
	if (granularity == "MONTH" || granularity == "MONTHLY") && daysDiff < 30 {
		granularity = "DAY"
		a.Log.Debug("Auto-adjusted granularity from MONTH to DAY due to small date range",
			"days", daysDiff,
			"original", originalGranularity,
		)
	}

	// HALF_HOUR only makes sense for ranges up to 7 days (too much data otherwise)
	if granularity == "HALF_HOUR" && daysDiff > 7 {
		granularity = "DAY"
		a.Log.Debug("Auto-adjusted granularity from HALF_HOUR to DAY due to large date range",
			"days", daysDiff,
			"original", originalGranularity,
		)
	}

	// Template analytics have a 90-day lookback limit
	if analyticsType == string(whatsapp.AnalyticsTypeTemplate) {
		ninetyDaysAgo := time.Now().AddDate(0, 0, -90)
		if startDate.Before(ninetyDaysAgo) {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Template analytics have a 90-day lookback limit", nil, "")
		}
	}

	// Convert dates to Unix timestamps
	startUnix := startDate.Unix()
	endUnix := endDate.Add(24*time.Hour - time.Second).Unix() // End of day

	// Get accounts to query
	var accounts []models.WhatsAppAccount
	if accountID != "" {
		// Specific account
		var account models.WhatsAppAccount
		if err := a.DB.Where("id = ? AND organization_id = ?", accountID, orgID).First(&account).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Account not found", nil, "")
		}
		accounts = append(accounts, account)
	} else {
		// All accounts for the organization
		if err := a.DB.Where("organization_id = ?", orgID).Find(&accounts).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to fetch accounts", nil, "")
		}
	}

	if len(accounts) == 0 {
		return r.SendEnvelope(map[string]interface{}{
			"accounts": []MetaAnalyticsResponse{},
			"message":  "No WhatsApp accounts found",
		})
	}

	// Build cache key
	cacheKey := a.buildMetaAnalyticsCacheKey(orgID, accountID, analyticsType, startUnix, endUnix, granularity)

	// Try cache first
	ctx := context.Background()
	cached, err := a.Redis.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var cachedResponse []MetaAnalyticsResponse
		if err := json.Unmarshal([]byte(cached), &cachedResponse); err == nil {
			a.Log.Debug("Meta analytics cache hit", "cache_key", cacheKey)
			return r.SendEnvelope(map[string]interface{}{
				"accounts": cachedResponse,
				"cached":   true,
			})
		}
	}

	// Cache miss - fetch from Meta API
	a.Log.Debug("Meta analytics cache miss", "cache_key", cacheKey)

	var results []MetaAnalyticsResponse
	for _, account := range accounts {
		waAccount := a.toWhatsAppAccount(&account)

		req := &whatsapp.AnalyticsRequest{
			Start:       startUnix,
			End:         endUnix,
			Granularity: granularity,
		}

		// Get template IDs if this is template analytics (not template_group_analytics)
		// Note: template_group_analytics requires template_group_ids which are different from template IDs
		if analyticsType == string(whatsapp.AnalyticsTypeTemplate) {
			// Check if template_ids provided in query params
			templateIDsStr := string(r.RequestCtx.QueryArgs().Peek("template_ids"))
			if templateIDsStr != "" {
				var templateIDs []string
				if err := json.Unmarshal([]byte(templateIDsStr), &templateIDs); err == nil {
					req.TemplateIDs = templateIDs
				}
			}

			// If no template IDs provided, fetch from database
			if len(req.TemplateIDs) == 0 {
				var templates []models.Template
				if err := a.DB.Select("meta_template_id").
					Where("organization_id = ? AND whats_app_account = ? AND meta_template_id != '' AND meta_template_id IS NOT NULL",
						orgID, account.Name).
					Find(&templates).Error; err == nil {
					for _, t := range templates {
						if t.MetaTemplateID != "" {
							req.TemplateIDs = append(req.TemplateIDs, t.MetaTemplateID)
						}
					}
				}
				a.Log.Debug("Auto-fetched template IDs for analytics",
					"account_name", account.Name,
					"template_count", len(req.TemplateIDs),
				)
			}

			// Skip if no templates found
			if len(req.TemplateIDs) == 0 {
				a.Log.Debug("No templates found for account, skipping template analytics",
					"account_id", account.ID,
					"account_name", account.Name,
				)
				results = append(results, MetaAnalyticsResponse{
					AccountID:   account.ID.String(),
					AccountName: account.Name,
					Data:        nil,
				})
				continue
			}
		}

		data, err := a.WhatsApp.GetAnalytics(ctx, waAccount, whatsapp.AnalyticsType(analyticsType), req)
		if err != nil {
			a.Log.Error("Failed to fetch Meta analytics",
				"error", err,
				"account_id", account.ID,
				"account_name", account.Name,
				"analytics_type", analyticsType,
				"start", startUnix,
				"end", endUnix,
				"granularity", granularity,
			)
			// Continue with other accounts
			results = append(results, MetaAnalyticsResponse{
				AccountID:   account.ID.String(),
				AccountName: account.Name,
				Data:        nil,
			})
			continue
		}

		// Log successful fetch with data point count
		dataPointCount := 0
		if data != nil {
			switch analyticsType {
			case string(whatsapp.AnalyticsTypeMessaging):
				if data.Analytics != nil {
					dataPointCount = len(data.Analytics.DataPoints)
				}
			case string(whatsapp.AnalyticsTypePricing):
				if data.PricingAnalytics != nil {
					dataPointCount = len(data.PricingAnalytics.DataPoints)
				}
			case string(whatsapp.AnalyticsTypeTemplate):
				if data.TemplateAnalytics != nil {
					dataPointCount = len(data.TemplateAnalytics.DataPoints)
				}
			case string(whatsapp.AnalyticsTypeCall):
				if data.CallAnalytics != nil {
					dataPointCount = len(data.CallAnalytics.DataPoints)
				}
			}
		}
		a.Log.Debug("Meta analytics fetched successfully",
			"account_id", account.ID,
			"account_name", account.Name,
			"analytics_type", analyticsType,
			"data_points", dataPointCount,
		)

		// Build template names map if this is template analytics
		var templateNames map[string]string
		if analyticsType == string(whatsapp.AnalyticsTypeTemplate) && data != nil {
			templateNames = make(map[string]string)

			// Get unique template IDs from the analytics data using a map for deduplication
			templateIDSet := make(map[string]struct{})
			if data.TemplateAnalytics != nil {
				for _, dp := range data.TemplateAnalytics.DataPoints {
					if dp.TemplateID != "" {
						templateIDSet[dp.TemplateID] = struct{}{}
					}
				}
			}

			// Convert set to slice
			templateIDs := make([]string, 0, len(templateIDSet))
			for id := range templateIDSet {
				templateIDs = append(templateIDs, id)
			}

			// Fetch template names from database
			if len(templateIDs) > 0 {
				var templates []models.Template
				if err := a.DB.Select("meta_template_id, name, display_name").
					Where("organization_id = ? AND meta_template_id IN ?", orgID, templateIDs).
					Find(&templates).Error; err == nil {
					for _, t := range templates {
						name := t.DisplayName
						if name == "" {
							name = t.Name
						}
						templateNames[t.MetaTemplateID] = name
					}
				}
			}
		}

		results = append(results, MetaAnalyticsResponse{
			AccountID:     account.ID.String(),
			AccountName:   account.Name,
			Data:          data,
			TemplateNames: templateNames,
		})
	}

	// Cache the results
	cacheTTL := a.getMetaAnalyticsCacheTTL(granularity)
	if cacheData, err := json.Marshal(results); err == nil {
		a.Redis.Set(ctx, cacheKey, cacheData, cacheTTL)
	}

	response := map[string]interface{}{
		"accounts": results,
		"cached":   false,
	}

	// Include adjusted granularity if it was changed
	if granularity != originalGranularity {
		response["adjusted_granularity"] = granularity
		response["original_granularity"] = originalGranularity
	}

	return r.SendEnvelope(response)
}

// ListMetaAccountsForAnalytics lists WhatsApp accounts available for analytics
func (a *App) ListMetaAccountsForAnalytics(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check permission
	if !a.HasPermission(userID, "analytics", "read", orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Permission denied", nil, "")
	}

	type AccountInfo struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		PhoneID string `json:"phone_id"`
	}

	var accounts []models.WhatsAppAccount
	if err := a.DB.Select("id, name, phone_id").Where("organization_id = ?", orgID).Find(&accounts).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to fetch accounts", nil, "")
	}

	result := make([]AccountInfo, 0, len(accounts))
	for _, acc := range accounts {
		result = append(result, AccountInfo{
			ID:      acc.ID.String(),
			Name:    acc.Name,
			PhoneID: acc.PhoneID,
		})
	}

	return r.SendEnvelope(map[string]interface{}{
		"accounts": result,
	})
}

// RefreshMetaAnalyticsCache invalidates the cache for Meta analytics
func (a *App) RefreshMetaAnalyticsCache(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check permission
	if !a.HasPermission(userID, "analytics", "write", orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Permission denied", nil, "")
	}

	// Delete all cached analytics for this organization
	ctx := context.Background()
	pattern := fmt.Sprintf("%s%s:*", metaAnalyticsCachePrefix, orgID.String())
	a.deleteKeysByPattern(ctx, pattern)

	return r.SendEnvelope(map[string]interface{}{
		"message": "Analytics cache cleared successfully",
	})
}

// buildMetaAnalyticsCacheKey builds a cache key for Meta analytics
func (a *App) buildMetaAnalyticsCacheKey(orgID uuid.UUID, accountID, analyticsType string, start, end int64, granularity string) string {
	if accountID == "" {
		accountID = "all"
	}
	return fmt.Sprintf("%s%s:%s:%s:%d:%d:%s",
		metaAnalyticsCachePrefix,
		orgID.String(),
		accountID,
		analyticsType,
		start,
		end,
		granularity,
	)
}

// getMetaAnalyticsCacheTTL returns the appropriate cache TTL based on granularity
func (a *App) getMetaAnalyticsCacheTTL(granularity string) time.Duration {
	switch granularity {
	case "HALF_HOUR":
		return metaAnalyticsCacheHalfHourTTL
	case "DAY":
		return metaAnalyticsCacheDayTTL
	case "MONTH":
		return metaAnalyticsCacheMonthTTL
	default:
		return metaAnalyticsCacheDayTTL
	}
}
