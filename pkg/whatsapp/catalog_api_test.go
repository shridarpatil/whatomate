package whatsapp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- CreateCatalog ---

func TestClient_CreateCatalog_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/owned_product_catalogs")

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "My Catalog", body["name"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "catalog-123"})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	id, err := client.CreateCatalog(context.Background(), account, "My Catalog")
	require.NoError(t, err)
	assert.Equal(t, "catalog-123", id)
}

func TestClient_CreateCatalog_APIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(whatsapp.MetaAPIError{
			Error: struct {
				Message      string `json:"message"`
				Type         string `json:"type"`
				Code         int    `json:"code"`
				ErrorSubcode int    `json:"error_subcode"`
				ErrorUserMsg string `json:"error_user_msg"`
				ErrorData    struct {
					Details string `json:"details"`
				} `json:"error_data"`
				FBTraceID string `json:"fbtrace_id"`
			}{
				Message: "Catalog limit reached",
				Code:    100,
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	_, err := client.CreateCatalog(context.Background(), account, "Too Many")
	require.Error(t, err)
}

// --- ListCatalogs ---

func TestClient_ListCatalogs_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{
				{"id": "cat-1", "name": "Catalog 1"},
				{"id": "cat-2", "name": "Catalog 2"},
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	catalogs, err := client.ListCatalogs(context.Background(), account)
	require.NoError(t, err)
	require.Len(t, catalogs, 2)
	assert.Equal(t, "cat-1", catalogs[0].ID)
	assert.Equal(t, "Catalog 1", catalogs[0].Name)
}

func TestClient_ListCatalogs_Empty(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	catalogs, err := client.ListCatalogs(context.Background(), account)
	require.NoError(t, err)
	assert.Empty(t, catalogs)
}

// --- DeleteCatalog ---

func TestClient_DeleteCatalog_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	err := client.DeleteCatalog(context.Background(), account, "catalog-123")
	require.NoError(t, err)
}

// --- ListCatalogProducts ---

func TestClient_ListCatalogProducts_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/products")

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "prod-1", "name": "Product 1", "price": "1000", "currency": "USD"},
				{"id": "prod-2", "name": "Product 2", "price": "2000", "currency": "USD"},
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	products, err := client.ListCatalogProducts(context.Background(), account, "catalog-123")
	require.NoError(t, err)
	require.Len(t, products, 2)
	assert.Equal(t, "prod-1", products[0].ID)
	assert.Equal(t, "Product 1", products[0].Name)
}

func TestClient_ListCatalogProducts_Empty(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	products, err := client.ListCatalogProducts(context.Background(), account, "catalog-123")
	require.NoError(t, err)
	assert.Empty(t, products)
}

// --- CreateProduct ---

func TestClient_CreateProduct_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/products")

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Test Product", body["name"])
		assert.Equal(t, "USD", body["currency"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "prod-new"})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	product := &whatsapp.ProductInput{
		Name:        "Test Product",
		Price:       1999,
		Currency:    "USD",
		URL:         "https://example.com/product",
		ImageURL:    "https://example.com/image.jpg",
		RetailerID:  "SKU-001",
		Description: "A test product",
	}

	id, err := client.CreateProduct(context.Background(), account, "catalog-123", product)
	require.NoError(t, err)
	assert.Equal(t, "prod-new", id)
}

// --- UpdateProduct ---

func TestClient_UpdateProduct_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Updated Product", body["name"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	product := &whatsapp.ProductInput{
		Name:  "Updated Product",
		Price: 2999,
	}

	err := client.UpdateProduct(context.Background(), account, "prod-123", product)
	require.NoError(t, err)
}

// --- DeleteProduct ---

func TestClient_DeleteProduct_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	err := client.DeleteProduct(context.Background(), account, "prod-123")
	require.NoError(t, err)
}

func TestClient_DeleteProduct_APIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(whatsapp.MetaAPIError{
			Error: struct {
				Message      string `json:"message"`
				Type         string `json:"type"`
				Code         int    `json:"code"`
				ErrorSubcode int    `json:"error_subcode"`
				ErrorUserMsg string `json:"error_user_msg"`
				ErrorData    struct {
					Details string `json:"details"`
				} `json:"error_data"`
				FBTraceID string `json:"fbtrace_id"`
			}{
				Message: "Product not found",
				Code:    100,
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	err := client.DeleteProduct(context.Background(), account, "nonexistent")
	require.Error(t, err)
}
