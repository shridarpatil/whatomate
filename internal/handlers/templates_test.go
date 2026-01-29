package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/templateutil"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// --- ExtParamNames Tests (existing) ---

func TestExtParamNames_PositionalParams(t *testing.T) {
	t.Parallel()
	content := "Hello {{1}}, your order {{2}} is ready!"
	result := templateutil.ExtParamNames(content)
	assert.Equal(t, []string{"1", "2"}, result)
}

func TestExtParamNames_NamedParams(t *testing.T) {
	t.Parallel()
	content := "Hello {{name}}, your order {{order_id}} is ready!"
	result := templateutil.ExtParamNames(content)
	assert.Equal(t, []string{"name", "order_id"}, result)
}

func TestExtParamNames_MixedParams(t *testing.T) {
	t.Parallel()
	content := "Hello {{1}}, your order {{order_id}} is ready! Amount: {{3}}"
	result := templateutil.ExtParamNames(content)
	assert.Equal(t, []string{"1", "order_id", "3"}, result)
}

func TestExtParamNames_NoParams(t *testing.T) {
	t.Parallel()
	content := "Hello, your order is ready!"
	result := templateutil.ExtParamNames(content)
	assert.Nil(t, result)
}

func TestExtParamNames_DuplicateParams(t *testing.T) {
	t.Parallel()
	content := "Hello {{name}}, {{name}} your order {{order_id}} is ready!"
	result := templateutil.ExtParamNames(content)
	// Should only return unique names in order of first occurrence
	assert.Equal(t, []string{"name", "order_id"}, result)
}

func TestExtParamNames_UnderscoreParams(t *testing.T) {
	t.Parallel()
	content := "Hello {{customer_name}}, order {{order_number}} total {{total_amount}}"
	result := templateutil.ExtParamNames(content)
	assert.Equal(t, []string{"customer_name", "order_number", "total_amount"}, result)
}

// --- Template handler test helpers ---

// createTestTemplateInDB creates a template directly in the database for testing.
func createTestTemplateInDB(t *testing.T, app *handlers.App, orgID uuid.UUID, accountName, name, status string) *models.Template {
	t.Helper()

	tmpl := &models.Template{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: accountName,
		Name:            name,
		DisplayName:     name,
		Language:        "en",
		Category:        "MARKETING",
		Status:          status,
		BodyContent:     "Hello {{1}}, welcome!",
	}
	require.NoError(t, app.DB.Create(tmpl).Error)
	return tmpl
}

// newMockTemplateServer creates a mock WhatsApp API server for template operations.
func newMockTemplateServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			// SubmitTemplate response
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "meta-tmpl-" + uuid.New().String()[:8],
			})
		case http.MethodGet:
			// FetchTemplates response
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":       "meta-synced-1",
						"name":     "synced_template_one",
						"language": "en",
						"category": "MARKETING",
						"status":   "APPROVED",
						"components": []map[string]interface{}{
							{"type": "BODY", "text": "Synced body content"},
						},
					},
					{
						"id":       "meta-synced-2",
						"name":     "synced_template_two",
						"language": "en",
						"category": "UTILITY",
						"status":   "PENDING",
						"components": []map[string]interface{}{
							{"type": "BODY", "text": "Another synced body"},
						},
					},
				},
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// newTemplateTestApp creates an App instance for template testing with a mock WhatsApp server.
func newTemplateTestApp(t *testing.T, server *httptest.Server) *handlers.App {
	t.Helper()

	log := testutil.NopLogger()
	waClient := whatsapp.NewWithBaseURL(log, server.URL)
	return newTestApp(t, withWhatsApp(waClient))
}

// --- ListTemplates Tests ---

func TestApp_ListTemplates_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	createTestTemplateInDB(t, app, org.ID, account.Name, "template_one", "APPROVED")
	createTestTemplateInDB(t, app, org.ID, account.Name, "template_two", "DRAFT")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListTemplates(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Templates []handlers.TemplateResponse `json:"templates"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Len(t, resp.Data.Templates, 2)
}

func TestApp_ListTemplates_EmptyList(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListTemplates(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Templates []handlers.TemplateResponse `json:"templates"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Len(t, resp.Data.Templates, 0)
}

func TestApp_ListTemplates_FilterByAccount(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	account2 := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	createTestTemplateInDB(t, app, org.ID, account1.Name, "tmpl_a1", "APPROVED")
	createTestTemplateInDB(t, app, org.ID, account1.Name, "tmpl_a2", "APPROVED")
	createTestTemplateInDB(t, app, org.ID, account2.Name, "tmpl_b1", "APPROVED")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "account", account1.Name)

	err := app.ListTemplates(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Templates []handlers.TemplateResponse `json:"templates"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Len(t, resp.Data.Templates, 2)
	for _, tmpl := range resp.Data.Templates {
		assert.Equal(t, account1.Name, tmpl.WhatsAppAccount)
	}
}

func TestApp_ListTemplates_FilterByStatus(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	createTestTemplateInDB(t, app, org.ID, account.Name, "approved_tmpl", "APPROVED")
	createTestTemplateInDB(t, app, org.ID, account.Name, "draft_tmpl", "DRAFT")
	createTestTemplateInDB(t, app, org.ID, account.Name, "pending_tmpl", "PENDING")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "status", "APPROVED")

	err := app.ListTemplates(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Templates []handlers.TemplateResponse `json:"templates"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Len(t, resp.Data.Templates, 1)
	assert.Equal(t, "APPROVED", resp.Data.Templates[0].Status)
}

func TestApp_ListTemplates_FilterByCategory(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	createTestTemplateInDB(t, app, org.ID, account.Name, "marketing_tmpl", "APPROVED")

	// Create a UTILITY template directly
	utilTmpl := &models.Template{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "utility_tmpl",
		DisplayName:     "utility_tmpl",
		Language:        "en",
		Category:        "UTILITY",
		Status:          "APPROVED",
		BodyContent:     "Your OTP is {{1}}",
	}
	require.NoError(t, app.DB.Create(utilTmpl).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "category", "UTILITY")

	err := app.ListTemplates(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Templates []handlers.TemplateResponse `json:"templates"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Len(t, resp.Data.Templates, 1)
	assert.Equal(t, "UTILITY", resp.Data.Templates[0].Category)
}

func TestApp_ListTemplates_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
	account1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)
	account2 := testutil.CreateTestWhatsAppAccount(t, app.DB, org2.ID)

	createTestTemplateInDB(t, app, org1.ID, account1.Name, "org1_tmpl", "APPROVED")
	createTestTemplateInDB(t, app, org2.ID, account2.Name, "org2_tmpl", "APPROVED")

	// User from org1 should only see org1 templates
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org1.ID, user1.ID)

	err := app.ListTemplates(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Templates []handlers.TemplateResponse `json:"templates"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Len(t, resp.Data.Templates, 1)
	assert.Equal(t, "org1_tmpl", resp.Data.Templates[0].Name)
}

// --- CreateTemplate Tests ---

func TestApp_CreateTemplate_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	body := map[string]interface{}{
		"whatsapp_account": account.Name,
		"name":             "My New Template",
		"language":         "en",
		"category":         "marketing",
		"body_content":     "Hello {{1}}, your order is ready!",
		"header_type":      "TEXT",
		"header_content":   "Order Update",
		"footer_content":   "Reply STOP to unsubscribe",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateTemplate(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.TemplateResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, "my_new_template", resp.Data.Name) // normalized
	assert.Equal(t, "My New Template", resp.Data.DisplayName)
	assert.Equal(t, "en", resp.Data.Language)
	assert.Equal(t, "MARKETING", resp.Data.Category) // uppercased
	assert.Equal(t, "DRAFT", resp.Data.Status)
	assert.Equal(t, "TEXT", resp.Data.HeaderType)
	assert.Equal(t, "Order Update", resp.Data.HeaderContent)
	assert.Equal(t, "Hello {{1}}, your order is ready!", resp.Data.BodyContent)
	assert.Equal(t, "Reply STOP to unsubscribe", resp.Data.FooterContent)
	assert.Equal(t, account.Name, resp.Data.WhatsAppAccount)
	assert.NotEqual(t, uuid.Nil, resp.Data.ID)
}

func TestApp_CreateTemplate_MissingRequiredFields(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	// Missing name, language, category, body_content
	body := map[string]interface{}{
		"whatsapp_account": "some-account",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "required")
}

func TestApp_CreateTemplate_MissingBodyContent(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	body := map[string]interface{}{
		"whatsapp_account": account.Name,
		"name":             "test_template",
		"language":         "en",
		"category":         "MARKETING",
		// body_content is missing
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "required")
}

func TestApp_CreateTemplate_AccountNotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	body := map[string]interface{}{
		"whatsapp_account": "nonexistent-account",
		"name":             "test_template",
		"language":         "en",
		"category":         "MARKETING",
		"body_content":     "Hello!",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "WhatsApp account not found")
}

func TestApp_CreateTemplate_DuplicateName(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	// Create first template
	createTestTemplateInDB(t, app, org.ID, account.Name, "duplicate_name", "DRAFT")

	// Try to create another with the same name
	body := map[string]interface{}{
		"whatsapp_account": account.Name,
		"name":             "duplicate_name",
		"language":         "en",
		"category":         "MARKETING",
		"body_content":     "Hello!",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusConflict, "already exists")
}

func TestApp_CreateTemplate_AccountFromAnotherOrg(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
	account2 := testutil.CreateTestWhatsAppAccount(t, app.DB, org2.ID)

	body := map[string]interface{}{
		"whatsapp_account": account2.Name,
		"name":             "test_template",
		"language":         "en",
		"category":         "MARKETING",
		"body_content":     "Hello!",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org1.ID, user1.ID)

	err := app.CreateTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "WhatsApp account not found")
}

func TestApp_CreateTemplate_InvalidJSON(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBody([]byte(`{invalid json`))
	req := &fastglue.Request{RequestCtx: ctx}
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid request body")
}

func TestApp_CreateTemplate_NameNormalization(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	body := map[string]interface{}{
		"whatsapp_account": account.Name,
		"name":             "My Template-Name With Spaces!",
		"language":         "en",
		"category":         "MARKETING",
		"body_content":     "Hello!",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateTemplate(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.TemplateResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	// Should be lowercase, spaces->underscores, hyphens->underscores, special chars removed
	assert.Equal(t, "my_template_name_with_spaces", resp.Data.Name)
}

// --- GetTemplate Tests ---

func TestApp_GetTemplate_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	tmpl := createTestTemplateInDB(t, app, org.ID, account.Name, "get_me", "APPROVED")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", tmpl.ID.String())

	err := app.GetTemplate(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.TemplateResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, tmpl.ID, resp.Data.ID)
	assert.Equal(t, "get_me", resp.Data.Name)
	assert.Equal(t, "APPROVED", resp.Data.Status)
	assert.Equal(t, account.Name, resp.Data.WhatsAppAccount)
}

func TestApp_GetTemplate_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "not found")
}

func TestApp_GetTemplate_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.GetTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid template ID")
}

func TestApp_GetTemplate_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
	account2 := testutil.CreateTestWhatsAppAccount(t, app.DB, org2.ID)

	tmpl := createTestTemplateInDB(t, app, org2.ID, account2.Name, "org2_private", "APPROVED")

	// User from org1 should not be able to access org2 template
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org1.ID, user1.ID)
	testutil.SetPathParam(req, "id", tmpl.ID.String())

	err := app.GetTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "not found")
}

// --- UpdateTemplate Tests ---

func TestApp_UpdateTemplate_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	tmpl := createTestTemplateInDB(t, app, org.ID, account.Name, "update_me", "DRAFT")

	body := map[string]interface{}{
		"display_name": "Updated Display Name",
		"body_content": "Updated body {{1}}",
		"category":     "utility",
		"language":     "es",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", tmpl.ID.String())

	err := app.UpdateTemplate(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.TemplateResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, "Updated Display Name", resp.Data.DisplayName)
	assert.Equal(t, "Updated body {{1}}", resp.Data.BodyContent)
	assert.Equal(t, "UTILITY", resp.Data.Category)
	assert.Equal(t, "es", resp.Data.Language)
}

func TestApp_UpdateTemplate_ApprovedToDraft(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	// Create an approved template
	tmpl := createTestTemplateInDB(t, app, org.ID, account.Name, "approved_tmpl", "APPROVED")

	body := map[string]interface{}{
		"body_content": "Updated body content",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", tmpl.ID.String())

	err := app.UpdateTemplate(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.TemplateResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, "Updated body content", resp.Data.BodyContent)
	assert.Equal(t, "DRAFT", resp.Data.Status, "Status should change to DRAFT after editing approved template")
}

func TestApp_UpdateTemplate_RejectedToDraft(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	// Create a rejected template
	tmpl := createTestTemplateInDB(t, app, org.ID, account.Name, "rejected_tmpl", "REJECTED")

	body := map[string]interface{}{
		"body_content": "Fixed body content",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", tmpl.ID.String())

	err := app.UpdateTemplate(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.TemplateResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, "Fixed body content", resp.Data.BodyContent)
	assert.Equal(t, "DRAFT", resp.Data.Status, "Status should change to DRAFT after editing rejected template")
}

func TestApp_UpdateTemplate_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	body := map[string]interface{}{
		"body_content": "Updated content",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.UpdateTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "not found")
}

func TestApp_UpdateTemplate_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	body := map[string]interface{}{
		"body_content": "Updated content",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "bad-uuid")

	err := app.UpdateTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid template ID")
}

func TestApp_UpdateTemplate_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
	account2 := testutil.CreateTestWhatsAppAccount(t, app.DB, org2.ID)

	tmpl := createTestTemplateInDB(t, app, org2.ID, account2.Name, "org2_tmpl", "DRAFT")

	body := map[string]interface{}{
		"body_content": "Trying to update other org's template",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org1.ID, user1.ID)
	testutil.SetPathParam(req, "id", tmpl.ID.String())

	err := app.UpdateTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "not found")
}

func TestApp_UpdateTemplate_RejectedTemplateEditable(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	tmpl := createTestTemplateInDB(t, app, org.ID, account.Name, "rejected_tmpl", "REJECTED")

	body := map[string]interface{}{
		"body_content": "Fixed content after rejection",
	}

	req := testutil.NewJSONRequest(t, body)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", tmpl.ID.String())

	err := app.UpdateTemplate(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.TemplateResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, "Fixed content after rejection", resp.Data.BodyContent)
}

// --- DeleteTemplate Tests ---

func TestApp_DeleteTemplate_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	tmpl := createTestTemplateInDB(t, app, org.ID, account.Name, "delete_me", "DRAFT")

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", tmpl.ID.String())

	err := app.DeleteTemplate(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Contains(t, resp.Data.Message, "deleted successfully")

	// Verify template is soft-deleted
	var count int64
	app.DB.Model(&models.Template{}).Where("id = ?", tmpl.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestApp_DeleteTemplate_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "not found")
}

func TestApp_DeleteTemplate_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "invalid-uuid")

	err := app.DeleteTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid template ID")
}

func TestApp_DeleteTemplate_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
	account2 := testutil.CreateTestWhatsAppAccount(t, app.DB, org2.ID)

	tmpl := createTestTemplateInDB(t, app, org2.ID, account2.Name, "org2_tmpl", "DRAFT")

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org1.ID, user1.ID)
	testutil.SetPathParam(req, "id", tmpl.ID.String())

	err := app.DeleteTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "not found")

	// Verify the template still exists in org2
	var count int64
	app.DB.Model(&models.Template{}).Where("id = ?", tmpl.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}

// --- SubmitTemplate Tests ---

func TestApp_SubmitTemplate_Success(t *testing.T) {
	t.Parallel()

	server := newMockTemplateServer(t)
	defer server.Close()
	app := newTemplateTestApp(t, server)

	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	tmpl := createTestTemplateInDB(t, app, org.ID, account.Name, "submit_me", "DRAFT")

	// Add sample values so the WhatsApp API submission includes required examples
	tmpl.SampleValues = models.JSONBArray{
		map[string]interface{}{"component": "body", "index": 1, "value": "John"},
	}
	require.NoError(t, app.DB.Save(tmpl).Error)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", tmpl.ID.String())

	err := app.SubmitTemplate(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Message        string                    `json:"message"`
			MetaTemplateID string                    `json:"meta_template_id"`
			Status         string                    `json:"status"`
			Template       handlers.TemplateResponse `json:"template"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Contains(t, resp.Data.Message, "submitted")
	assert.Equal(t, "PENDING", resp.Data.Status)
	assert.NotEmpty(t, resp.Data.MetaTemplateID)
	assert.Equal(t, "PENDING", resp.Data.Template.Status)
}

func TestApp_SubmitTemplate_AlreadySubmitted(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	// Create a template that already has a MetaTemplateID and is PENDING
	tmpl := &models.Template{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "already_submitted",
		DisplayName:     "already_submitted",
		Language:        "en",
		Category:        "MARKETING",
		Status:          "PENDING",
		MetaTemplateID:  "meta-existing-123",
		BodyContent:     "Hello!",
	}
	require.NoError(t, app.DB.Create(tmpl).Error)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", tmpl.ID.String())

	err := app.SubmitTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "pending approval")
}

func TestApp_SubmitTemplate_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.SubmitTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "not found")
}

func TestApp_SubmitTemplate_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "bad-id")

	err := app.SubmitTemplate(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid template ID")
}

// --- SyncTemplates Tests ---

func TestApp_SyncTemplates_Success(t *testing.T) {
	t.Parallel()

	server := newMockTemplateServer(t)
	defer server.Close()
	app := newTemplateTestApp(t, server)

	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"whatsapp_account": account.Name,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.SyncTemplates(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Message string `json:"message"`
			Count   int    `json:"count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, 2, resp.Data.Count)
	assert.Contains(t, resp.Data.Message, "Synced 2 templates")

	// Verify templates were created in the database
	var templates []models.Template
	app.DB.Where("organization_id = ?", org.ID).Find(&templates)
	assert.Len(t, templates, 2)
}

func TestApp_SyncTemplates_MissingAccount(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.SyncTemplates(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "whatsapp_account is required")
}

func TestApp_SyncTemplates_AccountNotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"whatsapp_account": "nonexistent-account",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.SyncTemplates(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "WhatsApp account not found")
}

func TestApp_SyncTemplates_ViaQueryParam(t *testing.T) {
	t.Parallel()

	server := newMockTemplateServer(t)
	defer server.Close()
	app := newTemplateTestApp(t, server)

	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "account", account.Name)

	err := app.SyncTemplates(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Count int `json:"count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, 2, resp.Data.Count)
}
