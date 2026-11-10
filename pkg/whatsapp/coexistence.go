package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Sync types for POST /{phone_number_id}/smb_app_data. Both are one-time
// requests that must be made within 24 hours of onboarding a number that
// stays on the WhatsApp Business app (coexistence).
const (
	// SMBSyncTypeContacts asks Meta to send the Business app address book
	// as smb_app_state_sync webhooks.
	SMBSyncTypeContacts = "smb_app_state_sync"
	// SMBSyncTypeHistory asks Meta to send up to 180 days of chat history
	// as history webhooks, if the business agreed to share it.
	SMBSyncTypeHistory = "history"
)

// SMBAppDataResponse is the response from the smb_app_data endpoint.
type SMBAppDataResponse struct {
	MessagingProduct string `json:"messaging_product"`
	RequestID        string `json:"request_id"`
}

// RequestSMBAppDataSync starts a coexistence contacts or history sync for the
// account's phone number and returns Meta's request ID.
func (c *Client) RequestSMBAppDataSync(ctx context.Context, account *Account, syncType string) (string, error) {
	if syncType != SMBSyncTypeContacts && syncType != SMBSyncTypeHistory {
		return "", fmt.Errorf("unsupported sync type %q", syncType)
	}

	url := fmt.Sprintf("%s/%s/%s/smb_app_data", c.getBaseURL(), account.APIVersion, account.PhoneID)
	payload := map[string]string{
		"messaging_product": "whatsapp",
		"sync_type":         syncType,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return "", fmt.Errorf("smb_app_data %s sync failed: %w", syncType, err)
	}

	var resp SMBAppDataResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse smb_app_data response: %w", err)
	}

	c.Log.Info("Requested coexistence sync", "phone_id", account.PhoneID, "sync_type", syncType, "request_id", resp.RequestID)
	return resp.RequestID, nil
}
