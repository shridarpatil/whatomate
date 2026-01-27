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
