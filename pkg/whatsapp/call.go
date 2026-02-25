package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// buildCallsURL builds the calls endpoint URL for the WhatsApp Calling API
func (c *Client) buildCallsURL(account *Account) string {
	return fmt.Sprintf("%s/%s/%s/calls", c.getBaseURL(), account.APIVersion, account.PhoneID)
}

// PreAcceptCall sends the SDP answer to Meta as a pre-accept signal.
// Per the WhatsApp Business Calling API, pre_accept requires the session object
// with the SDP answer to keep the call alive while WebRTC is finalized.
func (c *Client) PreAcceptCall(ctx context.Context, account *Account, callID, sdpAnswer string) error {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"call_id":           callID,
		"action":            "pre_accept",
		"session": map[string]string{
			"sdp_type": "answer",
			"sdp":      sdpAnswer,
		},
	}

	url := c.buildCallsURL(account)
	c.Log.Info("Pre-accepting call", "call_id", callID)

	_, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to pre-accept call: %w", err)
	}

	c.Log.Info("Call pre-accepted", "call_id", callID)
	return nil
}

// AcceptCall accepts an incoming call by sending our SDP answer.
// Per the WhatsApp Business Calling API, accept uses the same session object format.
// The API returns { success: true } on success.
func (c *Client) AcceptCall(ctx context.Context, account *Account, callID, sdpAnswer string) error {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"call_id":           callID,
		"action":            "accept",
		"session": map[string]string{
			"sdp_type": "answer",
			"sdp":      sdpAnswer,
		},
	}

	url := c.buildCallsURL(account)
	c.Log.Info("Accepting call", "call_id", callID)

	_, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to accept call: %w", err)
	}

	c.Log.Info("Call accepted", "call_id", callID)
	return nil
}

// RejectCall rejects an incoming call.
func (c *Client) RejectCall(ctx context.Context, account *Account, callID string) error {
	payload := map[string]string{
		"messaging_product": "whatsapp",
		"call_id":           callID,
		"action":            "reject",
	}

	url := c.buildCallsURL(account)
	c.Log.Info("Rejecting call", "call_id", callID)

	_, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to reject call: %w", err)
	}

	c.Log.Info("Call rejected", "call_id", callID)
	return nil
}

// SendCallPermissionRequest sends an interactive call_permission_request message
// to the consumer. The consumer must accept before outgoing calls can be placed.
// Permission is valid for 72 hours once accepted.
func (c *Client) SendCallPermissionRequest(ctx context.Context, account *Account, phoneNumber, bodyText string) (string, error) {
	if bodyText == "" {
		bodyText = "We'd like to call you to assist with your query."
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                phoneNumber,
		"type":              "interactive",
		"interactive": map[string]interface{}{
			"type": "call_permission_request",
			"action": map[string]string{
				"name": "call_permission_request",
			},
			"body": map[string]string{
				"text": bodyText,
			},
		},
	}

	url := c.buildMessagesURL(account)
	c.Log.Info("Sending call permission request", "phone", phoneNumber)

	respBody, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to send call permission request: %w", err)
	}

	// Parse message ID from response
	var resp struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if parseErr := json.Unmarshal(respBody, &resp); parseErr == nil && len(resp.Messages) > 0 {
		c.Log.Info("Call permission request sent", "phone", phoneNumber, "message_id", resp.Messages[0].ID)
		return resp.Messages[0].ID, nil
	}

	return "", nil
}

// GetCallPermission checks the current call permission state for a user.
// Returns the permission status ("no_permission", "temporary", "permanent").
func (c *Client) GetCallPermission(ctx context.Context, account *Account, userPhone string) (string, error) {
	url := fmt.Sprintf("%s/%s/%s/call_permissions?user_wa_id=%s",
		c.getBaseURL(), account.APIVersion, account.PhoneID, userPhone)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil, account.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to get call permission: %w", err)
	}

	var resp struct {
		Permission struct {
			Status string `json:"status"`
		} `json:"permission"`
	}
	if parseErr := json.Unmarshal(respBody, &resp); parseErr != nil {
		return "", fmt.Errorf("failed to parse call permission response: %w", parseErr)
	}

	return resp.Permission.Status, nil
}

// InitiateCall places an outgoing call to a WhatsApp user with an SDP offer.
// Returns the call_id assigned by WhatsApp on success.
func (c *Client) InitiateCall(ctx context.Context, account *Account, phoneNumber, sdpOffer string) (string, error) {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                phoneNumber,
		"action":            "connect",
		"session": map[string]string{
			"sdp_type": "offer",
			"sdp":      sdpOffer,
		},
	}

	url := c.buildCallsURL(account)
	c.Log.Info("Initiating outgoing call", "phone", phoneNumber)

	respBody, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to initiate call: %w", err)
	}

	// Parse call ID from response: {"calls": [{"id": "wacid.xxx"}]}
	var resp struct {
		Calls []struct {
			ID string `json:"id"`
		} `json:"calls"`
	}
	if parseErr := json.Unmarshal(respBody, &resp); parseErr != nil || len(resp.Calls) == 0 || resp.Calls[0].ID == "" {
		return "", fmt.Errorf("failed to parse call_id from response: %s", string(respBody))
	}

	c.Log.Info("Outgoing call initiated", "phone", phoneNumber, "call_id", resp.Calls[0].ID)
	return resp.Calls[0].ID, nil
}

// TerminateCall terminates an active call.
func (c *Client) TerminateCall(ctx context.Context, account *Account, callID string) error {
	payload := map[string]string{
		"messaging_product": "whatsapp",
		"call_id":           callID,
		"action":            "terminate",
	}

	url := c.buildCallsURL(account)
	c.Log.Info("Terminating call", "call_id", callID)

	_, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to terminate call: %w", err)
	}

	c.Log.Info("Call terminated", "call_id", callID)
	return nil
}
