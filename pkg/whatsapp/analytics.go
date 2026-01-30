package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// AnalyticsRequest represents parameters for fetching analytics from Meta API
type AnalyticsRequest struct {
	Start        int64    `json:"start"`        // Unix timestamp (seconds)
	End          int64    `json:"end"`          // Unix timestamp (seconds)
	Granularity  string   `json:"granularity"`  // "HALF_HOUR", "DAY", "MONTH"
	PhoneNumbers []string `json:"phone_numbers"` // Optional filter by phone numbers
	TemplateIDs  []string `json:"template_ids"`  // Optional filter for template analytics
	CountryCodes []string `json:"country_codes"` // Optional filter by country codes
}

// AnalyticsType represents the type of analytics to fetch
type AnalyticsType string

const (
	AnalyticsTypeMessaging    AnalyticsType = "analytics"
	AnalyticsTypeConversation AnalyticsType = "conversation_analytics"
	AnalyticsTypePricing      AnalyticsType = "pricing_analytics"
	AnalyticsTypeTemplate     AnalyticsType = "template_analytics"
	AnalyticsTypeCall         AnalyticsType = "call_analytics"
)

// MessagingAnalyticsDataPoint represents a single data point for messaging analytics
type MessagingAnalyticsDataPoint struct {
	Start     int64 `json:"start"`
	End       int64 `json:"end"`
	Sent      int64 `json:"sent"`
	Delivered int64 `json:"delivered"`
}

// MessagingAnalyticsEntry represents a single phone number's messaging data
type MessagingAnalyticsEntry struct {
	PhoneNumber string                        `json:"phone_number,omitempty"`
	DataPoints  []MessagingAnalyticsDataPoint `json:"data_points"`
}

// MessagingAnalyticsRaw represents the raw response from Meta API
type MessagingAnalyticsRaw struct {
	Granularity string                    `json:"granularity"`
	Data        []MessagingAnalyticsEntry `json:"data"`
	// Also support direct data_points for backward compatibility
	DataPoints []MessagingAnalyticsDataPoint `json:"data_points,omitempty"`
}

// MessagingAnalytics represents messaging analytics response (flattened)
type MessagingAnalytics struct {
	Granularity string                        `json:"granularity"`
	DataPoints  []MessagingAnalyticsDataPoint `json:"data_points"`
}

// ConversationAnalyticsDataPoint represents a single data point for conversation analytics
type ConversationAnalyticsDataPoint struct {
	Start                 int64   `json:"start"`
	End                   int64   `json:"end"`
	Conversation          int64   `json:"conversation"`
	ConversationType      string  `json:"conversation_type"`      // "REGULAR", "FREE_TIER", "FREE_ENTRY_POINT"
	ConversationDirection string  `json:"conversation_direction"` // "BUSINESS_INITIATED", "USER_INITIATED"
	ConversationCategory  string  `json:"conversation_category"`  // "MARKETING", "UTILITY", "AUTHENTICATION", "SERVICE", "REFERRAL_CONVERSION"
	Cost                  float64 `json:"cost"`
}

// ConversationAnalyticsEntry represents a single phone number's conversation data
type ConversationAnalyticsEntry struct {
	PhoneNumber string                           `json:"phone_number,omitempty"`
	DataPoints  []ConversationAnalyticsDataPoint `json:"data_points"`
}

// ConversationAnalyticsRaw represents the raw response from Meta API
type ConversationAnalyticsRaw struct {
	Granularity string                       `json:"granularity"`
	Data        []ConversationAnalyticsEntry `json:"data"`
	// Also support direct data_points for backward compatibility
	DataPoints []ConversationAnalyticsDataPoint `json:"data_points,omitempty"`
}

// ConversationAnalytics represents conversation analytics response (flattened)
type ConversationAnalytics struct {
	Granularity string                           `json:"granularity"`
	DataPoints  []ConversationAnalyticsDataPoint `json:"data_points"`
}

// PricingAnalyticsDataPoint represents a single data point for pricing analytics
// With dimensions, this includes detailed breakdown by category, type, and country
type PricingAnalyticsDataPoint struct {
	Start           int64   `json:"start"`
	End             int64   `json:"end"`
	Volume          int64   `json:"volume"`                      // Message count
	Cost            float64 `json:"cost"`                        // Cost in account currency
	Country         string  `json:"country,omitempty"`           // Country code (IN, US, etc.)
	PricingType     string  `json:"pricing_type,omitempty"`      // FREE_CUSTOMER_SERVICE, FREE_ENTRY_POINT, REGULAR
	PricingCategory string  `json:"pricing_category,omitempty"`  // MARKETING, UTILITY, AUTHENTICATION, SERVICE, etc.
	Tier            string  `json:"tier,omitempty"`              // Pricing tier
}

// PricingAnalyticsEntry represents a single phone number's pricing data
type PricingAnalyticsEntry struct {
	PhoneNumber string                      `json:"phone_number,omitempty"`
	DataPoints  []PricingAnalyticsDataPoint `json:"data_points"`
}

// PricingAnalyticsRaw represents the raw response from Meta API
type PricingAnalyticsRaw struct {
	Granularity string                  `json:"granularity"`
	Data        []PricingAnalyticsEntry `json:"data"`
	// Also support direct data_points for backward compatibility
	DataPoints []PricingAnalyticsDataPoint `json:"data_points,omitempty"`
}

// PricingAnalytics represents pricing analytics response (flattened)
type PricingAnalytics struct {
	Granularity string                      `json:"granularity"`
	DataPoints  []PricingAnalyticsDataPoint `json:"data_points"`
}

// TemplateCostItem represents a cost item in template analytics
type TemplateCostItem struct {
	Type   string  `json:"type"`
	Amount float64 `json:"amount,omitempty"`
}

// TemplateAnalyticsDataPoint represents a single data point for template analytics
// This matches Meta's actual response where template_id is in each data point
type TemplateAnalyticsDataPoint struct {
	TemplateID string             `json:"template_id"`
	Start      int64              `json:"start"`
	End        int64              `json:"end"`
	Sent       int64              `json:"sent"`
	Delivered  int64              `json:"delivered"`
	Read       int64              `json:"read"`
	Replied    int64              `json:"replied,omitempty"`
	Clicked    int64              `json:"clicked,omitempty"`
	Cost       []TemplateCostItem `json:"cost,omitempty"`
}

// TemplateAnalyticsDataEntry represents one entry in the data array
type TemplateAnalyticsDataEntry struct {
	Granularity string                       `json:"granularity"`
	ProductType string                       `json:"product_type"`
	DataPoints  []TemplateAnalyticsDataPoint `json:"data_points"`
}

// TemplateAnalyticsRaw represents the raw response from Meta API for template analytics
type TemplateAnalyticsRaw struct {
	Data []TemplateAnalyticsDataEntry `json:"data"`
}

// TemplateAnalytics represents template analytics response (flattened for easier consumption)
type TemplateAnalytics struct {
	Granularity string                       `json:"granularity"`
	DataPoints  []TemplateAnalyticsDataPoint `json:"data_points"`
}

// CallAnalyticsDataPoint represents a single data point for call analytics
type CallAnalyticsDataPoint struct {
	Start         int64  `json:"start"`
	End           int64  `json:"end"`
	TotalCalls    int64  `json:"total_calls"`
	CallDuration  int64  `json:"call_duration"`  // Total duration in seconds
	CallType      string `json:"call_type"`      // "VOICE", "VIDEO"
	CallDirection string `json:"call_direction"` // "INBOUND", "OUTBOUND"
}

// CallAnalyticsEntry represents a single phone number's call data
type CallAnalyticsEntry struct {
	PhoneNumber string                   `json:"phone_number,omitempty"`
	DataPoints  []CallAnalyticsDataPoint `json:"data_points"`
}

// CallAnalyticsRaw represents the raw response from Meta API
type CallAnalyticsRaw struct {
	Granularity string               `json:"granularity"`
	Data        []CallAnalyticsEntry `json:"data"`
	// Also support direct data_points for backward compatibility
	DataPoints []CallAnalyticsDataPoint `json:"data_points,omitempty"`
}

// CallAnalytics represents call analytics response (flattened)
type CallAnalytics struct {
	Granularity string                   `json:"granularity"`
	DataPoints  []CallAnalyticsDataPoint `json:"data_points"`
}

// MetaAnalyticsResponse is a generic response that holds any analytics type
type MetaAnalyticsResponse struct {
	ID                    string                 `json:"id"`
	Analytics             *MessagingAnalytics    `json:"analytics,omitempty"`
	ConversationAnalytics *ConversationAnalytics `json:"conversation_analytics,omitempty"`
	PricingAnalytics      *PricingAnalytics      `json:"pricing_analytics,omitempty"`
	TemplateAnalytics     *TemplateAnalytics     `json:"template_analytics,omitempty"`
	CallAnalytics         *CallAnalytics         `json:"call_analytics,omitempty"`
}

// metaAnalyticsRawResponse represents the raw response from Meta API
type metaAnalyticsRawResponse struct {
	ID                    string          `json:"id"`
	Analytics             json.RawMessage `json:"analytics,omitempty"`
	ConversationAnalytics json.RawMessage `json:"conversation_analytics,omitempty"`
	PricingAnalytics      json.RawMessage `json:"pricing_analytics,omitempty"`
	TemplateAnalytics     json.RawMessage `json:"template_analytics,omitempty"`
	CallAnalytics         json.RawMessage `json:"call_analytics,omitempty"`
}

// metaPagingCursors represents the cursors in Meta API pagination
type metaPagingCursors struct {
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

// metaPaging represents the pagination info in Meta API response
type metaPaging struct {
	Cursors metaPagingCursors `json:"cursors,omitempty"`
	Next    string            `json:"next,omitempty"`
}

// templateAnalyticsWithPaging represents template analytics response with pagination
type templateAnalyticsWithPaging struct {
	Data   []TemplateAnalyticsDataEntry `json:"data"`
	Paging metaPaging                   `json:"paging,omitempty"`
}

// GetAnalytics fetches analytics from Meta API
func (c *Client) GetAnalytics(ctx context.Context, account *Account, analyticsType AnalyticsType, req *AnalyticsRequest) (*MetaAnalyticsResponse, error) {
	url := c.buildAnalyticsURL(account, analyticsType, req)
	c.Log.Debug("Fetching Meta analytics", "type", analyticsType, "url", url)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil, account.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", analyticsType, err)
	}

	// Log raw response for debugging
	c.Log.Debug("Meta analytics raw response", "type", analyticsType, "response", string(respBody))

	// Parse raw response first
	var rawResp metaAnalyticsRawResponse
	if err := json.Unmarshal(respBody, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to parse analytics response: %w", err)
	}

	response := &MetaAnalyticsResponse{
		ID: rawResp.ID,
	}

	// Handle template analytics with pagination
	if analyticsType == AnalyticsTypeTemplate && len(rawResp.TemplateAnalytics) > 0 {
		allDataPoints, err := c.fetchAllTemplateAnalyticsPages(ctx, account, rawResp.TemplateAnalytics)
		if err != nil {
			return nil, err
		}
		analytics := TemplateAnalytics{
			Granularity: "DAILY", // Template analytics only supports DAILY
			DataPoints:  allDataPoints,
		}
		response.TemplateAnalytics = &analytics
		return response, nil
	}

	// Parse the specific analytics type
	switch analyticsType {
	case AnalyticsTypeMessaging:
		if len(rawResp.Analytics) > 0 {
			var rawAnalytics MessagingAnalyticsRaw
			if err := json.Unmarshal(rawResp.Analytics, &rawAnalytics); err != nil {
				return nil, fmt.Errorf("failed to parse messaging analytics: %w", err)
			}
			// Flatten if nested, otherwise use direct data_points
			analytics := MessagingAnalytics{
				Granularity: rawAnalytics.Granularity,
				DataPoints:  make([]MessagingAnalyticsDataPoint, 0),
			}
			if len(rawAnalytics.Data) > 0 {
				for _, entry := range rawAnalytics.Data {
					analytics.DataPoints = append(analytics.DataPoints, entry.DataPoints...)
				}
			} else if len(rawAnalytics.DataPoints) > 0 {
				analytics.DataPoints = rawAnalytics.DataPoints
			}
			response.Analytics = &analytics
		}
	case AnalyticsTypeConversation:
		if len(rawResp.ConversationAnalytics) > 0 {
			var rawAnalytics ConversationAnalyticsRaw
			if err := json.Unmarshal(rawResp.ConversationAnalytics, &rawAnalytics); err != nil {
				return nil, fmt.Errorf("failed to parse conversation analytics: %w", err)
			}
			// Flatten if nested, otherwise use direct data_points
			analytics := ConversationAnalytics{
				Granularity: rawAnalytics.Granularity,
				DataPoints:  make([]ConversationAnalyticsDataPoint, 0),
			}
			if len(rawAnalytics.Data) > 0 {
				for _, entry := range rawAnalytics.Data {
					analytics.DataPoints = append(analytics.DataPoints, entry.DataPoints...)
				}
			} else if len(rawAnalytics.DataPoints) > 0 {
				analytics.DataPoints = rawAnalytics.DataPoints
			}
			response.ConversationAnalytics = &analytics
		}
	case AnalyticsTypePricing:
		if len(rawResp.PricingAnalytics) > 0 {
			var rawAnalytics PricingAnalyticsRaw
			if err := json.Unmarshal(rawResp.PricingAnalytics, &rawAnalytics); err != nil {
				return nil, fmt.Errorf("failed to parse pricing analytics: %w", err)
			}
			// Flatten if nested, otherwise use direct data_points
			analytics := PricingAnalytics{
				Granularity: rawAnalytics.Granularity,
				DataPoints:  make([]PricingAnalyticsDataPoint, 0),
			}
			if len(rawAnalytics.Data) > 0 {
				for _, entry := range rawAnalytics.Data {
					analytics.DataPoints = append(analytics.DataPoints, entry.DataPoints...)
				}
			} else if len(rawAnalytics.DataPoints) > 0 {
				analytics.DataPoints = rawAnalytics.DataPoints
			}
			response.PricingAnalytics = &analytics
		}
	case AnalyticsTypeTemplate:
		if len(rawResp.TemplateAnalytics) > 0 {
			var rawAnalytics TemplateAnalyticsRaw
			if err := json.Unmarshal(rawResp.TemplateAnalytics, &rawAnalytics); err != nil {
				return nil, fmt.Errorf("failed to parse template analytics: %w", err)
			}
			// Flatten the nested structure - template_id is in each data_point
			analytics := TemplateAnalytics{
				DataPoints: make([]TemplateAnalyticsDataPoint, 0),
			}
			// Get granularity from first entry if available
			if len(rawAnalytics.Data) > 0 {
				analytics.Granularity = rawAnalytics.Data[0].Granularity
			}
			// Flatten all data points from all entries
			for _, entry := range rawAnalytics.Data {
				analytics.DataPoints = append(analytics.DataPoints, entry.DataPoints...)
			}
			response.TemplateAnalytics = &analytics
		}
	case AnalyticsTypeCall:
		if len(rawResp.CallAnalytics) > 0 {
			var rawAnalytics CallAnalyticsRaw
			if err := json.Unmarshal(rawResp.CallAnalytics, &rawAnalytics); err != nil {
				return nil, fmt.Errorf("failed to parse call analytics: %w", err)
			}
			// Flatten if nested, otherwise use direct data_points
			analytics := CallAnalytics{
				Granularity: rawAnalytics.Granularity,
				DataPoints:  make([]CallAnalyticsDataPoint, 0),
			}
			if len(rawAnalytics.Data) > 0 {
				for _, entry := range rawAnalytics.Data {
					analytics.DataPoints = append(analytics.DataPoints, entry.DataPoints...)
				}
			} else if len(rawAnalytics.DataPoints) > 0 {
				analytics.DataPoints = rawAnalytics.DataPoints
			}
			response.CallAnalytics = &analytics
		}
	}

	return response, nil
}

// buildAnalyticsURL builds the analytics endpoint URL with filters
func (c *Client) buildAnalyticsURL(account *Account, analyticsType AnalyticsType, req *AnalyticsRequest) string {
	// Build the field with filters
	// Format: field.start(ts).end(ts).granularity(GRAN)[.phone_numbers(["+1234"])][.template_ids(["123"])]
	var filters []string

	filters = append(filters, fmt.Sprintf("start(%d)", req.Start))
	filters = append(filters, fmt.Sprintf("end(%d)", req.End))

	if req.Granularity != "" {
		// Normalize granularity based on analytics type (Meta API is inconsistent)
		normalizedGranularity := NormalizeGranularity(req.Granularity, analyticsType)
		filters = append(filters, fmt.Sprintf("granularity(%s)", normalizedGranularity))
	}

	if len(req.PhoneNumbers) > 0 {
		// Format phone numbers as JSON array
		phonesJSON, _ := json.Marshal(req.PhoneNumbers)
		filters = append(filters, fmt.Sprintf("phone_numbers(%s)", string(phonesJSON)))
	}

	if len(req.TemplateIDs) > 0 && analyticsType == AnalyticsTypeTemplate {
		templatesJSON, _ := json.Marshal(req.TemplateIDs)
		filters = append(filters, fmt.Sprintf("template_ids(%s)", string(templatesJSON)))
	}

	if len(req.CountryCodes) > 0 && analyticsType == AnalyticsTypePricing {
		countriesJSON, _ := json.Marshal(req.CountryCodes)
		filters = append(filters, fmt.Sprintf("country_codes(%s)", string(countriesJSON)))
	}

	// Add dimensions for pricing_analytics to get detailed breakdown
	if analyticsType == AnalyticsTypePricing {
		filters = append(filters, "dimensions(PRICING_CATEGORY,PRICING_TYPE,COUNTRY)")
	}

	// Add phone_numbers and dimensions for conversation_analytics (per Meta docs)
	if analyticsType == AnalyticsTypeConversation {
		// phone_numbers is required even if empty
		if len(req.PhoneNumbers) > 0 {
			phonesJSON, _ := json.Marshal(req.PhoneNumbers)
			filters = append(filters, fmt.Sprintf("phone_numbers(%s)", string(phonesJSON)))
		} else {
			filters = append(filters, "phone_numbers([])")
		}
		// dimensions for breakdown data
		filters = append(filters, "dimensions([\"CONVERSATION_CATEGORY\",\"CONVERSATION_TYPE\",\"COUNTRY\",\"PHONE\"])")
	}

	field := fmt.Sprintf("%s.%s", analyticsType, strings.Join(filters, "."))

	return fmt.Sprintf("%s/%s/%s?fields=%s", c.getBaseURL(), account.APIVersion, account.BusinessID, field)
}

// ValidateGranularity validates the granularity value (accepts both formats)
func ValidateGranularity(granularity string) bool {
	switch granularity {
	case "HALF_HOUR", "DAY", "DAILY", "MONTH", "MONTHLY":
		return true
	default:
		return false
	}
}

// NormalizeGranularity converts granularity to the correct format for each analytics type
// Meta API is inconsistent - some endpoints use DAY/MONTH, others use DAILY/MONTHLY
// Template analytics only supports DAILY
func NormalizeGranularity(granularity string, analyticsType AnalyticsType) string {
	// Normalize input to standard format first
	normalizedInput := granularity
	switch granularity {
	case "DAILY":
		normalizedInput = "DAY"
	case "MONTHLY":
		normalizedInput = "MONTH"
	}

	// Template analytics only supports DAILY granularity
	if analyticsType == AnalyticsTypeTemplate {
		return "DAILY"
	}

	// Some endpoints use DAILY/MONTHLY format
	useDailyFormat := false
	switch analyticsType {
	case AnalyticsTypeConversation, AnalyticsTypePricing, AnalyticsTypeCall:
		useDailyFormat = true
	}

	if useDailyFormat {
		switch normalizedInput {
		case "DAY":
			return "DAILY"
		case "MONTH":
			return "MONTHLY"
		}
	}

	return normalizedInput
}

// ValidateAnalyticsType validates the analytics type value
func ValidateAnalyticsType(analyticsType string) bool {
	switch AnalyticsType(analyticsType) {
	case AnalyticsTypeMessaging, AnalyticsTypeConversation, AnalyticsTypePricing,
		AnalyticsTypeTemplate, AnalyticsTypeCall:
		return true
	default:
		return false
	}
}

// fetchAllTemplateAnalyticsPages fetches all pages of template analytics using pagination
func (c *Client) fetchAllTemplateAnalyticsPages(ctx context.Context, account *Account, firstPageData json.RawMessage) ([]TemplateAnalyticsDataPoint, error) {
	var allDataPoints []TemplateAnalyticsDataPoint

	// Parse first page
	var firstPage templateAnalyticsWithPaging
	if err := json.Unmarshal(firstPageData, &firstPage); err != nil {
		return nil, fmt.Errorf("failed to parse template analytics: %w", err)
	}

	// Collect data points from first page
	for _, entry := range firstPage.Data {
		allDataPoints = append(allDataPoints, entry.DataPoints...)
	}

	// Follow pagination
	nextURL := firstPage.Paging.Next
	pageCount := 1
	maxPages := 50 // Safety limit to prevent infinite loops

	for nextURL != "" && pageCount < maxPages {
		c.Log.Debug("Fetching next page of template analytics", "page", pageCount+1, "url", nextURL)

		respBody, err := c.doRequest(ctx, http.MethodGet, nextURL, nil, account.AccessToken)
		if err != nil {
			c.Log.Error("Failed to fetch template analytics page", "error", err, "page", pageCount+1)
			break // Return what we have so far
		}

		// The paginated response has a different structure - data is at root level
		var pageResp templateAnalyticsWithPaging
		if err := json.Unmarshal(respBody, &pageResp); err != nil {
			c.Log.Error("Failed to parse template analytics page", "error", err, "page", pageCount+1)
			break
		}

		// Collect data points from this page
		for _, entry := range pageResp.Data {
			allDataPoints = append(allDataPoints, entry.DataPoints...)
		}

		nextURL = pageResp.Paging.Next
		pageCount++
	}

	c.Log.Debug("Template analytics pagination complete", "total_pages", pageCount, "total_data_points", len(allDataPoints))
	return allDataPoints, nil
}
