package testutil

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/middleware"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/stretchr/testify/require"
	"github.com/zerodha/fastglue"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// TestJWTSecret is the shared JWT secret for tests.
const TestJWTSecret = "test-secret-key-must-be-at-least-32-chars"

// --- Organization ---

// CreateTestOrganization creates a test organization in the database.
func CreateTestOrganization(t *testing.T, db *gorm.DB) *models.Organization {
	t.Helper()

	org := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Test Organization " + uuid.New().String()[:8],
		Slug:      "test-org-" + uuid.New().String()[:8],
	}
	require.NoError(t, db.Create(org).Error)
	return org
}

// --- User ---

// UserOption configures a test user.
type UserOption func(*models.User)

// WithEmail sets the email for the test user.
func WithEmail(email string) UserOption {
	return func(u *models.User) {
		u.Email = email
	}
}

// WithPassword sets the password (bcrypt hashed) for the test user.
func WithPassword(password string) UserOption {
	return func(u *models.User) {
		hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		u.PasswordHash = string(hash)
	}
}

// WithRoleID sets the role ID for the test user.
func WithRoleID(roleID *uuid.UUID) UserOption {
	return func(u *models.User) {
		u.RoleID = roleID
	}
}

// WithInactive marks the test user as inactive.
func WithInactive() UserOption {
	return func(u *models.User) {
		u.IsActive = false
	}
}

// WithSuperAdmin marks the test user as a super admin.
func WithSuperAdmin() UserOption {
	return func(u *models.User) {
		u.IsSuperAdmin = true
	}
}

// WithFullName sets the test user's full name.
func WithFullName(name string) UserOption {
	return func(u *models.User) {
		u.FullName = name
	}
}

// CreateTestUser creates a test user in the database.
// By default, the user is active with a hashed "password123" password and a unique email.
func CreateTestUser(t *testing.T, db *gorm.DB, orgID uuid.UUID, opts ...UserOption) *models.User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	require.NoError(t, err)

	user := &models.User{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		Email:          UniqueEmail("test"),
		PasswordHash:   string(hash),
		FullName:       "Test User",
		IsActive:       true,
		IsAvailable:    true,
	}

	for _, opt := range opts {
		opt(user)
	}

	// Track whether the user should be inactive (GORM ignores false due to default:true tag)
	shouldBeInactive := !user.IsActive

	user.IsActive = true
	require.NoError(t, db.Create(user).Error)

	if shouldBeInactive {
		require.NoError(t, db.Model(user).Update("is_active", false).Error)
		user.IsActive = false
	}

	// Also create user_organizations entry for the user's home org
	userOrg := &models.UserOrganization{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		UserID:         user.ID,
		OrganizationID: orgID,
		RoleID:         user.RoleID,
		IsDefault:      true,
	}
	require.NoError(t, db.Create(userOrg).Error)

	return user
}

// --- Auth Context ---

// SetAuthContext sets organization_id and user_id in the request context.
func SetAuthContext(req *fastglue.Request, orgID, userID uuid.UUID) {
	req.RequestCtx.SetUserValue("organization_id", orgID)
	req.RequestCtx.SetUserValue("user_id", userID)
}

// SetFullAuthContext sets org, user, role, and super admin status in the request context.
func SetFullAuthContext(req *fastglue.Request, orgID, userID uuid.UUID, roleID *uuid.UUID, isSuperAdmin bool) {
	req.RequestCtx.SetUserValue("organization_id", orgID)
	req.RequestCtx.SetUserValue("user_id", userID)
	if roleID != nil {
		req.RequestCtx.SetUserValue("role_id", *roleID)
	}
	req.RequestCtx.SetUserValue("is_super_admin", isSuperAdmin)
}

// --- WhatsApp Account ---

// CreateTestWhatsAppAccount creates a test WhatsApp account in the database.
func CreateTestWhatsAppAccount(t *testing.T, db *gorm.DB, orgID uuid.UUID) *models.WhatsAppAccount {
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
	require.NoError(t, db.Create(account).Error)
	return account
}

// WhatsAppAccountOption configures a test WhatsApp account.
type WhatsAppAccountOption func(*models.WhatsAppAccount)

// WithAccountName sets the account name.
func WithAccountName(name string) WhatsAppAccountOption {
	return func(a *models.WhatsAppAccount) {
		a.Name = name
	}
}

// CreateTestWhatsAppAccountWith creates a test WhatsApp account with options.
func CreateTestWhatsAppAccountWith(t *testing.T, db *gorm.DB, orgID uuid.UUID, opts ...WhatsAppAccountOption) *models.WhatsAppAccount {
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

	for _, opt := range opts {
		opt(account)
	}

	require.NoError(t, db.Create(account).Error)
	return account
}

// --- Contact ---

// CreateTestContact creates a test contact in the database.
func CreateTestContact(t *testing.T, db *gorm.DB, orgID uuid.UUID) *models.Contact {
	t.Helper()

	uniqueID := uuid.New().String()[:8]
	contact := &models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		PhoneNumber:    "+1234567890" + uniqueID[:4],
		ProfileName:    "Test Contact " + uniqueID,
	}
	require.NoError(t, db.Create(contact).Error)
	return contact
}

// ContactOption configures a test contact.
type ContactOption func(*models.Contact)

// WithContactAccount sets the WhatsApp account name on the contact.
func WithContactAccount(accountName string) ContactOption {
	return func(c *models.Contact) {
		c.WhatsAppAccount = accountName
	}
}

// WithPhoneNumber sets the phone number on the contact.
func WithPhoneNumber(phone string) ContactOption {
	return func(c *models.Contact) {
		c.PhoneNumber = phone
	}
}

// CreateTestContactWith creates a test contact with options.
func CreateTestContactWith(t *testing.T, db *gorm.DB, orgID uuid.UUID, opts ...ContactOption) *models.Contact {
	t.Helper()

	uniqueID := uuid.New().String()[:8]
	contact := &models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		PhoneNumber:    "+1234567890" + uniqueID[:4],
		ProfileName:    "Test Contact " + uniqueID,
	}

	for _, opt := range opts {
		opt(contact)
	}

	require.NoError(t, db.Create(contact).Error)
	return contact
}

// --- Template ---

// CreateTestTemplate creates a test template in the database.
func CreateTestTemplate(t *testing.T, db *gorm.DB, orgID uuid.UUID, accountName string) *models.Template {
	t.Helper()

	template := &models.Template{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: accountName,
		Name:            "test-template-" + uuid.New().String()[:8],
		MetaTemplateID:  "meta-" + uuid.New().String()[:8],
		Category:        "MARKETING",
		Language:        "en",
		Status:          string(models.TemplateStatusApproved),
		BodyContent:     "Hello {{1}}",
	}
	require.NoError(t, db.Create(template).Error)
	return template
}

// --- Permissions & Roles ---

// GetOrCreateTestPermissions gets existing permissions or creates the default set.
func GetOrCreateTestPermissions(t *testing.T, db *gorm.DB) []models.Permission {
	t.Helper()

	var existingPerms []models.Permission
	if err := db.Order("resource, action").Find(&existingPerms).Error; err == nil && len(existingPerms) > 0 {
		return existingPerms
	}

	perms := models.DefaultPermissions()
	for i := range perms {
		perms[i].ID = uuid.New()
	}
	require.NoError(t, db.Create(&perms).Error)
	return perms
}

// CreateTestRole creates a role with specified permissions for testing.
func CreateTestRole(t *testing.T, db *gorm.DB, orgID uuid.UUID, name string, permissions []models.Permission) *models.CustomRole {
	t.Helper()

	role := &models.CustomRole{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		Name:           name + "_" + uuid.New().String()[:8],
		Description:    "Test role for " + name,
		IsSystem:       false,
		IsDefault:      false,
		Permissions:    permissions,
	}
	require.NoError(t, db.Create(role).Error)
	return role
}

// CreateTestRoleExact creates a role with an exact name (no UUID suffix).
// Use this when the test needs to verify the role name.
func CreateTestRoleExact(t *testing.T, db *gorm.DB, orgID uuid.UUID, name string, isSystem, isDefault bool, permissions []models.Permission) *models.CustomRole {
	t.Helper()

	role := &models.CustomRole{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		Name:           name,
		Description:    "Test role: " + name,
		IsSystem:       isSystem,
		IsDefault:      isDefault,
		Permissions:    permissions,
	}
	require.NoError(t, db.Create(role).Error)
	return role
}

// CreateTestRoleWithKeys creates a role with permissions specified by resource:action keys.
func CreateTestRoleWithKeys(t *testing.T, db *gorm.DB, orgID uuid.UUID, name string, permissionKeys []string) *models.CustomRole {
	t.Helper()

	perms := GetOrCreateTestPermissions(t, db)

	permMap := make(map[string]models.Permission)
	for _, p := range perms {
		key := p.Resource + ":" + p.Action
		permMap[key] = p
	}

	var rolePerms []models.Permission
	for _, key := range permissionKeys {
		if p, ok := permMap[key]; ok {
			rolePerms = append(rolePerms, p)
		}
	}

	return CreateTestRole(t, db, orgID, name, rolePerms)
}

// CreateAdminRole creates an admin role with all permissions.
func CreateAdminRole(t *testing.T, db *gorm.DB, orgID uuid.UUID) *models.CustomRole {
	t.Helper()

	allPerms := GetOrCreateTestPermissions(t, db)
	return CreateTestRole(t, db, orgID, "admin", allPerms)
}

// CreateAgentRole creates an agent role with limited permissions.
func CreateAgentRole(t *testing.T, db *gorm.DB, orgID uuid.UUID) *models.CustomRole {
	t.Helper()

	agentPerms := []string{
		"chat:read", "chat:write",
		"contacts:read",
		"analytics.agents:read",
		"transfers:read", "transfers:pickup",
		"canned_responses:read",
	}
	return CreateTestRoleWithKeys(t, db, orgID, "agent", agentPerms)
}

// --- Email ---

// UniqueEmail generates a unique email for test isolation.
func UniqueEmail(prefix string) string {
	return prefix + "-" + uuid.New().String()[:8] + "@example.com"
}

// --- JWT ---

// GenerateTestRefreshToken creates a valid refresh token for testing.
func GenerateTestRefreshToken(t *testing.T, user *models.User, secret string, expiry time.Duration) string {
	t.Helper()

	claims := middleware.JWTClaims{
		UserID:         user.ID,
		OrganizationID: user.OrganizationID,
		Email:          user.Email,
		RoleID:         user.RoleID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "whatomate",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return tokenString
}
