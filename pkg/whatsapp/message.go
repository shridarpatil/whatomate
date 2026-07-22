package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SendTextMessage sends a text message to a recipient with optional reply context
func (c *Client) SendTextMessage(ctx context.Context, account *Account, rcpt Recipient, text string, replyToMsgID ...string) (string, error) {
	payload := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"type":              "text",
		"text": map[string]any{
			"preview_url": false,
			"body":        text,
		},
	}
	rcpt.SetOnPayload(payload)

	// Add reply context if provided
	if len(replyToMsgID) > 0 && replyToMsgID[0] != "" {
		payload["context"] = map[string]any{
			"message_id": replyToMsgID[0],
		}
	}

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending text message", "phone", rcpt.Phone, "url", url)

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to send text message", "error", err, "phone", rcpt.Phone)
		return "", fmt.Errorf("failed to send text message: %w", err)
	}

	var resp MetaAPIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Messages) == 0 {
		return "", fmt.Errorf("no message ID in response")
	}

	messageID := resp.Messages[0].ID
	c.Log.Info("Text message sent", "message_id", messageID, "phone", rcpt.Phone)
	return messageID, nil
}

// SendInteractiveButtons sends an interactive message with buttons or list
// If buttons <= 3, sends as buttons; if 4-10, sends as list
func (c *Client) SendInteractiveButtons(ctx context.Context, account *Account, rcpt Recipient, bodyText string, buttons []Button) (string, error) {
	if len(buttons) == 0 {
		return "", fmt.Errorf("at least one button is required")
	}
	if len(buttons) > 10 {
		return "", fmt.Errorf("maximum 10 buttons allowed")
	}

	var interactive map[string]any

	if len(buttons) <= 3 {
		// Use button format
		buttonsList := make([]map[string]any, 0, len(buttons))
		for _, btn := range buttons {
			title := btn.Title
			if len(title) > 20 {
				title = title[:20]
			}
			buttonsList = append(buttonsList, map[string]any{
				"type": "reply",
				"reply": map[string]any{
					"id":    btn.ID,
					"title": title,
				},
			})
		}

		interactive = map[string]any{
			"type": "button",
			"body": map[string]any{
				"text": bodyText,
			},
			"action": map[string]any{
				"buttons": buttonsList,
			},
		}
	} else {
		// Use list format for 4-10 items
		rows := make([]map[string]any, 0, len(buttons))
		for _, btn := range buttons {
			title := btn.Title
			if len(title) > 24 {
				title = title[:24]
			}
			rows = append(rows, map[string]any{
				"id":    btn.ID,
				"title": title,
			})
		}

		interactive = map[string]any{
			"type": "list",
			"body": map[string]any{
				"text": bodyText,
			},
			"action": map[string]any{
				"button": "Select an option",
				"sections": []map[string]any{
					{
						"title": "Options",
						"rows":  rows,
					},
				},
			},
		}
	}

	payload := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"type":              "interactive",
		"interactive":       interactive,
	}
	rcpt.SetOnPayload(payload)

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending interactive message", "phone", rcpt.Phone, "button_count", len(buttons))

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to send interactive message", "error", err, "phone", rcpt.Phone)
		return "", fmt.Errorf("failed to send interactive message: %w", err)
	}

	var resp MetaAPIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Messages) == 0 {
		return "", fmt.Errorf("no message ID in response")
	}

	messageID := resp.Messages[0].ID
	c.Log.Info("Interactive message sent", "message_id", messageID, "phone", rcpt.Phone)
	return messageID, nil
}

// SendCTAURLButton sends an interactive message with a CTA URL button
// This opens a URL when clicked instead of sending a reply
func (c *Client) SendCTAURLButton(ctx context.Context, account *Account, rcpt Recipient, bodyText, buttonText, url string) (string, error) {
	if buttonText == "" || url == "" {
		return "", fmt.Errorf("button text and URL are required")
	}

	// Truncate button text to 20 chars (WhatsApp limit)
	if len(buttonText) > 20 {
		buttonText = buttonText[:20]
	}

	interactive := map[string]any{
		"type": "cta_url",
		"body": map[string]any{
			"text": bodyText,
		},
		"action": map[string]any{
			"name": "cta_url",
			"parameters": map[string]any{
				"display_text": buttonText,
				"url":          url,
			},
		},
	}

	payload := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"type":              "interactive",
		"interactive":       interactive,
	}
	rcpt.SetOnPayload(payload)

	apiURL := c.buildMessagesURL(account)
	c.Log.Debug("Sending CTA URL button message", "phone", rcpt.Phone, "url", url)

	respBody, err := c.doRequest(ctx, "POST", apiURL, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to send CTA URL button message", "error", err, "phone", rcpt.Phone)
		return "", fmt.Errorf("failed to send CTA URL button message: %w", err)
	}

	var resp MetaAPIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Messages) == 0 {
		return "", fmt.Errorf("no message ID in response")
	}

	messageID := resp.Messages[0].ID
	c.Log.Info("CTA URL button message sent", "message_id", messageID, "phone", rcpt.Phone)
	return messageID, nil
}

// TemplateParam represents a parameter for template message
type TemplateParam struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Image *struct {
		Link string `json:"link"`
	} `json:"image,omitempty"`
	Document *struct {
		Link     string `json:"link"`
		Filename string `json:"filename"`
	} `json:"document,omitempty"`
	Video *struct {
		Link string `json:"link"`
	} `json:"video,omitempty"`
}

// SendTemplateMessage sends a template message
// BodyParamsToComponents converts a bodyParams map into WhatsApp template components.
// Supports both positional (numeric keys) and named parameters.
func BodyParamsToComponents(bodyParams map[string]string) []map[string]any {
	if len(bodyParams) == 0 {
		return nil
	}

	// Check if using named parameters (non-numeric keys like "name", "order_id")
	isNamedParams := false
	for key := range bodyParams {
		if _, err := strconv.Atoi(key); err != nil {
			isNamedParams = true
			break
		}
	}

	// Get sorted keys for deterministic ordering
	keys := make([]string, 0, len(bodyParams))
	for k := range bodyParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	params := make([]map[string]any, 0, len(bodyParams))
	for _, key := range keys {
		param := map[string]any{
			"type": "text",
			"text": bodyParams[key],
		}
		if isNamedParams {
			param["parameter_name"] = key
		}
		params = append(params, param)
	}

	return []map[string]any{
		{
			"type":       "body",
			"parameters": params,
		},
	}
}

// BuildTemplateComponents builds the full WhatsApp template components array,
// including an optional header component (for IMAGE/VIDEO/DOCUMENT) and body parameters.
func BuildTemplateComponents(bodyParams map[string]string, headerType string, headerMediaID string) []map[string]any {
	var components []map[string]any

	// Add header component if media is provided
	if headerMediaID != "" {
		mediaType := strings.ToLower(headerType) // "image", "video", "document"
		headerParam := map[string]any{
			"type": mediaType,
			mediaType: map[string]any{
				"id": headerMediaID,
			},
		}
		components = append(components, map[string]any{
			"type":       "header",
			"parameters": []map[string]any{headerParam},
		})
	}

	// Add body component with text parameters
	bodyComponents := BodyParamsToComponents(bodyParams)
	components = append(components, bodyComponents...)

	if len(components) == 0 {
		return nil
	}
	return components
}

// AutoButtonComponents generates button components for button types that require
// server-generated parameters (FLOW needs flow_token, OTP needs the code).
// These are auto-generated and don't require user input.
func AutoButtonComponents(templateButtons []any) []map[string]any {
	if len(templateButtons) == 0 {
		return nil
	}

	var components []map[string]any
	for i, raw := range templateButtons {
		btn, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		t, _ := btn["type"].(string)
		t = strings.ToUpper(t)

		switch t {
		case "FLOW":
			components = append(components, map[string]any{
				"type":     "button",
				"sub_type": "flow",
				"index":    fmt.Sprintf("%d", i),
				"parameters": []map[string]any{
					{
						"type": "action",
						"action": map[string]any{
							"flow_token": fmt.Sprintf("flow_%d", time.Now().UnixNano()),
						},
					},
				},
			})
		}
	}
	return components
}

// ButtonURLParamsToComponents converts button parameters to WhatsApp API button components.
// buttonParams maps button index (as string like "0", "1") to the dynamic parameter value.
// templateButtons is the JSONB buttons array from the template, used to determine button type.
// URL buttons produce: {"type": "button", "sub_type": "url", "index": "0", "parameters": [{"type": "text", "text": "value"}]}
// COPY_CODE buttons produce: {"type": "button", "sub_type": "copy_code", "index": "0", "parameters": [{"type": "coupon_code", "coupon_code": "value"}]}
func ButtonURLParamsToComponents(buttonParams map[string]string, templateButtons ...[]any) []map[string]any {
	if len(buttonParams) == 0 {
		return nil
	}

	// Build a lookup of button index -> effective type from template buttons.
	// OTP buttons resolve to their otp_type (COPY_CODE, ONE_TAP, ZERO_TAP)
	// so the message sending logic handles them correctly.
	// btnIsOTP tracks whether the button was originally an OTP button (auth templates
	// need sub_type "url" instead of "copy_code").
	btnTypes := map[string]string{}
	btnIsOTP := map[string]bool{}
	if len(templateButtons) > 0 {
		for i, raw := range templateButtons[0] {
			if btn, ok := raw.(map[string]any); ok {
				if t, ok := btn["type"].(string); ok {
					key := fmt.Sprintf("%d", i)
					effectiveType := strings.ToUpper(t)
					if effectiveType == "OTP" {
						btnIsOTP[key] = true
						if otpType, ok := btn["otp_type"].(string); ok {
							effectiveType = strings.ToUpper(otpType)
						}
					}
					btnTypes[key] = effectiveType
				}
			}
		}
	}

	keys := make([]string, 0, len(buttonParams))
	for k := range buttonParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	components := make([]map[string]any, 0, len(buttonParams))
	for _, index := range keys {
		value := buttonParams[index]
		// Skip button types that don't accept dynamic parameters
		if t := btnTypes[index]; t == "QUICK_REPLY" || t == "FLOW" || t == "PHONE_NUMBER" || t == "VOICE_CALL" || t == "ONE_TAP" || t == "ZERO_TAP" {
			continue
		}
		if btnTypes[index] == "COPY_CODE" && !btnIsOTP[index] {
			// Regular COPY_CODE button (e.g. coupon codes)
			components = append(components, map[string]any{
				"type":     "button",
				"sub_type": "copy_code",
				"index":    index,
				"parameters": []map[string]any{
					{"type": "coupon_code", "coupon_code": value},
				},
			})
		} else {
			components = append(components, map[string]any{
				"type":     "button",
				"sub_type": "url",
				"index":    index,
				"parameters": []map[string]any{
					{"type": "text", "text": value},
				},
			})
		}
	}
	return components
}

// SendFlowMessage sends an interactive WhatsApp Flow message
// flowID is the Meta Flow ID, headerText is optional header, bodyText is the message body,
// ctaText is the button text, flowToken is a unique token for tracking the flow response,
// and firstScreen is the name of the first screen to navigate to
func (c *Client) SendFlowMessage(ctx context.Context, account *Account, rcpt Recipient, flowID, headerText, bodyText, ctaText, flowToken, firstScreen string) (string, error) {
	if flowID == "" {
		return "", fmt.Errorf("flow ID is required")
	}
	if bodyText == "" {
		return "", fmt.Errorf("body text is required")
	}
	if ctaText == "" {
		ctaText = "Open" // Default CTA text
	}
	if flowToken == "" {
		flowToken = fmt.Sprintf("flow_%d", time.Now().UnixNano())
	}
	if firstScreen == "" {
		firstScreen = "FIRST_SCREEN" // Default fallback
	}

	// Truncate CTA text to 20 chars (WhatsApp limit)
	if len(ctaText) > 20 {
		ctaText = ctaText[:20]
	}

	interactive := map[string]any{
		"type": "flow",
		"body": map[string]any{
			"text": bodyText,
		},
		"action": map[string]any{
			"name": "flow",
			"parameters": map[string]any{
				"flow_message_version": "3",
				"flow_token":           flowToken,
				"flow_id":              flowID,
				"flow_cta":             ctaText,
				"flow_action":          "navigate",
				"flow_action_payload": map[string]any{
					"screen": firstScreen,
				},
			},
		},
	}

	// Add header if provided
	if headerText != "" {
		interactive["header"] = map[string]any{
			"type": "text",
			"text": headerText,
		}
	}

	payload := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"type":              "interactive",
		"interactive":       interactive,
	}
	rcpt.SetOnPayload(payload)

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending flow message", "phone", rcpt.Phone, "flow_id", flowID)

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to send flow message", "error", err, "phone", rcpt.Phone, "flow_id", flowID)
		return "", fmt.Errorf("failed to send flow message: %w", err)
	}

	var resp MetaAPIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Messages) == 0 {
		return "", fmt.Errorf("no message ID in response")
	}

	messageID := resp.Messages[0].ID
	c.Log.Info("Flow message sent", "message_id", messageID, "phone", rcpt.Phone, "flow_id", flowID)
	return messageID, nil
}

// SendTemplateMessage sends a template message with optional components (header, body, buttons, etc.)
func (c *Client) SendTemplateMessage(ctx context.Context, account *Account, rcpt Recipient, templateName, languageCode string, components []map[string]any) (string, error) {
	template := map[string]any{
		"name": templateName,
		"language": map[string]any{
			"code": languageCode,
		},
	}

	if len(components) > 0 {
		template["components"] = components
	}

	payload := map[string]any{
		"messaging_product": "whatsapp",
		"type":              "template",
		"template":          template,
	}
	rcpt.SetOnPayload(payload)

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending template message with components", "phone", rcpt.Phone, "template", templateName)

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to send template message", "error", err, "phone", rcpt.Phone, "template", templateName)
		return "", fmt.Errorf("failed to send template message: %w", err)
	}

	var resp MetaAPIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Messages) == 0 {
		return "", fmt.Errorf("no message ID in response")
	}

	messageID := resp.Messages[0].ID
	c.Log.Info("Template message sent", "message_id", messageID, "phone", rcpt.Phone, "template", templateName)
	return messageID, nil
}
