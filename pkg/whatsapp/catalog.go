package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// buildCatalogsURL builds the catalogs endpoint URL for a business
func (c *Client) buildCatalogsURL(account *Account) string {
	return fmt.Sprintf("%s/%s/%s/owned_product_catalogs", c.getBaseURL(), account.APIVersion, account.BusinessID)
}

// buildCatalogProductsURL builds the products endpoint URL for a catalog
func (c *Client) buildCatalogProductsURL(account *Account, catalogID string) string {
	return fmt.Sprintf("%s/%s/%s/products", c.getBaseURL(), account.APIVersion, catalogID)
}

// buildProductURL builds the URL for a specific product
func (c *Client) buildProductURL(account *Account, productID string) string {
	return fmt.Sprintf("%s/%s/%s", c.getBaseURL(), account.APIVersion, productID)
}

// CreateCatalog creates a new product catalog
func (c *Client) CreateCatalog(ctx context.Context, account *Account, name string) (string, error) {
	apiURL := c.buildCatalogsURL(account)

	body := map[string]string{
		"name": name,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, apiURL, body, account.AccessToken)
	if err != nil {
		return "", err
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return resp.ID, nil
}

// ListCatalogs lists all catalogs for a business
func (c *Client) ListCatalogs(ctx context.Context, account *Account) ([]CatalogInfo, error) {
	apiURL := c.buildCatalogsURL(account)

	respBody, err := c.doRequest(ctx, http.MethodGet, apiURL, nil, account.AccessToken)
	if err != nil {
		return nil, err
	}

	var resp CatalogListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return resp.Data, nil
}

// DeleteCatalog deletes a catalog
func (c *Client) DeleteCatalog(ctx context.Context, account *Account, catalogID string) error {
	apiURL := fmt.Sprintf("%s/%s/%s", c.getBaseURL(), account.APIVersion, catalogID)

	_, err := c.doRequest(ctx, http.MethodDelete, apiURL, nil, account.AccessToken)
	return err
}

// ListCatalogProducts lists all products in a catalog
func (c *Client) ListCatalogProducts(ctx context.Context, account *Account, catalogID string) ([]ProductInfo, error) {
	apiURL := c.buildCatalogProductsURL(account, catalogID)

	// Add fields parameter to get all product details
	params := url.Values{}
	params.Add("fields", "id,name,price,currency,url,image_url,retailer_id,description")
	apiURL = apiURL + "?" + params.Encode()

	respBody, err := c.doRequest(ctx, http.MethodGet, apiURL, nil, account.AccessToken)
	if err != nil {
		return nil, err
	}

	var resp ProductListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return resp.Data, nil
}

// CreateProduct adds a product to a catalog
func (c *Client) CreateProduct(ctx context.Context, account *Account, catalogID string, product *ProductInput) (string, error) {
	apiURL := c.buildCatalogProductsURL(account, catalogID)

	// Meta API expects price as string with currency code
	priceStr := strconv.FormatInt(product.Price, 10)

	body := map[string]string{
		"name":        product.Name,
		"price":       priceStr,
		"currency":    product.Currency,
		"url":         product.URL,
		"image_url":   product.ImageURL,
		"retailer_id": product.RetailerID,
	}

	if product.Description != "" {
		body["description"] = product.Description
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, apiURL, body, account.AccessToken)
	if err != nil {
		return "", err
	}

	var resp ProductCreateResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return resp.ID, nil
}

// UpdateProduct updates a product
func (c *Client) UpdateProduct(ctx context.Context, account *Account, productID string, product *ProductInput) error {
	apiURL := c.buildProductURL(account, productID)

	body := make(map[string]string)

	if product.Name != "" {
		body["name"] = product.Name
	}
	if product.Price > 0 {
		body["price"] = strconv.FormatInt(product.Price, 10)
	}
	if product.Currency != "" {
		body["currency"] = product.Currency
	}
	if product.URL != "" {
		body["url"] = product.URL
	}
	if product.ImageURL != "" {
		body["image_url"] = product.ImageURL
	}
	if product.Description != "" {
		body["description"] = product.Description
	}

	_, err := c.doRequest(ctx, http.MethodPost, apiURL, body, account.AccessToken)
	return err
}

// DeleteProduct deletes a product
func (c *Client) DeleteProduct(ctx context.Context, account *Account, productID string) error {
	apiURL := c.buildProductURL(account, productID)

	_, err := c.doRequest(ctx, http.MethodDelete, apiURL, nil, account.AccessToken)
	return err
}
