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
	"github.com/zerodha/fastglue"
)

// getChatbotFlowPermissions returns flows.chatbot permissions from the full permission set.
func getChatbotFlowPermissions(t *testing.T, app *handlers.App) []models.Permission {
	t.Helper()

	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)

	var flowPerms []models.Permission
	for _, p := range allPerms {
		if p.Resource == models.ResourceFlowsChatbot {
			flowPerms = append(flowPerms, p)
		}
	}
	require.NotEmpty(t, flowPerms, "expected flows.chatbot permissions in default set")
	return flowPerms
}

// createTestKeywordRule creates a keyword rule directly in the DB for testing.
func createTestKeywordRule(t *testing.T, app *handlers.App, orgID uuid.UUID, name string, keywords []string) *models.KeywordRule {
	t.Helper()

	rule := &models.KeywordRule{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		Name:            name,
		Keywords:        keywords,
		MatchType:       models.MatchTypeContains,
		ResponseType:    models.ResponseTypeText,
		ResponseContent: models.JSONB{"text": "Auto reply"},
		Priority:        10,
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(rule).Error)
	return rule
}

// createTestChatbotFlow creates a chatbot flow directly in the DB for testing.
func createTestChatbotFlow(t *testing.T, app *handlers.App, orgID uuid.UUID, name string) *models.ChatbotFlow {
	t.Helper()

	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		Name:            name,
		Description:     "Test flow",
		TriggerKeywords: models.StringArray{"hello", "start"},
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(flow).Error)
	return flow
}

// createTestAIContext creates an AI context directly in the DB for testing.
func createTestAIContext(t *testing.T, app *handlers.App, orgID uuid.UUID, name string) *models.AIContext {
	t.Helper()

	ctx := &models.AIContext{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		Name:            name,
		ContextType:     models.ContextTypeStatic,
		TriggerKeywords: models.StringArray{"faq"},
		StaticContent:   "Our business hours are 9-5.",
		Priority:        10,
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(ctx).Error)
	return ctx
}

// =============================================================================
// GetChatbotSettings
// =============================================================================

func TestApp_GetChatbotSettings(t *testing.T) {
	t.Parallel()

	t.Run("success returns default settings when none exist", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.GetChatbotSettings(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Settings handlers.ChatbotSettingsResponse `json:"settings"`
				Stats    handlers.ChatbotStatsResponse    `json:"stats"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		// Default settings should have chatbot disabled
		assert.False(t, resp.Data.Settings.Enabled)
		assert.Equal(t, "Hello! How can I help you today?", resp.Data.Settings.GreetingMessage)
		assert.Equal(t, 30, resp.Data.Settings.SessionTimeoutMinutes)
		assert.False(t, resp.Data.Settings.AIEnabled)

		// Stats should all be zero for a fresh org
		assert.Equal(t, int64(0), resp.Data.Stats.TotalSessions)
		assert.Equal(t, int64(0), resp.Data.Stats.KeywordsCount)
	})
}

// =============================================================================
// UpdateChatbotSettings
// =============================================================================

func TestApp_UpdateChatbotSettings(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		enabled := true
		greeting := "Welcome to our shop!"
		timeout := 60

		req := testutil.NewJSONRequest(t, map[string]any{
			"enabled":                 enabled,
			"greeting_message":        greeting,
			"session_timeout_minutes": timeout,
			"fallback_message":        "Sorry, I did not understand that.",
			"ai_enabled":              true,
			"ai_provider":             "openai",
			"ai_model":                "gpt-4",
			"ai_max_tokens":           1000,
			"ai_system_prompt":        "You are a helpful assistant.",
			"sla_enabled":             true,
			"sla_response_minutes":    10,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.UpdateChatbotSettings(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Settings updated successfully", resp.Data.Message)

		// Verify settings were persisted by reading them back
		getReq := testutil.NewGETRequest(t)
		testutil.SetAuthContext(getReq, org.ID, user.ID)

		err = app.GetChatbotSettings(getReq)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(getReq))

		var getResp struct {
			Data struct {
				Settings handlers.ChatbotSettingsResponse `json:"settings"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(getReq), &getResp)
		require.NoError(t, err)

		assert.True(t, getResp.Data.Settings.Enabled)
		assert.Equal(t, greeting, getResp.Data.Settings.GreetingMessage)
		assert.Equal(t, timeout, getResp.Data.Settings.SessionTimeoutMinutes)
		assert.Equal(t, "Sorry, I did not understand that.", getResp.Data.Settings.FallbackMessage)
		assert.True(t, getResp.Data.Settings.AIEnabled)
		assert.Equal(t, models.AIProvider("openai"), getResp.Data.Settings.AIProvider)
		assert.Equal(t, "gpt-4", getResp.Data.Settings.AIModel)
		assert.Equal(t, 1000, getResp.Data.Settings.AIMaxTokens)
		assert.True(t, getResp.Data.Settings.SLAEnabled)
		assert.Equal(t, 10, getResp.Data.Settings.SLAResponseMinutes)
	})
}

// =============================================================================
// ListKeywordRules
// =============================================================================

func TestApp_ListKeywordRules(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		createTestKeywordRule(t, app, org.ID, "Greeting Rule", []string{"hello", "hi"})
		createTestKeywordRule(t, app, org.ID, "Help Rule", []string{"help", "support"})

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListKeywordRules(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Rules []handlers.KeywordRuleResponse `json:"rules"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Rules, 2)
	})

	t.Run("empty list", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListKeywordRules(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Rules []handlers.KeywordRuleResponse `json:"rules"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Rules, 0)
	})
}

// =============================================================================
// CreateKeywordRule
// =============================================================================

func TestApp_CreateKeywordRule(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":          "Greeting Rule",
			"keywords":      []string{"hello", "hi", "hey"},
			"match_type":    "contains",
			"response_type": "text",
			"response_content": map[string]any{
				"text": "Hello! How can I help you?",
			},
			"priority": 20,
			"enabled":  true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID      string `json:"id"`
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Data.ID)
		assert.Equal(t, "Keyword rule created successfully", resp.Data.Message)

		// Verify it was persisted
		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var rule models.KeywordRule
		require.NoError(t, app.DB.First(&rule, "id = ?", parsedID).Error)
		assert.Equal(t, "Greeting Rule", rule.Name)
		assert.Equal(t, models.MatchTypeContains, rule.MatchType)
		assert.Equal(t, 20, rule.Priority)
		assert.True(t, rule.IsEnabled)
	})

	t.Run("validation error missing keywords", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":          "Bad Rule",
			"keywords":      []string{},
			"response_type": "text",
			"response_content": map[string]any{
				"text": "test",
			},
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("defaults name to first keyword when name empty", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"keywords":      []string{"pricing"},
			"response_type": "text",
			"response_content": map[string]any{
				"text": "Check our website for pricing.",
			},
			"enabled": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var rule models.KeywordRule
		require.NoError(t, app.DB.First(&rule, "id = ?", parsedID).Error)
		assert.Equal(t, "pricing", rule.Name)
	})
}

// =============================================================================
// GetKeywordRule
// =============================================================================

func TestApp_GetKeywordRule(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		rule := createTestKeywordRule(t, app, org.ID, "Greeting", []string{"hello"})

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", rule.ID.String())

		err := app.GetKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.KeywordRuleResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, rule.ID.String(), resp.Data.ID)
		assert.Equal(t, "Greeting", resp.Data.Name)
		assert.Equal(t, []string{"hello"}, resp.Data.Keywords)
		assert.Equal(t, models.MatchTypeContains, resp.Data.MatchType)
		assert.True(t, resp.Data.Enabled)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.GetKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// =============================================================================
// UpdateKeywordRule
// =============================================================================

func TestApp_UpdateKeywordRule(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		rule := createTestKeywordRule(t, app, org.ID, "Original", []string{"hello"})

		updatedName := "Updated Greeting"
		updatedPriority := 50
		disabled := false

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":     updatedName,
			"keywords": []string{"hello", "hi", "hey"},
			"priority": updatedPriority,
			"enabled":  disabled,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", rule.ID.String())

		err := app.UpdateKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Keyword rule updated successfully", resp.Data.Message)

		// Verify the update persisted
		var updated models.KeywordRule
		require.NoError(t, app.DB.First(&updated, "id = ?", rule.ID).Error)
		assert.Equal(t, updatedName, updated.Name)
		assert.Equal(t, updatedPriority, updated.Priority)
		assert.False(t, updated.IsEnabled)
		assert.Len(t, updated.Keywords, 3)
	})
}

// =============================================================================
// DeleteKeywordRule
// =============================================================================

func TestApp_DeleteKeywordRule(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		rule := createTestKeywordRule(t, app, org.ID, "To Delete", []string{"delete"})

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", rule.ID.String())

		err := app.DeleteKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Keyword rule deleted successfully", resp.Data.Message)

		// Verify it was soft-deleted
		var count int64
		app.DB.Model(&models.KeywordRule{}).Where("id = ?", rule.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.DeleteKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// =============================================================================
// ListChatbotFlows
// =============================================================================

func TestApp_ListChatbotFlows(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("list-flows")),
			testutil.WithRoleID(&role.ID),
		)

		createTestChatbotFlow(t, app, org.ID, "Welcome Flow")
		createTestChatbotFlow(t, app, org.ID, "Support Flow")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListChatbotFlows(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Flows []handlers.ChatbotFlowResponse `json:"flows"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Flows, 2)
	})

	t.Run("empty list", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("list-flows-empty")),
			testutil.WithRoleID(&role.ID),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListChatbotFlows(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Flows []handlers.ChatbotFlowResponse `json:"flows"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Flows, 0)
	})
}

// =============================================================================
// CreateChatbotFlow
// =============================================================================

func TestApp_CreateChatbotFlow(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("create-flow")),
			testutil.WithRoleID(&role.ID),
		)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":               "Onboarding Flow",
			"description":        "Collects user info",
			"trigger_keywords":   []string{"onboard", "start"},
			"initial_message":    "Welcome! Let us get you set up.",
			"completion_message": "All done! Thank you.",
			"on_complete_action": "none",
			"enabled":            true,
			"steps": []map[string]any{
				{
					"step_name":  "ask_name",
					"step_order": 1,
					"message":    "What is your name?",
					"input_type": "text",
					"store_as":   "customer_name",
					"next_step":  "ask_email",
				},
				{
					"step_name":        "ask_email",
					"step_order":       2,
					"message":          "What is your email?",
					"input_type":       "email",
					"store_as":         "customer_email",
					"validation_regex": `^[^\s@]+@[^\s@]+\.[^\s@]+$`,
					"validation_error": "Please enter a valid email.",
				},
			},
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID      string `json:"id"`
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Data.ID)
		assert.Equal(t, "Flow created successfully", resp.Data.Message)

		// Verify flow and steps persisted
		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var flow models.ChatbotFlow
		require.NoError(t, app.DB.Preload("Steps").First(&flow, "id = ?", parsedID).Error)
		assert.Equal(t, "Onboarding Flow", flow.Name)
		assert.True(t, flow.IsEnabled)
		assert.Len(t, flow.Steps, 2)
		assert.Equal(t, "ask_name", flow.Steps[0].StepName)
	})
}

// =============================================================================
// GetChatbotFlow
// =============================================================================

func TestApp_GetChatbotFlow(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("get-flow")),
			testutil.WithRoleID(&role.ID),
		)
		flow := createTestChatbotFlow(t, app, org.ID, "My Flow")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", flow.ID.String())

		err := app.GetChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data models.ChatbotFlow `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, flow.ID, resp.Data.ID)
		assert.Equal(t, "My Flow", resp.Data.Name)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("get-flow-nf")),
			testutil.WithRoleID(&role.ID),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.GetChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// =============================================================================
// UpdateChatbotFlow
// =============================================================================

func TestApp_UpdateChatbotFlow(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("update-flow")),
			testutil.WithRoleID(&role.ID),
		)
		flow := createTestChatbotFlow(t, app, org.ID, "Original Flow")

		updatedName := "Renamed Flow"
		updatedDesc := "Updated description"
		disabled := false

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":        updatedName,
			"description": updatedDesc,
			"enabled":     disabled,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", flow.ID.String())

		err := app.UpdateChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Flow updated successfully", resp.Data.Message)

		// Verify update persisted
		var updated models.ChatbotFlow
		require.NoError(t, app.DB.First(&updated, "id = ?", flow.ID).Error)
		assert.Equal(t, updatedName, updated.Name)
		assert.Equal(t, updatedDesc, updated.Description)
		assert.False(t, updated.IsEnabled)
	})
}

// =============================================================================
// DeleteChatbotFlow
// =============================================================================

func TestApp_DeleteChatbotFlow(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("delete-flow")),
			testutil.WithRoleID(&role.ID),
		)
		flow := createTestChatbotFlow(t, app, org.ID, "To Delete Flow")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", flow.ID.String())

		err := app.DeleteChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Flow deleted successfully", resp.Data.Message)

		// Verify soft-deleted
		var count int64
		app.DB.Model(&models.ChatbotFlow{}).Where("id = ?", flow.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("delete-flow-nf")),
			testutil.WithRoleID(&role.ID),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.DeleteChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// =============================================================================
// ListAIContexts
// =============================================================================

func TestApp_ListAIContexts(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		createTestAIContext(t, app, org.ID, "FAQ Context")
		createTestAIContext(t, app, org.ID, "Product Context")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListAIContexts(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Contexts []handlers.AIContextResponse `json:"contexts"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Contexts, 2)
	})

	t.Run("empty list", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListAIContexts(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Contexts []handlers.AIContextResponse `json:"contexts"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Contexts, 0)
	})
}

// =============================================================================
// CreateAIContext
// =============================================================================

func TestApp_CreateAIContext(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":             "Product FAQ",
			"context_type":     "static",
			"trigger_keywords": []string{"product", "pricing"},
			"static_content":   "Our product costs $99/month. We offer a free trial.",
			"priority":         20,
			"enabled":          true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID      string `json:"id"`
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Data.ID)
		assert.Equal(t, "AI context created successfully", resp.Data.Message)

		// Verify persisted
		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var ctx models.AIContext
		require.NoError(t, app.DB.First(&ctx, "id = ?", parsedID).Error)
		assert.Equal(t, "Product FAQ", ctx.Name)
		assert.Equal(t, models.ContextTypeStatic, ctx.ContextType)
		assert.Equal(t, 20, ctx.Priority)
		assert.True(t, ctx.IsEnabled)
	})
}

// =============================================================================
// GetAIContext
// =============================================================================

func TestApp_GetAIContext(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		ctx := createTestAIContext(t, app, org.ID, "FAQ Context")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", ctx.ID.String())

		err := app.GetAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data models.AIContext `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, ctx.ID, resp.Data.ID)
		assert.Equal(t, "FAQ Context", resp.Data.Name)
		assert.Equal(t, models.ContextTypeStatic, resp.Data.ContextType)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.GetAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// =============================================================================
// DeleteAIContext
// =============================================================================

func TestApp_DeleteAIContext(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		ctx := createTestAIContext(t, app, org.ID, "To Delete Context")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", ctx.ID.String())

		err := app.DeleteAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "AI context deleted successfully", resp.Data.Message)

		// Verify soft-deleted
		var count int64
		app.DB.Model(&models.AIContext{}).Where("id = ?", ctx.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.DeleteAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// =============================================================================
// GetChatbotSettings — additional coverage
// =============================================================================

func TestApp_GetChatbotSettings_ExistingSettings(t *testing.T) {
	t.Parallel()

	t.Run("returns persisted settings when they exist", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		// Create settings directly in the DB
		settings := &models.ChatbotSettings{
			BaseModel:      models.BaseModel{ID: uuid.New()},
			OrganizationID: org.ID,
			IsEnabled:      true,
			DefaultResponse: "Custom greeting!",
			FallbackMessage: "I do not understand.",
			SessionTimeoutMins: 45,
			AI: models.AIConfig{
				Enabled:  true,
				Provider: models.AIProviderOpenAI,
				Model:    "gpt-4o",
			},
		}
		require.NoError(t, app.DB.Create(settings).Error)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.GetChatbotSettings(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Settings handlers.ChatbotSettingsResponse `json:"settings"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.True(t, resp.Data.Settings.Enabled)
		assert.Equal(t, "Custom greeting!", resp.Data.Settings.GreetingMessage)
		assert.Equal(t, "I do not understand.", resp.Data.Settings.FallbackMessage)
		assert.Equal(t, 45, resp.Data.Settings.SessionTimeoutMinutes)
		assert.True(t, resp.Data.Settings.AIEnabled)
		assert.Equal(t, models.AIProviderOpenAI, resp.Data.Settings.AIProvider)
		assert.Equal(t, "gpt-4o", resp.Data.Settings.AIModel)
	})

	t.Run("stats reflect actual data counts", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		// Create some keyword rules and flows so stats are non-zero
		createTestKeywordRule(t, app, org.ID, "Rule A", []string{"hi"})
		createTestKeywordRule(t, app, org.ID, "Rule B", []string{"bye"})
		createTestChatbotFlow(t, app, org.ID, "Flow A")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.GetChatbotSettings(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Stats handlers.ChatbotStatsResponse `json:"stats"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, int64(2), resp.Data.Stats.KeywordsCount)
		assert.Equal(t, int64(1), resp.Data.Stats.FlowsCount)
	})
}

// =============================================================================
// UpdateChatbotSettings — additional coverage
// =============================================================================

func TestApp_UpdateChatbotSettings_PartialUpdate(t *testing.T) {
	t.Parallel()

	t.Run("partial update only changes provided fields", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		// First, create full settings
		setupReq := testutil.NewJSONRequest(t, map[string]any{
			"enabled":                 true,
			"greeting_message":        "Hello!",
			"session_timeout_minutes": 60,
			"fallback_message":        "Sorry, I did not get that.",
		})
		testutil.SetAuthContext(setupReq, org.ID, user.ID)
		err := app.UpdateChatbotSettings(setupReq)
		require.NoError(t, err)

		// Now update only the greeting message
		updateReq := testutil.NewJSONRequest(t, map[string]any{
			"greeting_message": "Welcome!",
		})
		testutil.SetAuthContext(updateReq, org.ID, user.ID)
		err = app.UpdateChatbotSettings(updateReq)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(updateReq))

		// Verify: greeting changed, other fields preserved
		getReq := testutil.NewGETRequest(t)
		testutil.SetAuthContext(getReq, org.ID, user.ID)
		err = app.GetChatbotSettings(getReq)
		require.NoError(t, err)

		var resp struct {
			Data struct {
				Settings handlers.ChatbotSettingsResponse `json:"settings"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(getReq), &resp)
		require.NoError(t, err)

		assert.True(t, resp.Data.Settings.Enabled)
		assert.Equal(t, "Welcome!", resp.Data.Settings.GreetingMessage)
		assert.Equal(t, 60, resp.Data.Settings.SessionTimeoutMinutes)
		assert.Equal(t, "Sorry, I did not get that.", resp.Data.Settings.FallbackMessage)
	})

	t.Run("update business hours settings", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"business_hours_enabled":        true,
			"out_of_hours_message":          "We are closed right now.",
			"allow_automated_outside_hours": false,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.UpdateChatbotSettings(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		// Read back
		getReq := testutil.NewGETRequest(t)
		testutil.SetAuthContext(getReq, org.ID, user.ID)
		err = app.GetChatbotSettings(getReq)
		require.NoError(t, err)

		var resp struct {
			Data struct {
				Settings handlers.ChatbotSettingsResponse `json:"settings"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(getReq), &resp)
		require.NoError(t, err)

		assert.True(t, resp.Data.Settings.BusinessHoursEnabled)
		assert.Equal(t, "We are closed right now.", resp.Data.Settings.OutOfHoursMessage)
		assert.False(t, resp.Data.Settings.AllowAutomatedOutsideHours)
	})

	t.Run("update agent assignment settings", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"allow_agent_queue_pickup":        false,
			"assign_to_same_agent":            true,
			"agent_current_conversation_only": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.UpdateChatbotSettings(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		getReq := testutil.NewGETRequest(t)
		testutil.SetAuthContext(getReq, org.ID, user.ID)
		err = app.GetChatbotSettings(getReq)
		require.NoError(t, err)

		var resp struct {
			Data struct {
				Settings handlers.ChatbotSettingsResponse `json:"settings"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(getReq), &resp)
		require.NoError(t, err)

		assert.False(t, resp.Data.Settings.AllowAgentQueuePickup)
		assert.True(t, resp.Data.Settings.AssignToSameAgent)
		assert.True(t, resp.Data.Settings.AgentCurrentConversationOnly)
	})

	t.Run("update client inactivity settings", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"client_reminder_enabled":   true,
			"client_reminder_minutes":   15,
			"client_reminder_message":   "Are you still there?",
			"client_auto_close_minutes": 30,
			"client_auto_close_message": "Session closed due to inactivity.",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.UpdateChatbotSettings(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		getReq := testutil.NewGETRequest(t)
		testutil.SetAuthContext(getReq, org.ID, user.ID)
		err = app.GetChatbotSettings(getReq)
		require.NoError(t, err)

		var resp struct {
			Data struct {
				Settings handlers.ChatbotSettingsResponse `json:"settings"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(getReq), &resp)
		require.NoError(t, err)

		assert.True(t, resp.Data.Settings.ClientReminderEnabled)
		assert.Equal(t, 15, resp.Data.Settings.ClientReminderMinutes)
		assert.Equal(t, "Are you still there?", resp.Data.Settings.ClientReminderMessage)
		assert.Equal(t, 30, resp.Data.Settings.ClientAutoCloseMinutes)
		assert.Equal(t, "Session closed due to inactivity.", resp.Data.Settings.ClientAutoCloseMessage)
	})

	t.Run("update SLA settings", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"sla_enabled":           true,
			"sla_response_minutes":  5,
			"sla_resolution_minutes": 120,
			"sla_escalation_minutes": 45,
			"sla_auto_close_hours":  48,
			"sla_auto_close_message": "Conversation auto-closed.",
			"sla_warning_message":   "SLA warning: response time exceeded.",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.UpdateChatbotSettings(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		getReq := testutil.NewGETRequest(t)
		testutil.SetAuthContext(getReq, org.ID, user.ID)
		err = app.GetChatbotSettings(getReq)
		require.NoError(t, err)

		var resp struct {
			Data struct {
				Settings handlers.ChatbotSettingsResponse `json:"settings"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(getReq), &resp)
		require.NoError(t, err)

		assert.True(t, resp.Data.Settings.SLAEnabled)
		assert.Equal(t, 5, resp.Data.Settings.SLAResponseMinutes)
		assert.Equal(t, 120, resp.Data.Settings.SLAResolutionMinutes)
		assert.Equal(t, 45, resp.Data.Settings.SLAEscalationMinutes)
		assert.Equal(t, 48, resp.Data.Settings.SLAAutoCloseHours)
		assert.Equal(t, "Conversation auto-closed.", resp.Data.Settings.SLAAutoCloseMessage)
		assert.Equal(t, "SLA warning: response time exceeded.", resp.Data.Settings.SLAWarningMessage)
	})

	t.Run("invalid JSON body returns 400", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetContentType("application/json")
		ctx.Request.Header.SetMethod("POST")
		ctx.Request.SetBody([]byte(`{invalid json`))
		req := &fastglue.Request{RequestCtx: ctx}
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.UpdateChatbotSettings(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})
}

// =============================================================================
// CreateKeywordRule — additional match types and response types
// =============================================================================

func TestApp_CreateKeywordRule_MatchTypes(t *testing.T) {
	t.Parallel()

	t.Run("exact match type", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":          "Exact Match Rule",
			"keywords":      []string{"STOP"},
			"match_type":    "exact",
			"response_type": "text",
			"response_content": map[string]any{
				"text": "You have been unsubscribed.",
			},
			"priority": 100,
			"enabled":  true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var rule models.KeywordRule
		require.NoError(t, app.DB.First(&rule, "id = ?", parsedID).Error)
		assert.Equal(t, models.MatchTypeExact, rule.MatchType)
		assert.Equal(t, 100, rule.Priority)
	})

	t.Run("starts_with match type", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":          "Prefix Rule",
			"keywords":      []string{"order#"},
			"match_type":    "starts_with",
			"response_type": "text",
			"response_content": map[string]any{
				"text": "Looking up your order...",
			},
			"enabled": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var rule models.KeywordRule
		require.NoError(t, app.DB.First(&rule, "id = ?", parsedID).Error)
		assert.Equal(t, models.MatchTypeStartsWith, rule.MatchType)
	})

	t.Run("regex match type", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":          "Regex Rule",
			"keywords":      []string{`^\d{5,6}$`},
			"match_type":    "regex",
			"response_type": "text",
			"response_content": map[string]any{
				"text": "Got your PIN code.",
			},
			"enabled": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var rule models.KeywordRule
		require.NoError(t, app.DB.First(&rule, "id = ?", parsedID).Error)
		assert.Equal(t, models.MatchTypeRegex, rule.MatchType)
	})

	t.Run("transfer response type", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":          "Transfer Rule",
			"keywords":      []string{"agent", "human"},
			"match_type":    "contains",
			"response_type": "transfer",
			"response_content": map[string]any{
				"text": "Connecting you with a live agent.",
			},
			"priority": 99,
			"enabled":  true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var rule models.KeywordRule
		require.NoError(t, app.DB.First(&rule, "id = ?", parsedID).Error)
		assert.Equal(t, models.ResponseTypeTransfer, rule.ResponseType)
		assert.Equal(t, 99, rule.Priority)
	})

	t.Run("defaults match_type and response_type when omitted", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":     "Default Types Rule",
			"keywords": []string{"test"},
			"response_content": map[string]any{
				"text": "default types test",
			},
			"enabled": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var rule models.KeywordRule
		require.NoError(t, app.DB.First(&rule, "id = ?", parsedID).Error)
		assert.Equal(t, models.MatchTypeContains, rule.MatchType)
		assert.Equal(t, models.ResponseTypeText, rule.ResponseType)
	})
}

// =============================================================================
// UpdateKeywordRule — additional coverage
// =============================================================================

func TestApp_UpdateKeywordRule_Additional(t *testing.T) {
	t.Parallel()

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name": "Ghost",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.UpdateKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("update match_type and response_content", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		rule := createTestKeywordRule(t, app, org.ID, "MatchUpdate", []string{"hello"})

		newMatchType := models.MatchTypeExact
		req := testutil.NewJSONRequest(t, map[string]any{
			"match_type": string(newMatchType),
			"response_content": map[string]any{
				"text": "Updated response content.",
			},
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", rule.ID.String())

		err := app.UpdateKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var updated models.KeywordRule
		require.NoError(t, app.DB.First(&updated, "id = ?", rule.ID).Error)
		assert.Equal(t, models.MatchTypeExact, updated.MatchType)
		assert.Equal(t, "Updated response content.", updated.ResponseContent["text"])
	})

	t.Run("update response_type to transfer", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		rule := createTestKeywordRule(t, app, org.ID, "TypeSwitch", []string{"agent"})

		newRespType := models.ResponseTypeTransfer
		req := testutil.NewJSONRequest(t, map[string]any{
			"response_type": string(newRespType),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", rule.ID.String())

		err := app.UpdateKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var updated models.KeywordRule
		require.NoError(t, app.DB.First(&updated, "id = ?", rule.ID).Error)
		assert.Equal(t, models.ResponseTypeTransfer, updated.ResponseType)
	})
}

// =============================================================================
// ListKeywordRules — cross-org isolation
// =============================================================================

func TestApp_ListKeywordRules_OrgIsolation(t *testing.T) {
	t.Parallel()

	t.Run("rules from other org are not visible", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
		createTestKeywordRule(t, app, org1.ID, "Org1 Rule", []string{"org1"})

		org2 := testutil.CreateTestOrganization(t, app.DB)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("org2-kw")),
		)
		createTestKeywordRule(t, app, org2.ID, "Org2 Rule", []string{"org2"})

		// User from org1 should only see org1's rules
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org1.ID, user1.ID)

		err := app.ListKeywordRules(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Rules []handlers.KeywordRuleResponse `json:"rules"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Rules, 1)
		assert.Equal(t, "Org1 Rule", resp.Data.Rules[0].Name)

		// User from org2 should only see org2's rules
		req2 := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req2, org2.ID, user2.ID)

		err = app.ListKeywordRules(req2)
		require.NoError(t, err)

		var resp2 struct {
			Data struct {
				Rules []handlers.KeywordRuleResponse `json:"rules"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req2), &resp2)
		require.NoError(t, err)
		assert.Len(t, resp2.Data.Rules, 1)
		assert.Equal(t, "Org2 Rule", resp2.Data.Rules[0].Name)
	})
}

// =============================================================================
// CreateChatbotFlow — additional coverage
// =============================================================================

func TestApp_CreateChatbotFlow_Additional(t *testing.T) {
	t.Parallel()

	t.Run("validation error missing name", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("create-flow-noname")),
			testutil.WithRoleID(&role.ID),
		)

		req := testutil.NewJSONRequest(t, map[string]any{
			"description":      "Missing name flow",
			"trigger_keywords": []string{"test"},
			"enabled":          true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("create flow without steps", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("create-flow-nosteps")),
			testutil.WithRoleID(&role.ID),
		)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":             "No Steps Flow",
			"description":      "A flow with no steps",
			"trigger_keywords": []string{"nostep"},
			"enabled":          true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var flow models.ChatbotFlow
		require.NoError(t, app.DB.Preload("Steps").First(&flow, "id = ?", parsedID).Error)
		assert.Equal(t, "No Steps Flow", flow.Name)
		assert.Len(t, flow.Steps, 0)
	})

	t.Run("create flow with completion config", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("create-flow-complete")),
			testutil.WithRoleID(&role.ID),
		)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":               "Webhook Flow",
			"trigger_keywords":   []string{"webhook"},
			"initial_message":    "Starting form...",
			"completion_message": "Thank you for submitting!",
			"on_complete_action": "webhook",
			"completion_config": map[string]any{
				"url":    "https://example.com/webhook",
				"method": "POST",
			},
			"enabled": true,
			"steps": []map[string]any{
				{
					"step_name":  "ask_info",
					"step_order": 1,
					"message":    "Please enter your info.",
					"input_type": "text",
					"store_as":   "user_info",
				},
			},
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var flow models.ChatbotFlow
		require.NoError(t, app.DB.First(&flow, "id = ?", parsedID).Error)
		assert.Equal(t, "webhook", flow.OnCompleteAction)
		assert.Equal(t, "Starting form...", flow.InitialMessage)
		assert.Equal(t, "Thank you for submitting!", flow.CompletionMessage)
	})
}

// =============================================================================
// UpdateChatbotFlow — additional coverage
// =============================================================================

func TestApp_UpdateChatbotFlow_Additional(t *testing.T) {
	t.Parallel()

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("update-flow-nf")),
			testutil.WithRoleID(&role.ID),
		)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name": "Ghost Flow",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.UpdateChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("update replaces steps when provided", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("update-flow-steps")),
			testutil.WithRoleID(&role.ID),
		)

		// Create flow with one step
		createReq := testutil.NewJSONRequest(t, map[string]any{
			"name":             "Steps Flow",
			"trigger_keywords": []string{"steps"},
			"enabled":          true,
			"steps": []map[string]any{
				{
					"step_name":  "old_step",
					"step_order": 1,
					"message":    "Old step",
					"input_type": "text",
					"store_as":   "old_data",
				},
			},
		})
		testutil.SetAuthContext(createReq, org.ID, user.ID)

		err := app.CreateChatbotFlow(createReq)
		require.NoError(t, err)

		var createResp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(createReq), &createResp)
		require.NoError(t, err)

		// Update: replace steps with two new ones
		updateReq := testutil.NewJSONRequest(t, map[string]any{
			"steps": []map[string]any{
				{
					"step_name":  "new_step_1",
					"step_order": 1,
					"message":    "First new step",
					"input_type": "text",
					"store_as":   "new_data_1",
				},
				{
					"step_name":  "new_step_2",
					"step_order": 2,
					"message":    "Second new step",
					"input_type": "email",
					"store_as":   "new_data_2",
				},
			},
		})
		testutil.SetAuthContext(updateReq, org.ID, user.ID)
		testutil.SetPathParam(updateReq, "id", createResp.Data.ID)

		err = app.UpdateChatbotFlow(updateReq)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(updateReq))

		// Verify old step is gone and new steps exist
		parsedID, err := uuid.Parse(createResp.Data.ID)
		require.NoError(t, err)

		var flow models.ChatbotFlow
		require.NoError(t, app.DB.Preload("Steps").First(&flow, "id = ?", parsedID).Error)
		assert.Len(t, flow.Steps, 2)

		// Steps should be ordered
		stepNames := make([]string, len(flow.Steps))
		for i, s := range flow.Steps {
			stepNames[i] = s.StepName
		}
		assert.Contains(t, stepNames, "new_step_1")
		assert.Contains(t, stepNames, "new_step_2")
		assert.NotContains(t, stepNames, "old_step")
	})

	t.Run("update trigger keywords", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role := testutil.CreateTestRole(t, app.DB, org.ID, "flow-admin", perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("update-flow-kw")),
			testutil.WithRoleID(&role.ID),
		)
		flow := createTestChatbotFlow(t, app, org.ID, "Keyword Flow")

		req := testutil.NewJSONRequest(t, map[string]any{
			"trigger_keywords": []string{"newkw1", "newkw2", "newkw3"},
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", flow.ID.String())

		err := app.UpdateChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var updated models.ChatbotFlow
		require.NoError(t, app.DB.First(&updated, "id = ?", flow.ID).Error)
		assert.Equal(t, models.StringArray{"newkw1", "newkw2", "newkw3"}, updated.TriggerKeywords)
	})
}

// =============================================================================
// ListChatbotFlows — cross-org isolation
// =============================================================================

func TestApp_ListChatbotFlows_OrgIsolation(t *testing.T) {
	t.Parallel()

	t.Run("flows from other org are not visible", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role1 := testutil.CreateTestRole(t, app.DB, org1.ID, "flow-admin", perms)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID,
			testutil.WithEmail(testutil.UniqueEmail("iso-flow1")),
			testutil.WithRoleID(&role1.ID),
		)
		createTestChatbotFlow(t, app, org1.ID, "Org1 Flow")

		org2 := testutil.CreateTestOrganization(t, app.DB)
		role2 := testutil.CreateTestRole(t, app.DB, org2.ID, "flow-admin", perms)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("iso-flow2")),
			testutil.WithRoleID(&role2.ID),
		)
		createTestChatbotFlow(t, app, org2.ID, "Org2 Flow")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org1.ID, user1.ID)

		err := app.ListChatbotFlows(req)
		require.NoError(t, err)

		var resp struct {
			Data struct {
				Flows []handlers.ChatbotFlowResponse `json:"flows"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Flows, 1)
		assert.Equal(t, "Org1 Flow", resp.Data.Flows[0].Name)

		// Org2 sees only its flow
		req2 := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req2, org2.ID, user2.ID)

		err = app.ListChatbotFlows(req2)
		require.NoError(t, err)

		var resp2 struct {
			Data struct {
				Flows []handlers.ChatbotFlowResponse `json:"flows"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req2), &resp2)
		require.NoError(t, err)
		assert.Len(t, resp2.Data.Flows, 1)
		assert.Equal(t, "Org2 Flow", resp2.Data.Flows[0].Name)
	})
}

// =============================================================================
// CreateAIContext — additional coverage
// =============================================================================

func TestApp_CreateAIContext_Additional(t *testing.T) {
	t.Parallel()

	t.Run("validation error missing name", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"context_type":     "static",
			"trigger_keywords": []string{"help"},
			"static_content":   "Some content",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("defaults context_type to static when omitted", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":             "Defaulted Context",
			"trigger_keywords": []string{"default"},
			"static_content":   "Default content.",
			"enabled":          true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var ctx models.AIContext
		require.NoError(t, app.DB.First(&ctx, "id = ?", parsedID).Error)
		assert.Equal(t, models.ContextTypeStatic, ctx.ContextType)
	})

	t.Run("create API context type", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":             "API Context",
			"context_type":     "api",
			"trigger_keywords": []string{"live_data"},
			"priority":         50,
			"enabled":          true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		parsedID, err := uuid.Parse(resp.Data.ID)
		require.NoError(t, err)

		var ctx models.AIContext
		require.NoError(t, app.DB.First(&ctx, "id = ?", parsedID).Error)
		assert.Equal(t, models.ContextTypeAPI, ctx.ContextType)
		assert.Equal(t, 50, ctx.Priority)
	})
}

// =============================================================================
// UpdateAIContext
// =============================================================================

func TestApp_UpdateAIContext(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		aiCtx := createTestAIContext(t, app, org.ID, "Original Context")

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":             "Updated Context",
			"static_content":   "Updated content for the context.",
			"priority":         99,
			"trigger_keywords": []string{"updated", "new"},
			"enabled":          false,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", aiCtx.ID.String())

		err := app.UpdateAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "AI context updated successfully", resp.Data.Message)

		// Verify persisted
		var updated models.AIContext
		require.NoError(t, app.DB.First(&updated, "id = ?", aiCtx.ID).Error)
		assert.Equal(t, "Updated Context", updated.Name)
		assert.Equal(t, "Updated content for the context.", updated.StaticContent)
		assert.Equal(t, 99, updated.Priority)
		assert.False(t, updated.IsEnabled)
		assert.Equal(t, models.StringArray{"updated", "new"}, updated.TriggerKeywords)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name": "Ghost Context",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.UpdateAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("partial update only changes provided fields", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		aiCtx := createTestAIContext(t, app, org.ID, "Partial Update Ctx")

		// Only update the name
		req := testutil.NewJSONRequest(t, map[string]any{
			"name": "Renamed Context",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", aiCtx.ID.String())

		err := app.UpdateAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var updated models.AIContext
		require.NoError(t, app.DB.First(&updated, "id = ?", aiCtx.ID).Error)
		assert.Equal(t, "Renamed Context", updated.Name)
		// Original fields should be preserved
		assert.Equal(t, models.ContextTypeStatic, updated.ContextType)
		assert.Equal(t, "Our business hours are 9-5.", updated.StaticContent)
		assert.Equal(t, 10, updated.Priority)
		assert.True(t, updated.IsEnabled)
	})

	t.Run("change context_type from static to api", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		aiCtx := createTestAIContext(t, app, org.ID, "Type Change Ctx")

		req := testutil.NewJSONRequest(t, map[string]any{
			"context_type": "api",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", aiCtx.ID.String())

		err := app.UpdateAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var updated models.AIContext
		require.NoError(t, app.DB.First(&updated, "id = ?", aiCtx.ID).Error)
		assert.Equal(t, models.ContextTypeAPI, updated.ContextType)
	})
}

// =============================================================================
// ListAIContexts — cross-org isolation
// =============================================================================

func TestApp_ListAIContexts_OrgIsolation(t *testing.T) {
	t.Parallel()

	t.Run("contexts from other org are not visible", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
		createTestAIContext(t, app, org1.ID, "Org1 Context")

		org2 := testutil.CreateTestOrganization(t, app.DB)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("org2-ai")),
		)
		createTestAIContext(t, app, org2.ID, "Org2 Context")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org1.ID, user1.ID)

		err := app.ListAIContexts(req)
		require.NoError(t, err)

		var resp struct {
			Data struct {
				Contexts []handlers.AIContextResponse `json:"contexts"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Contexts, 1)
		assert.Equal(t, "Org1 Context", resp.Data.Contexts[0].Name)

		req2 := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req2, org2.ID, user2.ID)

		err = app.ListAIContexts(req2)
		require.NoError(t, err)

		var resp2 struct {
			Data struct {
				Contexts []handlers.AIContextResponse `json:"contexts"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req2), &resp2)
		require.NoError(t, err)
		assert.Len(t, resp2.Data.Contexts, 1)
		assert.Equal(t, "Org2 Context", resp2.Data.Contexts[0].Name)
	})
}

// =============================================================================
// ListChatbotSessions
// =============================================================================

// createSessionForChatbotTest creates a chatbot session directly in the DB for testing.
func createSessionForChatbotTest(t *testing.T, app *handlers.App, orgID, contactID uuid.UUID, phone string, status models.SessionStatus) *models.ChatbotSession {
	t.Helper()

	now := time.Now()
	session := &models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		ContactID:       contactID,
		PhoneNumber:     phone,
		Status:          status,
		SessionData:     models.JSONB{},
		StartedAt:       now,
		LastActivityAt:  now,
	}
	require.NoError(t, app.DB.Create(session).Error)
	return session
}

func TestApp_ListChatbotSessions(t *testing.T) {
	t.Parallel()

	t.Run("success returns all sessions", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		createSessionForChatbotTest(t, app, org.ID, contact.ID, "+1234567890", models.SessionStatusActive)
		createSessionForChatbotTest(t, app, org.ID, contact.ID, "+1234567890", models.SessionStatusCompleted)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListChatbotSessions(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Sessions []models.ChatbotSession `json:"sessions"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Sessions, 2)
	})

	t.Run("empty list", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListChatbotSessions(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Sessions []models.ChatbotSession `json:"sessions"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Sessions, 0)
	})

	t.Run("filter by status active", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		createSessionForChatbotTest(t, app, org.ID, contact.ID, "+1111111111", models.SessionStatusActive)
		createSessionForChatbotTest(t, app, org.ID, contact.ID, "+1111111111", models.SessionStatusCompleted)
		createSessionForChatbotTest(t, app, org.ID, contact.ID, "+1111111111", models.SessionStatusActive)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetQueryParam(req, "status", "active")

		err := app.ListChatbotSessions(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Sessions []models.ChatbotSession `json:"sessions"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Sessions, 2)
		for _, s := range resp.Data.Sessions {
			assert.Equal(t, models.SessionStatusActive, s.Status)
		}
	})

	t.Run("filter by status completed", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		createSessionForChatbotTest(t, app, org.ID, contact.ID, "+2222222222", models.SessionStatusActive)
		createSessionForChatbotTest(t, app, org.ID, contact.ID, "+2222222222", models.SessionStatusCompleted)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetQueryParam(req, "status", "completed")

		err := app.ListChatbotSessions(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Sessions []models.ChatbotSession `json:"sessions"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Sessions, 1)
		assert.Equal(t, models.SessionStatusCompleted, resp.Data.Sessions[0].Status)
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
		contact1 := testutil.CreateTestContact(t, app.DB, org1.ID)
		createSessionForChatbotTest(t, app, org1.ID, contact1.ID, "+3333333333", models.SessionStatusActive)

		org2 := testutil.CreateTestOrganization(t, app.DB)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("org2-sess")),
		)
		contact2 := testutil.CreateTestContact(t, app.DB, org2.ID)
		createSessionForChatbotTest(t, app, org2.ID, contact2.ID, "+4444444444", models.SessionStatusActive)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org1.ID, user1.ID)

		err := app.ListChatbotSessions(req)
		require.NoError(t, err)

		var resp struct {
			Data struct {
				Sessions []models.ChatbotSession `json:"sessions"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Sessions, 1)

		req2 := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req2, org2.ID, user2.ID)

		err = app.ListChatbotSessions(req2)
		require.NoError(t, err)

		var resp2 struct {
			Data struct {
				Sessions []models.ChatbotSession `json:"sessions"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req2), &resp2)
		require.NoError(t, err)
		assert.Len(t, resp2.Data.Sessions, 1)
	})
}

// =============================================================================
// GetChatbotSession
// =============================================================================

func TestApp_GetChatbotSession(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		session := createSessionForChatbotTest(t, app, org.ID, contact.ID, "+5555555555", models.SessionStatusActive)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", session.ID.String())

		err := app.GetChatbotSession(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data models.ChatbotSession `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, session.ID, resp.Data.ID)
		assert.Equal(t, models.SessionStatusActive, resp.Data.Status)
		assert.Equal(t, "+5555555555", resp.Data.PhoneNumber)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.GetChatbotSession(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("session with messages", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		session := createSessionForChatbotTest(t, app, org.ID, contact.ID, "+6666666666", models.SessionStatusActive)

		// Add messages to the session
		msg1 := &models.ChatbotSessionMessage{
			BaseModel: models.BaseModel{ID: uuid.New()},
			SessionID: session.ID,
			Direction: models.DirectionIncoming,
			Message:   "Hello",
			StepName:  "greeting",
		}
		msg2 := &models.ChatbotSessionMessage{
			BaseModel: models.BaseModel{ID: uuid.New()},
			SessionID: session.ID,
			Direction: models.DirectionOutgoing,
			Message:   "Hi! How can I help you?",
			StepName:  "greeting",
		}
		require.NoError(t, app.DB.Create(msg1).Error)
		require.NoError(t, app.DB.Create(msg2).Error)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", session.ID.String())

		err := app.GetChatbotSession(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data models.ChatbotSession `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, session.ID, resp.Data.ID)
		assert.Len(t, resp.Data.Messages, 2)
	})

	t.Run("cross-org isolation prevents access", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		contact1 := testutil.CreateTestContact(t, app.DB, org1.ID)
		session := createSessionForChatbotTest(t, app, org1.ID, contact1.ID, "+7777777777", models.SessionStatusActive)

		org2 := testutil.CreateTestOrganization(t, app.DB)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("org2-getsess")),
		)

		// User from org2 tries to get org1's session
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", session.ID.String())

		err := app.GetChatbotSession(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// =============================================================================
// DeleteKeywordRule — cross-org isolation
// =============================================================================

func TestApp_DeleteKeywordRule_CrossOrg(t *testing.T) {
	t.Parallel()

	t.Run("cannot delete rule from another org", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		rule := createTestKeywordRule(t, app, org1.ID, "Org1 Rule", []string{"org1"})

		org2 := testutil.CreateTestOrganization(t, app.DB)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("org2-delkw")),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", rule.ID.String())

		err := app.DeleteKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

		// Verify the rule still exists
		var count int64
		app.DB.Model(&models.KeywordRule{}).Where("id = ?", rule.ID).Count(&count)
		assert.Equal(t, int64(1), count)
	})
}

// =============================================================================
// DeleteChatbotFlow — cross-org isolation
// =============================================================================

func TestApp_DeleteChatbotFlow_CrossOrg(t *testing.T) {
	t.Parallel()

	t.Run("cannot delete flow from another org", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		flow := createTestChatbotFlow(t, app, org1.ID, "Org1 Flow")

		org2 := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role2 := testutil.CreateTestRole(t, app.DB, org2.ID, "flow-admin", perms)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("org2-delflow")),
			testutil.WithRoleID(&role2.ID),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", flow.ID.String())

		err := app.DeleteChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

		// Verify the flow still exists
		var count int64
		app.DB.Model(&models.ChatbotFlow{}).Where("id = ?", flow.ID).Count(&count)
		assert.Equal(t, int64(1), count)
	})
}

// =============================================================================
// DeleteAIContext — cross-org isolation
// =============================================================================

func TestApp_DeleteAIContext_CrossOrg(t *testing.T) {
	t.Parallel()

	t.Run("cannot delete context from another org", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		aiCtx := createTestAIContext(t, app, org1.ID, "Org1 AI Context")

		org2 := testutil.CreateTestOrganization(t, app.DB)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("org2-delai")),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", aiCtx.ID.String())

		err := app.DeleteAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

		// Verify the context still exists
		var count int64
		app.DB.Model(&models.AIContext{}).Where("id = ?", aiCtx.ID).Count(&count)
		assert.Equal(t, int64(1), count)
	})
}

// =============================================================================
// GetKeywordRule — cross-org isolation
// =============================================================================

func TestApp_GetKeywordRule_CrossOrg(t *testing.T) {
	t.Parallel()

	t.Run("cannot get rule from another org", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		rule := createTestKeywordRule(t, app, org1.ID, "Org1 Rule", []string{"secret"})

		org2 := testutil.CreateTestOrganization(t, app.DB)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("org2-getkw")),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", rule.ID.String())

		err := app.GetKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// =============================================================================
// GetChatbotFlow — cross-org isolation
// =============================================================================

func TestApp_GetChatbotFlow_CrossOrg(t *testing.T) {
	t.Parallel()

	t.Run("cannot get flow from another org", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		flow := createTestChatbotFlow(t, app, org1.ID, "Secret Flow")

		org2 := testutil.CreateTestOrganization(t, app.DB)
		perms := getChatbotFlowPermissions(t, app)
		role2 := testutil.CreateTestRole(t, app.DB, org2.ID, "flow-admin", perms)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("org2-getflow")),
			testutil.WithRoleID(&role2.ID),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", flow.ID.String())

		err := app.GetChatbotFlow(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// =============================================================================
// GetAIContext — cross-org isolation
// =============================================================================

func TestApp_GetAIContext_CrossOrg(t *testing.T) {
	t.Parallel()

	t.Run("cannot get context from another org", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		aiCtx := createTestAIContext(t, app, org1.ID, "Org1 Secret Context")

		org2 := testutil.CreateTestOrganization(t, app.DB)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("org2-getai")),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", aiCtx.ID.String())

		err := app.GetAIContext(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// =============================================================================
// GetKeywordRule — response content validation
// =============================================================================

func TestApp_GetKeywordRule_ResponseFields(t *testing.T) {
	t.Parallel()

	t.Run("response includes all expected fields", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		rule := &models.KeywordRule{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			Name:            "Full Rule",
			Keywords:        []string{"keyword1", "keyword2"},
			MatchType:       models.MatchTypeStartsWith,
			ResponseType:    models.ResponseTypeTemplate,
			ResponseContent: models.JSONB{"template_name": "welcome_tpl", "lang": "en"},
			Priority:        42,
			IsEnabled:       true, // Create as enabled first
		}
		require.NoError(t, app.DB.Create(rule).Error)
		// Explicitly disable: GORM skips zero-value bools with default:true on INSERT.
		require.NoError(t, app.DB.Model(rule).Update("is_enabled", false).Error)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", rule.ID.String())

		err := app.GetKeywordRule(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.KeywordRuleResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, rule.ID.String(), resp.Data.ID)
		assert.Equal(t, "Full Rule", resp.Data.Name)
		assert.Equal(t, []string{"keyword1", "keyword2"}, resp.Data.Keywords)
		assert.Equal(t, models.MatchTypeStartsWith, resp.Data.MatchType)
		assert.Equal(t, models.ResponseTypeTemplate, resp.Data.ResponseType)
		assert.Equal(t, 42, resp.Data.Priority)
		assert.False(t, resp.Data.Enabled)
		assert.NotEmpty(t, resp.Data.CreatedAt)
	})
}
