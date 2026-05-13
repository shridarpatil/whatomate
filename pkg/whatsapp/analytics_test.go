package whatsapp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ValidateGranularity ---

func TestValidateGranularity(t *testing.T) {
	cases := map[string]bool{
		"HALF_HOUR": true,
		"DAY":       true,
		"DAILY":     true,
		"MONTH":     true,
		"MONTHLY":   true,
		"daily":     false, // case-sensitive
		"":          false,
		"WEEKLY":    false,
	}
	for in, want := range cases {
		assert.Equal(t, want, whatsapp.ValidateGranularity(in), "input=%q", in)
	}
}

func TestValidateAnalyticsType(t *testing.T) {
	cases := map[string]bool{
		"analytics":          true,
		"pricing_analytics":  true,
		"template_analytics": true,
		"call_analytics":     true,
		"unknown":            false,
		"":                   false,
	}
	for in, want := range cases {
		assert.Equal(t, want, whatsapp.ValidateAnalyticsType(in), "input=%q", in)
	}
}

// --- NormalizeGranularity ---

func TestNormalizeGranularity(t *testing.T) {
	type tc struct {
		in   string
		typ  whatsapp.AnalyticsType
		want string
	}
	cases := []tc{
		// Template always uses DAILY regardless of input.
		{"DAY", whatsapp.AnalyticsTypeTemplate, "DAILY"},
		{"MONTH", whatsapp.AnalyticsTypeTemplate, "DAILY"},
		{"HALF_HOUR", whatsapp.AnalyticsTypeTemplate, "DAILY"},

		// Pricing/Call: DAY → DAILY, MONTH → MONTHLY.
		{"DAY", whatsapp.AnalyticsTypePricing, "DAILY"},
		{"MONTH", whatsapp.AnalyticsTypePricing, "MONTHLY"},
		{"DAY", whatsapp.AnalyticsTypeCall, "DAILY"},
		{"MONTH", whatsapp.AnalyticsTypeCall, "MONTHLY"},

		// Pricing also accepts already-normalized DAILY/MONTHLY.
		{"DAILY", whatsapp.AnalyticsTypePricing, "DAILY"},
		{"MONTHLY", whatsapp.AnalyticsTypePricing, "MONTHLY"},

		// Messaging: DAY/MONTH passed through.
		{"DAY", whatsapp.AnalyticsTypeMessaging, "DAY"},
		{"MONTH", whatsapp.AnalyticsTypeMessaging, "MONTH"},

		// Messaging accepts DAILY input but normalizes back to DAY.
		{"DAILY", whatsapp.AnalyticsTypeMessaging, "DAY"},
		{"MONTHLY", whatsapp.AnalyticsTypeMessaging, "MONTH"},

		// HALF_HOUR is preserved everywhere except template.
		{"HALF_HOUR", whatsapp.AnalyticsTypeMessaging, "HALF_HOUR"},
		{"HALF_HOUR", whatsapp.AnalyticsTypeCall, "HALF_HOUR"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, whatsapp.NormalizeGranularity(c.in, c.typ),
			"in=%q type=%q", c.in, c.typ)
	}
}

// --- GetAnalytics: Messaging (nested under analytics) ---

func TestClient_GetAnalytics_MessagingNestedData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "fields=analytics.start(100).end(200).granularity(DAY)")
		_, _ = w.Write([]byte(`{
			"id": "WABA-1",
			"analytics": {
				"granularity": "DAY",
				"data_points": [
					{"start": 100, "end": 150, "sent": 5, "delivered": 4},
					{"start": 150, "end": 200, "sent": 7, "delivered": 6}
				]
			}
		}`))
	}))
	t.Cleanup(srv.Close)
	client := whatsapp.NewWithBaseURL(testutil.NopLogger(), srv.URL)

	resp, err := client.GetAnalytics(context.Background(), &whatsapp.Account{
		BusinessID: "WABA-1", APIVersion: "v18.0", AccessToken: "tok",
	}, whatsapp.AnalyticsTypeMessaging, &whatsapp.AnalyticsRequest{
		Start: 100, End: 200, Granularity: "DAY",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Analytics)
	assert.Equal(t, "DAY", resp.Analytics.Granularity)
	require.Len(t, resp.Analytics.DataPoints, 2)
	assert.Equal(t, int64(5), resp.Analytics.DataPoints[0].Sent)
	assert.Equal(t, int64(7), resp.Analytics.DataPoints[1].Sent)
}

// --- GetAnalytics: Messaging with phone-number-grouped data ---

func TestClient_GetAnalytics_MessagingFlattensPhoneEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id": "WABA-1",
			"analytics": {
				"granularity": "DAY",
				"data": [
					{"phone_number":"+111","data_points":[{"start":1,"end":2,"sent":1,"delivered":1}]},
					{"phone_number":"+222","data_points":[{"start":1,"end":2,"sent":2,"delivered":2}]}
				]
			}
		}`))
	}))
	t.Cleanup(srv.Close)
	client := whatsapp.NewWithBaseURL(testutil.NopLogger(), srv.URL)

	resp, err := client.GetAnalytics(context.Background(), &whatsapp.Account{
		BusinessID: "WABA-1", APIVersion: "v18.0", AccessToken: "tok",
	}, whatsapp.AnalyticsTypeMessaging, &whatsapp.AnalyticsRequest{
		Start: 1, End: 2, Granularity: "DAY",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Analytics)
	assert.Len(t, resp.Analytics.DataPoints, 2, "data points from both phone entries should be flattened")
}

// --- GetAnalytics: Pricing dimensions in URL ---

func TestClient_GetAnalytics_PricingIncludesDimensions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "dimensions(PRICING_CATEGORY,PRICING_TYPE,COUNTRY)",
			"pricing must include the dimensions filter")
		// MONTH should be normalized to MONTHLY for pricing analytics.
		assert.Contains(t, r.URL.RawQuery, "granularity(MONTHLY)")
		_, _ = w.Write([]byte(`{
			"id":"WABA-1",
			"pricing_analytics":{"granularity":"MONTHLY","data_points":[{"start":1,"end":2,"volume":10,"cost":1.5,"country":"IN"}]}
		}`))
	}))
	t.Cleanup(srv.Close)
	client := whatsapp.NewWithBaseURL(testutil.NopLogger(), srv.URL)

	resp, err := client.GetAnalytics(context.Background(), &whatsapp.Account{
		BusinessID: "WABA-1", APIVersion: "v18.0", AccessToken: "tok",
	}, whatsapp.AnalyticsTypePricing, &whatsapp.AnalyticsRequest{
		Start: 1, End: 2, Granularity: "MONTH",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.PricingAnalytics)
	require.Len(t, resp.PricingAnalytics.DataPoints, 1)
	assert.Equal(t, "IN", resp.PricingAnalytics.DataPoints[0].Country)
}

// --- GetAnalytics: Call dimensions + metric_types ---

func TestClient_GetAnalytics_CallIncludesDimensionsAndMetricTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "dimensions(direction)")
		assert.Contains(t, r.URL.RawQuery, "metric_types(COUNT,COST,AVERAGE_DURATION)")
		_, _ = w.Write([]byte(`{
			"id":"WABA-1",
			"call_analytics":{"granularity":"DAILY","data_points":[{"start":1,"end":2,"count":3,"cost":0.5,"average_duration":120,"direction":"USER_INITIATED"}]}
		}`))
	}))
	t.Cleanup(srv.Close)
	client := whatsapp.NewWithBaseURL(testutil.NopLogger(), srv.URL)

	resp, err := client.GetAnalytics(context.Background(), &whatsapp.Account{
		BusinessID: "WABA-1", APIVersion: "v18.0", AccessToken: "tok",
	}, whatsapp.AnalyticsTypeCall, &whatsapp.AnalyticsRequest{
		Start: 1, End: 2, Granularity: "DAY",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.CallAnalytics)
	require.Len(t, resp.CallAnalytics.DataPoints, 1)
	assert.Equal(t, "USER_INITIATED", resp.CallAnalytics.DataPoints[0].Direction)
}

// --- GetAnalytics: Template analytics uses dedicated endpoint + paginates ---

func TestClient_GetAnalytics_TemplatePaginatesAndFlattens(t *testing.T) {
	var hits int
	var firstURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			firstURL = r.URL.String()
			assert.Contains(t, r.URL.Path, "/template_analytics")
			assert.Contains(t, r.URL.RawQuery, "metric_types=cost,clicked,delivered,read,sent")
			_, _ = w.Write([]byte(`{
				"data":[{"granularity":"DAILY","product_type":"X","data_points":[{"template_id":"T1","start":1,"end":2,"sent":10,"delivered":9,"read":8}]}],
				"paging":{"next":"PAGE_TWO"}
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"data":[{"granularity":"DAILY","product_type":"X","data_points":[{"template_id":"T1","start":2,"end":3,"sent":20,"delivered":18,"read":15}]}]
		}`))
	}))
	t.Cleanup(srv.Close)
	// Replace the second-page "next" URL with one pointing back at our server.
	pageTwoURL := srv.URL + "/page-two"
	client := whatsapp.NewWithBaseURL(testutil.NopLogger(), srv.URL)

	// Tweak the response to put a real URL in `next` so the pagination loop can follow it.
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			firstURL = r.URL.String()
			_, _ = w.Write([]byte(`{
				"data":[{"granularity":"DAILY","product_type":"X","data_points":[{"template_id":"T1","start":1,"end":2,"sent":10,"delivered":9,"read":8}]}],
				"paging":{"next":"` + pageTwoURL + `"}
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"data":[{"granularity":"DAILY","product_type":"X","data_points":[{"template_id":"T1","start":2,"end":3,"sent":20,"delivered":18,"read":15}]}]
		}`))
	})

	resp, err := client.GetAnalytics(context.Background(), &whatsapp.Account{
		BusinessID: "WABA-1", APIVersion: "v18.0", AccessToken: "tok",
	}, whatsapp.AnalyticsTypeTemplate, &whatsapp.AnalyticsRequest{
		Start: 1, End: 3, Granularity: "DAILY", TemplateIDs: []string{"T1"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.TemplateAnalytics)
	assert.Len(t, resp.TemplateAnalytics.DataPoints, 2, "data points from both pages should be aggregated")
	assert.Greater(t, hits, 1, "should follow pagination Next link")
	assert.Contains(t, firstURL, "template_ids=[T1]", "template_ids should be a numeric-style array")
}

// --- GetAnalytics: API error wrapped ---

func TestClient_GetAnalytics_APIErrorWrapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"insufficient permissions","code":10}}`))
	}))
	t.Cleanup(srv.Close)
	client := whatsapp.NewWithBaseURL(testutil.NopLogger(), srv.URL)

	_, err := client.GetAnalytics(context.Background(), &whatsapp.Account{
		BusinessID: "WABA-1", APIVersion: "v18.0", AccessToken: "tok",
	}, whatsapp.AnalyticsTypeMessaging, &whatsapp.AnalyticsRequest{Start: 1, End: 2, Granularity: "DAY"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch analytics")
	assert.Contains(t, err.Error(), "insufficient permissions")
}
