package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/microsoft"
)

// OAuth provider configurations (endpoints are hardcoded, only need client credentials)
var oauthProviders = map[string]struct {
	Endpoint    oauth2.Endpoint
	Scopes      []string
	UserInfoURL string
}{
	"google": {
		Endpoint:    google.Endpoint,
		Scopes:      []string{"openid", "email", "profile"},
		UserInfoURL: "https://www.googleapis.com/oauth2/v2/userinfo",
	},
	"microsoft": {
		Endpoint:    microsoft.AzureADEndpoint("common"),
		Scopes:      []string{"openid", "email", "profile", "User.Read"},
		UserInfoURL: "https://graph.microsoft.com/v1.0/me",
	},
	"github": {
		Endpoint:    github.Endpoint,
		Scopes:      []string{"user:email", "read:user"},
		UserInfoURL: "https://api.github.com/user",
	},
	"facebook": {
		Endpoint:    facebook.Endpoint,
		Scopes:      []string{"email", "public_profile"},
		UserInfoURL: "https://graph.facebook.com/me?fields=id,email,name",
	},
}

// SSOState represents the state stored in Redis during OAuth flow
type SSOState struct {
	OrgID     string    `json:"org_id"`
	Provider  string    `json:"provider"`
	Nonce     string    `json:"nonce"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SSOProviderPublic represents public SSO provider info (no secrets)
type SSOProviderPublic struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
}

// SSOProviderRequest represents SSO provider config from admin
type SSOProviderRequest struct {
	ClientID        string `json:"client_id" validate:"required"`
	ClientSecret    string `json:"client_secret"`
	IsEnabled       bool   `json:"is_enabled"`
	AllowAutoCreate bool   `json:"allow_auto_create"`
	DefaultRole     string `json:"default_role"`
	AllowedDomains  string `json:"allowed_domains"`
	// Custom provider fields
	AuthURL     string `json:"auth_url"`
	TokenURL    string `json:"token_url"`
	UserInfoURL string `json:"user_info_url"`
}

// SSOProviderResponse represents SSO provider config response (masked secret)
type SSOProviderResponse struct {
	Provider        string `json:"provider"`
	ClientID        string `json:"client_id"`
	HasSecret       bool   `json:"has_secret"`
	IsEnabled       bool   `json:"is_enabled"`
	AllowAutoCreate bool   `json:"allow_auto_create"`
	DefaultRole     string `json:"default_role"`
	AllowedDomains  string `json:"allowed_domains"`
	AuthURL         string `json:"auth_url,omitempty"`
	TokenURL        string `json:"token_url,omitempty"`
	UserInfoURL     string `json:"user_info_url,omitempty"`
}

// providerDisplayNames maps provider keys to display names
var providerDisplayNames = map[string]string{
	"google":    "Google",
	"microsoft": "Microsoft",
	"github":    "GitHub",
	"facebook":  "Facebook",
	"custom":    "Custom SSO",
}

// GetPublicSSOProviders returns enabled SSO providers for login page (public, no auth)
func (a *App) GetPublicSSOProviders(r *fastglue.Request) error {
	// Get all enabled SSO providers (deduplicated by provider type)
	var providers []models.SSOProvider
	if err := a.DB.Where("is_enabled = ?", true).Find(&providers).Error; err != nil {
		a.Log.Error("Failed to fetch SSO providers", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to fetch providers", nil, "")
	}

	// Deduplicate by provider type (in case multiple orgs have same provider)
	seen := make(map[string]bool)
	result := make([]SSOProviderPublic, 0)
	for _, p := range providers {
		if seen[p.Provider] {
			continue
		}
		seen[p.Provider] = true
		name := providerDisplayNames[p.Provider]
		if name == "" {
			name = p.Provider
		}
		result = append(result, SSOProviderPublic{
			Provider: p.Provider,
			Name:     name,
		})
	}

	return r.SendEnvelope(result)
}

// InitSSO initiates OAuth flow for a provider
func (a *App) InitSSO(r *fastglue.Request) error {
	provider := r.RequestCtx.UserValue("provider").(string)

	// Validate provider
	if provider != "custom" {
		if _, ok := oauthProviders[provider]; !ok {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid SSO provider", nil, "")
		}
	}

	// Get first enabled SSO provider config for this provider type
	var ssoConfig models.SSOProvider
	if err := a.DB.Where("provider = ? AND is_enabled = ?", provider, true).First(&ssoConfig).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "SSO provider not configured or disabled", nil, "")
	}

	// Generate state token
	nonce := generateRandomString(32)
	state := SSOState{
		OrgID:     ssoConfig.OrganizationID.String(),
		Provider:  provider,
		Nonce:     nonce,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	stateJSON, _ := json.Marshal(state)
	stateKey := "sso:state:" + nonce

	// Store state in Redis (5 min TTL)
	if err := a.Redis.Set(r.RequestCtx, stateKey, stateJSON, 5*time.Minute).Err(); err != nil {
		a.Log.Error("Failed to store SSO state", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to initiate SSO", nil, "")
	}

	// Build OAuth config
	oauthConfig := a.buildOAuthConfig(provider, &ssoConfig, r)

	// Redirect to provider
	authURL := oauthConfig.AuthCodeURL(nonce, oauth2.AccessTypeOffline)
	r.RequestCtx.Redirect(authURL, fasthttp.StatusTemporaryRedirect)
	return nil
}

// CallbackSSO handles OAuth callback
func (a *App) CallbackSSO(r *fastglue.Request) error {
	provider := r.RequestCtx.UserValue("provider").(string)
	code := string(r.RequestCtx.QueryArgs().Peek("code"))
	stateNonce := string(r.RequestCtx.QueryArgs().Peek("state"))
	errorParam := string(r.RequestCtx.QueryArgs().Peek("error"))

	// Check for OAuth error
	if errorParam != "" {
		errorDesc := string(r.RequestCtx.QueryArgs().Peek("error_description"))
		a.redirectWithError(r, "SSO failed: "+errorDesc)
		return nil
	}

	if code == "" || stateNonce == "" {
		a.redirectWithError(r, "Invalid callback parameters")
		return nil
	}

	// Retrieve and validate state from Redis
	stateKey := "sso:state:" + stateNonce
	stateJSON, err := a.Redis.Get(r.RequestCtx, stateKey).Bytes()
	if err != nil {
		a.redirectWithError(r, "Invalid or expired state")
		return nil
	}

	// Delete state immediately to prevent replay
	a.Redis.Del(r.RequestCtx, stateKey)

	var state SSOState
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		a.redirectWithError(r, "Invalid state")
		return nil
	}

	// Validate state
	if state.Provider != provider || time.Now().After(state.ExpiresAt) {
		a.redirectWithError(r, "Invalid or expired state")
		return nil
	}

	// Parse org ID from state
	orgID, err := uuid.Parse(state.OrgID)
	if err != nil {
		a.redirectWithError(r, "Invalid organization")
		return nil
	}

	// Get SSO provider config
	var ssoConfig models.SSOProvider
	if err := a.DB.Where("organization_id = ? AND provider = ?", orgID, provider).First(&ssoConfig).Error; err != nil {
		a.redirectWithError(r, "SSO provider not configured")
		return nil
	}

	// Build OAuth config and exchange code for token
	oauthConfig := a.buildOAuthConfig(provider, &ssoConfig, r)
	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		a.Log.Error("Failed to exchange OAuth code", "error", err, "provider", provider)
		a.redirectWithError(r, "Failed to authenticate with provider")
		return nil
	}

	// Fetch user info from provider
	userInfo, err := a.fetchUserInfo(provider, &ssoConfig, token)
	if err != nil {
		a.Log.Error("Failed to fetch user info", "error", err, "provider", provider)
		a.redirectWithError(r, "Failed to get user information")
		return nil
	}

	// Validate email domain if configured
	if ssoConfig.AllowedDomains != "" {
		domains := strings.Split(ssoConfig.AllowedDomains, ",")
		emailParts := strings.Split(userInfo.Email, "@")
		if len(emailParts) != 2 {
			a.redirectWithError(r, "Invalid email from provider")
			return nil
		}
		emailDomain := strings.TrimSpace(emailParts[1])
		allowed := false
		for _, d := range domains {
			if strings.TrimSpace(d) == emailDomain {
				allowed = true
				break
			}
		}
		if !allowed {
			a.redirectWithError(r, "Email domain not allowed for this organization")
			return nil
		}
	}

	// Find user by email (across all orgs, like regular login)
	var user models.User
	if err := a.DB.Where("email = ?", userInfo.Email).First(&user).Error; err != nil {
		// User doesn't exist - check if auto-create is enabled
		if !ssoConfig.AllowAutoCreate {
			a.redirectWithError(r, "User not found. Contact your administrator.")
			return nil
		}

		// Auto-create user in the SSO config's organization
		roleName := ssoConfig.DefaultRoleName
		if roleName == "" {
			roleName = "agent"
		}

		// Look up the CustomRole by name for this organization
		var customRole models.CustomRole
		if err := a.DB.Where("organization_id = ? AND name = ?", orgID, roleName).First(&customRole).Error; err != nil {
			a.Log.Error("Failed to find role for SSO user", "error", err, "role_name", roleName)
			a.redirectWithError(r, "Failed to create user account: role not found")
			return nil
		}

		user = models.User{
			OrganizationID: orgID,
			Email:          userInfo.Email,
			FullName:       userInfo.Name,
			RoleID:         &customRole.ID,
			IsActive:       true,
			IsAvailable:    true,
			SSOProvider:    provider,
			SSOProviderID:  userInfo.ID,
		}

		if err := a.DB.Create(&user).Error; err != nil {
			a.Log.Error("Failed to create SSO user", "error", err, "email", userInfo.Email)
			a.redirectWithError(r, "Failed to create user account")
			return nil
		}

		// Create UserOrganization entry
		userOrg := models.UserOrganization{
			UserID:         user.ID,
			OrganizationID: orgID,
			RoleID:         &customRole.ID,
			IsDefault:      true,
		}
		if err := a.DB.Create(&userOrg).Error; err != nil {
			a.Log.Error("Failed to create user organization entry for SSO user", "error", err)
			// Non-fatal: user was already created
		}

		a.Log.Info("Created SSO user", "user_id", user.ID, "email", user.Email, "provider", provider)
	} else {
		// User exists - update SSO info if not set
		if user.SSOProvider == "" {
			user.SSOProvider = provider
			user.SSOProviderID = userInfo.ID
			a.DB.Save(&user)
		}

		// Check if user is active
		if !user.IsActive {
			a.redirectWithError(r, "Account is disabled")
			return nil
		}
	}

	// Generate JWT tokens
	accessToken, err := a.generateAccessToken(&user)
	if err != nil {
		a.Log.Error("Failed to generate access token", "error", err)
		a.redirectWithError(r, "Failed to complete authentication")
		return nil
	}

	refreshToken, err := a.generateRefreshToken(&user)
	if err != nil {
		a.Log.Error("Failed to generate refresh token", "error", err)
		a.redirectWithError(r, "Failed to complete authentication")
		return nil
	}

	// Redirect to frontend with tokens in URL fragment
	basePath := sanitizeRedirectPath(a.Config.Server.BasePath)
	redirectURL := fmt.Sprintf("%s/auth/sso/callback#access_token=%s&refresh_token=%s&expires_in=%d",
		basePath, accessToken, refreshToken, a.Config.JWT.AccessExpiryMins*60)

	r.RequestCtx.Redirect(redirectURL, fasthttp.StatusTemporaryRedirect)
	return nil
}

// GetSSOSettings returns all SSO provider configs for the organization (admin only)
func (a *App) GetSSOSettings(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var providers []models.SSOProvider
	if err := a.DB.Where("organization_id = ?", orgID).Find(&providers).Error; err != nil {
		a.Log.Error("Failed to fetch SSO providers", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to fetch SSO settings", nil, "")
	}

	// Map to response (hide secrets)
	result := make([]SSOProviderResponse, 0, len(providers))
	for _, p := range providers {
		result = append(result, SSOProviderResponse{
			Provider:        p.Provider,
			ClientID:        p.ClientID,
			HasSecret:       p.ClientSecret != "",
			IsEnabled:       p.IsEnabled,
			AllowAutoCreate: p.AllowAutoCreate,
			DefaultRole:     p.DefaultRoleName,
			AllowedDomains:  p.AllowedDomains,
			AuthURL:         p.AuthURL,
			TokenURL:        p.TokenURL,
			UserInfoURL:     p.UserInfoURL,
		})
	}

	return r.SendEnvelope(result)
}

// UpdateSSOProvider creates or updates an SSO provider config (admin only)
func (a *App) UpdateSSOProvider(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	provider := r.RequestCtx.UserValue("provider").(string)

	// Validate provider
	validProviders := []string{"google", "microsoft", "github", "facebook", "custom"}
	isValid := false
	for _, p := range validProviders {
		if p == provider {
			isValid = true
			break
		}
	}
	if !isValid {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid provider", nil, "")
	}

	var req SSOProviderRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Validate custom provider fields
	if provider == "custom" {
		if req.AuthURL == "" || req.TokenURL == "" || req.UserInfoURL == "" {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Custom provider requires auth_url, token_url, and user_info_url", nil, "")
		}
	}

	// Find or create SSO provider config
	var ssoConfig models.SSOProvider
	err = a.DB.Where("organization_id = ? AND provider = ?", orgID, provider).First(&ssoConfig).Error

	if err != nil {
		// Create new
		ssoConfig = models.SSOProvider{
			OrganizationID: orgID,
			Provider:       provider,
		}
	}

	// Update fields
	ssoConfig.ClientID = req.ClientID
	if req.ClientSecret != "" {
		ssoConfig.ClientSecret = req.ClientSecret
	}
	ssoConfig.IsEnabled = req.IsEnabled
	ssoConfig.AllowAutoCreate = req.AllowAutoCreate
	ssoConfig.DefaultRoleName = req.DefaultRole
	if ssoConfig.DefaultRoleName == "" {
		ssoConfig.DefaultRoleName = "agent"
	}
	ssoConfig.AllowedDomains = req.AllowedDomains
	ssoConfig.AuthURL = req.AuthURL
	ssoConfig.TokenURL = req.TokenURL
	ssoConfig.UserInfoURL = req.UserInfoURL

	if err := a.DB.Save(&ssoConfig).Error; err != nil {
		a.Log.Error("Failed to save SSO provider", "error", err, "provider", provider)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save SSO settings", nil, "")
	}

	return r.SendEnvelope(SSOProviderResponse{
		Provider:        ssoConfig.Provider,
		ClientID:        ssoConfig.ClientID,
		HasSecret:       ssoConfig.ClientSecret != "",
		IsEnabled:       ssoConfig.IsEnabled,
		AllowAutoCreate: ssoConfig.AllowAutoCreate,
		DefaultRole:     ssoConfig.DefaultRoleName,
		AllowedDomains:  ssoConfig.AllowedDomains,
		AuthURL:         ssoConfig.AuthURL,
		TokenURL:        ssoConfig.TokenURL,
		UserInfoURL:     ssoConfig.UserInfoURL,
	})
}

// DeleteSSOProvider removes an SSO provider config (admin only)
func (a *App) DeleteSSOProvider(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	provider := r.RequestCtx.UserValue("provider").(string)

	result := a.DB.Where("organization_id = ? AND provider = ?", orgID, provider).Delete(&models.SSOProvider{})
	if result.Error != nil {
		a.Log.Error("Failed to delete SSO provider", "error", result.Error, "provider", provider)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete SSO provider", nil, "")
	}

	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "SSO provider not found", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "SSO provider deleted"})
}

// Helper functions

func (a *App) buildOAuthConfig(provider string, ssoConfig *models.SSOProvider, r *fastglue.Request) *oauth2.Config {
	var endpoint oauth2.Endpoint
	var scopes []string

	if provider == "custom" {
		endpoint = oauth2.Endpoint{
			AuthURL:  ssoConfig.AuthURL,
			TokenURL: ssoConfig.TokenURL,
		}
		scopes = []string{"openid", "email", "profile"}
	} else {
		providerCfg := oauthProviders[provider]
		endpoint = providerCfg.Endpoint
		scopes = providerCfg.Scopes
	}

	// Build callback URL from request
	scheme := "https"
	if !r.RequestCtx.IsTLS() && a.Config.App.Environment == "development" {
		scheme = "http"
	}
	host := string(r.RequestCtx.Host())
	basePath := sanitizeRedirectPath(a.Config.Server.BasePath)
	callbackURL := fmt.Sprintf("%s://%s%s/api/auth/sso/%s/callback", scheme, host, basePath, provider)

	return &oauth2.Config{
		ClientID:     ssoConfig.ClientID,
		ClientSecret: ssoConfig.ClientSecret,
		Endpoint:     endpoint,
		Scopes:       scopes,
		RedirectURL:  callbackURL,
	}
}

// UserInfo represents normalized user info from OAuth providers
type UserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (a *App) fetchUserInfo(provider string, ssoConfig *models.SSOProvider, token *oauth2.Token) (*UserInfo, error) {
	var userInfoURL string

	if provider == "custom" {
		userInfoURL = ssoConfig.UserInfoURL
	} else {
		userInfoURL = oauthProviders[provider].UserInfoURL
	}

	req, err := http.NewRequest(http.MethodGet, userInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	if provider == "github" {
		req.Header.Set("Accept", "application/vnd.github+json")
	}

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user info request failed: %s", string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse based on provider
	var userInfo UserInfo
	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, err
	}

	switch provider {
	case "google":
		userInfo.ID = getString(rawData, "id")
		userInfo.Email = getString(rawData, "email")
		userInfo.Name = getString(rawData, "name")
	case "microsoft":
		userInfo.ID = getString(rawData, "id")
		userInfo.Email = getString(rawData, "mail")
		if userInfo.Email == "" {
			userInfo.Email = getString(rawData, "userPrincipalName")
		}
		userInfo.Name = getString(rawData, "displayName")
	case "github":
		userInfo.ID = fmt.Sprintf("%v", rawData["id"])
		userInfo.Email = getString(rawData, "email")
		userInfo.Name = getString(rawData, "name")
		if userInfo.Name == "" {
			userInfo.Name = getString(rawData, "login")
		}
		// GitHub might not return email in user info, need separate API call
		if userInfo.Email == "" {
			email, err := a.fetchGitHubEmail(token)
			if err == nil {
				userInfo.Email = email
			}
		}
	case "facebook":
		userInfo.ID = getString(rawData, "id")
		userInfo.Email = getString(rawData, "email")
		userInfo.Name = getString(rawData, "name")
	default: // custom
		userInfo.ID = getString(rawData, "sub")
		if userInfo.ID == "" {
			userInfo.ID = getString(rawData, "id")
		}
		userInfo.Email = getString(rawData, "email")
		userInfo.Name = getString(rawData, "name")
		if userInfo.Name == "" {
			userInfo.Name = getString(rawData, "preferred_username")
		}
	}

	if userInfo.Email == "" {
		return nil, fmt.Errorf("email not provided by SSO provider")
	}

	return &userInfo, nil
}

func (a *App) fetchGitHubEmail(token *oauth2.Token) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	// Find primary verified email
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	// Fallback to first verified email
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	return "", fmt.Errorf("no verified email found")
}

func (a *App) redirectWithError(r *fastglue.Request, message string) {
	basePath := sanitizeRedirectPath(a.Config.Server.BasePath)
	encodedMsg := url.QueryEscape(message)
	redirectURL := fmt.Sprintf("%s/login?sso_error=%s", basePath, encodedMsg)
	r.RequestCtx.Redirect(redirectURL, fasthttp.StatusTemporaryRedirect)
}

// sanitizeRedirectPath ensures the path is safe for redirects by preventing
// open redirect vulnerabilities (e.g., //evil.com or /\evil.com)
func sanitizeRedirectPath(path string) string {
	if path == "" {
		return ""
	}
	// Ensure path starts with /
	if path[0] != '/' {
		path = "/" + path
	}
	// Prevent protocol-relative URLs (//...) and backslash escapes (/\...)
	// by stripping dangerous characters after the leading slash
	for len(path) > 1 && (path[1] == '/' || path[1] == '\\') {
		path = "/" + path[2:]
	}
	return path
}

func generateRandomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:n]
}

func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
