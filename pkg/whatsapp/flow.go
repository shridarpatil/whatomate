package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
)

// FlowCreateRequest represents the request to create a flow
type FlowCreateRequest struct {
	Name       string   `json:"name"`
	Categories []string `json:"categories"`
}

// FlowCreateResponse represents the response from creating a flow
type FlowCreateResponse struct {
	ID string `json:"id"`
}

// FlowUpdateResponse represents the response from updating flow assets
type FlowUpdateResponse struct {
	Success          bool     `json:"success"`
	ValidationErrors []string `json:"validation_errors,omitempty"`
}

// FlowPublishResponse represents the response from publishing a flow
type FlowPublishResponse struct {
	Success bool `json:"success"`
}

// FlowGetResponse represents a flow fetched from Meta
type FlowGetResponse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Categories []string `json:"categories"`
	PreviewURL string   `json:"preview_url,omitempty"`
}

// FlowListResponse represents the response from listing flows
type FlowListResponse struct {
	Data   []FlowGetResponse `json:"data"`
	Paging struct {
		Cursors struct {
			Before string `json:"before"`
			After  string `json:"after"`
		} `json:"cursors"`
	} `json:"paging"`
}

// FlowJSON represents the flow definition
type FlowJSON struct {
	Version       string        `json:"version"`
	DataAPIVersion string       `json:"data_api_version,omitempty"`
	RoutingModel  map[string]interface{} `json:"routing_model,omitempty"`
	Screens       []interface{} `json:"screens"`
}

// CreateFlow creates a new flow in Meta
func (c *Client) CreateFlow(ctx context.Context, account *Account, name string, categories []string) (string, error) {
	url := c.buildFlowsURL(account)

	payload := FlowCreateRequest{
		Name:       name,
		Categories: categories,
	}

	c.Log.Info("Creating flow in Meta", "name", name, "categories", categories, "url", url, "business_id", account.BusinessID)

	respBody, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to create flow", "error", err, "name", name, "url", url)
		return "", err
	}

	var result FlowCreateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	c.Log.Info("Flow created in Meta", "flow_id", result.ID, "name", name)
	return result.ID, nil
}

// UpdateFlowJSON updates the flow's JSON definition
// This uses multipart form upload as required by Meta's API
func (c *Client) UpdateFlowJSON(ctx context.Context, account *Account, flowID string, flowJSON *FlowJSON) error {
	url := fmt.Sprintf("%s/%s/%s/assets", BaseURL, account.APIVersion, flowID)

	// Convert flow JSON to bytes
	jsonBytes, err := json.Marshal(flowJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal flow JSON: %w", err)
	}

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add the file field with correct Content-Type (application/json)
	// Using CreatePart instead of CreateFormFile to set the correct MIME type
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="flow.json"`)
	h.Set("Content-Type", "application/json")
	part, err := writer.CreatePart(h)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(jsonBytes); err != nil {
		return fmt.Errorf("failed to write flow JSON: %w", err)
	}

	// Add the name field
	if err := writer.WriteField("name", "flow.json"); err != nil {
		return fmt.Errorf("failed to write name field: %w", err)
	}

	// Add asset_type field
	if err := writer.WriteField("asset_type", "FLOW_JSON"); err != nil {
		return fmt.Errorf("failed to write asset_type field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+account.AccessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c.Log.Info("Updating flow JSON", "flow_id", flowID)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr MetaAPIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error.Message != "" {
			return fmt.Errorf("API error %d: %s", apiErr.Error.Code, apiErr.Error.Message)
		}
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result FlowUpdateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		if len(result.ValidationErrors) > 0 {
			return fmt.Errorf("flow validation errors: %v", result.ValidationErrors)
		}
		return fmt.Errorf("failed to update flow JSON")
	}

	c.Log.Info("Flow JSON updated", "flow_id", flowID)
	return nil
}

// PublishFlow publishes a draft flow
func (c *Client) PublishFlow(ctx context.Context, account *Account, flowID string) error {
	url := fmt.Sprintf("%s/%s/%s/publish", BaseURL, account.APIVersion, flowID)

	c.Log.Info("Publishing flow", "flow_id", flowID)

	respBody, err := c.doRequest(ctx, http.MethodPost, url, nil, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to publish flow", "error", err, "flow_id", flowID)
		return err
	}

	var result FlowPublishResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to publish flow")
	}

	c.Log.Info("Flow published", "flow_id", flowID)
	return nil
}

// DeprecateFlow deprecates a published flow
func (c *Client) DeprecateFlow(ctx context.Context, account *Account, flowID string) error {
	url := fmt.Sprintf("%s/%s/%s/deprecate", BaseURL, account.APIVersion, flowID)

	c.Log.Info("Deprecating flow", "flow_id", flowID)

	respBody, err := c.doRequest(ctx, http.MethodPost, url, nil, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to deprecate flow", "error", err, "flow_id", flowID)
		return err
	}

	var result FlowPublishResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to deprecate flow")
	}

	c.Log.Info("Flow deprecated", "flow_id", flowID)
	return nil
}

// DeleteFlow deletes a flow from Meta
func (c *Client) DeleteFlow(ctx context.Context, account *Account, flowID string) error {
	url := fmt.Sprintf("%s/%s/%s", BaseURL, account.APIVersion, flowID)

	c.Log.Info("Deleting flow from Meta", "flow_id", flowID)

	_, err := c.doRequest(ctx, http.MethodDelete, url, nil, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to delete flow", "error", err, "flow_id", flowID)
		return err
	}

	c.Log.Info("Flow deleted from Meta", "flow_id", flowID)
	return nil
}

// GetFlow fetches a single flow from Meta
func (c *Client) GetFlow(ctx context.Context, account *Account, flowID string) (*FlowGetResponse, error) {
	url := fmt.Sprintf("%s/%s/%s?fields=id,name,status,categories,preview.invalidate(false)", BaseURL, account.APIVersion, flowID)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to get flow", "error", err, "flow_id", flowID)
		return nil, err
	}

	var result FlowGetResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// FlowAssetsResponse represents the response from getting flow assets
type FlowAssetsResponse struct {
	Data []struct {
		Name      string `json:"name"`
		AssetType string `json:"asset_type"`
		Download  string `json:"download_url"`
	} `json:"data"`
}

// GetFlowAssets fetches the flow JSON assets from Meta
func (c *Client) GetFlowAssets(ctx context.Context, account *Account, flowID string) (*FlowJSON, error) {
	// First get the assets list to find the download URL
	assetsURL := fmt.Sprintf("%s/%s/%s/assets", BaseURL, account.APIVersion, flowID)

	c.Log.Info("Fetching flow assets", "flow_id", flowID, "url", assetsURL)

	respBody, err := c.doRequest(ctx, http.MethodGet, assetsURL, nil, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to get flow assets", "error", err, "flow_id", flowID)
		return nil, err
	}

	var assetsResp FlowAssetsResponse
	if err := json.Unmarshal(respBody, &assetsResp); err != nil {
		return nil, fmt.Errorf("failed to parse assets response: %w", err)
	}

	// Find the FLOW_JSON asset
	var downloadURL string
	for _, asset := range assetsResp.Data {
		if asset.AssetType == "FLOW_JSON" {
			downloadURL = asset.Download
			break
		}
	}

	if downloadURL == "" {
		c.Log.Info("No FLOW_JSON asset found for flow", "flow_id", flowID)
		return nil, nil // No flow JSON yet
	}

	// Download the flow JSON
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download flow JSON: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	flowJSONBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read flow JSON: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.Log.Error("Failed to download flow JSON", "status", resp.StatusCode, "body", string(flowJSONBody))
		return nil, fmt.Errorf("failed to download flow JSON: status %d", resp.StatusCode)
	}

	var flowJSON FlowJSON
	if err := json.Unmarshal(flowJSONBody, &flowJSON); err != nil {
		return nil, fmt.Errorf("failed to parse flow JSON: %w", err)
	}

	c.Log.Info("Fetched flow JSON", "flow_id", flowID, "screens_count", len(flowJSON.Screens))
	return &flowJSON, nil
}

// ListFlows fetches all flows from Meta
func (c *Client) ListFlows(ctx context.Context, account *Account) ([]FlowGetResponse, error) {
	url := fmt.Sprintf("%s?fields=id,name,status,categories,preview.invalidate(false)", c.buildFlowsURL(account))

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil, account.AccessToken)
	if err != nil {
		c.Log.Error("Failed to list flows", "error", err)
		return nil, err
	}

	var result FlowListResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	c.Log.Info("Fetched flows from Meta", "count", len(result.Data))
	return result.Data, nil
}

// buildFlowsURL builds the flows endpoint URL
func (c *Client) buildFlowsURL(account *Account) string {
	return fmt.Sprintf("%s/%s/%s/flows", BaseURL, account.APIVersion, account.BusinessID)
}
