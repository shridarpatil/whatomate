package websocket

// ClientSendChan exposes the client's send channel for testing.
func ClientSendChan(c *Client) <-chan []byte {
	return c.send
}
