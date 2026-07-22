package handlers_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

func contactsExportRole(t *testing.T, db *gorm.DB, orgID uuid.UUID) *models.CustomRole {
	t.Helper()
	return testutil.CreateTestRoleWithKeys(t, db, orgID, "contacts-exporter", []string{"contacts:export", "contacts:import"})
}

// --- GetExportConfig ---

func TestApp_GetExportConfig_Contacts(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := contactsExportRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "table", "contacts")

	require.NoError(t, app.GetExportConfig(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Table          string              `json:"table"`
			Columns        []map[string]string `json:"columns"`
			DefaultColumns []string            `json:"default_columns"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, "contacts", resp.Data.Table)
	assert.NotEmpty(t, resp.Data.Columns, "columns must be returned")
	assert.NotEmpty(t, resp.Data.DefaultColumns)

	// Sanity: column entries have key + label.
	got := make(map[string]string, len(resp.Data.Columns))
	for _, c := range resp.Data.Columns {
		got[c["key"]] = c["label"]
	}
	assert.Equal(t, "Phone Number", got["phone_number"])
	assert.Contains(t, got, "tags")
}

func TestApp_GetExportConfig_InvalidTable(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := contactsExportRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "table", "users") // not in exportConfigs

	require.NoError(t, app.GetExportConfig(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid table")
}

func TestApp_GetExportConfig_PermissionDenied(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "no-export", false, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "table", "contacts")

	require.NoError(t, app.GetExportConfig(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

// --- GetImportConfig ---

func TestApp_GetImportConfig_Contacts(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := contactsExportRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "table", "contacts")

	require.NoError(t, app.GetImportConfig(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Table           string              `json:"table"`
			RequiredColumns []map[string]string `json:"required_columns"`
			OptionalColumns []map[string]string `json:"optional_columns"`
			UniqueColumn    string              `json:"unique_column"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, "contacts", resp.Data.Table)
	assert.Equal(t, "phone_number", resp.Data.UniqueColumn)
	require.Len(t, resp.Data.RequiredColumns, 1)
	assert.Equal(t, "phone_number", resp.Data.RequiredColumns[0]["key"])
	assert.NotEmpty(t, resp.Data.OptionalColumns)
}

func TestApp_GetImportConfig_InvalidTable(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := contactsExportRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "table", "made-up")

	require.NoError(t, app.GetImportConfig(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid table")
}

// --- ExportData ---

func TestApp_ExportData_Contacts_OnlyOwnOrg(t *testing.T) {
	app := newTestApp(t)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)
	role := contactsExportRole(t, app.DB, orgA.ID)
	userA := testutil.CreateTestUser(t, app.DB, orgA.ID, testutil.WithRoleID(&role.ID))

	// Create contacts in BOTH orgs.
	c1 := testutil.CreateTestContactWith(t, app.DB, orgA.ID, testutil.WithPhoneNumber("+11111111"))
	c2 := testutil.CreateTestContactWith(t, app.DB, orgA.ID, testutil.WithPhoneNumber("+22222222"))
	// Other org contact MUST NOT appear in user A's export.
	other := testutil.CreateTestContactWith(t, app.DB, orgB.ID, testutil.WithPhoneNumber("+99999999"))

	body, _ := json.Marshal(map[string]any{
		"table":   "contacts",
		"columns": []string{"phone_number", "profile_name"},
	})
	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.SetBody(body)
	testutil.SetAuthContext(req, orgA.ID, userA.ID)

	require.NoError(t, app.ExportData(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	csv := string(testutil.GetResponseBody(req))

	// Header + 2 rows from orgA, no orgB row.
	lines := strings.Split(strings.TrimRight(csv, "\n"), "\n")
	require.Len(t, lines, 3, "expected header + 2 rows")
	assert.Contains(t, lines[0], "Phone Number")
	combined := strings.Join(lines[1:], "\n")
	assert.Contains(t, combined, c1.PhoneNumber)
	assert.Contains(t, combined, c2.PhoneNumber)
	assert.NotContains(t, combined, other.PhoneNumber, "other org's contact must not leak into export")

	// Content-Type and download header.
	assert.Equal(t, "text/csv", string(req.RequestCtx.Response.Header.Peek("Content-Type")))
	assert.Contains(t, string(req.RequestCtx.Response.Header.Peek("Content-Disposition")), "attachment")
}

func TestApp_ExportData_RejectsDisallowedColumn(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := contactsExportRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	body, _ := json.Marshal(map[string]any{
		"table":   "contacts",
		"columns": []string{"phone_number", "password_hash"}, // not in AllowedColumns
	})
	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.SetBody(body)
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.ExportData(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "not allowed for export")
}

func TestApp_ExportData_PermissionDenied(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	// Role with no contacts:export.
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "read-only", []string{"contacts:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	body, _ := json.Marshal(map[string]any{"table": "contacts"})
	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.SetBody(body)
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.ExportData(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_ExportData_InvalidTable(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := contactsExportRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	body, _ := json.Marshal(map[string]any{"table": "users"})
	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.SetBody(body)
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.ExportData(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid table")
}

func TestApp_ExportData_InvalidJSONBody(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := contactsExportRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.SetBody([]byte("not json"))
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.ExportData(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid request body")
}

func TestApp_ExportData_DefaultColumnsWhenEmpty(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := contactsExportRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithPhoneNumber("+5555"))

	// Empty columns array → should fall back to default columns.
	body, _ := json.Marshal(map[string]any{"table": "contacts"})
	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.SetBody(body)
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.ExportData(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	csv := string(testutil.GetResponseBody(req))
	// Default columns are phone_number, profile_name, tags → header has all 3.
	header := strings.SplitN(csv, "\n", 2)[0]
	assert.Contains(t, header, "Phone Number")
	assert.Contains(t, header, "Name")
	assert.Contains(t, header, "Tags")
}

func TestApp_ExportData_CSVInjectionEscaped(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := contactsExportRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	// Insert a contact whose name starts with '=' — a classic CSV injection vector.
	require.NoError(t, app.DB.Create(&models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		PhoneNumber:    "+1234",
		ProfileName:    "=cmd|'/c calc'!A1",
	}).Error)

	body, _ := json.Marshal(map[string]any{
		"table":   "contacts",
		"columns": []string{"profile_name"},
	})
	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.SetBody(body)
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.ExportData(req))
	csv := string(testutil.GetResponseBody(req))
	// The dangerous cell must be prefixed with a single quote.
	assert.Contains(t, csv, "'=cmd",
		"cells starting with '=' must be prefixed with a single quote to defuse CSV injection")
}
