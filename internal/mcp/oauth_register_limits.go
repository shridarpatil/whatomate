package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/valyala/fasthttp"
)

const (
	// defaultGlobalRegisterMax caps dynamic client registrations per hour
	// across all IPs.
	defaultGlobalRegisterMax = 200
	globalRegisterWindow     = time.Hour
	defaultGlobalRegisterKey = "mcp_register_global"
	// unusedClientMaxAge is how long a registered client may exist without
	// ever being granted access before it is pruned.
	unusedClientMaxAge = 24 * time.Hour
	// pruneBatch bounds the rows one registration may delete.
	pruneBatch = 500
)

// checkGlobalRegisterLimit enforces the cross-IP registration cap with a Redis
// fixed-window counter. It fails closed: without a working Redis no client can
// register (503). It returns 0 when the registration may proceed.
func (s *Server) checkGlobalRegisterLimit() (int, string) {
	if s.app.Redis == nil {
		return fasthttp.StatusServiceUnavailable, "app registration is unavailable right now; try again later"
	}
	window := int64(globalRegisterWindow / time.Second)
	key := fmt.Sprintf("%s:%d", s.globalRegisterKey, time.Now().Unix()/window)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	count, err := s.app.Redis.Incr(ctx, key).Result()
	if err != nil {
		s.app.Log.Error("MCP registration counter unavailable; refusing registration", "error", err)
		return fasthttp.StatusServiceUnavailable, "app registration is unavailable right now; try again later"
	}
	if count == 1 {
		// The key is per window, so a failed EXPIRE only delays cleanup.
		if err := s.app.Redis.Expire(ctx, key, 2*globalRegisterWindow).Err(); err != nil {
			s.app.Log.Error("Failed to set MCP registration counter expiry", "error", err)
		}
	}
	if count > int64(s.globalRegisterMax) {
		return fasthttp.StatusTooManyRequests, "too many apps were registered recently; try again later"
	}
	return 0, ""
}

// pruneUnusedClients deletes registered clients older than unusedClientMaxAge
// that never received a grant. One bounded DELETE per registration keeps the
// table from growing through abandoned or abusive registrations.
func (s *Server) pruneUnusedClients() {
	res := s.app.DB.Exec(`DELETE FROM mcp_oauth_clients WHERE id IN (
		SELECT c.id FROM mcp_oauth_clients c
		WHERE c.created_at < ?
		  AND NOT EXISTS (SELECT 1 FROM mcp_grants g WHERE g.client_id = c.client_id)
		ORDER BY c.created_at
		LIMIT ?)`, time.Now().Add(-unusedClientMaxAge), pruneBatch)
	if res.Error != nil {
		s.app.Log.Error("Failed to prune unused MCP OAuth clients", "error", res.Error)
	}
}
