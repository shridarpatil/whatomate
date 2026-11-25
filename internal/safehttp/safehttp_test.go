package safehttp_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/internal/safehttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlocked(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "::1", // loopback
		"10.1.2.3", "172.16.0.1", "192.168.10.5", "fd00::1", // private
		"169.254.169.254", "fe80::1", // link-local, cloud metadata
		"100.64.0.1",               // carrier-grade NAT
		"0.0.0.0", "0.1.2.3", "::", // unspecified / "this network"
		"224.0.0.1", "ff02::1", // multicast
		"240.0.0.1", "255.255.255.255", // reserved, broadcast
		"::ffff:10.0.0.1", // v4-mapped private
	}
	for _, s := range blocked {
		assert.True(t, safehttp.Blocked(netip.MustParseAddr(s), nil), s)
	}

	public := []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111", "::ffff:8.8.8.8"}
	for _, s := range public {
		assert.False(t, safehttp.Blocked(netip.MustParseAddr(s), nil), s)
	}
}

func TestBlocked_AllowListOverrides(t *testing.T) {
	allow := []netip.Prefix{netip.MustParsePrefix("192.168.10.5/32"), netip.MustParsePrefix("10.20.0.0/16")}

	assert.False(t, safehttp.Blocked(netip.MustParseAddr("192.168.10.5"), allow))
	assert.False(t, safehttp.Blocked(netip.MustParseAddr("10.20.5.5"), allow))
	assert.False(t, safehttp.Blocked(netip.MustParseAddr("::ffff:192.168.10.5"), allow), "v4-mapped form matches too")
	assert.True(t, safehttp.Blocked(netip.MustParseAddr("192.168.10.6"), allow), "neighbours stay blocked")
	assert.True(t, safehttp.Blocked(netip.MustParseAddr("10.21.0.1"), allow))
}

func TestParseCIDRs(t *testing.T) {
	got, err := safehttp.ParseCIDRs([]string{" 192.168.0.0/16 ", "10.0.0.5", "fd00::/8", ""})
	require.NoError(t, err)
	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("10.0.0.5/32"),
		netip.MustParsePrefix("fd00::/8"),
	}, got)

	// Comma-separated entries (as an env var arrives) are split.
	got, err = safehttp.ParseCIDRs([]string{"192.168.10.5/32,192.168.20.7"})
	require.NoError(t, err)
	assert.Len(t, got, 2)

	for _, bad := range []string{"not-an-ip", "192.168.0.0/33", "auth.example.com"} {
		_, err := safehttp.ParseCIDRs([]string{bad})
		assert.Error(t, err, bad)
	}
}

func TestClient_BlocksPrivateUnlessAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// httptest listens on 127.0.0.1: blocked by default.
	_, err := safehttp.NewClient(nil, 5*time.Second).Get(srv.URL)
	require.Error(t, err)
	assert.True(t, errors.Is(err, safehttp.ErrBlocked), "got %v", err)

	// Allowed once its address is listed.
	allow := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	resp, err := safehttp.NewClient(allow, 5*time.Second).Get(srv.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestClient_StopsAfterThreeRedirects(t *testing.T) {
	hops := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops++
		http.Redirect(w, r, srv.URL+"/next", http.StatusFound)
	}))
	defer srv.Close()

	allow := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	_, err := safehttp.NewClient(allow, 5*time.Second).Get(srv.URL)
	require.Error(t, err)
	assert.Equal(t, 4, hops, "the first request plus 3 redirects")
}
