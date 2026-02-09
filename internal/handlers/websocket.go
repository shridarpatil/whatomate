package handlers

import (
	"github.com/fasthttp/websocket"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/middleware"
	ws "github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// newUpgrader creates a WebSocket upgrader that validates origins against the
// configured allowed origins. If no origins are configured, all are allowed.
func newUpgrader(allowedOrigins map[string]bool) websocket.FastHTTPUpgrader {
	return websocket.FastHTTPUpgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			origin := string(ctx.Request.Header.Peek("Origin"))
			return middleware.IsOriginAllowed(origin, allowedOrigins)
		},
	}
}

// wsUpgrader returns a WebSocket upgrader configured with the app's allowed origins.
func (a *App) wsUpgrader() websocket.FastHTTPUpgrader {
	allowedOrigins := middleware.ParseAllowedOrigins(a.Config.Server.AllowedOrigins)
	return newUpgrader(allowedOrigins)
}

// WebSocketHandler handles WebSocket connections
func (a *App) WebSocketHandler(r *fastglue.Request) error {
	// Get token from query parameter
	token := string(r.RequestCtx.QueryArgs().Peek("token"))
	if token == "" {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Missing token", nil, "")
	}

	// Validate JWT token
	userID, orgID, err := a.validateWSToken(token)
	if err != nil {
		a.Log.Error("WebSocket auth failed", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid token", nil, "")
	}

	// Upgrade to WebSocket
	up := a.wsUpgrader()
	err = up.Upgrade(r.RequestCtx, func(conn *websocket.Conn) {
		client := ws.NewClient(a.WSHub, conn, userID, orgID)

		// Register client with hub
		a.WSHub.Register(client)

		// Start pumps in goroutines
		go client.WritePump()
		client.ReadPump() // Blocking - runs until connection closes
	})

	if err != nil {
		a.Log.Error("WebSocket upgrade failed", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "WebSocket upgrade failed", nil, "")
	}

	return nil
}

// validateWSToken validates a JWT token and returns user ID and organization ID
func (a *App) validateWSToken(tokenString string) (uuid.UUID, uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &middleware.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.Config.JWT.Secret), nil
	})

	if err != nil || !token.Valid {
		return uuid.Nil, uuid.Nil, err
	}

	claims, ok := token.Claims.(*middleware.JWTClaims)
	if !ok {
		return uuid.Nil, uuid.Nil, jwt.ErrTokenInvalidClaims
	}

	return claims.UserID, claims.OrganizationID, nil
}
