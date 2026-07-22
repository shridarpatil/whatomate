package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/logf"
	"gorm.io/gorm"
)

// fakeAnalyticsServer simulates Meta's analytics endpoint for tests.
type fakeAnalyticsServer struct {
	server *httptest.Server
	hits   int64
}

func newFakeAnalyticsServer(t *testing.T, response string) *fakeAnalyticsServer {
	t.Helper()
	f := &fakeAnalyticsServer{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&f.hits, 1)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeAnalyticsServer) Hits() int64 { return atomic.LoadInt64(&f.hits) }

func newAppForMetaAnalytics(t *testing.T, fakeURL string) *handlers.App {
	t.Helper()
	app := newTestApp(t)
	app.WhatsApp = whatsapp.NewWithBaseURL(logf.New(logf.Opts{Level: logf.ErrorLevel}), fakeURL)
	app.Config = &config.Config{
		JWT:      config.JWTConfig{Secret: testutil.TestJWTSecret, AccessExpiryMins: 15, RefreshExpiryDays: 7},
		WhatsApp: config.WhatsAppConfig{BaseURL: fakeURL, APIVersion: "v18.0"},
	}
	return app
}

func mkAnalyticsAccount(t *testing.T, db *gorm.DB, orgID uuid.UUID) *models.WhatsAppAccount {
	t.Helper()
	acc := &models.WhatsAppAccount{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     orgID,
		Name:               "ma-acc-" + uuid.New().String()[:6],
		PhoneID:            "phone-" + uuid.New().String()[:8],
		BusinessID:         "biz-" + uuid.New().String()[:8],
		AccessToken:        "tok",
		WebhookVerifyToken: "vt-" + uuid.New().String()[:8],
		APIVersion:         "v18.0",
		Status:             "active",
	}
	require.NoError(t, db.Create(acc).Error)
	return acc
}

func metaAnalyticsRole(t *testing.T, db *gorm.DB, orgID uuid.UUID) *models.CustomRole {
	t.Helper()
	return testutil.CreateTestRoleWithKeys(t, db, orgID, "meta-analytics", []string{"analytics:read", "analytics:write"})
}

// dateRange returns last week as YYYY-MM-DD strings, suitable for the handler.
func dateRange() (string, string) {
	now := time.Now()
	return now.AddDate(0, 0, -7).Format("2006-01-02"), now.Format("2006-01-02")
}

// --- GetMetaAnalytics ---

func TestApp_GetMetaAnalytics_RequiresAnalyticsRead(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "no-analytics", false, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "analytics_type", "analytics")

	require.NoError(t, app.GetMetaAnalytics(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_GetMetaAnalytics_MissingAnalyticsType(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := metaAnalyticsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	// No analytics_type query param.

	require.NoError(t, app.GetMetaAnalytics(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "analytics_type is required")
}

func TestApp_GetMetaAnalytics_InvalidAnalyticsType(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := metaAnalyticsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "analytics_type", "made_up")

	require.NoError(t, app.GetMetaAnalytics(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid analytics_type")
}

func TestApp_GetMetaAnalytics_MissingDates(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := metaAnalyticsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "analytics_type", "analytics")
	// No start/end.

	require.NoError(t, app.GetMetaAnalytics(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "start and end dates are required")
}

func TestApp_GetMetaAnalytics_EndBeforeStartRejected(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := metaAnalyticsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "analytics_type", "analytics")
	testutil.SetQueryParam(req, "start", "2024-12-31")
	testutil.SetQueryParam(req, "end", "2024-01-01")

	require.NoError(t, app.GetMetaAnalytics(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "End date must be after start date")
}

func TestApp_GetMetaAnalytics_TemplateAnalyticsBeyond90DaysRejected(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := metaAnalyticsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	from := time.Now().AddDate(0, 0, -120).Format("2006-01-02") // 120 days ago > 90-day limit
	to := time.Now().AddDate(0, 0, -100).Format("2006-01-02")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "analytics_type", "template_analytics")
	testutil.SetQueryParam(req, "start", from)
	testutil.SetQueryParam(req, "end", to)

	require.NoError(t, app.GetMetaAnalytics(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "90-day lookback")
}

func TestApp_GetMetaAnalytics_NoAccountsReturnsEmptyList(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := metaAnalyticsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	from, to := dateRange()
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "analytics_type", "analytics")
	testutil.SetQueryParam(req, "start", from)
	testutil.SetQueryParam(req, "end", to)

	require.NoError(t, app.GetMetaAnalytics(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Accounts []map[string]any `json:"accounts"`
			Message  string           `json:"message"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Empty(t, resp.Data.Accounts)
	assert.Contains(t, resp.Data.Message, "No WhatsApp accounts")
	assert.Equal(t, int64(0), srv.Hits(), "Meta must not be called when no accounts exist")
}

func TestApp_GetMetaAnalytics_SpecificAccountNotFound(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := metaAnalyticsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	from, to := dateRange()
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "analytics_type", "analytics")
	testutil.SetQueryParam(req, "start", from)
	testutil.SetQueryParam(req, "end", to)
	testutil.SetQueryParam(req, "account_id", uuid.New().String())

	require.NoError(t, app.GetMetaAnalytics(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_GetMetaAnalytics_SpecificAccountCrossOrg(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)
	roleB := metaAnalyticsRole(t, app.DB, orgB.ID)
	userB := testutil.CreateTestUser(t, app.DB, orgB.ID, testutil.WithRoleID(&roleB.ID))
	accA := mkAnalyticsAccount(t, app.DB, orgA.ID)

	from, to := dateRange()
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, orgB.ID, userB.ID)
	testutil.SetQueryParam(req, "analytics_type", "analytics")
	testutil.SetQueryParam(req, "start", from)
	testutil.SetQueryParam(req, "end", to)
	testutil.SetQueryParam(req, "account_id", accA.ID.String())

	require.NoError(t, app.GetMetaAnalytics(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req),
		"cross-org account access must look like not-found")
	assert.Equal(t, int64(0), srv.Hits(), "Meta must not be called for cross-org account")
}

func TestApp_GetMetaAnalytics_CacheHitSkipsMetaCall(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{"id":"WABA","analytics":{"granularity":"DAY","data_points":[{"start":1,"end":2,"sent":1,"delivered":1}]}}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := metaAnalyticsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	mkAnalyticsAccount(t, app.DB, org.ID)

	from, to := dateRange()
	doRequest := func() *struct {
		Data struct {
			Cached bool `json:"cached"`
		} `json:"data"`
	} {
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetQueryParam(req, "analytics_type", "analytics")
		testutil.SetQueryParam(req, "start", from)
		testutil.SetQueryParam(req, "end", to)
		testutil.SetQueryParam(req, "granularity", "DAY")
		require.NoError(t, app.GetMetaAnalytics(req))
		require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
		var resp struct {
			Data struct {
				Cached bool `json:"cached"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		return &resp
	}

	// First call: cache miss → Meta hit.
	r1 := doRequest()
	assert.False(t, r1.Data.Cached, "first call must be a cache miss")
	hitsAfterFirst := srv.Hits()
	assert.Greater(t, hitsAfterFirst, int64(0), "Meta must have been called on cache miss")

	// Second call with same params: cache hit, Meta NOT called again.
	r2 := doRequest()
	assert.True(t, r2.Data.Cached, "second identical call must be a cache hit")
	assert.Equal(t, hitsAfterFirst, srv.Hits(), "Meta must NOT be called on cache hit")
}

func TestApp_GetMetaAnalytics_AdjustedGranularityReportedWhenChanged(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{"id":"WABA","analytics":{"granularity":"DAILY","data_points":[]}}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := metaAnalyticsRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	mkAnalyticsAccount(t, app.DB, org.ID)

	from, to := dateRange() // 7-day range; MONTH would be auto-adjusted to DAY

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "analytics_type", "analytics")
	testutil.SetQueryParam(req, "start", from)
	testutil.SetQueryParam(req, "end", to)
	testutil.SetQueryParam(req, "granularity", "MONTH")

	require.NoError(t, app.GetMetaAnalytics(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			AdjustedGranularity string `json:"adjusted_granularity"`
			OriginalGranularity string `json:"original_granularity"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, "DAY", resp.Data.AdjustedGranularity)
	assert.Equal(t, "MONTH", resp.Data.OriginalGranularity)
}

// --- ListMetaAccountsForAnalytics ---

func TestApp_ListMetaAccountsForAnalytics_OnlyOwnOrg(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)
	role := metaAnalyticsRole(t, app.DB, orgA.ID)
	user := testutil.CreateTestUser(t, app.DB, orgA.ID, testutil.WithRoleID(&role.ID))
	accA := mkAnalyticsAccount(t, app.DB, orgA.ID)
	mkAnalyticsAccount(t, app.DB, orgB.ID) // must not appear

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, orgA.ID, user.ID)

	require.NoError(t, app.ListMetaAccountsForAnalytics(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Accounts []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"accounts"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	require.Len(t, resp.Data.Accounts, 1)
	assert.Equal(t, accA.ID.String(), resp.Data.Accounts[0].ID)
}

func TestApp_ListMetaAccountsForAnalytics_PermissionDenied(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "no-analytics-list", false, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.ListMetaAccountsForAnalytics(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

// --- RefreshMetaAnalyticsCache ---

func TestApp_RefreshMetaAnalyticsCache_RequiresAnalyticsWrite(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	// Read-only role.
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "analytics-r-only", []string{"analytics:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetMethod("POST")
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.RefreshMetaAnalyticsCache(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_RefreshMetaAnalyticsCache_ClearsOnlyOrgScopedKeys(t *testing.T) {
	srv := newFakeAnalyticsServer(t, `{}`)
	app := newAppForMetaAnalytics(t, srv.server.URL)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)
	role := metaAnalyticsRole(t, app.DB, orgA.ID)
	user := testutil.CreateTestUser(t, app.DB, orgA.ID, testutil.WithRoleID(&role.ID))

	ctx := context.Background()
	// Plant cache entries for both orgs.
	keysA := []string{
		fmt.Sprintf("meta:analytics:%s:all:analytics:1:2:DAY", orgA.ID.String()),
		fmt.Sprintf("meta:analytics:%s:specific:pricing_analytics:1:2:DAY", orgA.ID.String()),
	}
	keysB := []string{
		fmt.Sprintf("meta:analytics:%s:all:analytics:1:2:DAY", orgB.ID.String()),
	}
	for _, k := range keysA {
		require.NoError(t, app.Redis.Set(ctx, k, "{}", time.Hour).Err())
	}
	for _, k := range keysB {
		require.NoError(t, app.Redis.Set(ctx, k, "{}", time.Hour).Err())
	}

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetMethod("POST")
	testutil.SetAuthContext(req, orgA.ID, user.ID)

	require.NoError(t, app.RefreshMetaAnalyticsCache(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	for _, k := range keysA {
		exists, err := app.Redis.Exists(ctx, k).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), exists, "orgA key must be deleted: %s", k)
	}
	for _, k := range keysB {
		exists, err := app.Redis.Exists(ctx, k).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists, "orgB key must NOT be deleted: %s", k)
	}
}
