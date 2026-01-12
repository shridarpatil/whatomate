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
	Name            string
	Language        string
	Category        string
	ParameterFormat string // "positional" or "named" - default is positional
	HeaderType      string
	HeaderContent   string
	BodyContent     string
	FooterContent   string
	Buttons         []interface{}
	SampleValues    []interface{} // For named: [{param_name: "name", value: "John"}, ...]
}

// SubmitTemplate submits a template to Meta's API
func (c *Client) SubmitTemplate(ctx context.Context, account *Account, template *TemplateSubmission) (string, error) {
	url := c.buildTemplatesURL(account)

	// Build components array
	components := []map[string]interface{}{}

	// Check if using named parameters
	isNamedParams := template.ParameterFormat == "named" || hasNamedParams(template.BodyContent)

	// Header component (must come before BODY)
	if template.HeaderType != "" && template.HeaderType != "NONE" {
		header := map[string]interface{}{
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
						header["example"] = map[string]interface{}{
							"header_text_named_params": namedExamples,
						}
					}
				} else {
					headerExamples := extractExamplesForComponent(template.SampleValues, "header")
					if len(headerExamples) > 0 {
						header["example"] = map[string]interface{}{
							"header_text": headerExamples,
						}
					}
				}
			}
		case "IMAGE", "VIDEO", "DOCUMENT":
			// Media headers require a handle - skip if not provided
			if template.HeaderContent != "" {
				header["example"] = map[string]interface{}{
					"header_handle": []string{template.HeaderContent},
				}
			} else {
				// Don't add media header without a handle
				addHeader = false
			}
		}
		if addHeader {
			components = append(components, header)
		}
	}

	// Body component (required)
	body := map[string]interface{}{
		"type": "BODY",
		"text": template.BodyContent,
	}
	// Add examples if there are variables in body
	if strings.Contains(template.BodyContent, "{{") {
		if isNamedParams {
			namedExamples := extractNamedExamplesForComponent(template.SampleValues, "body")
			if len(namedExamples) > 0 {
				body["example"] = map[string]interface{}{
					"body_text_named_params": namedExamples,
				}
			} else {
				varCount := strings.Count(template.BodyContent, "{{")
				if varCount > 0 {
					return "", fmt.Errorf("sample values are required for template variables. Found %d variable(s) in body but no sample values provided", varCount)
				}
			}
		} else {
			bodyExamples := extractExamplesForComponent(template.SampleValues, "body")
			if len(bodyExamples) > 0 {
				body["example"] = map[string]interface{}{
					"body_text": [][]string{bodyExamples},
				}
			} else {
				varCount := strings.Count(template.BodyContent, "{{")
				if varCount > 0 {
					return "", fmt.Errorf("sample values are required for template variables. Found %d variable(s) in body but no sample values provided", varCount)
				}
			}
		}
	}
	components = append(components, body)

	// Footer component
	if template.FooterContent != "" {
		components = append(components, map[string]interface{}{
			"type": "FOOTER",
			"text": template.FooterContent,
		})
	}

	// Buttons component
	if len(template.Buttons) > 0 {
		buttons := []map[string]interface{}{}
		for _, btn := range template.Buttons {
			if btnMap, ok := btn.(map[string]interface{}); ok {
				btnType, _ := btnMap["type"].(string)
				btnType = strings.ToUpper(btnType)
				btnText, _ := btnMap["text"].(string)

				if btnText == "" {
					continue
				}

				button := map[string]interface{}{}

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
						if example, ok := btnMap["example"].(string); ok && example != "" {
							button["example"] = []string{example}
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
					if example, ok := btnMap["example"].(string); ok && example != "" {
						button["example"] = example
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
			components = append(components, map[string]interface{}{
				"type":    "BUTTONS",
				"buttons": buttons,
			})
		}
	}

	// Build request payload
	payload := map[string]interface{}{
		"name":       template.Name,
		"language":   template.Language,
		"category":   template.Category,
		"components": components,
	}

	// Add parameter_format for named parameters
	if isNamedParams {
		payload["parameter_format"] = "NAMED"
	}

	// Log payload for debugging
	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
	c.Log.Info("Submitting template to Meta", "url", url, "name", template.Name, "payload", string(payloadJSON))

	respBody, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to submit template", "error", err, "name", template.Name)
		return "", err
	}

	var result TemplateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	c.Log.Info("Template submitted", "template_id", result.ID, "name", template.Name)
	return result.ID, nil
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
func extractExamplesForComponent(sampleValues []interface{}, componentType string) []string {
	type indexedSample struct {
		index int
		value string
	}
	samples := []indexedSample{}

	for _, sv := range sampleValues {
		if svMap, ok := sv.(map[string]interface{}); ok {
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
				if values, ok := svMap["values"].([]interface{}); ok {
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
func extractNamedExamplesForComponent(sampleValues []interface{}, componentType string) []map[string]string {
	results := []map[string]string{}

	for _, sv := range sampleValues {
		if svMap, ok := sv.(map[string]interface{}); ok {
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
