package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zerodha/logf"
)

const (
	// DefaultTimeout for HTTP requests
	DefaultTimeout = 30 * time.Second
	// BaseURL for Meta Graph API
	BaseURL = "https://graph.facebook.com"
)

// Client is the WhatsApp Cloud API client
type Client struct {
	HTTPClient *http.Client
	Log        logf.Logger
	baseURL    string // For testing with mock servers
}

// New creates a new WhatsApp client
func New(log logf.Logger) *Client {
	return &Client{
		HTTPClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		Log:     log,
		baseURL: BaseURL,
	}
}

// NewWithTimeout creates a new WhatsApp client with custom timeout
func NewWithTimeout(log logf.Logger, timeout time.Duration) *Client {
	return &Client{
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		Log:     log,
		baseURL: BaseURL,
	}
}

// NewWithBaseURL creates a new WhatsApp client with a custom base URL (for testing)
func NewWithBaseURL(log logf.Logger, baseURL string) *Client {
	return &Client{
		HTTPClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		Log:     log,
		baseURL: baseURL,
	}
}

// getBaseURL returns the base URL for API requests
func (c *Client) getBaseURL() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return BaseURL
}

// doRequest performs an HTTP request to the Meta API
func (c *Client) doRequest(ctx context.Context, method, url string, body interface{}, accessToken string) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr MetaAPIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error.Message != "" {
			errMsg := fmt.Sprintf("API error %d: %s", apiErr.Error.Code, apiErr.Error.Message)
			if apiErr.Error.ErrorData.Details != "" {
				errMsg += " - Details: " + apiErr.Error.ErrorData.Details
			}
			if apiErr.Error.ErrorUserMsg != "" {
				errMsg += " - " + apiErr.Error.ErrorUserMsg
			}
			return nil, fmt.Errorf("%s", errMsg)
		}
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// buildMessagesURL builds the messages endpoint URL
func (c *Client) buildMessagesURL(account *Account) string {
	return fmt.Sprintf("%s/%s/%s/messages", c.getBaseURL(), account.APIVersion, account.PhoneID)
}

// buildTemplatesURL builds the message_templates endpoint URL
func (c *Client) buildTemplatesURL(account *Account) string {
	return fmt.Sprintf("%s/%s/%s/message_templates", c.getBaseURL(), account.APIVersion, account.BusinessID)
}

// MediaURLResponse represents the response from Meta's media endpoint
type MediaURLResponse struct {
	URL           string `json:"url"`
	MimeType      string `json:"mime_type"`
	SHA256        string `json:"sha256"`
	FileSize      int64  `json:"file_size"`
	MessagingProduct string `json:"messaging_product"`
}

// GetMediaURL retrieves the download URL for a media file from Meta's API
func (c *Client) GetMediaURL(ctx context.Context, mediaID string, account *Account) (string, error) {
	url := fmt.Sprintf("%s/%s/%s", c.getBaseURL(), account.APIVersion, mediaID)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil, account.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to get media URL: %w", err)
	}

	var mediaResp MediaURLResponse
	if err := json.Unmarshal(respBody, &mediaResp); err != nil {
		return "", fmt.Errorf("failed to parse media response: %w", err)
	}

	if mediaResp.URL == "" {
		return "", fmt.Errorf("no URL in media response")
	}

	return mediaResp.URL, nil
}

// DownloadMedia downloads media content from Meta's CDN URL
func (c *Client) DownloadMedia(ctx context.Context, mediaURL string, accessToken string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	// Meta requires Bearer token for media download
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download media: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("media download failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read media content: %w", err)
	}

	return data, nil
}

// UploadMediaResponse represents the response from uploading media
type UploadMediaResponse struct {
	ID string `json:"id"`
}

// UploadMedia uploads media to WhatsApp's servers and returns the media ID
func (c *Client) UploadMedia(ctx context.Context, account *Account, data []byte, mimeType, filename string) (string, error) {
	url := fmt.Sprintf("%s/%s/%s/media", c.getBaseURL(), account.APIVersion, account.PhoneID)

	// Create multipart form body
	body := &bytes.Buffer{}
	boundary := "----WebKitFormBoundary7MA4YWxkTrZu0gW"

	// Build multipart body manually
	fmt.Fprintf(body, "--%s\r\n", boundary)
	body.WriteString("Content-Disposition: form-data; name=\"messaging_product\"\r\n\r\n")
	body.WriteString("whatsapp\r\n")

	fmt.Fprintf(body, "--%s\r\n", boundary)
	fmt.Fprintf(body, "Content-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\n", filename)
	fmt.Fprintf(body, "Content-Type: %s\r\n\r\n", mimeType)
	body.Write(data)
	body.WriteString("\r\n")

	fmt.Fprintf(body, "--%s--\r\n", boundary)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+account.AccessToken)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload media: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read upload response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("media upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var uploadResp UploadMediaResponse
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return "", fmt.Errorf("failed to parse upload response: %w", err)
	}

	if uploadResp.ID == "" {
		return "", fmt.Errorf("no media ID in upload response")
	}

	c.Log.Info("Media uploaded", "media_id", uploadResp.ID)
	return uploadResp.ID, nil
}

// SendImageMessage sends an image message using a media ID
func (c *Client) SendImageMessage(ctx context.Context, account *Account, phoneNumber, mediaID, caption string) (string, error) {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                phoneNumber,
		"type":              "image",
		"image": map[string]interface{}{
			"id":      mediaID,
			"caption": caption,
		},
	}

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending image message", "phone", phoneNumber, "media_id", mediaID)

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to send image message: %w", err)
	}

	var resp MetaAPIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Messages) == 0 {
		return "", fmt.Errorf("no message ID in response")
	}

	messageID := resp.Messages[0].ID
	c.Log.Info("Image message sent", "message_id", messageID, "phone", phoneNumber)
	return messageID, nil
}

// SendDocumentMessage sends a document message using a media ID
func (c *Client) SendDocumentMessage(ctx context.Context, account *Account, phoneNumber, mediaID, filename, caption string) (string, error) {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                phoneNumber,
		"type":              "document",
		"document": map[string]interface{}{
			"id":       mediaID,
			"filename": filename,
			"caption":  caption,
		},
	}

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending document message", "phone", phoneNumber, "media_id", mediaID)

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to send document message: %w", err)
	}

	var resp MetaAPIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Messages) == 0 {
		return "", fmt.Errorf("no message ID in response")
	}

	messageID := resp.Messages[0].ID
	c.Log.Info("Document message sent", "message_id", messageID, "phone", phoneNumber)
	return messageID, nil
}

// SendVideoMessage sends a video message using a media ID
func (c *Client) SendVideoMessage(ctx context.Context, account *Account, phoneNumber, mediaID, caption string) (string, error) {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                phoneNumber,
		"type":              "video",
		"video": map[string]interface{}{
			"id":      mediaID,
			"caption": caption,
		},
	}

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending video message", "phone", phoneNumber, "media_id", mediaID)

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to send video message: %w", err)
	}

	var resp MetaAPIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Messages) == 0 {
		return "", fmt.Errorf("no message ID in response")
	}

	messageID := resp.Messages[0].ID
	c.Log.Info("Video message sent", "message_id", messageID, "phone", phoneNumber)
	return messageID, nil
}

// SendAudioMessage sends an audio message using a media ID
func (c *Client) SendAudioMessage(ctx context.Context, account *Account, phoneNumber, mediaID string) (string, error) {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                phoneNumber,
		"type":              "audio",
		"audio": map[string]interface{}{
			"id": mediaID,
		},
	}

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending audio message", "phone", phoneNumber, "media_id", mediaID)

	respBody, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to send audio message: %w", err)
	}

	var resp MetaAPIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Messages) == 0 {
		return "", fmt.Errorf("no message ID in response")
	}

	messageID := resp.Messages[0].ID
	c.Log.Info("Audio message sent", "message_id", messageID, "phone", phoneNumber)
	return messageID, nil
}

// MarkMessageRead sends a read receipt for a message
func (c *Client) MarkMessageRead(ctx context.Context, account *Account, messageID string) error {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"status":            "read",
		"message_id":        messageID,
	}

	url := c.buildMessagesURL(account)
	c.Log.Debug("Sending read receipt", "message_id", messageID)

	_, err := c.doRequest(ctx, "POST", url, payload, account.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to send read receipt: %w", err)
	}

	c.Log.Debug("Read receipt sent", "message_id", messageID)
	return nil
}

// ResumableUploadResponse represents response from creating upload session
type ResumableUploadResponse struct {
	ID string `json:"id"` // Upload session ID
}

// ResumableUploadFinishResponse represents response from completing upload
type ResumableUploadFinishResponse struct {
	Handle string `json:"h"` // File handle for use in templates
}

// ResumableUpload performs a resumable upload to get a file handle for template media samples.
// This is required for IMAGE, VIDEO, DOCUMENT header types in templates.
// Returns a handle (like "4::aW1hZ2...") that can be used in template creation.
func (c *Client) ResumableUpload(ctx context.Context, account *Account, data []byte, mimeType, filename string) (string, error) {
	if account.AppID == "" {
		return "", fmt.Errorf("app_id is required for resumable upload")
	}

	// Step 1: Create upload session
	sessionURL := fmt.Sprintf("%s/%s/%s/uploads", c.getBaseURL(), account.APIVersion, account.AppID)

	sessionPayload := map[string]interface{}{
		"file_length": len(data),
		"file_type":   mimeType,
		"file_name":   filename,
	}

	c.Log.Info("Creating upload session", "url", sessionURL, "file_size", len(data), "mime_type", mimeType)

	sessionResp, err := c.doRequest(ctx, http.MethodPost, sessionURL, sessionPayload, account.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to create upload session: %w", err)
	}

	var uploadSession ResumableUploadResponse
	if err := json.Unmarshal(sessionResp, &uploadSession); err != nil {
		return "", fmt.Errorf("failed to parse upload session response: %w", err)
	}

	if uploadSession.ID == "" {
		return "", fmt.Errorf("no session ID in upload response")
	}

	c.Log.Info("Upload session created", "session_id", uploadSession.ID)

	// Step 2: Upload file data to session
	uploadURL := fmt.Sprintf("%s/%s/%s", c.getBaseURL(), account.APIVersion, uploadSession.ID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Authorization", "OAuth "+account.AccessToken)
	req.Header.Set("file_offset", "0")
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload file data: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read upload response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var finishResp ResumableUploadFinishResponse
	if err := json.Unmarshal(respBody, &finishResp); err != nil {
		return "", fmt.Errorf("failed to parse upload finish response: %w", err)
	}

	if finishResp.Handle == "" {
		return "", fmt.Errorf("no handle in upload response")
	}

	c.Log.Info("Resumable upload completed", "handle", finishResp.Handle[:20]+"...")
	return finishResp.Handle, nil
}
