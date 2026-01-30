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
	AnalyticsTypeMessaging AnalyticsType = "analytics"
	AnalyticsTypePricing   AnalyticsType = "pricing_analytics"
	AnalyticsTypeTemplate  AnalyticsType = "template_analytics"
	AnalyticsTypeCall      AnalyticsType = "call_analytics"
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
	Type  string  `json:"type"`            // amount_spent, cost_per_delivered, cost_per_url_button_click
	Value float64 `json:"value,omitempty"` // The cost value
}

// TemplateClickItem represents a click item in template analytics
type TemplateClickItem struct {
	Type          string `json:"type"`           // quick_reply_button, unique_url_button
	ButtonContent string `json:"button_content"` // The button text
	Count         int64  `json:"count"`          // Number of clicks
}

// TemplateAnalyticsDataPoint represents a single data point for template analytics
// This matches Meta's actual response where template_id is in each data point
type TemplateAnalyticsDataPoint struct {
	TemplateID string              `json:"template_id"`
	Start      int64               `json:"start"`
	End        int64               `json:"end"`
	Sent       int64               `json:"sent"`
	Delivered  int64               `json:"delivered"`
	Read       int64               `json:"read"`
	Replied    int64               `json:"replied,omitempty"`
	Clicked    []TemplateClickItem `json:"clicked,omitempty"` // Array of button click details
	Cost       []TemplateCostItem  `json:"cost,omitempty"`
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
	ID                string              `json:"id"`
	Analytics         *MessagingAnalytics `json:"analytics,omitempty"`
	PricingAnalytics  *PricingAnalytics   `json:"pricing_analytics,omitempty"`
	TemplateAnalytics *TemplateAnalytics  `json:"template_analytics,omitempty"`
	CallAnalytics     *CallAnalytics      `json:"call_analytics,omitempty"`
}

// metaAnalyticsRawResponse represents the raw response from Meta API
type metaAnalyticsRawResponse struct {
	ID                string          `json:"id"`
	Analytics         json.RawMessage `json:"analytics,omitempty"`
	PricingAnalytics  json.RawMessage `json:"pricing_analytics,omitempty"`
	TemplateAnalytics json.RawMessage `json:"template_analytics,omitempty"`
	CallAnalytics     json.RawMessage `json:"call_analytics,omitempty"`
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

	// Template analytics uses a different endpoint that returns data directly (not nested under template_analytics)
	if analyticsType == AnalyticsTypeTemplate {
		return c.parseTemplateAnalyticsResponse(ctx, account, respBody)
	}

	// Parse raw response first
	var rawResp metaAnalyticsRawResponse
	if err := json.Unmarshal(respBody, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to parse analytics response: %w", err)
	}

	response := &MetaAnalyticsResponse{
		ID: rawResp.ID,
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
	// Template analytics uses a different endpoint format
	// https://graph.facebook.com/{version}/{waba_id}/template_analytics?start=...&end=...&granularity=...&metric_types=...&template_ids=[...]
	if analyticsType == AnalyticsTypeTemplate {
		return c.buildTemplateAnalyticsURL(account, req)
	}

	// Other analytics use the fields syntax
	// Format: field.start(ts).end(ts).granularity(GRAN)[.phone_numbers(["+1234"])]
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

	if len(req.CountryCodes) > 0 && analyticsType == AnalyticsTypePricing {
		countriesJSON, _ := json.Marshal(req.CountryCodes)
		filters = append(filters, fmt.Sprintf("country_codes(%s)", string(countriesJSON)))
	}

	// Add dimensions for pricing_analytics to get detailed breakdown
	if analyticsType == AnalyticsTypePricing {
		filters = append(filters, "dimensions(PRICING_CATEGORY,PRICING_TYPE,COUNTRY)")
	}

	field := fmt.Sprintf("%s.%s", analyticsType, strings.Join(filters, "."))

	return fmt.Sprintf("%s/%s/%s?fields=%s", c.getBaseURL(), account.APIVersion, account.BusinessID, field)
}

// buildTemplateAnalyticsURL builds the template analytics endpoint URL
// Uses dedicated endpoint: /{waba_id}/template_analytics?start=...&end=...&granularity=...&metric_types=...&template_ids=[...]
func (c *Client) buildTemplateAnalyticsURL(account *Account, req *AnalyticsRequest) string {
	baseURL := fmt.Sprintf("%s/%s/%s/template_analytics", c.getBaseURL(), account.APIVersion, account.BusinessID)

	// Build query parameters
	params := []string{
		fmt.Sprintf("start=%d", req.Start),
		fmt.Sprintf("end=%d", req.End),
		"granularity=daily", // Template analytics only supports daily
		"metric_types=cost,clicked,delivered,read,sent",
	}

	// Add template IDs if provided - format as numeric array [123,456] not ["123","456"]
	// If no template IDs are provided, Meta will return all templates with activity
	if len(req.TemplateIDs) > 0 {
		templateIDsStr := "[" + strings.Join(req.TemplateIDs, ",") + "]"
		params = append(params, fmt.Sprintf("template_ids=%s", templateIDsStr))
		c.Log.Debug("Template analytics request", "template_ids", templateIDsStr, "count", len(req.TemplateIDs))
	} else {
		c.Log.Debug("Template analytics request without template_ids filter - will return all templates with activity")
	}

	return fmt.Sprintf("%s?%s", baseURL, strings.Join(params, "&"))
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
	case AnalyticsTypePricing, AnalyticsTypeCall:
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
	case AnalyticsTypeMessaging, AnalyticsTypePricing, AnalyticsTypeTemplate, AnalyticsTypeCall:
		return true
	default:
		return false
	}
}

// parseTemplateAnalyticsResponse parses the response from the direct template_analytics endpoint
// This endpoint returns {"data": [...], "paging": {...}} at root level (not nested under template_analytics)
func (c *Client) parseTemplateAnalyticsResponse(ctx context.Context, account *Account, respBody []byte) (*MetaAnalyticsResponse, error) {
	var firstPage templateAnalyticsWithPaging
	if err := json.Unmarshal(respBody, &firstPage); err != nil {
		return nil, fmt.Errorf("failed to parse template analytics response: %w", err)
	}

	// Collect data points from first page
	allDataPoints := make([]TemplateAnalyticsDataPoint, 0)
	for _, entry := range firstPage.Data {
		allDataPoints = append(allDataPoints, entry.DataPoints...)
	}

	// Follow pagination
	nextURL := firstPage.Paging.Next
	pageCount := 1
	maxPages := 50 // Safety limit

	for nextURL != "" && pageCount < maxPages {
		c.Log.Debug("Fetching next page of template analytics", "page", pageCount+1, "url", nextURL)

		pageRespBody, err := c.doRequest(ctx, http.MethodGet, nextURL, nil, account.AccessToken)
		if err != nil {
			c.Log.Error("Failed to fetch template analytics page", "error", err, "page", pageCount+1)
			break
		}

		var pageResp templateAnalyticsWithPaging
		if err := json.Unmarshal(pageRespBody, &pageResp); err != nil {
			c.Log.Error("Failed to parse template analytics page", "error", err, "page", pageCount+1)
			break
		}

		for _, entry := range pageResp.Data {
			allDataPoints = append(allDataPoints, entry.DataPoints...)
		}

		nextURL = pageResp.Paging.Next
		pageCount++
	}

	c.Log.Debug("Template analytics pagination complete", "total_pages", pageCount, "total_data_points", len(allDataPoints))

	response := &MetaAnalyticsResponse{
		ID: account.BusinessID,
		TemplateAnalytics: &TemplateAnalytics{
			Granularity: "DAILY",
			DataPoints:  allDataPoints,
		},
	}

	return response, nil
}

