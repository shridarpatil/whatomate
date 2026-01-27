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
