package middleware_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/middleware"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	t.Parallel()
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set")
	}

	rl := middleware.RateLimit(middleware.RateLimitOpts{
		Redis:     rdb,
		Log:       testutil.NopLogger(),
		Max:       5,
		Window:    10 * time.Second,
		KeyPrefix: "test_allow_" + uuid.New().String()[:8],
	})

	for i := 0; i < 5; i++ {
		req := newTestRequest()
		req.RequestCtx.SetRemoteAddr(mockAddr("10.0.0.1:12345"))
		result := rl(req)
		require.NotNil(t, result, "request %d should be allowed", i+1)
	}
}

func TestRateLimit_BlocksOverLimit(t *testing.T) {
	t.Parallel()
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set")
	}

	prefix := "test_block_" + uuid.New().String()[:8]
	rl := middleware.RateLimit(middleware.RateLimitOpts{
		Redis:     rdb,
		Log:       testutil.NopLogger(),
		Max:       3,
		Window:    10 * time.Second,
		KeyPrefix: prefix,
	})

	for i := 0; i < 3; i++ {
		req := newTestRequest()
		req.RequestCtx.SetRemoteAddr(mockAddr("10.0.0.2:12345"))
		result := rl(req)
		require.NotNil(t, result)
	}

	// 4th request should be blocked
	req := newTestRequest()
	req.RequestCtx.SetRemoteAddr(mockAddr("10.0.0.2:12345"))
	result := rl(req)
	assert.Nil(t, result, "request over limit should be blocked")
	assert.Equal(t, fasthttp.StatusTooManyRequests, req.RequestCtx.Response.StatusCode())
	assert.NotEmpty(t, string(req.RequestCtx.Response.Header.Peek("Retry-After")))
}

func TestRateLimit_DifferentIPsGetSeparateLimits(t *testing.T) {
	t.Parallel()
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set")
	}

	prefix := "test_sep_" + uuid.New().String()[:8]
	rl := middleware.RateLimit(middleware.RateLimitOpts{
		Redis:     rdb,
		Log:       testutil.NopLogger(),
		Max:       2,
		Window:    10 * time.Second,
		KeyPrefix: prefix,
	})

	// Exhaust limit for IP A
	for i := 0; i < 2; i++ {
		req := newTestRequest()
		req.RequestCtx.SetRemoteAddr(mockAddr("10.0.0.3:12345"))
		require.NotNil(t, rl(req))
	}

	// IP B should still be allowed
	req := newTestRequest()
	req.RequestCtx.SetRemoteAddr(mockAddr("10.0.0.4:12345"))
	result := rl(req)
	assert.NotNil(t, result, "different IP should have its own limit")
}

func TestUserAwareRateLimit_KeysByUserID(t *testing.T) {
	t.Parallel()
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set")
	}

	prefix := "test_user_" + uuid.New().String()[:8]
	rl := middleware.UserAwareRateLimit(middleware.RateLimitOpts{
		Redis:     rdb,
		Log:       testutil.NopLogger(),
		Max:       2,
		Window:    10 * time.Second,
		KeyPrefix: prefix,
	})

	userA := uuid.New()
	userB := uuid.New()

	// Exhaust limit for user A (same IP for both users — simulates VPN)
	for i := 0; i < 2; i++ {
		req := newTestRequest()
		req.RequestCtx.SetRemoteAddr(mockAddr("10.0.0.5:12345"))
		req.RequestCtx.SetUserValue(middleware.ContextKeyUserID, userA)
		require.NotNil(t, rl(req), "user A request %d should be allowed", i+1)
	}

	// User A is now blocked
	req := newTestRequest()
	req.RequestCtx.SetRemoteAddr(mockAddr("10.0.0.5:12345"))
	req.RequestCtx.SetUserValue(middleware.ContextKeyUserID, userA)
	assert.Nil(t, rl(req), "user A should be blocked after exceeding limit")

	// User B on the SAME IP should still be allowed
	req = newTestRequest()
	req.RequestCtx.SetRemoteAddr(mockAddr("10.0.0.5:12345"))
	req.RequestCtx.SetUserValue(middleware.ContextKeyUserID, userB)
	assert.NotNil(t, rl(req), "user B on same IP should have its own limit")
}

func TestUserAwareRateLimit_FallsBackToIP_WhenUnauthenticated(t *testing.T) {
	t.Parallel()
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set")
	}

	prefix := "test_noauth_" + uuid.New().String()[:8]
	rl := middleware.UserAwareRateLimit(middleware.RateLimitOpts{
		Redis:     rdb,
		Log:       testutil.NopLogger(),
		Max:       2,
		Window:    10 * time.Second,
		KeyPrefix: prefix,
	})

	// No user ID set — should key by IP
	for i := 0; i < 2; i++ {
		req := newTestRequest()
		req.RequestCtx.SetRemoteAddr(mockAddr("10.0.0.6:12345"))
		require.NotNil(t, rl(req))
	}

	req := newTestRequest()
	req.RequestCtx.SetRemoteAddr(mockAddr("10.0.0.6:12345"))
	assert.Nil(t, rl(req), "unauthenticated requests should be rate limited by IP")
}

// mockAddr implements net.Addr for testing.
type mockAddr string

func (a mockAddr) Network() string { return "tcp" }
func (a mockAddr) String() string  { return string(a) }

// Ensure newTestRequest is available (defined in middleware_test.go already,
// but if running in isolation this provides it).
func init() {
	_ = func() *fastglue.Request {
		ctx := &fasthttp.RequestCtx{}
		return &fastglue.Request{RequestCtx: ctx}
	}
}
