// Package safehttp builds the HTTP client Whatomate uses for URLs that come
// from users or org admins (webhooks, custom actions, SSO identity providers),
// so they can't aim the server at its own network: cloud metadata
// (169.254.169.254), loopback services (Postgres, Redis) or anything else on a
// private range.
//
// The destination is checked when connecting, on the IP actually dialed
// (net.Dialer.Control), so DNS rebinding and redirects can't get past it: every
// redirect hop dials through the same check.
//
// Operators whose identity provider or webhook receiver sits on a private
// network allow specific ranges back in with server.outbound_allow_cidrs. An
// allowed range overrides every block, loopback and metadata included, so list
// only what is needed.
package safehttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"syscall"
	"time"
)

// ErrBlocked is returned when the destination is outside the public internet.
var ErrBlocked = errors.New("connection to a private or reserved address is not allowed (list it in server.outbound_allow_cidrs to permit)")

// maxRedirects caps how many redirects the client follows.
const maxRedirects = 3

// blockedPrefixes are ranges netip's Is* predicates don't cover.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),     // "this network"
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("240.0.0.0/4"),   // reserved, includes broadcast
}

// Blocked reports whether a connection to addr must be refused: loopback,
// private, link-local (including cloud metadata), unspecified, multicast,
// carrier-grade NAT and reserved space, in v4 or v4-mapped v6 form, unless addr
// is inside one of the allow prefixes.
func Blocked(addr netip.Addr, allow []netip.Prefix) bool {
	addr = addr.Unmap()
	for _, p := range allow {
		if p.Contains(addr) {
			return false
		}
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() || addr.IsUnspecified() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// ParseCIDRs parses allow-list entries: "10.0.0.0/8", or a bare IP as a
// single-host range. Entries may themselves be comma-separated, as an
// environment variable arrives. Empty entries are skipped; a malformed one is
// an error.
func ParseCIDRs(entries []string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, entry := range entries {
		for s := range strings.SplitSeq(entry, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if strings.Contains(s, "/") {
				p, err := netip.ParsePrefix(s)
				if err != nil {
					return nil, fmt.Errorf("invalid CIDR %q: %w", s, err)
				}
				out = append(out, p.Masked())
				continue
			}
			a, err := netip.ParseAddr(s)
			if err != nil {
				return nil, fmt.Errorf("invalid IP or CIDR %q (hostnames aren't accepted)", s)
			}
			out = append(out, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
		}
	}
	return out, nil
}

// Dialer returns a net.Dialer that refuses blocked destinations at connect time.
func Dialer(allow []netip.Prefix) *net.Dialer {
	return &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrBlocked, address)
			}
			if Blocked(ap.Addr(), allow) {
				return fmt.Errorf("%w: %s", ErrBlocked, ap.Addr())
			}
			return nil
		},
	}
}

// NewClient returns an HTTP client that only reaches public addresses (plus the
// allowed ranges) and follows at most 3 redirects. timeout 0 means none.
func NewClient(allow []netip.Prefix, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               nil, // a proxy would dial on our behalf, past the check
			DialContext:         Dialer(allow).DialContext,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) > maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
}
