package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// --- ListContacts Tests ---

func TestApp_ListContacts(t *testing.T) {
	t.Parallel()

	t.Run("success with pagination", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		// Create 3 contacts
		for i := 0; i < 3; i++ {
			testutil.CreateTestContact(t, app.DB, org.ID)
		}

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetQueryParam(req, "page", 1)
		testutil.SetQueryParam(req, "limit", 2)

		err := app.ListContacts(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Contacts []handlers.ContactResponse `json:"contacts"`
				Total    int64                      `json:"total"`
				Page     int                        `json:"page"`
				Limit    int                        `json:"limit"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, int64(3), resp.Data.Total)
		assert.Len(t, resp.Data.Contacts, 2)
		assert.Equal(t, 1, resp.Data.Page)
		assert.Equal(t, 2, resp.Data.Limit)
	})

	t.Run("empty list", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListContacts(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Contacts []handlers.ContactResponse `json:"contacts"`
				Total    int64                      `json:"total"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, int64(0), resp.Data.Total)
		assert.Empty(t, resp.Data.Contacts)
	})

	t.Run("filter by search on phone number", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		// Create contacts with distinct phone numbers
		uniquePhone := "+9998887776"
		testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithPhoneNumber(uniquePhone))
		testutil.CreateTestContact(t, app.DB, org.ID) // different phone

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetQueryParam(req, "search", "9998887776")

		err := app.ListContacts(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Contacts []handlers.ContactResponse `json:"contacts"`
				Total    int64                      `json:"total"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, int64(1), resp.Data.Total)
		assert.Len(t, resp.Data.Contacts, 1)
		assert.Equal(t, uniquePhone, resp.Data.Contacts[0].PhoneNumber)
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org1.ID)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&adminRole.ID))

		// Create a contact in org2
		testutil.CreateTestContact(t, app.DB, org2.ID)

		// User from org1 should see no contacts
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org1.ID, user1.ID)

		err := app.ListContacts(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Contacts []handlers.ContactResponse `json:"contacts"`
				Total    int64                      `json:"total"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, int64(0), resp.Data.Total)
		assert.Empty(t, resp.Data.Contacts)
	})

	t.Run("returns contact fields correctly", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListContacts(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Contacts []handlers.ContactResponse `json:"contacts"`
				Total    int64                      `json:"total"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		require.Len(t, resp.Data.Contacts, 1)
		assert.Equal(t, contact.ID, resp.Data.Contacts[0].ID)
		assert.Equal(t, contact.PhoneNumber, resp.Data.Contacts[0].PhoneNumber)
		assert.Equal(t, contact.ProfileName, resp.Data.Contacts[0].ProfileName)
		assert.Equal(t, "active", resp.Data.Contacts[0].Status)
		assert.NotNil(t, resp.Data.Contacts[0].Tags)
	})

	t.Run("default pagination with no params", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListContacts(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Contacts []handlers.ContactResponse `json:"contacts"`
				Total    int64                      `json:"total"`
				Page     int                        `json:"page"`
				Limit    int                        `json:"limit"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		// Default pagination: page=1, limit=50
		assert.Equal(t, 1, resp.Data.Page)
		assert.Equal(t, 50, resp.Data.Limit)
	})
}

// --- GetContact Tests ---

func TestApp_GetContact(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.GetContact(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.ContactResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, contact.ID, resp.Data.ID)
		assert.Equal(t, contact.PhoneNumber, resp.Data.PhoneNumber)
		assert.Equal(t, contact.ProfileName, resp.Data.ProfileName)
		assert.Equal(t, "active", resp.Data.Status)
		assert.NotNil(t, resp.Data.Tags)
		assert.Equal(t, 0, resp.Data.UnreadCount)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.GetContact(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid ID", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", "not-a-uuid")

		err := app.GetContact(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org1.ID)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&adminRole.ID))

		// Create contact in org2
		contact := testutil.CreateTestContact(t, app.DB, org2.ID)

		// User from org1 should not access org2's contact
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org1.ID, user1.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.GetContact(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("returns unread count", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		// Create an incoming unread message
		msg := &models.Message{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			WhatsAppAccount: account.Name,
			ContactID:       contact.ID,
			Direction:       models.DirectionIncoming,
			MessageType:     models.MessageTypeText,
			Content:         "Hello",
			Status:          models.MessageStatusDelivered,
		}
		require.NoError(t, app.DB.Create(msg).Error)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.GetContact(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.ContactResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, 1, resp.Data.UnreadCount)
	})
}

// --- GetContactSessionData Tests ---

func TestApp_GetContactSessionData(t *testing.T) {
	t.Parallel()

	t.Run("success with no session", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.GetContactSessionData(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.ContactSessionDataResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Nil(t, resp.Data.SessionID)
		assert.NotNil(t, resp.Data.SessionData)
		assert.NotNil(t, resp.Data.PanelConfig)
	})

	t.Run("success with active session", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		// Create an active chatbot session
		session := &models.ChatbotSession{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			ContactID:       contact.ID,
			WhatsAppAccount: account.Name,
			PhoneNumber:     contact.PhoneNumber,
			Status:          models.SessionStatusActive,
			SessionData:     models.JSONB{"name": "Test User", "email": "test@example.com"},
			StartedAt:       time.Now(),
			LastActivityAt:  time.Now(),
		}
		require.NoError(t, app.DB.Create(session).Error)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.GetContactSessionData(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.ContactSessionDataResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.NotNil(t, resp.Data.SessionID)
		assert.Equal(t, session.ID, *resp.Data.SessionID)
	})

	t.Run("not found - contact does not exist", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.GetContactSessionData(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid contact ID", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", "not-a-uuid")

		err := app.GetContactSessionData(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org1.ID)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org2.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org1.ID, user1.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.GetContactSessionData(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// --- AssignContact Tests ---

func TestApp_AssignContact(t *testing.T) {
	t.Parallel()

	t.Run("success - assign to user", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		// Create another user to assign to
		assignee := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"user_id": assignee.ID.String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.AssignContact(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message        string     `json:"message"`
				AssignedUserID *uuid.UUID `json:"assigned_user_id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Contains(t, resp.Data.Message, "assigned successfully")
		assert.NotNil(t, resp.Data.AssignedUserID)
		assert.Equal(t, assignee.ID, *resp.Data.AssignedUserID)

		// Verify in database
		var updatedContact models.Contact
		require.NoError(t, app.DB.Where("id = ?", contact.ID).First(&updatedContact).Error)
		require.NotNil(t, updatedContact.AssignedUserID)
		assert.Equal(t, assignee.ID, *updatedContact.AssignedUserID)
	})

	t.Run("success - unassign", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		assignee := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		// Pre-assign the contact
		require.NoError(t, app.DB.Model(&contact).Update("assigned_user_id", assignee.ID).Error)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"user_id": nil,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.AssignContact(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message        string     `json:"message"`
				AssignedUserID *uuid.UUID `json:"assigned_user_id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Contains(t, resp.Data.Message, "assigned successfully")
		assert.Nil(t, resp.Data.AssignedUserID)

		// Verify in database
		var updatedContact models.Contact
		require.NoError(t, app.DB.Where("id = ?", contact.ID).First(&updatedContact).Error)
		assert.Nil(t, updatedContact.AssignedUserID)
	})

	t.Run("forbidden - user without write permission", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)

		// Create a role with only contacts:read (no contacts:write)
		readOnlyRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "readonly", []string{
			"contacts:read",
		})
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&readOnlyRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		assignee := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"user_id": assignee.ID.String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.AssignContact(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
	})

	t.Run("contact not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		assignee := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"user_id": assignee.ID.String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.AssignContact(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid contact ID", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"user_id": uuid.New().String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", "not-a-uuid")

		err := app.AssignContact(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("assign to non-existent user", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"user_id": uuid.New().String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.AssignContact(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation - cannot assign contact from another org", func(t *testing.T) {
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org1.ID)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&adminRole.ID))
		assignee := testutil.CreateTestUser(t, app.DB, org1.ID)

		// Contact belongs to org2
		contact := testutil.CreateTestContact(t, app.DB, org2.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"user_id": assignee.ID.String(),
		})
		testutil.SetAuthContext(req, org1.ID, user1.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.AssignContact(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// --- GetMessages Tests ---

func TestApp_GetMessages(t *testing.T) {
	t.Parallel()

	t.Run("success with messages", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

		// Create messages with staggered timestamps
		now := time.Now()
		for i := 0; i < 3; i++ {
			msg := &models.Message{
				BaseModel:       models.BaseModel{ID: uuid.New(), CreatedAt: now.Add(time.Duration(i) * time.Minute)},
				OrganizationID:  org.ID,
				WhatsAppAccount: account.Name,
				ContactID:       contact.ID,
				Direction:       models.DirectionIncoming,
				MessageType:     models.MessageTypeText,
				Content:         "Hello " + string(rune('A'+i)),
				Status:          models.MessageStatusDelivered,
			}
			require.NoError(t, app.DB.Create(msg).Error)
		}

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())
		testutil.SetQueryParam(req, "limit", 50)

		err := app.GetMessages(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Messages []handlers.MessageResponse `json:"messages"`
				Total    int64                      `json:"total"`
				HasMore  bool                       `json:"has_more"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, int64(3), resp.Data.Total)
		assert.Len(t, resp.Data.Messages, 3)
		assert.False(t, resp.Data.HasMore)
	})

	t.Run("empty messages", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.GetMessages(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Messages []handlers.MessageResponse `json:"messages"`
				Total    int64                      `json:"total"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, int64(0), resp.Data.Total)
		assert.Empty(t, resp.Data.Messages)
	})

	t.Run("contact not found", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.GetMessages(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid contact ID", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", "not-a-uuid")

		err := app.GetMessages(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org1.ID)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&adminRole.ID))

		// Contact belongs to org2
		contact := testutil.CreateTestContact(t, app.DB, org2.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org1.ID, user1.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.GetMessages(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("default pagination limit", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())
		// No limit set - should default to 50

		err := app.GetMessages(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Messages []handlers.MessageResponse `json:"messages"`
				Total    int64                      `json:"total"`
				Page     int                        `json:"page"`
				Limit    int                        `json:"limit"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, 1, resp.Data.Page)
		assert.Equal(t, 50, resp.Data.Limit)
	})

	t.Run("cursor-based pagination with before_id", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

		// Create messages with staggered timestamps
		now := time.Now()
		var msgIDs []uuid.UUID
		for i := 0; i < 5; i++ {
			msg := &models.Message{
				BaseModel:       models.BaseModel{ID: uuid.New(), CreatedAt: now.Add(time.Duration(i) * time.Minute)},
				OrganizationID:  org.ID,
				WhatsAppAccount: account.Name,
				ContactID:       contact.ID,
				Direction:       models.DirectionIncoming,
				MessageType:     models.MessageTypeText,
				Content:         "Message " + string(rune('A'+i)),
				Status:          models.MessageStatusDelivered,
			}
			require.NoError(t, app.DB.Create(msg).Error)
			msgIDs = append(msgIDs, msg.ID)
		}

		// Use before_id pointing to the 4th message (index 3), limit 2
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())
		testutil.SetQueryParam(req, "before_id", msgIDs[3].String())
		testutil.SetQueryParam(req, "limit", 2)

		err := app.GetMessages(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Messages []handlers.MessageResponse `json:"messages"`
				Total    int64                      `json:"total"`
				HasMore  bool                       `json:"has_more"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		// Should return messages before the 4th (so messages at index 1,2)
		assert.Len(t, resp.Data.Messages, 2)
		assert.True(t, resp.Data.HasMore)
	})

	t.Run("marks messages as read on page-based fetch", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

		// Create an unread incoming message
		msg := &models.Message{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			WhatsAppAccount: account.Name,
			ContactID:       contact.ID,
			Direction:       models.DirectionIncoming,
			MessageType:     models.MessageTypeText,
			Content:         "Hello",
			Status:          models.MessageStatusDelivered,
		}
		require.NoError(t, app.DB.Create(msg).Error)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.GetMessages(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		// Verify message was marked as read in the database
		var updatedMsg models.Message
		require.NoError(t, app.DB.Where("id = ?", msg.ID).First(&updatedMsg).Error)
		assert.Equal(t, models.MessageStatusRead, updatedMsg.Status)
	})

	t.Run("message response includes correct fields", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

		msg := &models.Message{
			BaseModel:         models.BaseModel{ID: uuid.New()},
			OrganizationID:    org.ID,
			WhatsAppAccount:   account.Name,
			ContactID:         contact.ID,
			WhatsAppMessageID: "wamid.test123",
			Direction:         models.DirectionIncoming,
			MessageType:       models.MessageTypeText,
			Content:           "Test message content",
			Status:            models.MessageStatusDelivered,
		}
		require.NoError(t, app.DB.Create(msg).Error)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.GetMessages(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Messages []handlers.MessageResponse `json:"messages"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		require.Len(t, resp.Data.Messages, 1)

		m := resp.Data.Messages[0]
		assert.Equal(t, msg.ID, m.ID)
		assert.Equal(t, contact.ID, m.ContactID)
		assert.Equal(t, models.DirectionIncoming, m.Direction)
		assert.Equal(t, models.MessageTypeText, m.MessageType)
		assert.Equal(t, "wamid.test123", m.WAMID)
		assert.NotNil(t, m.Content)
	})
}

// --- SendMessage Tests ---

func TestApp_SendMessage(t *testing.T) {
	t.Parallel()

	t.Run("success - text message", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := createTestAccount(t, app, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"type": "text",
			"content": map[string]string{
				"body": "Hello from agent!",
			},
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.SendMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.MessageResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, contact.ID, resp.Data.ContactID)
		assert.Equal(t, models.DirectionOutgoing, resp.Data.Direction)
		assert.Equal(t, models.MessageTypeText, resp.Data.MessageType)
	})

	t.Run("invalid request body", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		// Send non-JSON body
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetContentType("application/json")
		ctx.Request.Header.SetMethod("POST")
		ctx.Request.SetBody([]byte("not-json"))
		req := &fastglue.Request{RequestCtx: ctx}
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.SendMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("contact not found", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"type": "text",
			"content": map[string]string{
				"body": "Hello!",
			},
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.SendMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid contact ID", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"type": "text",
			"content": map[string]string{
				"body": "Hello!",
			},
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", "not-a-uuid")

		err := app.SendMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org1.ID)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&adminRole.ID))

		// Contact belongs to org2
		contact := testutil.CreateTestContact(t, app.DB, org2.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"type": "text",
			"content": map[string]string{
				"body": "Hello!",
			},
		})
		testutil.SetAuthContext(req, org1.ID, user1.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.SendMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("no whatsapp account configured", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		// Contact with no WhatsApp account set and no accounts in org
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"type": "text",
			"content": map[string]string{
				"body": "Hello!",
			},
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.SendMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("success with reply context", func(t *testing.T) {
		t.Parallel()
		mockServer := newMockWhatsAppServer()
		defer mockServer.close()

		app := newMsgTestApp(t, mockServer)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := createTestAccount(t, app, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

		// Create an original message to reply to
		origMsg := &models.Message{
			BaseModel:         models.BaseModel{ID: uuid.New()},
			OrganizationID:    org.ID,
			WhatsAppAccount:   account.Name,
			ContactID:         contact.ID,
			WhatsAppMessageID: "wamid.original123",
			Direction:         models.DirectionIncoming,
			MessageType:       models.MessageTypeText,
			Content:           "Original message",
			Status:            models.MessageStatusDelivered,
		}
		require.NoError(t, app.DB.Create(origMsg).Error)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"type": "text",
			"content": map[string]string{
				"body": "This is a reply",
			},
			"reply_to_message_id": origMsg.ID.String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())

		err := app.SendMessage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.MessageResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.True(t, resp.Data.IsReply)
		assert.NotNil(t, resp.Data.ReplyToMessageID)
		assert.NotNil(t, resp.Data.ReplyToMessage)
	})
}

// --- SendReaction Tests ---

func TestApp_SendReaction(t *testing.T) {
	t.Parallel()

	t.Run("success - add reaction", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t, withHTTPClient(&http.Client{}))
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

		msg := &models.Message{
			BaseModel:         models.BaseModel{ID: uuid.New()},
			OrganizationID:    org.ID,
			WhatsAppAccount:   account.Name,
			ContactID:         contact.ID,
			WhatsAppMessageID: "wamid.react123",
			Direction:         models.DirectionIncoming,
			MessageType:       models.MessageTypeText,
			Content:           "Hello",
			Status:            models.MessageStatusDelivered,
		}
		require.NoError(t, app.DB.Create(msg).Error)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"emoji": "\U0001F44D",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())
		testutil.SetPathParam(req, "message_id", msg.ID.String())

		err := app.SendReaction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				MessageID string `json:"message_id"`
				Reactions []struct {
					Emoji    string `json:"emoji"`
					FromUser string `json:"from_user"`
				} `json:"reactions"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		assert.Equal(t, msg.ID.String(), resp.Data.MessageID)
		require.Len(t, resp.Data.Reactions, 1)
		assert.Equal(t, "\U0001F44D", resp.Data.Reactions[0].Emoji)
		assert.Equal(t, user.ID.String(), resp.Data.Reactions[0].FromUser)
	})

	t.Run("success - remove reaction", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t, withHTTPClient(&http.Client{}))
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
		contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

		// Create message with an existing reaction
		msg := &models.Message{
			BaseModel:         models.BaseModel{ID: uuid.New()},
			OrganizationID:    org.ID,
			WhatsAppAccount:   account.Name,
			ContactID:         contact.ID,
			WhatsAppMessageID: "wamid.remove-react",
			Direction:         models.DirectionIncoming,
			MessageType:       models.MessageTypeText,
			Content:           "Hello",
			Status:            models.MessageStatusDelivered,
			Metadata: models.JSONB{
				"reactions": []interface{}{
					map[string]interface{}{
						"emoji":     "\U0001F44D",
						"from_user": user.ID.String(),
					},
				},
			},
		}
		require.NoError(t, app.DB.Create(msg).Error)

		// Send empty emoji to remove reaction
		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"emoji": "",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())
		testutil.SetPathParam(req, "message_id", msg.ID.String())

		err := app.SendReaction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Reactions []interface{} `json:"reactions"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		// Reaction should be removed (empty or nil)
		assert.Empty(t, resp.Data.Reactions)
	})

	t.Run("contact not found", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"emoji": "\U0001F44D",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())
		testutil.SetPathParam(req, "message_id", uuid.New().String())

		err := app.SendReaction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("message not found", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"emoji": "\U0001F44D",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())
		testutil.SetPathParam(req, "message_id", uuid.New().String())

		err := app.SendReaction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid contact ID", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"emoji": "\U0001F44D",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", "not-a-uuid")
		testutil.SetPathParam(req, "message_id", uuid.New().String())

		err := app.SendReaction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid message ID", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"emoji": "\U0001F44D",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())
		testutil.SetPathParam(req, "message_id", "not-a-uuid")

		err := app.SendReaction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org1.ID)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&adminRole.ID))

		account := testutil.CreateTestWhatsAppAccount(t, app.DB, org2.ID)
		contact := testutil.CreateTestContact(t, app.DB, org2.ID)

		msg := &models.Message{
			BaseModel:         models.BaseModel{ID: uuid.New()},
			OrganizationID:    org2.ID,
			WhatsAppAccount:   account.Name,
			ContactID:         contact.ID,
			WhatsAppMessageID: "wamid.cross-org",
			Direction:         models.DirectionIncoming,
			MessageType:       models.MessageTypeText,
			Content:           "Hello",
			Status:            models.MessageStatusDelivered,
		}
		require.NoError(t, app.DB.Create(msg).Error)

		req := testutil.NewJSONRequest(t, map[string]interface{}{
			"emoji": "\U0001F44D",
		})
		testutil.SetAuthContext(req, org1.ID, user1.ID)
		testutil.SetPathParam(req, "id", contact.ID.String())
		testutil.SetPathParam(req, "message_id", msg.ID.String())

		err := app.SendReaction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// --- ListContacts additional tests ---

func TestApp_ListContacts_SearchByProfileName(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

	// Create contact with a unique profile name
	contact := &models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		PhoneNumber:    "+1111111111",
		ProfileName:    "UniqueAlphaName",
	}
	require.NoError(t, app.DB.Create(contact).Error)

	// Create another contact with a different name
	testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "search", "UniqueAlpha")

	err := app.ListContacts(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Contacts []handlers.ContactResponse `json:"contacts"`
			Total    int64                      `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, int64(1), resp.Data.Total)
	require.Len(t, resp.Data.Contacts, 1)
	assert.Equal(t, "UniqueAlphaName", resp.Data.Contacts[0].ProfileName)
}

func TestApp_ListContacts_Page2(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

	// Create 5 contacts
	for i := 0; i < 5; i++ {
		testutil.CreateTestContact(t, app.DB, org.ID)
	}

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "page", 2)
	testutil.SetQueryParam(req, "limit", 2)

	err := app.ListContacts(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Contacts []handlers.ContactResponse `json:"contacts"`
			Total    int64                      `json:"total"`
			Page     int                        `json:"page"`
			Limit    int                        `json:"limit"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, int64(5), resp.Data.Total)
	assert.Len(t, resp.Data.Contacts, 2)
	assert.Equal(t, 2, resp.Data.Page)
	assert.Equal(t, 2, resp.Data.Limit)
}

// --- GetContact additional tests ---

func TestApp_GetContact_WithAssignedUser(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	assignee := testutil.CreateTestUser(t, app.DB, org.ID)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Assign the contact
	require.NoError(t, app.DB.Model(&contact).Update("assigned_user_id", assignee.ID).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", contact.ID.String())

	err := app.GetContact(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.ContactResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.NotNil(t, resp.Data.AssignedUserID)
	assert.Equal(t, assignee.ID, *resp.Data.AssignedUserID)
}

func TestApp_GetContact_MultipleUnreadMessages(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Create 3 unread incoming messages
	for i := 0; i < 3; i++ {
		msg := &models.Message{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			WhatsAppAccount: account.Name,
			ContactID:       contact.ID,
			Direction:       models.DirectionIncoming,
			MessageType:     models.MessageTypeText,
			Content:         "Unread message",
			Status:          models.MessageStatusDelivered,
		}
		require.NoError(t, app.DB.Create(msg).Error)
	}

	// Create 1 read incoming message
	readMsg := &models.Message{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		ContactID:       contact.ID,
		Direction:       models.DirectionIncoming,
		MessageType:     models.MessageTypeText,
		Content:         "Read message",
		Status:          models.MessageStatusRead,
	}
	require.NoError(t, app.DB.Create(readMsg).Error)

	// Create 1 outgoing message (should not count as unread)
	outMsg := &models.Message{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		ContactID:       contact.ID,
		Direction:       models.DirectionOutgoing,
		MessageType:     models.MessageTypeText,
		Content:         "Outgoing message",
		Status:          models.MessageStatusSent,
	}
	require.NoError(t, app.DB.Create(outMsg).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", contact.ID.String())

	err := app.GetContact(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.ContactResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	// Only 3 incoming delivered (not read) messages should be counted
	assert.Equal(t, 3, resp.Data.UnreadCount)
}

// --- GetContactSessionData additional tests ---

func TestApp_GetContactSessionData_CompletedSession(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	completedAt := time.Now()
	session := &models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.SessionStatusCompleted,
		SessionData:     models.JSONB{"order_id": "ORD-123", "amount": 99.99},
		StartedAt:       time.Now().Add(-1 * time.Hour),
		LastActivityAt:  time.Now(),
		CompletedAt:     &completedAt,
	}
	require.NoError(t, app.DB.Create(session).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", contact.ID.String())

	err := app.GetContactSessionData(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.ContactSessionDataResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.NotNil(t, resp.Data.SessionID)
	assert.Equal(t, session.ID, *resp.Data.SessionID)
}

func TestApp_GetContactSessionData_MostRecentSessionReturned(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Create an older completed session
	oldSession := &models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New(), CreatedAt: time.Now().Add(-2 * time.Hour)},
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.SessionStatusCompleted,
		SessionData:     models.JSONB{"key": "old"},
		StartedAt:       time.Now().Add(-2 * time.Hour),
		LastActivityAt:  time.Now().Add(-2 * time.Hour),
	}
	require.NoError(t, app.DB.Create(oldSession).Error)

	// Create a newer active session
	newSession := &models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New(), CreatedAt: time.Now()},
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.SessionStatusActive,
		SessionData:     models.JSONB{"key": "new"},
		StartedAt:       time.Now(),
		LastActivityAt:  time.Now(),
	}
	require.NoError(t, app.DB.Create(newSession).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", contact.ID.String())

	err := app.GetContactSessionData(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.ContactSessionDataResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	// Should return the most recent session
	require.NotNil(t, resp.Data.SessionID)
	assert.Equal(t, newSession.ID, *resp.Data.SessionID)
}

// --- AssignContact additional tests ---

func TestApp_AssignContact_ReassignToAnotherUser(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	assignee1 := testutil.CreateTestUser(t, app.DB, org.ID)
	assignee2 := testutil.CreateTestUser(t, app.DB, org.ID)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Pre-assign to assignee1
	require.NoError(t, app.DB.Model(&contact).Update("assigned_user_id", assignee1.ID).Error)

	// Reassign to assignee2
	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"user_id": assignee2.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", contact.ID.String())

	err := app.AssignContact(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			AssignedUserID *uuid.UUID `json:"assigned_user_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	require.NotNil(t, resp.Data.AssignedUserID)
	assert.Equal(t, assignee2.ID, *resp.Data.AssignedUserID)

	// Verify in database
	var updatedContact models.Contact
	require.NoError(t, app.DB.Where("id = ?", contact.ID).First(&updatedContact).Error)
	require.NotNil(t, updatedContact.AssignedUserID)
	assert.Equal(t, assignee2.ID, *updatedContact.AssignedUserID)
}

func TestApp_AssignContact_AssignUserFromDifferentOrg(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org1.ID)
	user := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org1.ID)

	// Create a user in a different org
	otherOrgUser := testutil.CreateTestUser(t, app.DB, org2.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"user_id": otherOrgUser.ID.String(),
	})
	testutil.SetAuthContext(req, org1.ID, user.ID)
	testutil.SetPathParam(req, "id", contact.ID.String())

	err := app.AssignContact(req)
	require.NoError(t, err)
	// User from a different org should not be found
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}
