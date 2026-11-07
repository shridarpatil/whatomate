package handlers_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/handlers"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/nikyjain/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type messageSearchResp struct {
	Status string `json:"status"`
	Data   struct {
		Messages []handlers.MessageSearchResult `json:"messages"`
		Total    int                            `json:"total"`
	} `json:"data"`
}

func searchMessagesHelper(t *testing.T, app *handlers.App, orgID, userID uuid.UUID, params map[string]string) (int, messageSearchResp) {
	t.Helper()
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, orgID, userID)
	for k, v := range params {
		testutil.SetQueryParam(req, k, v)
	}
	require.NoError(t, app.SearchMessages(req))
	var resp messageSearchResp
	if testutil.GetResponseStatusCode(req) == fasthttp.StatusOK {
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	}
	return testutil.GetResponseStatusCode(req), resp
}

func TestApp_SearchMessages(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	testutil.AssignRole(t, app.DB, user.ID, org.ID, "admin")

	// Create 2 contacts with unique phones
	c1 := testutil.CreateTestContact(t, app.DB, org.ID, "919999000101")
	c1.ProfileName = "Alice Wonderland"
	require.NoError(t, app.DB.Save(&c1).Error)

	c2 := testutil.CreateTestContact(t, app.DB, org.ID, "919999000102")
	c2.ProfileName = "Bob Builder"
	require.NoError(t, app.DB.Save(&c2).Error)

	now := time.Now()

	// Seed messages
	msg1 := models.Message{
		BaseModel:         models.BaseModel{ID: uuid.New(), CreatedAt: now.Add(-2 * time.Minute)},
		OrganizationID:    org.ID,
		ContactID:         c1.ID,
		WhatsAppAccount:   "acct-alpha",
		Direction:         models.DirectionIncoming,
		MessageType:       models.MessageTypeText,
		Content:           "Please send me the invoice for last month",
		WhatsAppMessageID: "wam-1-" + uuid.New().String(),
	}
	require.NoError(t, app.DB.Create(&msg1).Error)

	msg2 := models.Message{
		BaseModel:         models.BaseModel{ID: uuid.New(), CreatedAt: now.Add(-1 * time.Minute)},
		OrganizationID:    org.ID,
		ContactID:         c2.ID,
		WhatsAppAccount:   "acct-beta",
		Direction:         models.DirectionOutgoing,
		MessageType:       models.MessageTypeText,
		Content:           "Here is your official invoice document",
		WhatsAppMessageID: "wam-2-" + uuid.New().String(),
	}
	require.NoError(t, app.DB.Create(&msg2).Error)

	msg3 := models.Message{
		BaseModel:         models.BaseModel{ID: uuid.New(), CreatedAt: now},
		OrganizationID:    org.ID,
		ContactID:         c1.ID,
		WhatsAppAccount:   "acct-alpha",
		Direction:         models.DirectionIncoming,
		MessageType:       models.MessageTypeText,
		Content:           "Thank you very much!",
		WhatsAppMessageID: "wam-3-" + uuid.New().String(),
	}
	require.NoError(t, app.DB.Create(&msg3).Error)

	t.Run("empty query returns empty list", func(t *testing.T) {
		status, resp := searchMessagesHelper(t, app, org.ID, user.ID, map[string]string{"q": ""})
		assert.Equal(t, fasthttp.StatusOK, status)
		assert.Empty(t, resp.Data.Messages)
		assert.Equal(t, 0, resp.Data.Total)
	})

	t.Run("search matches across all chats", func(t *testing.T) {
		status, resp := searchMessagesHelper(t, app, org.ID, user.ID, map[string]string{"q": "invoice"})
		assert.Equal(t, fasthttp.StatusOK, status)
		assert.Len(t, resp.Data.Messages, 2)
		// Most recent first: msg2 then msg1
		assert.Equal(t, msg2.ID, resp.Data.Messages[0].ID)
		assert.Equal(t, "Bob Builder", resp.Data.Messages[0].ContactName)
		assert.Equal(t, msg1.ID, resp.Data.Messages[1].ID)
		assert.Equal(t, "Alice Wonderland", resp.Data.Messages[1].ContactName)
	})

	t.Run("search filtered by account", func(t *testing.T) {
		status, resp := searchMessagesHelper(t, app, org.ID, user.ID, map[string]string{
			"q":       "invoice",
			"account": "acct-alpha",
		})
		assert.Equal(t, fasthttp.StatusOK, status)
		require.Len(t, resp.Data.Messages, 1)
		assert.Equal(t, msg1.ID, resp.Data.Messages[0].ID)
	})

	t.Run("agent visibility scoping", func(t *testing.T) {
		// Agent with no contacts:read permission and no role assigned
		agent := testutil.CreateTestUser(t, app.DB, org.ID)

		// Search should return 0 since agent is not assigned to either contact
		status, resp := searchMessagesHelper(t, app, org.ID, agent.ID, map[string]string{"q": "invoice"})
		assert.Equal(t, fasthttp.StatusOK, status)
		assert.Empty(t, resp.Data.Messages)

		// Assign c1 to agent
		c1.AssignedUserID = &agent.ID
		require.NoError(t, app.DB.Save(&c1).Error)

		// Now agent should only see msg1 from c1
		status, resp = searchMessagesHelper(t, app, org.ID, agent.ID, map[string]string{"q": "invoice"})
		assert.Equal(t, fasthttp.StatusOK, status)
		require.Len(t, resp.Data.Messages, 1)
		assert.Equal(t, msg1.ID, resp.Data.Messages[0].ID)
	})
}

