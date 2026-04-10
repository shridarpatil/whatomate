package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// TemplateSubmission represents a template to be submitted to Meta
type TemplateSubmission struct {
	MetaTemplateID  string // If set, update existing template instead of creating new
	Name            string
	Language        string
	Category        string
	ParameterFormat string // "positional" or "named" - default is positional
	HeaderType      string
	HeaderContent   string
	BodyContent     string
	FooterContent   string
	Buttons         []any
	SampleValues    []any // For named: [{param_name: "name", value: "John"}, ...]

	// Authentication template fields
	AddSecurityRecommendation bool
	CodeExpirationMinutes     int // 1-90, 0 means no expiration footer
}

// SubmitTemplate submits a template to Meta's API (creates new or updates existing)
func (c *Client) SubmitTemplate(ctx context.Context, account *Account, template *TemplateSubmission) (string, error) {
	// If MetaTemplateID is set, this is an update to existing template
	isUpdate := template.MetaTemplateID != ""
	var url string
	if isUpdate {
		url = fmt.Sprintf("%s/%s", c.baseURL, template.MetaTemplateID)
	} else {
		url = c.buildTemplatesURL(account)
	}

	// Build components array
	var components []map[string]any
	var compErr error

	// Authentication templates have a different component structure per Meta API
	if strings.ToUpper(template.Category) == "AUTHENTICATION" {
		components = c.buildAuthComponents(template)
	} else {
		components, compErr = c.buildStandardComponents(template)
		if compErr != nil {
			return "", compErr
		}
	}

	// Build request payload
	var payload map[string]any
	if isUpdate {
		// Update only sends components (name, language, category are immutable)
		payload = map[string]any{
			"components": components,
		}
	} else {
		// Create sends full template
		payload = map[string]any{
			"name":       template.Name,
			"language":   template.Language,
			"category":   template.Category,
			"components": components,
		}
		// Add parameter_format for named parameters (only for create, not auth)
		if strings.ToUpper(template.Category) != "AUTHENTICATION" {
			isNamedParams := template.ParameterFormat == "named" || hasNamedParams(template.BodyContent)
			if isNamedParams {
				payload["parameter_format"] = "NAMED"
			}
		}
	}

	// Log payload for debugging
	action := "Submitting"
	if isUpdate {
		action = "Updating"
	}
	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
	c.Log.Info(action+" template to Meta", "url", url, "name", template.Name, "payload", string(payloadJSON))

	respBody, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to "+action+" template", "error", err, "name", template.Name)
		return "", err
	}

	// For updates, return existing ID; for creates, parse response for new ID
	if isUpdate {
		c.Log.Info("Template updated", "template_id", template.MetaTemplateID, "name", template.Name)
		return template.MetaTemplateID, nil
	}

	var result TemplateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	c.Log.Info("Template submitted", "template_id", result.ID, "name", template.Name)
	return result.ID, nil
}

// buildAuthComponents builds Meta API components for AUTHENTICATION templates.
// Auth templates have fixed preset body text and use special fields instead of free text.
func (c *Client) buildAuthComponents(template *TemplateSubmission) []map[string]any {
	components := []map[string]any{}

	// BODY component — no text field, only add_security_recommendation
	body := map[string]any{"type": "BODY"}
	if template.AddSecurityRecommendation {
		body["add_security_recommendation"] = true
	}
	components = append(components, body)

	// FOOTER component — only code_expiration_minutes (optional, 1-90)
	if template.CodeExpirationMinutes > 0 {
		components = append(components, map[string]any{
			"type":                   "FOOTER",
			"code_expiration_minutes": template.CodeExpirationMinutes,
		})
	}

	// BUTTONS component — OTP button with supported_apps for ONE_TAP/ZERO_TAP
	if len(template.Buttons) > 0 {
		for _, btn := range template.Buttons {
			btnMap, ok := btn.(map[string]any)
			if !ok {
				continue
			}
			btnType, _ := btnMap["type"].(string)
			if strings.ToUpper(btnType) != "OTP" {
				continue
			}
			otpType, _ := btnMap["otp_type"].(string)
			if otpType == "" {
				otpType = "COPY_CODE"
			}
			button := map[string]any{
				"type":     "OTP",
				"otp_type": otpType,
			}
			if otpType == "ONE_TAP" || otpType == "ZERO_TAP" {
				pkg, _ := btnMap["package_name"].(string)
				hash, _ := btnMap["signature_hash"].(string)
				if pkg != "" && hash != "" {
					button["supported_apps"] = []map[string]string{{
						"package_name":   pkg,
						"signature_hash": hash,
					}}
				}
			}
			components = append(components, map[string]any{
				"type":    "BUTTONS",
				"buttons": []map[string]any{button},
			})
			break // Only one OTP button allowed
		}
	}

	return components
}

// buildStandardComponents builds Meta API components for MARKETING/UTILITY templates.
func (c *Client) buildStandardComponents(template *TemplateSubmission) ([]map[string]any, error) {
	components := []map[string]any{}

	isNamedParams := template.ParameterFormat == "named" || hasNamedParams(template.BodyContent)

	// Header component (must come before BODY)
	if template.HeaderType != "" && template.HeaderType != "NONE" {
		header := map[string]any{
			"type":   "HEADER",
			"format": template.HeaderType,
		}
		addHeader := true
		switch template.HeaderType {
		case "TEXT":
			header["text"] = template.HeaderContent
			if strings.Contains(template.HeaderContent, "{{") {
				if isNamedParams {
					namedExamples := extractNamedExamplesForComponent(template.SampleValues, "header")
					if len(namedExamples) > 0 {
						header["example"] = map[string]any{
							"header_text_named_params": namedExamples,
						}
					}
				} else {
					headerExamples := extractExamplesForComponent(template.SampleValues, "header")
					if len(headerExamples) > 0 {
						header["example"] = map[string]any{
							"header_text": headerExamples,
						}
					}
				}
			}
		case "IMAGE", "VIDEO", "DOCUMENT":
			if template.HeaderContent != "" {
				header["example"] = map[string]any{
					"header_handle": []string{template.HeaderContent},
				}
			} else {
				addHeader = false
			}
		}
		if addHeader {
			components = append(components, header)
		}
	}

	// Body component (required)
	body := map[string]any{
		"type": "BODY",
		"text": template.BodyContent,
	}
	if strings.Contains(template.BodyContent, "{{") {
		if isNamedParams {
			namedExamples := extractNamedExamplesForComponent(template.SampleValues, "body")
			if len(namedExamples) > 0 {
				body["example"] = map[string]any{
					"body_text_named_params": namedExamples,
				}
			} else {
				varCount := strings.Count(template.BodyContent, "{{")
				if varCount > 0 {
					return nil, fmt.Errorf("sample values are required for template variables. Found %d variable(s) in body but no sample values provided", varCount)
				}
			}
		} else {
			bodyExamples := extractExamplesForComponent(template.SampleValues, "body")
			if len(bodyExamples) > 0 {
				body["example"] = map[string]any{
					"body_text": [][]string{bodyExamples},
				}
			} else {
				varCount := strings.Count(template.BodyContent, "{{")
				if varCount > 0 {
					return nil, fmt.Errorf("sample values are required for template variables. Found %d variable(s) in body but no sample values provided", varCount)
				}
			}
		}
	}
	components = append(components, body)

	// Footer component
	if template.FooterContent != "" {
		components = append(components, map[string]any{
			"type": "FOOTER",
			"text": template.FooterContent,
		})
	}

	// Buttons component
	if len(template.Buttons) > 0 {
		buttons := []map[string]any{}
		for _, btn := range template.Buttons {
			if btnMap, ok := btn.(map[string]any); ok {
				btnType, _ := btnMap["type"].(string)
				btnType = strings.ToUpper(btnType)
				btnText, _ := btnMap["text"].(string)

				if btnText == "" {
					continue
				}

				button := map[string]any{}

				switch btnType {
				case "QUICK_REPLY":
					button["type"] = "QUICK_REPLY"
					button["text"] = btnText
				case "URL":
					btnURL, _ := btnMap["url"].(string)
					if btnURL == "" {
						continue
					}
					button["type"] = "URL"
					button["text"] = btnText
					button["url"] = btnURL
					if strings.Contains(btnURL, "{{") {
						switch ex := btnMap["example"].(type) {
						case string:
							if ex != "" {
								button["example"] = []string{ex}
							}
						case []any:
							if len(ex) > 0 {
								if s, ok := ex[0].(string); ok && s != "" {
									button["example"] = []string{s}
								}
							}
						case []string:
							if len(ex) > 0 && ex[0] != "" {
								button["example"] = []string{ex[0]}
							}
						}
					}
				case "PHONE_NUMBER":
					phoneNum, _ := btnMap["phone_number"].(string)
					if phoneNum == "" {
						continue
					}
					button["type"] = "PHONE_NUMBER"
					button["text"] = btnText
					button["phone_number"] = phoneNum
				case "COPY_CODE":
					button["type"] = "COPY_CODE"
					button["text"] = btnText
					switch ex := btnMap["example"].(type) {
					case string:
						if ex != "" {
							button["example"] = ex
						}
					case []any:
						if len(ex) > 0 {
							if s, ok := ex[0].(string); ok && s != "" {
								button["example"] = s
							}
						}
					case []string:
						if len(ex) > 0 && ex[0] != "" {
							button["example"] = ex[0]
						}
					}
				case "FLOW":
					flowID, _ := btnMap["flow_id"].(string)
					if flowID == "" {
						continue
					}
					button["type"] = "FLOW"
					button["text"] = btnText
					button["flow_id"] = flowID
					flowAction, _ := btnMap["flow_action"].(string)
					if flowAction == "" {
						flowAction = "navigate"
					}
					button["flow_action"] = flowAction
					if screen, ok := btnMap["navigate_screen"].(string); ok && screen != "" {
						button["navigate_screen"] = screen
					}
				case "VOICE_CALL":
					button["type"] = "VOICE_CALL"
					button["text"] = btnText
				case "OTP":
					button["type"] = "OTP"
					otpType, _ := btnMap["otp_type"].(string)
					if otpType == "" {
						otpType = "COPY_CODE"
					}
					button["otp_type"] = otpType
					if otpType == "ONE_TAP" || otpType == "ZERO_TAP" {
						if pkg, ok := btnMap["package_name"].(string); ok && pkg != "" {
							button["package_name"] = pkg
						}
						if hash, ok := btnMap["signature_hash"].(string); ok && hash != "" {
							button["signature_hash"] = hash
						}
					}
					if otpType == "ONE_TAP" {
						if autofill, ok := btnMap["autofill_text"].(string); ok && autofill != "" {
							button["autofill_text"] = autofill
						}
					}
				default:
					button["type"] = "QUICK_REPLY"
					button["text"] = btnText
				}

				if len(button) > 0 {
					buttons = append(buttons, button)
				}
			}
		}
		if len(buttons) > 0 {
			components = append(components, map[string]any{
				"type":    "BUTTONS",
				"buttons": buttons,
			})
		}
	}

	return components, nil
}

// FetchTemplates fetches all templates from Meta's API
func (c *Client) FetchTemplates(ctx context.Context, account *Account) ([]MetaTemplate, error) {
	url := fmt.Sprintf("%s?limit=100", c.buildTemplatesURL(account))

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to fetch templates", "error", err)
		return nil, err
	}

	var result TemplateListResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	c.Log.Info("Fetched templates from Meta", "count", len(result.Data))
	return result.Data, nil
}

// DeleteTemplate deletes a template from Meta's API
func (c *Client) DeleteTemplate(ctx context.Context, account *Account, templateName string) error {
	url := fmt.Sprintf("%s?name=%s", c.buildTemplatesURL(account), templateName)

	_, err := c.doRequest(ctx, http.MethodDelete, url, nil, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to delete template", "error", err, "template", templateName)
		return err
	}

	c.Log.Info("Template deleted from Meta", "template", templateName)
	return nil
}

// extractExamplesForComponent extracts example values for a specific component from sample_values
func extractExamplesForComponent(sampleValues []any, componentType string) []string {
	type indexedSample struct {
		index int
		value string
	}
	samples := []indexedSample{}

	for _, sv := range sampleValues {
		if svMap, ok := sv.(map[string]any); ok {
			comp, _ := svMap["component"].(string)
			if comp == componentType {
				value, _ := svMap["value"].(string)
				if value != "" {
					idx := 1
					if idxFloat, ok := svMap["index"].(float64); ok {
						idx = int(idxFloat)
					} else if idxInt, ok := svMap["index"].(int); ok {
						idx = idxInt
					}
					samples = append(samples, indexedSample{index: idx, value: value})
				}
			}
			// Also support legacy format with "values" array
			if svMap["component"] == componentType {
				if values, ok := svMap["values"].([]any); ok {
					for i, v := range values {
						if str, ok := v.(string); ok {
							samples = append(samples, indexedSample{index: i + 1, value: str})
						}
					}
				}
			}
		}
	}

	// Sort by index and extract values
	if len(samples) > 0 {
		for i := 0; i < len(samples)-1; i++ {
			for j := i + 1; j < len(samples); j++ {
				if samples[i].index > samples[j].index {
					samples[i], samples[j] = samples[j], samples[i]
				}
			}
		}
		examples := make([]string, len(samples))
		for i, s := range samples {
			examples[i] = s.value
		}
		return examples
	}

	// Fallback: if no component-specific samples, try to get all string values
	examples := []string{}
	for _, sv := range sampleValues {
		if str, ok := sv.(string); ok {
			examples = append(examples, str)
		}
	}
	return examples
}

// hasNamedParams checks if the body content uses named parameters (non-numeric)
func hasNamedParams(content string) bool {
	// Extract all parameter names
	matches := strings.Split(content, "{{")
	for _, m := range matches[1:] { // Skip first part before any {{
		if idx := strings.Index(m, "}}"); idx > 0 {
			paramName := strings.TrimSpace(m[:idx])
			// If param name is not purely numeric, it's a named param
			if paramName != "" {
				isNumeric := true
				for _, c := range paramName {
					if c < '0' || c > '9' {
						isNumeric = false
						break
					}
				}
				if !isNumeric {
					return true
				}
			}
		}
	}
	return false
}

// extractNamedExamplesForComponent extracts named example values for Meta API format
// Returns: [{"param_name": "name", "example": "John"}, ...]
func extractNamedExamplesForComponent(sampleValues []any, componentType string) []map[string]string {
	results := []map[string]string{}

	for _, sv := range sampleValues {
		if svMap, ok := sv.(map[string]any); ok {
			comp, _ := svMap["component"].(string)
			// Match component type or accept if not specified (for body)
			if comp == componentType || (comp == "" && componentType == "body") {
				paramName, _ := svMap["param_name"].(string)
				value, _ := svMap["value"].(string)
				if paramName != "" && value != "" {
					results = append(results, map[string]string{
						"param_name": paramName,
						"example":    value,
					})
				}
			}
		}
	}

	return results
}
