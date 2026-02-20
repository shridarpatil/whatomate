package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// createTestTemplate creates an approved template in the database.
func createTestTemplate(t *testing.T, app *handlers.App, orgID uuid.UUID, accountName string) *models.Template {
	t.Helper()
	tpl := &models.Template{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: accountName,
		Name:            "order_confirm_" + uuid.New().String()[:8],
		DisplayName:     "Order Confirmation",
		MetaTemplateID:  "meta-" + uuid.New().String()[:8],
		Category:        "UTILITY",
		Language:        "en",
		Status:          string(models.TemplateStatusApproved),
		BodyContent:     "Hello {{name}}! Your order {{order_id}} has been confirmed.",
	}
	require.NoError(t, app.DB.Create(tpl).Error)
	return tpl
}

func TestApp_SendTemplateMessage(t *testing.T) {
	t.Parallel()

	t.Run("success with contact_id and template params", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := createTestAccount(t, app, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
		tpl := createTestTemplate(t, app, org.ID, account.Name)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    contact.ID.String(),
			"template_name": tpl.Name,
			"template_params": map[string]string{
				"name":     "Alice",
				"order_id": "ORD-42",
			},
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.MessageResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))

		assert.Equal(t, contact.ID, resp.Data.ContactID)
		assert.Equal(t, models.DirectionOutgoing, resp.Data.Direction)
		assert.Equal(t, models.MessageTypeTemplate, resp.Data.MessageType)
		assert.Equal(t, account.Name, resp.Data.WhatsAppAccount)

		// Verify rendered body content
		contentMap, ok := resp.Data.Content.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Hello Alice! Your order ORD-42 has been confirmed.", contentMap["body"])

		// Wait for async send to complete before checking mock
		app.WaitForBackgroundTasks()

		// Verify message was sent to WhatsApp API
		require.Len(t, mockServer.sentMessages, 1)
		sentMsg := mockServer.sentMessages[0]
		assert.Equal(t, "template", sentMsg["type"])

		// Verify message was persisted in DB
		var dbMsg models.Message
		require.NoError(t, app.DB.Where("contact_id = ? AND message_type = ?", contact.ID, models.MessageTypeTemplate).First(&dbMsg).Error)
		assert.Equal(t, "Hello Alice! Your order ORD-42 has been confirmed.", dbMsg.Content)
		assert.Equal(t, tpl.Name, dbMsg.TemplateName)
	})

	t.Run("success without params", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := createTestAccount(t, app, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

		// Template with no params
		tpl := &models.Template{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			WhatsAppAccount: account.Name,
			Name:            "simple_greeting_" + uuid.New().String()[:8],
			DisplayName:     "Simple Greeting",
			Language:        "en",
			Status:          string(models.TemplateStatusApproved),
			BodyContent:     "Welcome to our service!",
		}
		require.NoError(t, app.DB.Create(tpl).Error)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    contact.ID.String(),
			"template_name": tpl.Name,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.MessageResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))

		contentMap, ok := resp.Data.Content.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Welcome to our service!", contentMap["body"])
	})

	t.Run("success with template_id", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := createTestAccount(t, app, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
		tpl := createTestTemplate(t, app, org.ID, account.Name)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":  contact.ID.String(),
			"template_id": tpl.ID.String(),
			"template_params": map[string]string{
				"name":     "Bob",
				"order_id": "ORD-99",
			},
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.MessageResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, models.MessageTypeTemplate, resp.Data.MessageType)

		contentMap, ok := resp.Data.Content.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Hello Bob! Your order ORD-99 has been confirmed.", contentMap["body"])
	})

	t.Run("success with account_name override", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := createTestAccount(t, app, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
		tpl := createTestTemplate(t, app, org.ID, account.Name)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    contact.ID.String(),
			"template_name": tpl.Name,
			"account_name":  account.Name,
			"template_params": map[string]string{
				"name":     "Charlie",
				"order_id": "ORD-77",
			},
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.MessageResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, account.Name, resp.Data.WhatsAppAccount)
	})

	t.Run("missing contact_id and phone_number", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"template_name": "some_template",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("missing template_name and template_id", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id": contact.ID.String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("template not found", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    contact.ID.String(),
			"template_name": "nonexistent_template",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("template not approved", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		tpl := &models.Template{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			WhatsAppAccount: "test-account",
			Name:            "pending_template_" + uuid.New().String()[:8],
			Language:        "en",
			Status:          string(models.TemplateStatusPending),
			BodyContent:     "Some content",
		}
		require.NoError(t, app.DB.Create(tpl).Error)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    contact.ID.String(),
			"template_name": tpl.Name,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("contact not found", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := createTestAccount(t, app, org.ID)
		tpl := createTestTemplate(t, app, org.ID, account.Name)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    uuid.New().String(),
			"template_name": tpl.Name,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("missing required template params", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := createTestAccount(t, app, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
		tpl := createTestTemplate(t, app, org.ID, account.Name)

		// Send without required params — template has {{name}} and {{order_id}}
		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    contact.ID.String(),
			"template_name": tpl.Name,
			// no template_params
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid template_id format", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":  contact.ID.String(),
			"template_id": "not-a-uuid",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid contact_id format", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		tpl := &models.Template{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			WhatsAppAccount: "test-account",
			Name:            "test_tpl_" + uuid.New().String()[:8],
			Language:        "en",
			Status:          string(models.TemplateStatusApproved),
			BodyContent:     "Hello",
		}
		require.NoError(t, app.DB.Create(tpl).Error)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    "not-a-uuid",
			"template_name": tpl.Name,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation - template from another org", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org1.ID)
		user := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org1.ID)

		// Template belongs to org2
		tpl := &models.Template{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org2.ID,
			WhatsAppAccount: "other-account",
			Name:            "other_org_tpl_" + uuid.New().String()[:8],
			Language:        "en",
			Status:          string(models.TemplateStatusApproved),
			BodyContent:     "Hello",
		}
		require.NoError(t, app.DB.Create(tpl).Error)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    contact.ID.String(),
			"template_name": tpl.Name,
		})
		testutil.SetAuthContext(req, org1.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation - contact from another org", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org1.ID)
		user := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&adminRole.ID))
		account := createTestAccount(t, app, org1.ID)
		tpl := createTestTemplate(t, app, org1.ID, account.Name)

		// Contact belongs to org2
		contact := testutil.CreateTestContact(t, app.DB, org2.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    contact.ID.String(),
			"template_name": tpl.Name,
		})
		testutil.SetAuthContext(req, org1.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("account_name not found", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		tpl := &models.Template{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			WhatsAppAccount: "some-account",
			Name:            "tpl_" + uuid.New().String()[:8],
			Language:        "en",
			Status:          string(models.TemplateStatusApproved),
			BodyContent:     "Hello",
		}
		require.NoError(t, app.DB.Create(tpl).Error)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    contact.ID.String(),
			"template_name": tpl.Name,
			"account_name":  "nonexistent-account",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("response has correct MessageResponse shape", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := createTestAccount(t, app, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

		tpl := &models.Template{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			WhatsAppAccount: account.Name,
			Name:            "shape_test_" + uuid.New().String()[:8],
			Language:        "en",
			Status:          string(models.TemplateStatusApproved),
			BodyContent:     "Hello there!",
		}
		require.NoError(t, app.DB.Create(tpl).Error)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    contact.ID.String(),
			"template_name": tpl.Name,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)

		// Verify the response has all MessageResponse fields
		var raw map[string]interface{}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &raw))
		data := raw["data"].(map[string]interface{})

		assert.NotEmpty(t, data["id"])
		assert.NotEmpty(t, data["contact_id"])
		assert.Equal(t, "outgoing", data["direction"])
		assert.Equal(t, "template", data["message_type"])
		assert.NotNil(t, data["content"])
		assert.NotEmpty(t, data["status"])
		assert.NotEmpty(t, data["created_at"])
		assert.NotEmpty(t, data["updated_at"])
	})

	t.Run("template with buttons stores interactive_data", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := createTestAccount(t, app, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

		// Create template with buttons
		tpl := &models.Template{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			WhatsAppAccount: account.Name,
			Name:            "btn_tpl_" + uuid.New().String()[:8],
			DisplayName:     "Button Template",
			Language:        "en",
			Status:          string(models.TemplateStatusApproved),
			BodyContent:     "Would you like to proceed?",
			Buttons: models.JSONBArray{
				map[string]interface{}{"type": "QUICK_REPLY", "text": "Yes"},
				map[string]interface{}{"type": "QUICK_REPLY", "text": "No"},
			},
		}
		require.NoError(t, app.DB.Create(tpl).Error)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    contact.ID.String(),
			"template_name": tpl.Name,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		// Verify response includes interactive_data with buttons
		var raw map[string]interface{}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &raw))
		data := raw["data"].(map[string]interface{})

		interactiveData, ok := data["interactive_data"].(map[string]interface{})
		require.True(t, ok, "interactive_data should be present for template with buttons")
		assert.Equal(t, "button", interactiveData["type"])

		buttons, ok := interactiveData["buttons"].([]interface{})
		require.True(t, ok, "buttons should be an array")
		assert.Len(t, buttons, 2)

		btn0 := buttons[0].(map[string]interface{})
		assert.Equal(t, "QUICK_REPLY", btn0["type"])
		assert.Equal(t, "Yes", btn0["text"])

		btn1 := buttons[1].(map[string]interface{})
		assert.Equal(t, "No", btn1["text"])

		// Verify persisted in DB
		var dbMsg models.Message
		require.NoError(t, app.DB.Where("contact_id = ? AND message_type = ?", contact.ID, models.MessageTypeTemplate).First(&dbMsg).Error)
		assert.NotNil(t, dbMsg.InteractiveData)
		assert.Equal(t, "button", dbMsg.InteractiveData["type"])
	})

	t.Run("template without buttons has no interactive_data", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := createTestAccount(t, app, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

		tpl := &models.Template{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			WhatsAppAccount: account.Name,
			Name:            "no_btn_tpl_" + uuid.New().String()[:8],
			Language:        "en",
			Status:          string(models.TemplateStatusApproved),
			BodyContent:     "Simple message",
		}
		require.NoError(t, app.DB.Create(tpl).Error)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"contact_id":    contact.ID.String(),
			"template_name": tpl.Name,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.SendTemplateMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var raw map[string]interface{}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &raw))
		data := raw["data"].(map[string]interface{})

		// interactive_data should be absent (omitempty) or nil
		_, hasInteractive := data["interactive_data"]
		assert.False(t, hasInteractive, "interactive_data should not be present for template without buttons")
	})
}
