package websocket

import "github.com/google/uuid"

// ClientSendChan exposes the client's send channel for testing.
func ClientSendChan(c *Client) <-chan []byte {
	return c.send
}

// ClientAuthenticated returns whether the client has authenticated.
func ClientAuthenticated(c *Client) bool {
	return c.authenticated
}

// ClientUserID returns the client's user ID.
func ClientUserID(c *Client) uuid.UUID {
	return c.userID
}

// ClientOrgID returns the client's organization ID.
func ClientOrgID(c *Client) uuid.UUID {
	return c.organizationID
}

// ClientHandleAuthMessage exposes handleAuthMessage for testing.
func ClientHandleAuthMessage(c *Client, data []byte) bool {
	return c.handleAuthMessage(data)
}
