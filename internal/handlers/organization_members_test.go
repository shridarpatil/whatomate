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

// --- CreateOrganization Tests ---

func TestApp_CreateOrganization_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-org")), testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]string{
		"name": "New Test Organization",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateOrganization(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.OrganizationResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "New Test Organization", resp.Data.Name)
	assert.NotEmpty(t, resp.Data.Slug)
	assert.NotEqual(t, uuid.Nil, resp.Data.ID)
	assert.NotEmpty(t, resp.Data.CreatedAt)
}

func TestApp_CreateOrganization_EmptyName(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-org-empty")), testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]string{
		"name": "",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateOrganization(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateOrganization_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]string{
		"name": "Unauthorized Org",
	})

	err := app.CreateOrganization(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- ListOrganizationMembers Tests ---

func TestApp_ListOrganizationMembers_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("list-members")), testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListOrganizationMembers(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Members []handlers.MemberResponse `json:"members"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(resp.Data.Members), 1)

	// Find our user in members
	found := false
	for _, m := range resp.Data.Members {
		if m.UserID == user.ID {
			found = true
			assert.Equal(t, user.Email, m.Email)
			assert.Equal(t, user.FullName, m.FullName)
			assert.NotEmpty(t, m.CreatedAt)
			break
		}
	}
	assert.True(t, found, "expected to find user in members list")
}

func TestApp_ListOrganizationMembers_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)

	err := app.ListOrganizationMembers(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- AddOrganizationMember Tests ---

func TestApp_AddOrganizationMember_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	// Real orgs are seeded with a default role ("agent" via SeedSystemRolesForOrg);
	// AddOrganizationMember falls back to it when the request omits role_id.
	defaultRole := testutil.CreateTestRoleExact(t, app.DB, org.ID, "agent", true, true, nil)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("add-member-admin")), testutil.WithRoleID(&role.ID))

	// Create a second user in a different org to add
	org2 := testutil.CreateTestOrganization(t, app.DB)
	targetUser := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithEmail(testutil.UniqueEmail("add-member-target")))

	req := testutil.NewJSONRequest(t, map[string]any{
		"user_id": targetUser.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, admin.ID)

	err := app.AddOrganizationMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify membership was created, carrying the org's default role
	var userOrg models.UserOrganization
	require.NoError(t, app.DB.Where("user_id = ? AND organization_id = ?", targetUser.ID, org.ID).First(&userOrg).Error)
	require.NotNil(t, userOrg.RoleID, "membership must never be created with a NULL role")
	assert.Equal(t, defaultRole.ID, *userOrg.RoleID)
}

// A NULL-role membership resolves to no org-scoped role at permission-check time,
// so the member would land in the org with nothing. Reject it at add time.
func TestApp_AddOrganizationMember_NoDefaultRoleRejected(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("add-nodefault-admin")), testutil.WithRoleID(&role.ID))

	org2 := testutil.CreateTestOrganization(t, app.DB)
	targetUser := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithEmail(testutil.UniqueEmail("add-nodefault-target")))

	// No role_id in the request and no default role in the org.
	req := testutil.NewJSONRequest(t, map[string]any{
		"user_id": targetUser.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, admin.ID)

	require.NoError(t, app.AddOrganizationMember(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	var count int64
	app.DB.Model(&models.UserOrganization{}).Where("user_id = ? AND organization_id = ?", targetUser.ID, org.ID).Count(&count)
	assert.Equal(t, int64(0), count, "no membership row should be created")
}

// idx_user_org has no deleted_at predicate, so the soft-deleted row left by
// RemoveOrganizationMember would collide on re-add unless it is revived.
func TestApp_AddOrganizationMember_RevivesRemovedMembership(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	adminRole := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	agentRole := testutil.CreateTestRole(t, app.DB, org.ID, "agent", nil)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("revive-admin")), testutil.WithRoleID(&adminRole.ID))

	org2 := testutil.CreateTestOrganization(t, app.DB)
	targetUser := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithEmail(testutil.UniqueEmail("revive-target")))

	addReq := testutil.NewJSONRequest(t, map[string]any{
		"user_id": targetUser.ID.String(),
		"role_id": agentRole.ID.String(),
	})
	testutil.SetAuthContext(addReq, org.ID, admin.ID)
	require.NoError(t, app.AddOrganizationMember(addReq))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(addReq))

	removeReq := testutil.NewGETRequest(t)
	removeReq.RequestCtx.Request.Header.SetMethod("DELETE")
	testutil.SetAuthContext(removeReq, org.ID, admin.ID)
	testutil.SetPathParam(removeReq, "member_id", targetUser.ID.String())
	require.NoError(t, app.RemoveOrganizationMember(removeReq))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(removeReq))

	// Re-adding must succeed rather than hit the unique index on the soft-deleted row.
	readdReq := testutil.NewJSONRequest(t, map[string]any{
		"user_id": targetUser.ID.String(),
		"role_id": agentRole.ID.String(),
	})
	testutil.SetAuthContext(readdReq, org.ID, admin.ID)
	require.NoError(t, app.AddOrganizationMember(readdReq))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(readdReq))

	var count int64
	app.DB.Model(&models.UserOrganization{}).Where("user_id = ? AND organization_id = ?", targetUser.ID, org.ID).Count(&count)
	assert.Equal(t, int64(1), count, "re-add should revive the row, not duplicate it")
}

func TestApp_AddOrganizationMember_WithRole(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	adminRole := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	agentRole := testutil.CreateTestRole(t, app.DB, org.ID, "agent", nil)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("add-role-admin")), testutil.WithRoleID(&adminRole.ID))

	org2 := testutil.CreateTestOrganization(t, app.DB)
	targetUser := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithEmail(testutil.UniqueEmail("add-role-target")))

	req := testutil.NewJSONRequest(t, map[string]any{
		"user_id": targetUser.ID.String(),
		"role_id": agentRole.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, admin.ID)

	err := app.AddOrganizationMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify role was set
	var userOrg models.UserOrganization
	require.NoError(t, app.DB.Where("user_id = ? AND organization_id = ?", targetUser.ID, org.ID).First(&userOrg).Error)
	require.NotNil(t, userOrg.RoleID)
	assert.Equal(t, agentRole.ID, *userOrg.RoleID)
}

func TestApp_AddOrganizationMember_AlreadyMember(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("add-dup-admin")), testutil.WithRoleID(&role.ID))

	// Create user in same org (CreateTestUser auto-creates user_organizations entry)
	targetUser := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("add-dup-target")))

	req := testutil.NewJSONRequest(t, map[string]any{
		"user_id": targetUser.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, admin.ID)

	err := app.AddOrganizationMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))
}

func TestApp_AddOrganizationMember_MissingUserID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("add-no-id")), testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{})
	testutil.SetAuthContext(req, org.ID, admin.ID)

	err := app.AddOrganizationMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_AddOrganizationMember_UserNotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("add-404")), testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"user_id": uuid.New().String(),
	})
	testutil.SetAuthContext(req, org.ID, admin.ID)

	err := app.AddOrganizationMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_AddOrganizationMember_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]any{
		"user_id": uuid.New().String(),
	})

	err := app.AddOrganizationMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- RemoveOrganizationMember Tests ---

func TestApp_RemoveOrganizationMember_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("remove-admin")), testutil.WithRoleID(&role.ID))

	// Add a target user as member (CreateTestUser auto-creates user_organizations entry)
	targetUser := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("remove-target")))

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "member_id", targetUser.ID.String())

	err := app.RemoveOrganizationMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify membership was removed
	var count int64
	app.DB.Model(&models.UserOrganization{}).Where("user_id = ? AND organization_id = ?", targetUser.ID, org.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestApp_RemoveOrganizationMember_CannotRemoveSelf(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("remove-self")), testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "member_id", admin.ID.String())

	err := app.RemoveOrganizationMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_RemoveOrganizationMember_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("remove-404")), testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "member_id", uuid.New().String())

	err := app.RemoveOrganizationMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_RemoveOrganizationMember_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	testutil.SetPathParam(req, "member_id", uuid.New().String())

	err := app.RemoveOrganizationMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- UpdateOrganizationMemberRole Tests ---

func TestApp_UpdateOrganizationMemberRole_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	adminRole := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	agentRole := testutil.CreateTestRole(t, app.DB, org.ID, "agent", nil)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-role-admin")), testutil.WithRoleID(&adminRole.ID))

	// Target user is member via CreateTestUser (auto-creates user_organizations entry)
	targetUser := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-role-target")))

	req := testutil.NewJSONRequest(t, map[string]any{
		"role_id": agentRole.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "member_id", targetUser.ID.String())

	err := app.UpdateOrganizationMemberRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify role was updated
	var userOrg models.UserOrganization
	require.NoError(t, app.DB.Where("user_id = ? AND organization_id = ?", targetUser.ID, org.ID).First(&userOrg).Error)
	require.NotNil(t, userOrg.RoleID)
	assert.Equal(t, agentRole.ID, *userOrg.RoleID)
}

func TestApp_UpdateOrganizationMemberRole_MissingRoleID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-role-no-id")), testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{})
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "member_id", uuid.New().String())

	err := app.UpdateOrganizationMemberRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateOrganizationMemberRole_InvalidRole(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	role := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-role-invalid")), testutil.WithRoleID(&role.ID))

	targetUser := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-role-inv-target")))

	req := testutil.NewJSONRequest(t, map[string]any{
		"role_id": uuid.New().String(), // non-existent role
	})
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "member_id", targetUser.ID.String())

	err := app.UpdateOrganizationMemberRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateOrganizationMemberRole_MemberNotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	adminRole := testutil.CreateTestRole(t, app.DB, org.ID, "admin", allPerms)
	agentRole := testutil.CreateTestRole(t, app.DB, org.ID, "agent", nil)
	admin := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-role-mem-404")), testutil.WithRoleID(&adminRole.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"role_id": agentRole.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "member_id", uuid.New().String())

	err := app.UpdateOrganizationMemberRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateOrganizationMemberRole_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]any{
		"role_id": uuid.New().String(),
	})
	testutil.SetPathParam(req, "member_id", uuid.New().String())

	err := app.UpdateOrganizationMemberRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- ListMyOrganizations Tests ---

func TestApp_ListMyOrganizations_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithEmail(testutil.UniqueEmail("list-my-orgs")))

	// Add user to org2 (org1 membership is auto-created by CreateTestUser)
	require.NoError(t, app.DB.Create(&models.UserOrganization{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		UserID:         user.ID,
		OrganizationID: org2.ID,
		IsDefault:      false,
	}).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "user_id", user.ID)

	err := app.ListMyOrganizations(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Organizations []struct {
				OrganizationID uuid.UUID `json:"organization_id"`
				Name           string    `json:"name"`
				Slug           string    `json:"slug"`
				IsDefault      bool      `json:"is_default"`
			} `json:"organizations"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Len(t, resp.Data.Organizations, 2)

	orgIDs := make(map[uuid.UUID]bool)
	for _, o := range resp.Data.Organizations {
		orgIDs[o.OrganizationID] = true
		assert.NotEmpty(t, o.Name)
	}
	assert.True(t, orgIDs[org1.ID])
	assert.True(t, orgIDs[org2.ID])
}

func TestApp_ListMyOrganizations_SingleOrg(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("list-my-orgs-single")))

	// User has one org membership (auto-created by CreateTestUser)
	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "user_id", user.ID)

	err := app.ListMyOrganizations(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Organizations []struct {
				OrganizationID uuid.UUID `json:"organization_id"`
			} `json:"organizations"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Len(t, resp.Data.Organizations, 1)
	assert.Equal(t, org.ID, resp.Data.Organizations[0].OrganizationID)
}

func TestApp_ListMyOrganizations_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No user_id set

	err := app.ListMyOrganizations(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- SwitchOrg Tests ---

func TestApp_SwitchOrg_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithEmail(testutil.UniqueEmail("switch-org")), testutil.WithPassword("password123"))

	// Add user to org2
	agentRole := testutil.CreateTestRole(t, app.DB, org2.ID, "agent", nil)
	require.NoError(t, app.DB.Create(&models.UserOrganization{
		UserID:         user.ID,
		OrganizationID: org2.ID,
		RoleID:         &agentRole.ID,
		IsDefault:      false,
	}).Error)

	req := testutil.NewJSONRequest(t, map[string]any{
		"organization_id": org2.ID.String(),
	})
	testutil.SetPathParam(req, "user_id", user.ID)

	err := app.SwitchOrg(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Tokens are now in httpOnly cookies, not in the response body
	accessCookie := testutil.GetResponseCookie(req, "whm_access")
	refreshCookie := testutil.GetResponseCookie(req, "whm_refresh")
	assert.NotEmpty(t, accessCookie, "whm_access cookie should be set")
	assert.NotEmpty(t, refreshCookie, "whm_refresh cookie should be set")

	var resp struct {
		Data struct {
			ExpiresIn int `json:"expires_in"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Greater(t, resp.Data.ExpiresIn, 0)
}

func TestApp_SwitchOrg_SuperAdmin(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	superAdmin := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithEmail(testutil.UniqueEmail("switch-org-sa")), testutil.WithSuperAdmin())

	// Super admin can switch to any org without membership
	req := testutil.NewJSONRequest(t, map[string]any{
		"organization_id": org2.ID.String(),
	})
	testutil.SetPathParam(req, "user_id", superAdmin.ID)

	err := app.SwitchOrg(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

func TestApp_SwitchOrg_NotMember(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithEmail(testutil.UniqueEmail("switch-org-no-member")))

	// User is NOT a member of org2
	req := testutil.NewJSONRequest(t, map[string]any{
		"organization_id": org2.ID.String(),
	})
	testutil.SetPathParam(req, "user_id", user.ID)

	err := app.SwitchOrg(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_SwitchOrg_OrgNotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("switch-org-404")))

	req := testutil.NewJSONRequest(t, map[string]any{
		"organization_id": uuid.New().String(),
	})
	testutil.SetPathParam(req, "user_id", user.ID)

	err := app.SwitchOrg(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_SwitchOrg_MissingOrgID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("switch-org-no-id")))

	req := testutil.NewJSONRequest(t, map[string]any{})
	testutil.SetPathParam(req, "user_id", user.ID)

	err := app.SwitchOrg(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_SwitchOrg_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]any{
		"organization_id": uuid.New().String(),
	})

	err := app.SwitchOrg(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- Cross-org permission scoping ---

// A user's home-org role must not leak into an org they only belong to as a
// plain member: the permission lookup has to resolve the role from that org's
// membership row rather than falling back to users.role_id.
func TestApp_HasPermission_DoesNotInheritHomeOrgRoleInForeignOrg(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	homeOrg := testutil.CreateTestOrganization(t, app.DB)
	foreignOrg := testutil.CreateTestOrganization(t, app.DB)

	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	homeAdminRole := testutil.CreateTestRole(t, app.DB, homeOrg.ID, "admin", allPerms)
	user := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
		testutil.WithEmail(testutil.UniqueEmail("role-leak")), testutil.WithRoleID(&homeAdminRole.ID))

	assert.True(t, app.HasPermission(user.ID, models.ResourceTags, models.ActionWrite, homeOrg.ID),
		"admin in their home org")

	// Membership in the foreign org carrying no role — the shape an org with no
	// default role used to produce.
	require.NoError(t, app.DB.Create(&models.UserOrganization{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		UserID:         user.ID,
		OrganizationID: foreignOrg.ID,
	}).Error)

	assert.False(t, app.HasPermission(user.ID, models.ResourceTags, models.ActionWrite, foreignOrg.ID),
		"home-org admin role must not carry into a foreign org")
}

// Super admins hold no membership rows, so dropping the home-role fallback must
// not lock them out of other orgs — the flag alone grants access.
func TestApp_HasPermission_SuperAdminWithoutMembershipInForeignOrg(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	homeOrg := testutil.CreateTestOrganization(t, app.DB)
	foreignOrg := testutil.CreateTestOrganization(t, app.DB)

	superAdmin := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
		testutil.WithEmail(testutil.UniqueEmail("sa-foreign")), testutil.WithSuperAdmin())

	assert.True(t, app.HasPermission(superAdmin.ID, models.ResourceTags, models.ActionWrite, foreignOrg.ID),
		"super admin retains access to an org they hold no membership in")
}

// A member removed from an org must lose their permissions there even though
// their access token still names it.
func TestApp_HasPermission_RemovedMemberLosesForeignOrgAccess(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	homeOrg := testutil.CreateTestOrganization(t, app.DB)
	foreignOrg := testutil.CreateTestOrganization(t, app.DB)

	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	homeAdminRole := testutil.CreateTestRole(t, app.DB, homeOrg.ID, "admin", allPerms)
	foreignRole := testutil.CreateTestRole(t, app.DB, foreignOrg.ID, "agent", allPerms)
	user := testutil.CreateTestUser(t, app.DB, homeOrg.ID,
		testutil.WithEmail(testutil.UniqueEmail("removed-member")), testutil.WithRoleID(&homeAdminRole.ID))

	membership := models.UserOrganization{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		UserID:         user.ID,
		OrganizationID: foreignOrg.ID,
		RoleID:         &foreignRole.ID,
	}
	require.NoError(t, app.DB.Create(&membership).Error)
	assert.True(t, app.HasPermission(user.ID, models.ResourceTags, models.ActionWrite, foreignOrg.ID))

	require.NoError(t, app.DB.Delete(&membership).Error)
	app.InvalidateUserPermissionsCache(user.ID)

	assert.False(t, app.HasPermission(user.ID, models.ResourceTags, models.ActionWrite, foreignOrg.ID),
		"removed member must not fall back to their home-org role")
}
