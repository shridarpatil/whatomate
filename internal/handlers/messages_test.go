package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/templateutil"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWhatsAppServer creates a mock WhatsApp API server for testing.
// It handles various endpoints and returns configurable responses.
type mockWhatsAppServer struct {
	server         *httptest.Server
	sentMessages   []map[string]interface{}
	uploadedMedia  []map[string]interface{}
	returnError    bool
	errorMessage   string
	nextMessageID  string
	nextMediaID    string
}

func newMockWhatsAppServer() *mockWhatsAppServer {
	m := &mockWhatsAppServer{
		sentMessages:  make([]map[string]interface{}, 0),
		uploadedMedia: make([]map[string]interface{}, 0),
		nextMessageID: "wamid.test-" + uuid.New().String()[:8],
		nextMediaID:   "media-" + uuid.New().String()[:8],
	}

	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid access token",
					"code":    190,
				},
			})
			return
		}

		// Handle different endpoints
		switch {
		case r.URL.Path == "/v18.0/phone-123/messages" && r.Method == http.MethodPost:
			m.handleMessages(w, r)
		case r.URL.Path == "/v18.0/phone-123/media" && r.Method == http.MethodPost:
			m.handleMediaUpload(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	return m
}

func (m *mockWhatsAppServer) handleMessages(w http.ResponseWriter, r *http.Request) {
	if m.returnError {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": m.errorMessage,
				"code":    100,
			},
		})
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	m.sentMessages = append(m.sentMessages, body)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": []map[string]string{{"id": m.nextMessageID}},
	})
}

func (m *mockWhatsAppServer) handleMediaUpload(w http.ResponseWriter, r *http.Request) {
	if m.returnError {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	m.uploadedMedia = append(m.uploadedMedia, map[string]interface{}{
		"content_type": r.Header.Get("Content-Type"),
	})

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id": m.nextMediaID,
	})
}

func (m *mockWhatsAppServer) close() {
	m.server.Close()
}


// testServerTransport redirects all requests to the test server
type testServerTransport struct {
	serverURL string
}

func (t *testServerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	testReq := req.Clone(req.Context())
	testReq.URL.Scheme = "http"
	testReq.URL.Host = t.serverURL[7:] // Remove "http://"
	return http.DefaultTransport.RoundTrip(testReq)
}

// newMsgTestApp creates an App instance for message testing with a mock WhatsApp server.
func newMsgTestApp(t *testing.T, mockServer *mockWhatsAppServer) *handlers.App {
	t.Helper()

	log := testutil.NopLogger()
	waClient := whatsapp.NewWithTimeout(log, 5*time.Second)
	waClient.HTTPClient = &http.Client{
		Transport: &testServerTransport{serverURL: mockServer.server.URL},
	}

	return newTestApp(t, withWhatsApp(waClient))
}

// createTestAccount creates a test WhatsApp account in the database.
func createTestAccount(t *testing.T, app *handlers.App, orgID uuid.UUID) *models.WhatsAppAccount {
	t.Helper()

	account := &models.WhatsAppAccount{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     orgID,
		Name:               "test-account-" + uuid.New().String()[:8],
		PhoneID:            "phone-123",
		BusinessID:         "business-123",
		AccessToken:        "test-token",
		WebhookVerifyToken: "webhook-token",
		APIVersion:         "v18.0",
		Status:             "active",
	}
	require.NoError(t, app.DB.Create(account).Error)
	return account
}


// --- SendOutgoingMessage Tests ---

func TestApp_SendOutgoingMessage_TextMessage_Success(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account: account,
		Contact: contact,
		Type:    models.MessageTypeText,
		Content: "Hello, World!",
	}

	// Use sync options to wait for result
	opts := handlers.ChatbotSendOptions()

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	// Verify message was saved to database
	assert.Equal(t, models.MessageTypeText, msg.MessageType)
	assert.Equal(t, "Hello, World!", msg.Content)
	assert.Equal(t, models.DirectionOutgoing, msg.Direction)
	assert.Equal(t, contact.ID, msg.ContactID)
	assert.Equal(t, org.ID, msg.OrganizationID)

	// Verify message was sent to WhatsApp API
	require.Len(t, mockServer.sentMessages, 1)
	sentMsg := mockServer.sentMessages[0]
	assert.Equal(t, "text", sentMsg["type"])
	assert.Equal(t, contact.PhoneNumber, sentMsg["to"])

	textContent := sentMsg["text"].(map[string]interface{})
	assert.Equal(t, "Hello, World!", textContent["body"])

	// Verify message status was updated in DB
	var dbMsg models.Message
	require.NoError(t, app.DB.First(&dbMsg, msg.ID).Error)
	assert.Equal(t, models.MessageStatusSent, dbMsg.Status)
	assert.Equal(t, mockServer.nextMessageID, dbMsg.WhatsAppMessageID)
}

func TestApp_SendOutgoingMessage_TextMessage_APIError(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	mockServer.returnError = true
	mockServer.errorMessage = "Phone number is invalid"

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account: account,
		Contact: contact,
		Type:    models.MessageTypeText,
		Content: "Hello!",
	}

	opts := handlers.ChatbotSendOptions()

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	// Message is still returned (saved to DB) even if send fails
	require.NoError(t, err)
	require.NotNil(t, msg)

	// Verify message status is failed in DB
	var dbMsg models.Message
	require.NoError(t, app.DB.First(&dbMsg, msg.ID).Error)
	assert.Equal(t, models.MessageStatusFailed, dbMsg.Status)
	assert.Contains(t, dbMsg.ErrorMessage, "Phone number is invalid")
}

func TestApp_SendOutgoingMessage_ImageMessage_WithMediaID(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account:       account,
		Contact:       contact,
		Type:          models.MessageTypeImage,
		MediaID:       "existing-media-id",
		MediaMimeType: "image/jpeg",
		Caption:       "Check this out!",
	}

	opts := handlers.ChatbotSendOptions()

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	// Verify image message was sent
	require.Len(t, mockServer.sentMessages, 1)
	sentMsg := mockServer.sentMessages[0]
	assert.Equal(t, "image", sentMsg["type"])

	imageContent := sentMsg["image"].(map[string]interface{})
	assert.Equal(t, "existing-media-id", imageContent["id"])
	assert.Equal(t, "Check this out!", imageContent["caption"])

	// No media upload should have occurred
	assert.Len(t, mockServer.uploadedMedia, 0)
}

func TestApp_SendOutgoingMessage_ImageMessage_WithMediaData(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account:       account,
		Contact:       contact,
		Type:          models.MessageTypeImage,
		MediaData:     []byte("fake image data"),
		MediaMimeType: "image/jpeg",
		MediaFilename: "photo.jpg",
		Caption:       "Photo caption",
	}

	opts := handlers.ChatbotSendOptions()

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	// Verify media was uploaded
	require.Len(t, mockServer.uploadedMedia, 1)

	// Verify image message was sent with uploaded media ID
	require.Len(t, mockServer.sentMessages, 1)
	sentMsg := mockServer.sentMessages[0]
	assert.Equal(t, "image", sentMsg["type"])

	imageContent := sentMsg["image"].(map[string]interface{})
	assert.Equal(t, mockServer.nextMediaID, imageContent["id"])
}

func TestApp_SendOutgoingMessage_DocumentMessage(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account:       account,
		Contact:       contact,
		Type:          models.MessageTypeDocument,
		MediaID:       "doc-media-id",
		MediaMimeType: "application/pdf",
		MediaFilename: "report.pdf",
		Caption:       "Monthly report",
	}

	opts := handlers.ChatbotSendOptions()

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	// Verify document message was sent
	require.Len(t, mockServer.sentMessages, 1)
	sentMsg := mockServer.sentMessages[0]
	assert.Equal(t, "document", sentMsg["type"])

	docContent := sentMsg["document"].(map[string]interface{})
	assert.Equal(t, "doc-media-id", docContent["id"])
	assert.Equal(t, "report.pdf", docContent["filename"])
	assert.Equal(t, "Monthly report", docContent["caption"])
}

func TestApp_SendOutgoingMessage_VideoMessage(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account:       account,
		Contact:       contact,
		Type:          models.MessageTypeVideo,
		MediaID:       "video-media-id",
		MediaMimeType: "video/mp4",
		Caption:       "Watch this!",
	}

	opts := handlers.ChatbotSendOptions()

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	require.Len(t, mockServer.sentMessages, 1)
	sentMsg := mockServer.sentMessages[0]
	assert.Equal(t, "video", sentMsg["type"])

	videoContent := sentMsg["video"].(map[string]interface{})
	assert.Equal(t, "video-media-id", videoContent["id"])
	assert.Equal(t, "Watch this!", videoContent["caption"])
}

func TestApp_SendOutgoingMessage_AudioMessage(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account:       account,
		Contact:       contact,
		Type:          models.MessageTypeAudio,
		MediaID:       "audio-media-id",
		MediaMimeType: "audio/ogg",
	}

	opts := handlers.ChatbotSendOptions()

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	require.Len(t, mockServer.sentMessages, 1)
	sentMsg := mockServer.sentMessages[0]
	assert.Equal(t, "audio", sentMsg["type"])

	audioContent := sentMsg["audio"].(map[string]interface{})
	assert.Equal(t, "audio-media-id", audioContent["id"])
}

func TestApp_SendOutgoingMessage_InteractiveButtons(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account:         account,
		Contact:         contact,
		Type:            models.MessageTypeInteractive,
		InteractiveType: "button",
		BodyText:        "Choose an option:",
		Buttons: []whatsapp.Button{
			{ID: "btn_yes", Title: "Yes"},
			{ID: "btn_no", Title: "No"},
		},
	}

	opts := handlers.ChatbotSendOptions()

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	// Verify interactive message was saved
	assert.Equal(t, models.MessageTypeInteractive, msg.MessageType)
	assert.Equal(t, "Choose an option:", msg.Content)

	// Verify interactive message was sent
	require.Len(t, mockServer.sentMessages, 1)
	sentMsg := mockServer.sentMessages[0]
	assert.Equal(t, "interactive", sentMsg["type"])

	interactive := sentMsg["interactive"].(map[string]interface{})
	assert.Equal(t, "button", interactive["type"])

	body := interactive["body"].(map[string]interface{})
	assert.Equal(t, "Choose an option:", body["text"])

	action := interactive["action"].(map[string]interface{})
	buttons := action["buttons"].([]interface{})
	assert.Len(t, buttons, 2)
}

func TestApp_SendOutgoingMessage_InteractiveCTAURL(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account:         account,
		Contact:         contact,
		Type:            models.MessageTypeInteractive,
		InteractiveType: "cta_url",
		BodyText:        "Visit our website",
		ButtonText:      "Visit Now",
		URL:             "https://example.com",
	}

	opts := handlers.ChatbotSendOptions()

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	// Verify message content and interactive data
	assert.Equal(t, "Visit our website", msg.Content)
	assert.NotNil(t, msg.InteractiveData)
	assert.Equal(t, "cta_url", msg.InteractiveData["type"])

	// Verify CTA URL message was sent
	require.Len(t, mockServer.sentMessages, 1)
	sentMsg := mockServer.sentMessages[0]
	assert.Equal(t, "interactive", sentMsg["type"])

	interactive := sentMsg["interactive"].(map[string]interface{})
	assert.Equal(t, "cta_url", interactive["type"])
}

func TestApp_SendOutgoingMessage_TemplateMessage(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	// Create a test template
	template := &models.Template{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "hello_world",
		DisplayName:     "Hello World Template",
		MetaTemplateID:  "meta-123",
		Category:        "MARKETING",
		Language:        "en",
		Status:          string(models.TemplateStatusApproved),
		BodyContent:     "Hello {{1}}! Your order {{2}} is ready.",
	}
	require.NoError(t, app.DB.Create(template).Error)

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account:    account,
		Contact:    contact,
		Type:       models.MessageTypeTemplate,
		Template:   template,
		BodyParams: map[string]string{"1": "John", "2": "ORD-123"},
	}

	opts := handlers.ChatbotSendOptions()

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	// Verify template message was saved with rendered content
	assert.Equal(t, models.MessageTypeTemplate, msg.MessageType)
	assert.Equal(t, "Hello John! Your order ORD-123 is ready.", msg.Content)

	// Verify template metadata
	assert.NotNil(t, msg.Metadata)
	assert.Equal(t, "hello_world", msg.Metadata["template_name"])

	// Verify template message was sent
	require.Len(t, mockServer.sentMessages, 1)
	sentMsg := mockServer.sentMessages[0]
	assert.Equal(t, "template", sentMsg["type"])

	templateData := sentMsg["template"].(map[string]interface{})
	assert.Equal(t, "hello_world", templateData["name"])
	assert.Equal(t, "en", templateData["language"].(map[string]interface{})["code"])
}

func TestApp_SendOutgoingMessage_TemplateMessage_MissingTemplate(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account:    account,
		Contact:    contact,
		Type:       models.MessageTypeTemplate,
		Template:   nil, // Missing template
		BodyParams: map[string]string{"1": "param1"},
	}

	opts := handlers.ChatbotSendOptions()

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	// Message is created but send fails
	require.NoError(t, err)
	require.NotNil(t, msg)

	// Verify message status is failed
	var dbMsg models.Message
	require.NoError(t, app.DB.First(&dbMsg, msg.ID).Error)
	assert.Equal(t, models.MessageStatusFailed, dbMsg.Status)
	assert.Contains(t, dbMsg.ErrorMessage, "template is required")
}

func TestApp_SendOutgoingMessage_AsyncOption(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account: account,
		Contact: contact,
		Type:    models.MessageTypeText,
		Content: "Async message",
	}

	// Use async options
	opts := handlers.DefaultSendOptions()
	assert.True(t, opts.Async)

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	// Wait for async send to complete
	app.WaitForBackgroundTasks()

	// Now verify message was sent and status updated in DB
	var dbMsg models.Message
	require.NoError(t, app.DB.First(&dbMsg, msg.ID).Error)
	assert.Equal(t, models.MessageStatusSent, dbMsg.Status)
	assert.NotEmpty(t, dbMsg.WhatsAppMessageID)
}

func TestApp_SendOutgoingMessage_SyncOption(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account: account,
		Contact: contact,
		Type:    models.MessageTypeText,
		Content: "Sync message",
	}

	// Use sync options (ChatbotSendOptions has Async: false)
	opts := handlers.ChatbotSendOptions()
	assert.False(t, opts.Async)

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	// Message status should be updated immediately (sync)
	var dbMsg models.Message
	require.NoError(t, app.DB.First(&dbMsg, msg.ID).Error)
	assert.Equal(t, models.MessageStatusSent, dbMsg.Status)
}

func TestApp_SendOutgoingMessage_WithSentByUser(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	// Create a test user (required due to foreign key constraint)
	user := &models.User{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Email:          "agent-" + uuid.New().String()[:8] + "@test.com",
		FullName:       "Test Agent",
		IsActive:       true,
	}
	require.NoError(t, app.DB.Create(user).Error)
	userID := user.ID

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account: account,
		Contact: contact,
		Type:    models.MessageTypeText,
		Content: "Message from agent",
	}

	opts := handlers.DefaultSendOptions()
	opts.SentByUserID = &userID

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	// Wait for async send
	app.WaitForBackgroundTasks()

	// Verify sent by user is recorded
	var dbMsg models.Message
	require.NoError(t, app.DB.First(&dbMsg, msg.ID).Error)
	require.NotNil(t, dbMsg.SentByUserID)
	assert.Equal(t, userID, *dbMsg.SentByUserID)
}

func TestApp_SendOutgoingMessage_UnsupportedType(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account: account,
		Contact: contact,
		Type:    "unknown_type",
		Content: "Some content",
	}

	opts := handlers.ChatbotSendOptions()

	msg, err := app.SendOutgoingMessage(ctx, req, opts)

	require.NoError(t, err)
	require.NotNil(t, msg)

	// Verify message status is failed due to unsupported type
	var dbMsg models.Message
	require.NoError(t, app.DB.First(&dbMsg, msg.ID).Error)
	assert.Equal(t, models.MessageStatusFailed, dbMsg.Status)
	assert.Contains(t, dbMsg.ErrorMessage, "unsupported message type")
}

// --- Options Preset Tests ---

func TestDefaultSendOptions(t *testing.T) {
	opts := handlers.DefaultSendOptions()

	assert.True(t, opts.BroadcastWebSocket)
	assert.True(t, opts.DispatchWebhook)
	assert.False(t, opts.TrackSLA)
	assert.True(t, opts.Async)
	assert.Nil(t, opts.SentByUserID)
}

func TestChatbotSendOptions(t *testing.T) {
	opts := handlers.ChatbotSendOptions()

	assert.True(t, opts.BroadcastWebSocket)
	assert.False(t, opts.DispatchWebhook)
	assert.True(t, opts.TrackSLA)
	assert.False(t, opts.Async)
	assert.Nil(t, opts.SentByUserID)
}

func TestAPISendOptions(t *testing.T) {
	opts := handlers.APISendOptions()

	assert.False(t, opts.BroadcastWebSocket)
	assert.True(t, opts.DispatchWebhook)
	assert.False(t, opts.TrackSLA)
	assert.True(t, opts.Async)
	assert.Nil(t, opts.SentByUserID)
}

func TestSLASendOptions(t *testing.T) {
	opts := handlers.SLASendOptions()

	assert.True(t, opts.BroadcastWebSocket)
	assert.False(t, opts.DispatchWebhook)
	assert.False(t, opts.TrackSLA)
	assert.False(t, opts.Async)
	assert.Nil(t, opts.SentByUserID)
}

// --- Message Preview Tests ---

func TestApp_SendOutgoingMessage_ContactLastMessageUpdated(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account: account,
		Contact: contact,
		Type:    models.MessageTypeText,
		Content: "This is a test message for preview",
	}

	opts := handlers.ChatbotSendOptions()

	_, err := app.SendOutgoingMessage(ctx, req, opts)
	require.NoError(t, err)

	// Verify contact's last message was updated
	var updatedContact models.Contact
	require.NoError(t, app.DB.First(&updatedContact, contact.ID).Error)
	assert.NotNil(t, updatedContact.LastMessageAt)
	assert.Equal(t, "This is a test message for preview", updatedContact.LastMessagePreview)
}

func TestApp_SendOutgoingMessage_MediaPreview(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	// Test image without caption
	req := handlers.OutgoingMessageRequest{
		Account:       account,
		Contact:       contact,
		Type:          models.MessageTypeImage,
		MediaID:       "media-123",
		MediaMimeType: "image/jpeg",
	}

	opts := handlers.ChatbotSendOptions()

	_, err := app.SendOutgoingMessage(ctx, req, opts)
	require.NoError(t, err)

	var updatedContact models.Contact
	require.NoError(t, app.DB.First(&updatedContact, contact.ID).Error)
	assert.Equal(t, "[Image]", updatedContact.LastMessagePreview)
}

func TestApp_SendOutgoingMessage_DocumentPreview(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := createTestAccount(t, app, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	ctx := testutil.TestContext(t)

	req := handlers.OutgoingMessageRequest{
		Account:       account,
		Contact:       contact,
		Type:          models.MessageTypeDocument,
		MediaID:       "media-123",
		MediaFilename: "report.pdf",
	}

	opts := handlers.ChatbotSendOptions()

	_, err := app.SendOutgoingMessage(ctx, req, opts)
	require.NoError(t, err)

	var updatedContact models.Contact
	require.NoError(t, app.DB.First(&updatedContact, contact.ID).Error)
	assert.Equal(t, "[Document: report.pdf]", updatedContact.LastMessagePreview)
}

// --- Template Parameter Tests ---

func TestExtractParamNamesFromContent_Positional(t *testing.T) {
	content := "Hello {{1}}! Your order {{2}} is ready for pickup at {{3}}."
	names := templateutil.ExtParamNames(content)

	require.Len(t, names, 3)
	assert.Equal(t, "1", names[0])
	assert.Equal(t, "2", names[1])
	assert.Equal(t, "3", names[2])
}

func TestExtractParamNamesFromContent_Named(t *testing.T) {
	content := "Hi {{customer_name}}, your order {{order_id}} will arrive on {{delivery_date}}."
	names := templateutil.ExtParamNames(content)

	require.Len(t, names, 3)
	assert.Equal(t, "customer_name", names[0])
	assert.Equal(t, "order_id", names[1])
	assert.Equal(t, "delivery_date", names[2])
}

func TestExtractParamNamesFromContent_Mixed(t *testing.T) {
	content := "Hello {{name}}, your code is {{1}}."
	names := templateutil.ExtParamNames(content)

	require.Len(t, names, 2)
	assert.Equal(t, "name", names[0])
	assert.Equal(t, "1", names[1])
}

func TestExtractParamNamesFromContent_NoParams(t *testing.T) {
	content := "This is a static message with no parameters."
	names := templateutil.ExtParamNames(content)

	assert.Nil(t, names)
}

func TestExtractParamNamesFromContent_DuplicateParams(t *testing.T) {
	content := "Hello {{name}}, {{name}} is a great name!"
	names := templateutil.ExtParamNames(content)

	// Should deduplicate
	require.Len(t, names, 1)
	assert.Equal(t, "name", names[0])
}

func TestResolveParams_NamedMatch(t *testing.T) {
	paramNames := []string{"customer_name", "order_id"}
	params := map[string]string{
		"customer_name": "John",
		"order_id":      "ORD-123",
	}

	result := templateutil.ResolveParamsFromMap(paramNames, params)

	require.Len(t, result, 2)
	assert.Equal(t, "John", result[0])
	assert.Equal(t, "ORD-123", result[1])
}

func TestResolveParams_PositionalMatch(t *testing.T) {
	paramNames := []string{"1", "2"}
	params := map[string]string{
		"1": "First",
		"2": "Second",
	}

	result := templateutil.ResolveParamsFromMap(paramNames, params)

	require.Len(t, result, 2)
	assert.Equal(t, "First", result[0])
	assert.Equal(t, "Second", result[1])
}

func TestResolveParams_FallbackToPositional(t *testing.T) {
	// Template has named params, but user sends positional
	paramNames := []string{"name", "code"}
	params := map[string]string{
		"1": "John",
		"2": "ABC123",
	}

	result := templateutil.ResolveParamsFromMap(paramNames, params)

	require.Len(t, result, 2)
	assert.Equal(t, "John", result[0])
	assert.Equal(t, "ABC123", result[1])
}

func TestResolveParams_MissingParams(t *testing.T) {
	paramNames := []string{"name", "order_id", "date"}
	params := map[string]string{
		"name": "John",
		// order_id and date are missing
	}

	result := templateutil.ResolveParamsFromMap(paramNames, params)

	require.Len(t, result, 3)
	assert.Equal(t, "John", result[0])
	assert.Equal(t, "", result[1]) // Missing - defaults to empty
	assert.Equal(t, "", result[2]) // Missing - defaults to empty
}

func TestResolveParams_EmptyInputs(t *testing.T) {
	// Empty param names
	result1 := templateutil.ResolveParamsFromMap([]string{}, map[string]string{"a": "b"})
	assert.Nil(t, result1)

	// Empty params map
	result2 := templateutil.ResolveParamsFromMap([]string{"a"}, map[string]string{})
	assert.Nil(t, result2)

	// Both empty
	result3 := templateutil.ResolveParamsFromMap([]string{}, map[string]string{})
	assert.Nil(t, result3)
}

func TestResolveParams_WrongParamNames(t *testing.T) {
	// User sends different param names than template expects
	paramNames := []string{"1", "2"} // Template expects positional
	params := map[string]string{
		"lastrec":  "Nifty",     // Wrong name
		"misstime": "banknifty", // Wrong name
	}

	result := templateutil.ResolveParamsFromMap(paramNames, params)

	require.Len(t, result, 2)
	// Both should be empty since names don't match
	assert.Equal(t, "", result[0])
	assert.Equal(t, "", result[1])
}
