package handlers_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

// auditLogsRole creates a role with audit_logs:read permission.
func auditLogsRole(t *testing.T, db *gorm.DB, orgID uuid.UUID) *models.CustomRole {
	t.Helper()
	return testutil.CreateTestRoleWithKeys(t, db, orgID, "audit-reader", []string{"audit_logs:read"})
}

func makeAuditLog(t *testing.T, db *gorm.DB, orgID, userID, resourceID uuid.UUID, resType string, action models.AuditAction, when time.Time) *models.AuditLog {
	t.Helper()
	log := &models.AuditLog{
		ID:             uuid.New(),
		OrganizationID: orgID,
		ResourceType:   resType,
		ResourceID:     resourceID,
		UserID:         userID,
		UserName:       "tester",
		Action:         action,
		Changes:        models.JSONBArray{map[string]any{"field": "name", "old_value": nil, "new_value": "x"}},
	}
	require.NoError(t, db.Create(log).Error)
	// Force created_at since GORM autoCreateTime overrides on Create.
	require.NoError(t, db.Model(log).Update("created_at", when).Error)
	return log
}

// --- ListAuditLogs ---

func TestApp_ListAuditLogs_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := auditLogsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	now := time.Now()
	makeAuditLog(t, app.DB, org.ID, user.ID, uuid.New(), "contact", models.AuditActionCreated, now.Add(-2*time.Hour))
	makeAuditLog(t, app.DB, org.ID, user.ID, uuid.New(), "contact", models.AuditActionUpdated, now.Add(-1*time.Hour))
	makeAuditLog(t, app.DB, org.ID, user.ID, uuid.New(), "user", models.AuditActionDeleted, now)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.ListAuditLogs(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			AuditLogs []handlers.AuditLogResponse `json:"audit_logs"`
			Total     int                         `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, 3, resp.Data.Total)
	assert.Len(t, resp.Data.AuditLogs, 3)
	// DESC by created_at: newest first.
	assert.Equal(t, models.AuditActionDeleted, resp.Data.AuditLogs[0].Action)
}

func TestApp_ListAuditLogs_FilterByResourceType(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := auditLogsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	makeAuditLog(t, app.DB, org.ID, user.ID, uuid.New(), "contact", models.AuditActionCreated, time.Now())
	makeAuditLog(t, app.DB, org.ID, user.ID, uuid.New(), "user", models.AuditActionCreated, time.Now())
	makeAuditLog(t, app.DB, org.ID, user.ID, uuid.New(), "contact", models.AuditActionUpdated, time.Now())

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "resource_type", "contact")

	require.NoError(t, app.ListAuditLogs(req))
	var resp struct {
		Data struct {
			AuditLogs []handlers.AuditLogResponse `json:"audit_logs"`
			Total     int                         `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, 2, resp.Data.Total)
	for _, l := range resp.Data.AuditLogs {
		assert.Equal(t, "contact", l.ResourceType)
	}
}

func TestApp_ListAuditLogs_FilterByAction(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := auditLogsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	makeAuditLog(t, app.DB, org.ID, user.ID, uuid.New(), "contact", models.AuditActionCreated, time.Now())
	makeAuditLog(t, app.DB, org.ID, user.ID, uuid.New(), "contact", models.AuditActionDeleted, time.Now())

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "action", "deleted")

	require.NoError(t, app.ListAuditLogs(req))
	var resp struct {
		Data struct {
			AuditLogs []handlers.AuditLogResponse `json:"audit_logs"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	require.Len(t, resp.Data.AuditLogs, 1)
	assert.Equal(t, models.AuditActionDeleted, resp.Data.AuditLogs[0].Action)
}

func TestApp_ListAuditLogs_DateRangeFilter(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := auditLogsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	now := time.Now()
	old := now.Add(-72 * time.Hour)
	makeAuditLog(t, app.DB, org.ID, user.ID, uuid.New(), "x", models.AuditActionCreated, old)
	makeAuditLog(t, app.DB, org.ID, user.ID, uuid.New(), "x", models.AuditActionCreated, now)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "from", now.Add(-24*time.Hour).Format(time.RFC3339))

	require.NoError(t, app.ListAuditLogs(req))
	var resp struct {
		Data struct {
			AuditLogs []handlers.AuditLogResponse `json:"audit_logs"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	require.Len(t, resp.Data.AuditLogs, 1, "from-filter must exclude logs older than 24h ago")
}

func TestApp_ListAuditLogs_CrossOrgIsolation(t *testing.T) {
	app := newTestApp(t)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)
	roleA := auditLogsRole(t, app.DB, orgA.ID)
	roleB := auditLogsRole(t, app.DB, orgB.ID)
	userA := testutil.CreateTestUser(t, app.DB, orgA.ID, testutil.WithRoleID(&roleA.ID))
	userB := testutil.CreateTestUser(t, app.DB, orgB.ID, testutil.WithRoleID(&roleB.ID))

	makeAuditLog(t, app.DB, orgA.ID, userA.ID, uuid.New(), "x", models.AuditActionCreated, time.Now())
	makeAuditLog(t, app.DB, orgA.ID, userA.ID, uuid.New(), "x", models.AuditActionUpdated, time.Now())

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, orgB.ID, userB.ID)
	require.NoError(t, app.ListAuditLogs(req))
	var resp struct {
		Data struct {
			AuditLogs []handlers.AuditLogResponse `json:"audit_logs"`
			Total     int                         `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, 0, resp.Data.Total, "audit logs from another org must not be returned")
}

func TestApp_ListAuditLogs_PermissionDenied(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	// Role has no audit_logs:read.
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "no-audit-perms", false, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.ListAuditLogs(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

// --- GetAuditLog ---

func TestApp_GetAuditLog_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := auditLogsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	log := makeAuditLog(t, app.DB, org.ID, user.ID, uuid.New(), "contact", models.AuditActionUpdated, time.Now())

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", log.ID.String())

	require.NoError(t, app.GetAuditLog(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.AuditLogResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, log.ID, resp.Data.ID)
	assert.Equal(t, "contact", resp.Data.ResourceType)
}

func TestApp_GetAuditLog_NotFound(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := auditLogsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	require.NoError(t, app.GetAuditLog(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "Audit log not found")
}

func TestApp_GetAuditLog_CrossOrgIsolation(t *testing.T) {
	app := newTestApp(t)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)
	roleB := auditLogsRole(t, app.DB, orgB.ID)
	userA := testutil.CreateTestUser(t, app.DB, orgA.ID)
	userB := testutil.CreateTestUser(t, app.DB, orgB.ID, testutil.WithRoleID(&roleB.ID))
	log := makeAuditLog(t, app.DB, orgA.ID, userA.ID, uuid.New(), "x", models.AuditActionCreated, time.Now())

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, orgB.ID, userB.ID)
	testutil.SetPathParam(req, "id", log.ID.String())

	require.NoError(t, app.GetAuditLog(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req),
		"cross-org access must look like not-found")
}
