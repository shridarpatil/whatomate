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
	"github.com/zerodha/fastglue"
	"golang.org/x/crypto/bcrypt"
)

// --- Superadmin access enforcement ---

func TestApp_AdminEndpoints_RequireSuperAdmin(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	orgAdmin := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("org-admin")),
		testutil.WithRoleID(&adminRole.ID),
	)

	endpoints := []struct {
		name    string
		handler func(*fastglue.Request) error
	}{
		{"AdminListOrganizations", app.AdminListOrganizations},
		{"AdminCreateOrganization", app.AdminCreateOrganization},
		{"AdminUpdateOrganization", app.AdminUpdateOrganization},
		{"AdminListOrgRoles", app.AdminListOrgRoles},
		{"AdminListUsers", app.AdminListUsers},
		{"AdminCreateUser", app.AdminCreateUser},
		{"AdminSetUserStatus", app.AdminSetUserStatus},
		{"AdminResetUserPassword", app.AdminResetUserPassword},
	}

	for _, ep := range endpoints {
		t.Run(ep.name+" forbidden for org admin", func(t *testing.T) {
			req := testutil.NewJSONRequest(t, map[string]any{})
			testutil.SetAuthContext(req, org.ID, orgAdmin.ID)

			err := ep.handler(req)
			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
		})
	}
}

// --- Organizations ---

func TestApp_AdminListOrganizations(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	superAdmin := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("super")),
		testutil.WithSuperAdmin(),
	)
	// One regular user and one WhatsApp account in the org for the stats.
	testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("member")),
	)
	testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "search", org.Name)
	testutil.SetAuthContext(req, org.ID, superAdmin.ID)

	err := app.AdminListOrganizations(req)
	require.NoError(t, err)
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Organizations []handlers.AdminOrganizationResponse `json:"organizations"`
			Total         int64                                `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))

	require.Len(t, resp.Data.Organizations, 1)
	got := resp.Data.Organizations[0]
	assert.Equal(t, org.ID, got.ID)
	assert.Equal(t, int64(2), got.UserCount) // superadmin (home org) + member
	assert.Equal(t, int64(1), got.AccountCount)
}

func TestApp_AdminCreateOrganization(t *testing.T) {
	t.Parallel()

	t.Run("provisions org with default admin user and no superadmin membership", func(t *testing.T) {
		app := newTestApp(t)
		homeOrg := testutil.CreateTestOrganization(t, app.DB)
		superAdmin := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
			testutil.WithEmail(testutil.UniqueEmail("super")),
			testutil.WithSuperAdmin(),
		)

		orgName := "Admin Created Org " + uuid.New().String()[:8]
		adminEmail := testutil.UniqueEmail("org-owner")
		req := testutil.NewJSONRequest(t, map[string]any{
			"name":            orgName,
			"admin_full_name": "Org Owner",
			"admin_email":     adminEmail,
			"admin_password":  "secret123",
		})
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminCreateOrganization(req)
		require.NoError(t, err)
		require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.AdminOrganizationResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, orgName, resp.Data.Name)

		// System roles seeded
		var roleCount int64
		app.DB.Model(&models.CustomRole{}).
			Where("organization_id = ? AND is_system = ?", resp.Data.ID, true).
			Count(&roleCount)
		assert.GreaterOrEqual(t, roleCount, int64(3))

		// Chatbot settings seeded
		var cbCount int64
		app.DB.Model(&models.ChatbotSettings{}).
			Where("organization_id = ?", resp.Data.ID).
			Count(&cbCount)
		assert.Equal(t, int64(1), cbCount)

		// Default admin user created with the org's system admin role
		var adminUser models.User
		require.NoError(t, app.DB.Where("email = ?", adminEmail).First(&adminUser).Error)
		assert.Equal(t, resp.Data.ID, adminUser.OrganizationID)
		assert.False(t, adminUser.IsSuperAdmin)
		assert.True(t, adminUser.IsActive)
		var adminRole models.CustomRole
		require.NoError(t, app.DB.Where("organization_id = ? AND name = ? AND is_system = ?", resp.Data.ID, "admin", true).First(&adminRole).Error)
		require.NotNil(t, adminUser.RoleID)
		assert.Equal(t, adminRole.ID, *adminUser.RoleID)

		// Exactly one membership row: the new admin user, not the superadmin
		var memberships []models.UserOrganization
		require.NoError(t, app.DB.Where("organization_id = ?", resp.Data.ID).Find(&memberships).Error)
		require.Len(t, memberships, 1)
		assert.Equal(t, adminUser.ID, memberships[0].UserID)
		assert.True(t, memberships[0].IsDefault)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		app := newTestApp(t)
		homeOrg := testutil.CreateTestOrganization(t, app.DB)
		superAdmin := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
			testutil.WithEmail(testutil.UniqueEmail("super")),
			testutil.WithSuperAdmin(),
		)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":            "",
			"admin_full_name": "Org Owner",
			"admin_email":     testutil.UniqueEmail("org-owner"),
			"admin_password":  "secret123",
		})
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminCreateOrganization(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("rejects missing admin user fields", func(t *testing.T) {
		app := newTestApp(t)
		homeOrg := testutil.CreateTestOrganization(t, app.DB)
		superAdmin := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
			testutil.WithEmail(testutil.UniqueEmail("super")),
			testutil.WithSuperAdmin(),
		)

		req := testutil.NewJSONRequest(t, map[string]any{"name": "Org Without Admin"})
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminCreateOrganization(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("rejects duplicate admin email without creating org", func(t *testing.T) {
		app := newTestApp(t)
		homeOrg := testutil.CreateTestOrganization(t, app.DB)
		superAdmin := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
			testutil.WithEmail(testutil.UniqueEmail("super")),
			testutil.WithSuperAdmin(),
		)
		existing := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
			testutil.WithEmail(testutil.UniqueEmail("taken")),
		)

		orgName := "Dup Email Org " + uuid.New().String()[:8]
		req := testutil.NewJSONRequest(t, map[string]any{
			"name":            orgName,
			"admin_full_name": "Org Owner",
			"admin_email":     existing.Email,
			"admin_password":  "secret123",
		})
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminCreateOrganization(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))

		var orgCount int64
		app.DB.Model(&models.Organization{}).Where("name = ?", orgName).Count(&orgCount)
		assert.Equal(t, int64(0), orgCount)
	})
}

func TestApp_AdminUpdateOrganization(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	homeOrg := testutil.CreateTestOrganization(t, app.DB)
	superAdmin := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
		testutil.WithEmail(testutil.UniqueEmail("super")),
		testutil.WithSuperAdmin(),
	)
	target := testutil.CreateTestOrganization(t, app.DB)

	t.Run("renames organization", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, map[string]any{"name": "Renamed Org"})
		testutil.SetPathParam(req, "id", target.ID.String())
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminUpdateOrganization(req)
		require.NoError(t, err)
		require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var org models.Organization
		require.NoError(t, app.DB.First(&org, target.ID).Error)
		assert.Equal(t, "Renamed Org", org.Name)
	})

	t.Run("404 for unknown org", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, map[string]any{"name": "Whatever"})
		testutil.SetPathParam(req, "id", uuid.New().String())
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminUpdateOrganization(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

func TestApp_AdminListOrgRoles(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	homeOrg := testutil.CreateTestOrganization(t, app.DB)
	superAdmin := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
		testutil.WithEmail(testutil.UniqueEmail("super")),
		testutil.WithSuperAdmin(),
	)
	target := testutil.CreateTestOrganization(t, app.DB)
	testutil.CreateAdminRole(t, app.DB, target.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "id", target.ID.String())
	testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

	err := app.AdminListOrgRoles(req)
	require.NoError(t, err)
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Roles []handlers.AdminRoleResponse `json:"roles"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	require.Len(t, resp.Data.Roles, 1)
	assert.Contains(t, resp.Data.Roles[0].Name, "admin")
}

// --- Users ---

func TestApp_AdminListUsers(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)
	superAdmin := testutil.CreateTestUser(t, app.DB, orgA.ID,
		testutil.WithEmail(testutil.UniqueEmail("super")),
		testutil.WithSuperAdmin(),
	)
	userA := testutil.CreateTestUser(t, app.DB, orgA.ID,
		testutil.WithEmail(testutil.UniqueEmail("admin-list-a")),
		testutil.WithFullName("Admin List User A"),
	)
	userB := testutil.CreateTestUser(t, app.DB, orgB.ID,
		testutil.WithEmail(testutil.UniqueEmail("admin-list-b")),
		testutil.WithFullName("Admin List User B"),
		testutil.WithInactive(),
	)

	listUsers := func(t *testing.T, params map[string]any) []handlers.AdminUserResponse {
		req := testutil.NewGETRequest(t)
		for k, v := range params {
			testutil.SetQueryParam(req, k, v)
		}
		testutil.SetAuthContext(req, orgA.ID, superAdmin.ID)

		err := app.AdminListUsers(req)
		require.NoError(t, err)
		require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Users []handlers.AdminUserResponse `json:"users"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		return resp.Data.Users
	}

	t.Run("search spans organizations", func(t *testing.T) {
		users := listUsers(t, map[string]any{"search": "Admin List User"})
		require.Len(t, users, 2)
	})

	t.Run("filter by organization", func(t *testing.T) {
		users := listUsers(t, map[string]any{"search": "Admin List User", "organization_id": orgB.ID.String()})
		require.Len(t, users, 1)
		assert.Equal(t, userB.ID, users[0].ID)
		assert.Equal(t, orgB.Name, users[0].OrganizationName)
		assert.Equal(t, int64(1), users[0].OrgCount)
	})

	t.Run("filter by active status", func(t *testing.T) {
		users := listUsers(t, map[string]any{"search": "Admin List User", "is_active": "true"})
		require.Len(t, users, 1)
		assert.Equal(t, userA.ID, users[0].ID)

		users = listUsers(t, map[string]any{"search": "Admin List User", "is_active": "false"})
		require.Len(t, users, 1)
		assert.Equal(t, userB.ID, users[0].ID)
	})
}

func TestApp_AdminCreateUser(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	homeOrg := testutil.CreateTestOrganization(t, app.DB)
	superAdmin := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
		testutil.WithEmail(testutil.UniqueEmail("super")),
		testutil.WithSuperAdmin(),
	)
	target := testutil.CreateTestOrganization(t, app.DB)
	targetRole := testutil.CreateAdminRole(t, app.DB, target.ID)

	t.Run("creates user in target org", func(t *testing.T) {
		email := testutil.UniqueEmail("admin-created")
		req := testutil.NewJSONRequest(t, map[string]any{
			"organization_id": target.ID,
			"email":           email,
			"password":        "secret123",
			"full_name":       "Admin Created",
			"role_id":         targetRole.ID,
		})
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminCreateUser(req)
		require.NoError(t, err)
		require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.AdminUserResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, target.ID, resp.Data.OrganizationID)
		assert.Equal(t, target.Name, resp.Data.OrganizationName)
		assert.False(t, resp.Data.IsSuperAdmin)

		// Membership row created in the target org
		var membership models.UserOrganization
		require.NoError(t, app.DB.
			Where("user_id = ? AND organization_id = ?", resp.Data.ID, target.ID).
			First(&membership).Error)
		assert.True(t, membership.IsDefault)
	})

	t.Run("rejects role from another org", func(t *testing.T) {
		otherOrgRole := testutil.CreateAdminRole(t, app.DB, homeOrg.ID)
		req := testutil.NewJSONRequest(t, map[string]any{
			"organization_id": target.ID,
			"email":           testutil.UniqueEmail("role-mismatch"),
			"password":        "secret123",
			"full_name":       "Role Mismatch",
			"role_id":         otherOrgRole.ID,
		})
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminCreateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("rejects duplicate email", func(t *testing.T) {
		existing := testutil.CreateTestUser(t, app.DB, target.ID,
			testutil.WithEmail(testutil.UniqueEmail("dup")),
		)
		req := testutil.NewJSONRequest(t, map[string]any{
			"organization_id": target.ID,
			"email":           existing.Email,
			"password":        "secret123",
			"full_name":       "Duplicate",
		})
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminCreateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))
	})

	t.Run("rejects unknown organization", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, map[string]any{
			"organization_id": uuid.New(),
			"email":           testutil.UniqueEmail("no-org"),
			"password":        "secret123",
			"full_name":       "No Org",
		})
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminCreateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

func TestApp_AdminSetUserStatus(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	homeOrg := testutil.CreateTestOrganization(t, app.DB)
	superAdmin := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
		testutil.WithEmail(testutil.UniqueEmail("super")),
		testutil.WithSuperAdmin(),
	)
	org := testutil.CreateTestOrganization(t, app.DB)

	setStatus := func(t *testing.T, targetID uuid.UUID, isActive bool) *fastglue.Request {
		req := testutil.NewJSONRequest(t, map[string]any{"is_active": isActive})
		testutil.SetPathParam(req, "id", targetID.String())
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)
		require.NoError(t, app.AdminSetUserStatus(req))
		return req
	}

	t.Run("deactivates and reactivates a user", func(t *testing.T) {
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("status")),
		)

		req := setStatus(t, user.ID, false)
		require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var dbUser models.User
		require.NoError(t, app.DB.First(&dbUser, user.ID).Error)
		assert.False(t, dbUser.IsActive)

		req = setStatus(t, user.ID, true)
		require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
		require.NoError(t, app.DB.First(&dbUser, user.ID).Error)
		assert.True(t, dbUser.IsActive)
	})

	t.Run("cannot deactivate self", func(t *testing.T) {
		req := setStatus(t, superAdmin.ID, false)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("cannot deactivate another superadmin", func(t *testing.T) {
		otherSuper := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("super2")),
			testutil.WithSuperAdmin(),
		)
		req := setStatus(t, otherSuper.ID, false)
		assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
	})

	t.Run("404 for unknown user", func(t *testing.T) {
		req := setStatus(t, uuid.New(), false)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

func TestApp_AdminResetUserPassword(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	homeOrg := testutil.CreateTestOrganization(t, app.DB)
	superAdmin := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
		testutil.WithEmail(testutil.UniqueEmail("super")),
		testutil.WithSuperAdmin(),
	)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("reset")),
	)

	t.Run("sets a new working password", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, map[string]any{"new_password": "brand-new-pass"})
		testutil.SetPathParam(req, "id", user.ID.String())
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminResetUserPassword(req)
		require.NoError(t, err)
		require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var dbUser models.User
		require.NoError(t, app.DB.First(&dbUser, user.ID).Error)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte("brand-new-pass")))
		assert.Error(t, bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte("password123")))
	})

	t.Run("rejects short password", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, map[string]any{"new_password": "short"})
		testutil.SetPathParam(req, "id", user.ID.String())
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminResetUserPassword(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("404 for unknown user", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, map[string]any{"new_password": "brand-new-pass"})
		testutil.SetPathParam(req, "id", uuid.New().String())
		testutil.SetAuthContext(req, homeOrg.ID, superAdmin.ID)

		err := app.AdminResetUserPassword(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// --- Privacy regression: superadmins hidden from org-scoped lists ---

func TestApp_SuperAdminHiddenFromOrgLists(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	orgAdmin := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("org-admin")),
		testutil.WithRoleID(&adminRole.ID),
	)
	// Superadmin with a membership in the org (the pre-fix leak scenario).
	superAdmin := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("super")),
		testutil.WithSuperAdmin(),
	)

	t.Run("ListUsers hides superadmin from org admin", func(t *testing.T) {
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, orgAdmin.ID)

		require.NoError(t, app.ListUsers(req))
		require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Users []handlers.UserResponse `json:"users"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		require.Len(t, resp.Data.Users, 1)
		assert.Equal(t, orgAdmin.ID, resp.Data.Users[0].ID)
	})

	t.Run("ListUsers shows superadmin to superadmin", func(t *testing.T) {
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, superAdmin.ID)

		require.NoError(t, app.ListUsers(req))
		require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Users []handlers.UserResponse `json:"users"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Len(t, resp.Data.Users, 2)
	})

	t.Run("GetUser returns 404 for superadmin target", func(t *testing.T) {
		req := testutil.NewGETRequest(t)
		testutil.SetPathParam(req, "id", superAdmin.ID.String())
		testutil.SetAuthContext(req, org.ID, orgAdmin.ID)

		require.NoError(t, app.GetUser(req))
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("GetUser works for superadmin requester", func(t *testing.T) {
		req := testutil.NewGETRequest(t)
		testutil.SetPathParam(req, "id", superAdmin.ID.String())
		testutil.SetAuthContext(req, org.ID, superAdmin.ID)

		require.NoError(t, app.GetUser(req))
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	})

	t.Run("ListOrganizationMembers hides superadmin from org admin", func(t *testing.T) {
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, orgAdmin.ID)

		require.NoError(t, app.ListOrganizationMembers(req))
		require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Members []handlers.MemberResponse `json:"members"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		require.Len(t, resp.Data.Members, 1)
		assert.Equal(t, orgAdmin.ID, resp.Data.Members[0].UserID)
	})
}
