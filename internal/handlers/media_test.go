package handlers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// withStorageDir creates a temp dir, writes the given relative file with content,
// and configures app.Config.Storage.LocalPath. Returns the relative file path.
func withStorageDir(t *testing.T, app *handlers.App, relPath string, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	if app.Config == nil {
		app.Config = &config.Config{
			JWT: config.JWTConfig{Secret: testutil.TestJWTSecret, AccessExpiryMins: 15, RefreshExpiryDays: 7},
		}
	}
	app.Config.Storage.LocalPath = dir

	full := filepath.Join(dir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755))
	require.NoError(t, os.WriteFile(full, content, 0644))
	return relPath
}

// makeMediaMessage creates a Message row pointing at the given media path.
func makeMediaMessage(t *testing.T, app *handlers.App, orgID uuid.UUID, contactID uuid.UUID, mediaURL string) *models.Message {
	t.Helper()
	msg := &models.Message{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		ContactID:      contactID,
		Direction:      models.DirectionIncoming,
		MessageType:    models.MessageTypeImage,
		MediaURL:       mediaURL,
		Status:         models.MessageStatusDelivered,
	}
	require.NoError(t, app.DB.Create(msg).Error)
	return msg
}

// --- ServeMedia: happy path with permission ---

func TestApp_ServeMedia_Success_WithContactsRead(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contactsReadPerms := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "media-reader", []string{"contacts:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&contactsReadPerms.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	rel := withStorageDir(t, app, "images/cat.jpg", []byte("\xFF\xD8\xFF\xE0jpeg-bytes"))
	msg := makeMediaMessage(t, app, org.ID, contact.ID, rel)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "message_id", msg.ID.String())

	require.NoError(t, app.ServeMedia(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	assert.Equal(t, "image/jpeg", string(req.RequestCtx.Response.Header.Peek("Content-Type")))
	assert.Equal(t, "private, max-age=3600", string(req.RequestCtx.Response.Header.Peek("Cache-Control")))
	assert.Equal(t, "\xFF\xD8\xFF\xE0jpeg-bytes", string(testutil.GetResponseBody(req)))
}

// --- ServeMedia: directory traversal blocked ---

func TestApp_ServeMedia_RejectsDirectoryTraversal(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "media-r", []string{"contacts:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	dir := t.TempDir()
	app.Config = &config.Config{
		Storage: config.StorageConfig{LocalPath: dir},
		JWT:     config.JWTConfig{Secret: testutil.TestJWTSecret, AccessExpiryMins: 15, RefreshExpiryDays: 7},
	}
	// Plant a "secret" file outside the storage dir.
	outside := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("nope"), 0644))

	// Use a relative path that escapes storage dir.
	msg := makeMediaMessage(t, app, org.ID, contact.ID, "../../../etc/passwd")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "message_id", msg.ID.String())

	require.NoError(t, app.ServeMedia(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req),
		"path traversal must be blocked at the storage boundary")
}

// --- ServeMedia: symlink rejected ---

func TestApp_ServeMedia_RejectsSymlink(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "media-r-sym", []string{"contacts:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	dir := t.TempDir()
	app.Config = &config.Config{
		Storage: config.StorageConfig{LocalPath: dir},
		JWT:     config.JWTConfig{Secret: testutil.TestJWTSecret, AccessExpiryMins: 15, RefreshExpiryDays: 7},
	}
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "images"), 0755))

	// Real file outside storage.
	outsideDir := t.TempDir()
	target := filepath.Join(outsideDir, "real.txt")
	require.NoError(t, os.WriteFile(target, []byte("contents"), 0644))

	// Symlink inside storage pointing to outside file.
	link := filepath.Join(dir, "images", "linked.txt")
	require.NoError(t, os.Symlink(target, link))

	msg := makeMediaMessage(t, app, org.ID, contact.ID, "images/linked.txt")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "message_id", msg.ID.String())

	require.NoError(t, app.ServeMedia(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req),
		"symlinked media files must be rejected to prevent reading arbitrary host files")
}

// --- ServeMedia: file missing on disk ---

func TestApp_ServeMedia_FileMissingOnDisk(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "media-missing", []string{"contacts:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	dir := t.TempDir()
	app.Config = &config.Config{
		Storage: config.StorageConfig{LocalPath: dir},
		JWT:     config.JWTConfig{Secret: testutil.TestJWTSecret, AccessExpiryMins: 15, RefreshExpiryDays: 7},
	}
	msg := makeMediaMessage(t, app, org.ID, contact.ID, "images/missing.png")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "message_id", msg.ID.String())

	require.NoError(t, app.ServeMedia(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- ServeMedia: cross-org isolation ---

func TestApp_ServeMedia_CrossOrgIsolation(t *testing.T) {
	app := newTestApp(t)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)
	roleB := testutil.CreateTestRoleWithKeys(t, app.DB, orgB.ID, "media-cross", []string{"contacts:read"})
	userB := testutil.CreateTestUser(t, app.DB, orgB.ID, testutil.WithRoleID(&roleB.ID))
	contactA := testutil.CreateTestContact(t, app.DB, orgA.ID)

	rel := withStorageDir(t, app, "images/secret.png", []byte("orgA's data"))
	msg := makeMediaMessage(t, app, orgA.ID, contactA.ID, rel)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, orgB.ID, userB.ID)
	testutil.SetPathParam(req, "message_id", msg.ID.String())

	require.NoError(t, app.ServeMedia(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req),
		"users from other orgs must not see another org's media")
}

// --- ServeMedia: agent without contacts:read can read assigned contact's media ---

func TestApp_ServeMedia_AgentCanReadAssignedContactMedia(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	// Role with NO contacts:read.
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "limited", false, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Assign the contact to the user.
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", user.ID).Error)

	rel := withStorageDir(t, app, "images/mine.png", []byte("ok"))
	msg := makeMediaMessage(t, app, org.ID, contact.ID, rel)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "message_id", msg.ID.String())

	require.NoError(t, app.ServeMedia(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

// --- ServeMedia: agent without contacts:read or assignment is denied ---

func TestApp_ServeMedia_AgentWithoutAssignmentDenied(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "no-perms", false, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID) // not assigned to user

	rel := withStorageDir(t, app, "images/notmine.png", []byte("denied"))
	msg := makeMediaMessage(t, app, org.ID, contact.ID, rel)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "message_id", msg.ID.String())

	require.NoError(t, app.ServeMedia(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

// --- ServeMedia: agent reaches via team-transfer membership ---

func TestApp_ServeMedia_AgentViaTeamTransfer(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "team-only", false, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	team := &models.Team{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "support",
		IsActive:       true,
	}
	require.NoError(t, app.DB.Create(team).Error)
	require.NoError(t, app.DB.Create(&models.TeamMember{
		BaseModel: models.BaseModel{ID: uuid.New()},
		TeamID:    team.ID,
		UserID:    user.ID,
		Role:      models.TeamRoleAgent,
	}).Error)
	require.NoError(t, app.DB.Create(&models.AgentTransfer{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		ContactID:      contact.ID,
		TeamID:         &team.ID,
		Status:         models.TransferStatusActive,
	}).Error)

	rel := withStorageDir(t, app, "images/ticket.png", []byte("team data"))
	msg := makeMediaMessage(t, app, org.ID, contact.ID, rel)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "message_id", msg.ID.String())

	require.NoError(t, app.ServeMedia(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

// --- ServeMedia: empty MediaURL ---

func TestApp_ServeMedia_NoMediaInMessage(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "media-empty", []string{"contacts:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	msg := &models.Message{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		ContactID:      contact.ID,
		Direction:      models.DirectionIncoming,
		MessageType:    models.MessageTypeText,
		MediaURL:       "",
		Status:         models.MessageStatusDelivered,
	}
	require.NoError(t, app.DB.Create(msg).Error)

	dir := t.TempDir()
	app.Config = &config.Config{
		Storage: config.StorageConfig{LocalPath: dir},
		JWT:     config.JWTConfig{Secret: testutil.TestJWTSecret, AccessExpiryMins: 15, RefreshExpiryDays: 7},
	}

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "message_id", msg.ID.String())

	require.NoError(t, app.ServeMedia(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- ServeMedia: invalid message ID ---

func TestApp_ServeMedia_InvalidMessageID(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "message_id", "not-a-uuid")

	require.NoError(t, app.ServeMedia(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- ServeMedia: unauthorized ---

func TestApp_ServeMedia_Unauthorized(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "message_id", uuid.New().String())

	require.NoError(t, app.ServeMedia(req))
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}
