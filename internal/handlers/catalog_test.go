package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// --- Catalog Test Helpers ---

// mockCatalogServer creates a mock WhatsApp API server for catalog operations.
type mockCatalogServer struct {
	server *httptest.Server

	// Configurable responses
	nextCatalogID string
	nextProductID string
	returnError   bool
	errorMessage  string
}

func newMockCatalogServer() *mockCatalogServer {
	m := &mockCatalogServer{
		nextCatalogID: "meta-catalog-" + uuid.New().String()[:8],
		nextProductID: "meta-product-" + uuid.New().String()[:8],
	}

	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		switch r.Method {
		case http.MethodPost:
			// Handle catalog or product creation
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": m.nextCatalogID,
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
			})
		case http.MethodGet:
			// List catalogs
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]string{},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	return m
}

func (m *mockCatalogServer) close() {
	m.server.Close()
}

// newCatalogTestApp creates an App instance for catalog testing with a mock WhatsApp server.
func newCatalogTestApp(t *testing.T, mockServer *mockCatalogServer) *handlers.App {
	t.Helper()

	log := testutil.NopLogger()
	waClient := whatsapp.NewWithBaseURL(log, mockServer.server.URL)

	return newTestApp(t, withWhatsApp(waClient))
}

// createTestCatalog creates a test catalog directly in the database.
func createTestCatalog(t *testing.T, app *handlers.App, orgID uuid.UUID, accountName, name string) *models.Catalog {
	t.Helper()

	catalog := &models.Catalog{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: accountName,
		MetaCatalogID:   "meta-catalog-" + uuid.New().String()[:8],
		Name:            name,
		IsActive:        true,
	}
	require.NoError(t, app.DB.Create(catalog).Error)
	return catalog
}

// createTestCatalogProduct creates a test catalog product directly in the database.
func createTestCatalogProduct(t *testing.T, app *handlers.App, orgID, catalogID uuid.UUID, name string, price int64) *models.CatalogProduct {
	t.Helper()

	product := &models.CatalogProduct{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		CatalogID:      catalogID,
		MetaProductID:  "meta-product-" + uuid.New().String()[:8],
		Name:           name,
		Description:    "Test product description",
		Price:          price,
		Currency:       "USD",
		URL:            "https://example.com/product",
		ImageURL:       "https://example.com/image.jpg",
		RetailerID:     "SKU-" + uuid.New().String()[:8],
		IsActive:       true,
	}
	require.NoError(t, app.DB.Create(product).Error)
	return product
}

// createCatalogTestAccount creates a WhatsApp account with predictable fields for catalog tests.
func createCatalogTestAccount(t *testing.T, app *handlers.App, orgID uuid.UUID) *models.WhatsAppAccount {
	t.Helper()

	account := &models.WhatsAppAccount{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     orgID,
		Name:               "test-account-" + uuid.New().String()[:8],
		PhoneID:            "phone-" + uuid.New().String()[:8],
		BusinessID:         "business-" + uuid.New().String()[:8],
		AccessToken:        "test-token",
		WebhookVerifyToken: "webhook-token",
		APIVersion:         "v18.0",
		Status:             "active",
	}
	require.NoError(t, app.DB.Create(account).Error)
	return account
}

// --- ListCatalogs Tests ---

func TestApp_ListCatalogs_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	createTestCatalog(t, app, org.ID, account.Name, "Catalog A")
	createTestCatalog(t, app, org.ID, account.Name, "Catalog B")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListCatalogs(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Catalogs []handlers.CatalogResponse `json:"catalogs"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Catalogs, 2)

	// Catalogs are ordered by name ASC
	assert.Equal(t, "Catalog A", resp.Data.Catalogs[0].Name)
	assert.Equal(t, "Catalog B", resp.Data.Catalogs[1].Name)
	assert.True(t, resp.Data.Catalogs[0].IsActive)
}

func TestApp_ListCatalogs_Empty(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListCatalogs(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Catalogs []handlers.CatalogResponse `json:"catalogs"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Catalogs, 0)
}

func TestApp_ListCatalogs_FilterByWhatsAppAccount(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	account2 := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	createTestCatalog(t, app, org.ID, account1.Name, "Catalog for Account 1")
	createTestCatalog(t, app, org.ID, account2.Name, "Catalog for Account 2")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "whatsapp_account", account1.Name)

	err := app.ListCatalogs(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Catalogs []handlers.CatalogResponse `json:"catalogs"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Catalogs, 1)
	assert.Equal(t, account1.Name, resp.Data.Catalogs[0].WhatsAppAccount)
}

func TestApp_ListCatalogs_WithProductCount(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	catalog := createTestCatalog(t, app, org.ID, account.Name, "Catalog with Products")
	createTestCatalogProduct(t, app, org.ID, catalog.ID, "Product 1", 1000)
	createTestCatalogProduct(t, app, org.ID, catalog.ID, "Product 2", 2000)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListCatalogs(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Catalogs []handlers.CatalogResponse `json:"catalogs"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Data.Catalogs, 1)
	assert.Equal(t, 2, resp.Data.Catalogs[0].ProductCount)
}

func TestApp_ListCatalogs_OrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)
	account1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)

	createTestCatalog(t, app, org1.ID, account1.Name, "Org1 Catalog")

	// User from org2 should not see org1's catalogs
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)

	err := app.ListCatalogs(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Catalogs []handlers.CatalogResponse `json:"catalogs"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Catalogs, 0)
}

// --- CreateCatalog Tests ---

func TestApp_CreateCatalog_Success(t *testing.T) {
	mockServer := newMockCatalogServer()
	defer mockServer.close()

	app := newCatalogTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := createCatalogTestAccount(t, app, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "My Test Catalog",
		"whatsapp_account": account.Name,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCatalog(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CatalogResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "My Test Catalog", resp.Data.Name)
	assert.Equal(t, account.Name, resp.Data.WhatsAppAccount)
	assert.Equal(t, mockServer.nextCatalogID, resp.Data.MetaCatalogID)
	assert.True(t, resp.Data.IsActive)
	assert.Equal(t, 0, resp.Data.ProductCount)
	assert.NotEqual(t, uuid.Nil, resp.Data.ID)

	// Verify catalog was persisted in the database
	var dbCatalog models.Catalog
	require.NoError(t, app.DB.Where("id = ?", resp.Data.ID).First(&dbCatalog).Error)
	assert.Equal(t, "My Test Catalog", dbCatalog.Name)
	assert.Equal(t, mockServer.nextCatalogID, dbCatalog.MetaCatalogID)
}

func TestApp_CreateCatalog_MissingFields(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "missing_name",
			body: map[string]interface{}{
				"whatsapp_account": "some-account",
			},
		},
		{
			name: "missing_whatsapp_account",
			body: map[string]interface{}{
				"name": "My Catalog",
			},
		},
		{
			name: "all_fields_empty",
			body: map[string]interface{}{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.NewJSONRequest(t, tc.body)
			testutil.SetAuthContext(req, org.ID, user.ID)

			err := app.CreateCatalog(req)
			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
		})
	}
}

func TestApp_CreateCatalog_AccountNotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "My Catalog",
		"whatsapp_account": "nonexistent-account",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCatalog(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateCatalog_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "My Catalog",
		"whatsapp_account": "some-account",
	})
	// No auth context

	err := app.CreateCatalog(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- GetCatalog Tests ---

func TestApp_GetCatalog_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	catalog := createTestCatalog(t, app, org.ID, account.Name, "Test Catalog")
	product := createTestCatalogProduct(t, app, org.ID, catalog.ID, "Test Product", 1500)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", catalog.ID.String())

	err := app.GetCatalog(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CatalogResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, catalog.ID, resp.Data.ID)
	assert.Equal(t, "Test Catalog", resp.Data.Name)
	assert.Equal(t, account.Name, resp.Data.WhatsAppAccount)
	assert.True(t, resp.Data.IsActive)
	assert.Equal(t, 1, resp.Data.ProductCount)

	// Verify products are included
	require.Len(t, resp.Data.Products, 1)
	assert.Equal(t, product.ID, resp.Data.Products[0].ID)
	assert.Equal(t, "Test Product", resp.Data.Products[0].Name)
	assert.Equal(t, int64(1500), resp.Data.Products[0].Price)
	assert.Equal(t, "USD", resp.Data.Products[0].Currency)
}

func TestApp_GetCatalog_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetCatalog(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_GetCatalog_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.GetCatalog(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GetCatalog_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)
	account1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)

	catalog := createTestCatalog(t, app, org1.ID, account1.Name, "Org1 Catalog")

	// User from org2 tries to access org1's catalog
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", catalog.ID.String())

	err := app.GetCatalog(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- DeleteCatalog Tests ---

func TestApp_DeleteCatalog_Success(t *testing.T) {
	mockServer := newMockCatalogServer()
	defer mockServer.close()

	app := newCatalogTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := createCatalogTestAccount(t, app, org.ID)

	catalog := createTestCatalog(t, app, org.ID, account.Name, "Catalog to Delete")
	// Add a product to verify cascade deletion
	createTestCatalogProduct(t, app, org.ID, catalog.ID, "Product to Delete", 500)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", catalog.ID.String())

	err := app.DeleteCatalog(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Catalog deleted", resp.Data.Message)

	// Verify catalog is deleted from DB
	var catalogCount int64
	app.DB.Model(&models.Catalog{}).Where("id = ?", catalog.ID).Count(&catalogCount)
	assert.Equal(t, int64(0), catalogCount)

	// Verify products are also deleted
	var productCount int64
	app.DB.Model(&models.CatalogProduct{}).Where("catalog_id = ?", catalog.ID).Count(&productCount)
	assert.Equal(t, int64(0), productCount)
}

func TestApp_DeleteCatalog_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteCatalog(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteCatalog_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.DeleteCatalog(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteCatalog_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)
	account1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)

	catalog := createTestCatalog(t, app, org1.ID, account1.Name, "Org1 Catalog")

	// User from org2 tries to delete org1's catalog
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", catalog.ID.String())

	err := app.DeleteCatalog(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	// Verify catalog still exists
	var count int64
	app.DB.Model(&models.Catalog{}).Where("id = ?", catalog.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}

// --- ListCatalogProducts Tests ---

func TestApp_ListCatalogProducts_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	catalog := createTestCatalog(t, app, org.ID, account.Name, "Test Catalog")
	createTestCatalogProduct(t, app, org.ID, catalog.ID, "Alpha Product", 1000)
	createTestCatalogProduct(t, app, org.ID, catalog.ID, "Beta Product", 2000)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", catalog.ID.String())

	err := app.ListCatalogProducts(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Products []handlers.CatalogProductResponse `json:"products"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Products, 2)

	// Products are ordered by name ASC
	assert.Equal(t, "Alpha Product", resp.Data.Products[0].Name)
	assert.Equal(t, "Beta Product", resp.Data.Products[1].Name)
	assert.Equal(t, int64(1000), resp.Data.Products[0].Price)
	assert.Equal(t, int64(2000), resp.Data.Products[1].Price)
}

func TestApp_ListCatalogProducts_Empty(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	catalog := createTestCatalog(t, app, org.ID, account.Name, "Empty Catalog")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", catalog.ID.String())

	err := app.ListCatalogProducts(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Products []handlers.CatalogProductResponse `json:"products"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Products, 0)
}

func TestApp_ListCatalogProducts_CatalogNotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.ListCatalogProducts(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_ListCatalogProducts_OnlyShowsProductsForCatalog(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	catalog1 := createTestCatalog(t, app, org.ID, account.Name, "Catalog 1")
	catalog2 := createTestCatalog(t, app, org.ID, account.Name, "Catalog 2")

	createTestCatalogProduct(t, app, org.ID, catalog1.ID, "Product in Catalog 1", 1000)
	createTestCatalogProduct(t, app, org.ID, catalog2.ID, "Product in Catalog 2", 2000)

	// List products for catalog1 only
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", catalog1.ID.String())

	err := app.ListCatalogProducts(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Products []handlers.CatalogProductResponse `json:"products"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Data.Products, 1)
	assert.Equal(t, "Product in Catalog 1", resp.Data.Products[0].Name)
}

// --- CreateCatalogProduct Tests ---

func TestApp_CreateCatalogProduct_Success(t *testing.T) {
	mockServer := newMockCatalogServer()
	defer mockServer.close()

	// Override the nextCatalogID to serve as product ID
	mockServer.nextCatalogID = mockServer.nextProductID

	app := newCatalogTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := createCatalogTestAccount(t, app, org.ID)

	catalog := createTestCatalog(t, app, org.ID, account.Name, "Test Catalog")

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":        "New Product",
		"description": "A great product",
		"price":       2500,
		"currency":    "EUR",
		"url":         "https://example.com/product/1",
		"image_url":   "https://example.com/image/1.jpg",
		"retailer_id": "SKU-001",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", catalog.ID.String())

	err := app.CreateCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CatalogProductResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "New Product", resp.Data.Name)
	assert.Equal(t, "A great product", resp.Data.Description)
	assert.Equal(t, int64(2500), resp.Data.Price)
	assert.Equal(t, "EUR", resp.Data.Currency)
	assert.Equal(t, "https://example.com/product/1", resp.Data.URL)
	assert.Equal(t, "https://example.com/image/1.jpg", resp.Data.ImageURL)
	assert.Equal(t, "SKU-001", resp.Data.RetailerID)
	assert.True(t, resp.Data.IsActive)
	assert.NotEqual(t, uuid.Nil, resp.Data.ID)

	// Verify product was persisted in the database
	var dbProduct models.CatalogProduct
	require.NoError(t, app.DB.Where("id = ?", resp.Data.ID).First(&dbProduct).Error)
	assert.Equal(t, "New Product", dbProduct.Name)
	assert.Equal(t, catalog.ID, dbProduct.CatalogID)
	assert.Equal(t, org.ID, dbProduct.OrganizationID)
}

func TestApp_CreateCatalogProduct_DefaultCurrency(t *testing.T) {
	mockServer := newMockCatalogServer()
	defer mockServer.close()
	mockServer.nextCatalogID = mockServer.nextProductID

	app := newCatalogTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := createCatalogTestAccount(t, app, org.ID)

	catalog := createTestCatalog(t, app, org.ID, account.Name, "Test Catalog")

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":  "Product Without Currency",
		"price": 1000,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", catalog.ID.String())

	err := app.CreateCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CatalogProductResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "USD", resp.Data.Currency)
}

func TestApp_CreateCatalogProduct_MissingFields(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	catalog := createTestCatalog(t, app, org.ID, account.Name, "Test Catalog")

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "missing_name",
			body: map[string]interface{}{
				"price": 1000,
			},
		},
		{
			name: "missing_price",
			body: map[string]interface{}{
				"name": "Product Without Price",
			},
		},
		{
			name: "zero_price",
			body: map[string]interface{}{
				"name":  "Product With Zero Price",
				"price": 0,
			},
		},
		{
			name: "negative_price",
			body: map[string]interface{}{
				"name":  "Product With Negative Price",
				"price": -100,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.NewJSONRequest(t, tc.body)
			testutil.SetAuthContext(req, org.ID, user.ID)
			testutil.SetPathParam(req, "id", catalog.ID.String())

			err := app.CreateCatalogProduct(req)
			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
		})
	}
}

func TestApp_CreateCatalogProduct_CatalogNotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":  "Product",
		"price": 1000,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.CreateCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- GetCatalogProduct Tests ---

func TestApp_GetCatalogProduct_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	catalog := createTestCatalog(t, app, org.ID, account.Name, "Test Catalog")
	product := createTestCatalogProduct(t, app, org.ID, catalog.ID, "Test Product", 3000)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", product.ID.String())

	err := app.GetCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CatalogProductResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, product.ID, resp.Data.ID)
	assert.Equal(t, "Test Product", resp.Data.Name)
	assert.Equal(t, int64(3000), resp.Data.Price)
	assert.Equal(t, "USD", resp.Data.Currency)
	assert.Equal(t, product.RetailerID, resp.Data.RetailerID)
	assert.True(t, resp.Data.IsActive)
	assert.NotEmpty(t, resp.Data.CreatedAt)
	assert.NotEmpty(t, resp.Data.UpdatedAt)
}

func TestApp_GetCatalogProduct_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_GetCatalogProduct_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.GetCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GetCatalogProduct_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)
	account1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)

	catalog := createTestCatalog(t, app, org1.ID, account1.Name, "Org1 Catalog")
	product := createTestCatalogProduct(t, app, org1.ID, catalog.ID, "Org1 Product", 1000)

	// User from org2 tries to access org1's product
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", product.ID.String())

	err := app.GetCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- UpdateCatalogProduct Tests ---

func TestApp_UpdateCatalogProduct_Success(t *testing.T) {
	mockServer := newMockCatalogServer()
	defer mockServer.close()

	app := newCatalogTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := createCatalogTestAccount(t, app, org.ID)

	catalog := createTestCatalog(t, app, org.ID, account.Name, "Test Catalog")
	product := createTestCatalogProduct(t, app, org.ID, catalog.ID, "Original Product", 1000)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":        "Updated Product",
		"description": "Updated description",
		"price":       2500,
		"currency":    "EUR",
		"url":         "https://example.com/updated",
		"image_url":   "https://example.com/updated-image.jpg",
		"retailer_id": "SKU-UPDATED",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", product.ID.String())

	err := app.UpdateCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CatalogProductResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, product.ID, resp.Data.ID)
	assert.Equal(t, "Updated Product", resp.Data.Name)
	assert.Equal(t, "Updated description", resp.Data.Description)
	assert.Equal(t, int64(2500), resp.Data.Price)
	assert.Equal(t, "EUR", resp.Data.Currency)
	assert.Equal(t, "https://example.com/updated", resp.Data.URL)
	assert.Equal(t, "https://example.com/updated-image.jpg", resp.Data.ImageURL)
	assert.Equal(t, "SKU-UPDATED", resp.Data.RetailerID)

	// Verify changes persisted in database
	var dbProduct models.CatalogProduct
	require.NoError(t, app.DB.Where("id = ?", product.ID).First(&dbProduct).Error)
	assert.Equal(t, "Updated Product", dbProduct.Name)
	assert.Equal(t, int64(2500), dbProduct.Price)
	assert.Equal(t, "EUR", dbProduct.Currency)
}

func TestApp_UpdateCatalogProduct_PartialUpdate(t *testing.T) {
	mockServer := newMockCatalogServer()
	defer mockServer.close()

	app := newCatalogTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := createCatalogTestAccount(t, app, org.ID)

	catalog := createTestCatalog(t, app, org.ID, account.Name, "Test Catalog")
	product := createTestCatalogProduct(t, app, org.ID, catalog.ID, "Original Product", 1000)

	// Only update the name
	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name": "Only Name Changed",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", product.ID.String())

	err := app.UpdateCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CatalogProductResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Only Name Changed", resp.Data.Name)
	// Original values should be preserved
	assert.Equal(t, product.Price, resp.Data.Price)
	assert.Equal(t, product.Currency, resp.Data.Currency)
	assert.Equal(t, product.Description, resp.Data.Description)
}

func TestApp_UpdateCatalogProduct_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name": "Updated Name",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.UpdateCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateCatalogProduct_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name": "Updated Name",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.UpdateCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- DeleteCatalogProduct Tests ---

func TestApp_DeleteCatalogProduct_Success(t *testing.T) {
	mockServer := newMockCatalogServer()
	defer mockServer.close()

	app := newCatalogTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := createCatalogTestAccount(t, app, org.ID)

	catalog := createTestCatalog(t, app, org.ID, account.Name, "Test Catalog")
	product := createTestCatalogProduct(t, app, org.ID, catalog.ID, "Product to Delete", 1500)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", product.ID.String())

	err := app.DeleteCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Product deleted", resp.Data.Message)

	// Verify product is deleted from DB
	var count int64
	app.DB.Model(&models.CatalogProduct{}).Where("id = ?", product.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestApp_DeleteCatalogProduct_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteCatalogProduct_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.DeleteCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteCatalogProduct_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)
	account1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)

	catalog := createTestCatalog(t, app, org1.ID, account1.Name, "Org1 Catalog")
	product := createTestCatalogProduct(t, app, org1.ID, catalog.ID, "Org1 Product", 1000)

	// User from org2 tries to delete org1's product
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", product.ID.String())

	err := app.DeleteCatalogProduct(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	// Verify product still exists
	var count int64
	app.DB.Model(&models.CatalogProduct{}).Where("id = ?", product.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}
