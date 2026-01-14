package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// SendTextMessage sends a text message to a phone number
func (c *Client) SendTextMessage(ctx context.Context, account *Account, phoneNumber, text string) (string, error) {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                phoneNumber,
		"type":              "text",
		"text": map[string]interface{}{
			"preview_url": false,
			"body":        text,
		},
	}

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending text message", "phone", phoneNumber, "url", url)

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to send text message", "error", err, "phone", phoneNumber)
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
	c.Log.Info("Text message sent", "message_id", messageID, "phone", phoneNumber)
	return messageID, nil
}

// SendInteractiveButtons sends an interactive message with buttons or list
// If buttons <= 3, sends as buttons; if 4-10, sends as list
func (c *Client) SendInteractiveButtons(ctx context.Context, account *Account, phoneNumber, bodyText string, buttons []Button) (string, error) {
	if len(buttons) == 0 {
		return "", fmt.Errorf("at least one button is required")
	}
	if len(buttons) > 10 {
		return "", fmt.Errorf("maximum 10 buttons allowed")
	}

	var interactive map[string]interface{}

	if len(buttons) <= 3 {
		// Use button format
		buttonsList := make([]map[string]interface{}, 0, len(buttons))
		for _, btn := range buttons {
			title := btn.Title
			if len(title) > 20 {
				title = title[:20]
			}
			buttonsList = append(buttonsList, map[string]interface{}{
				"type": "reply",
				"reply": map[string]interface{}{
					"id":    btn.ID,
					"title": title,
				},
			})
		}

		interactive = map[string]interface{}{
			"type": "button",
			"body": map[string]interface{}{
				"text": bodyText,
			},
			"action": map[string]interface{}{
				"buttons": buttonsList,
			},
		}
	} else {
		// Use list format for 4-10 items
		rows := make([]map[string]interface{}, 0, len(buttons))
		for _, btn := range buttons {
			title := btn.Title
			if len(title) > 24 {
				title = title[:24]
			}
			rows = append(rows, map[string]interface{}{
				"id":    btn.ID,
				"title": title,
			})
		}

		interactive = map[string]interface{}{
			"type": "list",
			"body": map[string]interface{}{
				"text": bodyText,
			},
			"action": map[string]interface{}{
				"button": "Select an option",
				"sections": []map[string]interface{}{
					{
						"title": "Options",
						"rows":  rows,
					},
				},
			},
		}
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                phoneNumber,
		"type":              "interactive",
		"interactive":       interactive,
	}

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending interactive message", "phone", phoneNumber, "button_count", len(buttons))

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to send interactive message", "error", err, "phone", phoneNumber)
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
	c.Log.Info("Interactive message sent", "message_id", messageID, "phone", phoneNumber)
	return messageID, nil
}

// SendCTAURLButton sends an interactive message with a CTA URL button
// This opens a URL when clicked instead of sending a reply
func (c *Client) SendCTAURLButton(ctx context.Context, account *Account, phoneNumber, bodyText, buttonText, url string) (string, error) {
	if buttonText == "" || url == "" {
		return "", fmt.Errorf("button text and URL are required")
	}

	// Truncate button text to 20 chars (WhatsApp limit)
	if len(buttonText) > 20 {
		buttonText = buttonText[:20]
	}

	interactive := map[string]interface{}{
		"type": "cta_url",
		"body": map[string]interface{}{
			"text": bodyText,
		},
		"action": map[string]interface{}{
			"name": "cta_url",
			"parameters": map[string]interface{}{
				"display_text": buttonText,
				"url":          url,
			},
		},
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                phoneNumber,
		"type":              "interactive",
		"interactive":       interactive,
	}

	apiURL := c.buildMessagesURL(account)
	c.Log.Debug("Sending CTA URL button message", "phone", phoneNumber, "url", url)

	respBody, err := c.doRequest(ctx, "POST", apiURL, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to send CTA URL button message", "error", err, "phone", phoneNumber)
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
	c.Log.Info("CTA URL button message sent", "message_id", messageID, "phone", phoneNumber)
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
func (c *Client) SendTemplateMessage(ctx context.Context, account *Account, phoneNumber, templateName, languageCode string, bodyParams []string) (string, error) {
	template := map[string]interface{}{
		"name": templateName,
		"language": map[string]interface{}{
			"code": languageCode,
		},
	}

	// Add body parameters if provided
	if len(bodyParams) > 0 {
		params := make([]map[string]interface{}, 0, len(bodyParams))
		for _, p := range bodyParams {
			params = append(params, map[string]interface{}{
				"type": "text",
				"text": p,
			})
		}
		template["components"] = []map[string]interface{}{
			{
				"type":       "body",
				"parameters": params,
			},
		}
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                phoneNumber,
		"type":              "template",
		"template":          template,
	}

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending template message", "phone", phoneNumber, "template", templateName)

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to send template message", "error", err, "phone", phoneNumber, "template", templateName)
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
	c.Log.Info("Template message sent", "message_id", messageID, "phone", phoneNumber, "template", templateName)
	return messageID, nil
}

// SendFlowMessage sends an interactive WhatsApp Flow message
// flowID is the Meta Flow ID, headerText is optional header, bodyText is the message body,
// ctaText is the button text, flowToken is a unique token for tracking the flow response,
// and firstScreen is the name of the first screen to navigate to
func (c *Client) SendFlowMessage(ctx context.Context, account *Account, phoneNumber, flowID, headerText, bodyText, ctaText, flowToken, firstScreen string) (string, error) {
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

	interactive := map[string]interface{}{
		"type": "flow",
		"body": map[string]interface{}{
			"text": bodyText,
		},
		"action": map[string]interface{}{
			"name": "flow",
			"parameters": map[string]interface{}{
				"flow_message_version": "3",
				"flow_token":           flowToken,
				"flow_id":              flowID,
				"flow_cta":             ctaText,
				"flow_action":          "navigate",
				"flow_action_payload": map[string]interface{}{
					"screen": firstScreen,
				},
			},
		},
	}

	// Add header if provided
	if headerText != "" {
		interactive["header"] = map[string]interface{}{
			"type": "text",
			"text": headerText,
		}
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                phoneNumber,
		"type":              "interactive",
		"interactive":       interactive,
	}

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending flow message", "phone", phoneNumber, "flow_id", flowID)

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to send flow message", "error", err, "phone", phoneNumber, "flow_id", flowID)
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
	c.Log.Info("Flow message sent", "message_id", messageID, "phone", phoneNumber, "flow_id", flowID)
	return messageID, nil
}

// SendTemplateMessageWithComponents sends a template message with full component control
func (c *Client) SendTemplateMessageWithComponents(ctx context.Context, account *Account, phoneNumber, templateName, languageCode string, components []map[string]interface{}) (string, error) {
	template := map[string]interface{}{
		"name": templateName,
		"language": map[string]interface{}{
			"code": languageCode,
		},
	}

	if len(components) > 0 {
		template["components"] = components
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                phoneNumber,
		"type":              "template",
		"template":          template,
	}

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending template message with components", "phone", phoneNumber, "template", templateName)

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to send template message", "error", err, "phone", phoneNumber, "template", templateName)
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
	c.Log.Info("Template message sent", "message_id", messageID, "phone", phoneNumber, "template", templateName)
	return messageID, nil
}
