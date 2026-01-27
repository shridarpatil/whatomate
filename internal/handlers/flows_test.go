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

// createTestFlow creates a test WhatsApp flow in the database.
func createTestFlow(t *testing.T, app *handlers.App, orgID uuid.UUID, accountName, name string) *models.WhatsAppFlow {
	t.Helper()

	flow := &models.WhatsAppFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: accountName,
		Name:            name,
		Status:          "DRAFT",
		Category:        "SIGN_UP",
		JSONVersion:     "6.0",
	}
	require.NoError(t, app.DB.Create(flow).Error)
	return flow
}

// --- ListFlows Tests ---

func TestApp_ListFlows_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	createTestFlow(t, app, org.ID, account.Name, "Flow 1")
	createTestFlow(t, app, org.ID, account.Name, "Flow 2")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListFlows(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Flows []handlers.FlowResponse `json:"flows"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Flows, 2)
}

func TestApp_ListFlows_EmptyList(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListFlows(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Flows []handlers.FlowResponse `json:"flows"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Flows, 0)
}

func TestApp_ListFlows_FilterByAccount(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	account2 := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	createTestFlow(t, app, org.ID, account1.Name, "Flow A1")
	createTestFlow(t, app, org.ID, account1.Name, "Flow A2")
	createTestFlow(t, app, org.ID, account2.Name, "Flow B1")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "account", account1.Name)

	err := app.ListFlows(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Flows []handlers.FlowResponse `json:"flows"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Flows, 2)
	for _, f := range resp.Data.Flows {
		assert.Equal(t, account1.Name, f.WhatsAppAccount)
	}
}

func TestApp_ListFlows_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context set

	err := app.ListFlows(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- CreateFlow Tests ---

func TestApp_CreateFlow_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"whatsapp_account": account.Name,
		"name":             "My New Flow",
		"category":         "SIGN_UP",
		"json_version":     "6.0",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Flow handlers.FlowResponse `json:"flow"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "My New Flow", resp.Data.Flow.Name)
	assert.Equal(t, account.Name, resp.Data.Flow.WhatsAppAccount)
	assert.Equal(t, "DRAFT", resp.Data.Flow.Status)
	assert.Equal(t, "SIGN_UP", resp.Data.Flow.Category)
	assert.Equal(t, "6.0", resp.Data.Flow.JSONVersion)
	assert.NotEqual(t, uuid.Nil, resp.Data.Flow.ID)
}

func TestApp_CreateFlow_DefaultJSONVersion(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"whatsapp_account": account.Name,
		"name":             "Flow Without Version",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Flow handlers.FlowResponse `json:"flow"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "6.0", resp.Data.Flow.JSONVersion)
}

func TestApp_CreateFlow_MissingName(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"whatsapp_account": account.Name,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateFlow_MissingWhatsAppAccount(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Flow Without Account",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateFlow_AccountNotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"whatsapp_account": "nonexistent-account",
		"name":             "Flow With Bad Account",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateFlow_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]any{
		"whatsapp_account": "some-account",
		"name":             "Flow",
	})
	// No auth context

	err := app.CreateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- GetFlow Tests ---

func TestApp_GetFlow_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	flow := createTestFlow(t, app, org.ID, account.Name, "Test Flow")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", flow.ID.String())

	err := app.GetFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Flow handlers.FlowResponse `json:"flow"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, flow.ID, resp.Data.Flow.ID)
	assert.Equal(t, "Test Flow", resp.Data.Flow.Name)
	assert.Equal(t, account.Name, resp.Data.Flow.WhatsAppAccount)
	assert.Equal(t, "DRAFT", resp.Data.Flow.Status)
}

func TestApp_GetFlow_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_GetFlow_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.GetFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GetFlow_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)
	account1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)
	flow := createTestFlow(t, app, org1.ID, account1.Name, "Org1 Flow")

	// User from org2 tries to access org1's flow
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", flow.ID.String())

	err := app.GetFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- UpdateFlow Tests ---

func TestApp_UpdateFlow_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	flow := createTestFlow(t, app, org.ID, account.Name, "Original Flow")

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":     "Updated Flow",
		"category": "CUSTOMER_SUPPORT",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", flow.ID.String())

	err := app.UpdateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Flow handlers.FlowResponse `json:"flow"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, flow.ID, resp.Data.Flow.ID)
	assert.Equal(t, "Updated Flow", resp.Data.Flow.Name)
	assert.Equal(t, "CUSTOMER_SUPPORT", resp.Data.Flow.Category)
	assert.True(t, resp.Data.Flow.HasLocalChanges)
}

func TestApp_UpdateFlow_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Updated Flow",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.UpdateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateFlow_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Updated Flow",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.UpdateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateFlow_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Updated Flow",
	})
	// No auth context
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.UpdateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- DeleteFlow Tests ---

func TestApp_DeleteFlow_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	flow := createTestFlow(t, app, org.ID, account.Name, "Flow To Delete")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", flow.ID.String())

	err := app.DeleteFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Data.Message, "deleted")

	// Verify flow is soft-deleted
	var count int64
	app.DB.Model(&models.WhatsAppFlow{}).Where("id = ?", flow.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestApp_DeleteFlow_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteFlow_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.DeleteFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteFlow_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)
	account1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)
	flow := createTestFlow(t, app, org1.ID, account1.Name, "Org1 Flow")

	// User from org2 tries to delete org1's flow
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", flow.ID.String())

	err := app.DeleteFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	// Flow should still exist
	var count int64
	app.DB.Model(&models.WhatsAppFlow{}).Where("id = ?", flow.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestApp_DeleteFlow_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- DuplicateFlow Tests ---

func TestApp_DuplicateFlow_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	flow := createTestFlow(t, app, org.ID, account.Name, "Original Flow")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", flow.ID.String())

	err := app.DuplicateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Flow    handlers.FlowResponse `json:"flow"`
			Message string                `json:"message"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	// Duplicated flow should have a different ID
	assert.NotEqual(t, flow.ID, resp.Data.Flow.ID)
	assert.NotEqual(t, uuid.Nil, resp.Data.Flow.ID)

	// Duplicated flow should have "(Copy)" appended to the name
	assert.Equal(t, "Original Flow (Copy)", resp.Data.Flow.Name)

	// Duplicated flow should be in DRAFT status
	assert.Equal(t, "DRAFT", resp.Data.Flow.Status)

	// Duplicated flow should keep the same account
	assert.Equal(t, account.Name, resp.Data.Flow.WhatsAppAccount)

	// Duplicated flow should keep the same category
	assert.Equal(t, flow.Category, resp.Data.Flow.Category)

	// MetaFlowID should be empty for the duplicate
	assert.Empty(t, resp.Data.Flow.MetaFlowID)

	// Success message should be present
	assert.Contains(t, resp.Data.Message, "duplicated")
}

func TestApp_DuplicateFlow_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DuplicateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_DuplicateFlow_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.DuplicateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_DuplicateFlow_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)
	account1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)
	flow := createTestFlow(t, app, org1.ID, account1.Name, "Org1 Flow")

	// User from org2 tries to duplicate org1's flow
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", flow.ID.String())

	err := app.DuplicateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_DuplicateFlow_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DuplicateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

func TestApp_DuplicateFlow_PreservesFlowJSON(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	// Create a flow with flow_json and screens
	flow := &models.WhatsAppFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "Flow With JSON",
		Status:          "PUBLISHED",
		Category:        "SIGN_UP",
		JSONVersion:     "6.0",
		MetaFlowID:      "meta-flow-123",
		FlowJSON:        models.JSONB{"version": "6.0"},
		Screens: models.JSONBArray{
			map[string]interface{}{
				"id":   "SCREEN_ONE",
				"title": "Welcome",
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", flow.ID.String())

	err := app.DuplicateFlow(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Flow handlers.FlowResponse `json:"flow"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	// Duplicate should preserve screens
	assert.Len(t, resp.Data.Flow.Screens, 1)

	// Duplicate should be DRAFT regardless of original status
	assert.Equal(t, "DRAFT", resp.Data.Flow.Status)

	// MetaFlowID should be empty (it's a new local-only flow)
	assert.Empty(t, resp.Data.Flow.MetaFlowID)
}
