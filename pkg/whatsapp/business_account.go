package whatsapp

import (
	"context"
	"fmt"
	"net/http"
)

// BusinessAccountInfo is the subset of WhatsApp Business Account fields we read
// from the Meta Graph API. Currency is the ISO 4217 code the WABA is billed in
// and is what Meta's pricing and template analytics costs are denominated in.
type BusinessAccountInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Currency   string `json:"currency"`
	TimezoneID string `json:"timezone_id"`
}

// GetBusinessAccountInfo fetches the WABA's name, billing currency and timezone.
func (c *Client) GetBusinessAccountInfo(ctx context.Context, account *Account) (*BusinessAccountInfo, error) {
	url := fmt.Sprintf("%s/%s/%s?fields=currency,timezone_id,name",
		c.getBaseURL(), account.APIVersion, account.BusinessID)

	info, err := doJSON[BusinessAccountInfo](ctx, c, http.MethodGet, url, nil, account.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch business account info: %w", err)
	}

	return &info, nil
}
