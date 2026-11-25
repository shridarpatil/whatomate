package handlers_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// Hold/resume act on a live call by call log ID; they need call_transfers:write
// and must not reach another org's call.
func TestApp_HoldResumeCall_Permissions(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	otherOrg := testutil.CreateTestOrganization(t, app.DB)
	own := createTestCallLog(t, app, org.ID)
	foreign := createTestCallLog(t, app, otherOrg.ID)

	noPerm := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "chat only", []string{"chat:read", "chat:write"})
	noPermUser := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&noPerm.ID))
	agent := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "call agent", []string{"call_transfers:read", "call_transfers:write"})
	agentUser := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agent.ID))

	handlersByName := map[string]func(*handlers.App) func(*fastglue.Request) error{
		"hold":   func(a *handlers.App) func(*fastglue.Request) error { return a.HoldCall },
		"resume": func(a *handlers.App) func(*fastglue.Request) error { return a.ResumeCall },
	}
	for name, h := range handlersByName {
		t.Run(name, func(t *testing.T) {
			call := func(userID, callLogID uuid.UUID) int {
				req := testutil.NewJSONRequest(t, nil)
				testutil.SetAuthContext(req, org.ID, userID)
				testutil.SetPathParam(req, "id", callLogID.String())
				require.NoError(t, h(app)(req))
				return testutil.GetResponseStatusCode(req)
			}
			assert.Equal(t, fasthttp.StatusForbidden, call(noPermUser.ID, own.ID))
			assert.Equal(t, fasthttp.StatusNotFound, call(agentUser.ID, foreign.ID))
			// Own org's call passes the checks; the test app has no call manager.
			assert.Equal(t, fasthttp.StatusServiceUnavailable, call(agentUser.ID, own.ID))
		})
	}
}
