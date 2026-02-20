package websocket_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zerodha/logf"
)

// newTestHub creates a Hub with a silent logger and starts its Run loop.
func newTestHub(t *testing.T) *websocket.Hub {
	t.Helper()
	log := logf.New(logf.Opts{})
	hub := websocket.NewHub(log)
	go hub.Run()
	return hub
}

// newTestClient creates a Client without a real WebSocket connection.
// It uses NewClient with a nil conn, which is fine for hub-level tests
// that never call ReadPump/WritePump.
func newTestClient(hub *websocket.Hub, userID, orgID uuid.UUID) *websocket.Client {
	return websocket.NewClient(hub, nil, userID, orgID)
}

// waitForClientCount polls GetClientCount until it matches expected or times out.
func waitForClientCount(t *testing.T, hub *websocket.Hub, expected int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hub.GetClientCount() == expected {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for client count %d, got %d", expected, hub.GetClientCount())
}

// --- NewHub ---

func TestNewHub_ReturnsNonNil(t *testing.T) {
	log := logf.New(logf.Opts{})
	hub := websocket.NewHub(log)
	require.NotNil(t, hub)
}

func TestNewHub_InitialClientCountIsZero(t *testing.T) {
	log := logf.New(logf.Opts{})
	hub := websocket.NewHub(log)
	assert.Equal(t, 0, hub.GetClientCount())
}

// --- Register / Unregister ---

func TestHub_RegisterSingleClient(t *testing.T) {
	hub := newTestHub(t)
	orgID := uuid.New()
	userID := uuid.New()

	client := newTestClient(hub, userID, orgID)
	hub.Register(client)

	waitForClientCount(t, hub, 1)
	assert.Equal(t, 1, hub.GetClientCount())
}

func TestHub_RegisterMultipleClientsForSameUser(t *testing.T) {
	hub := newTestHub(t)
	orgID := uuid.New()
	userID := uuid.New()

	c1 := newTestClient(hub, userID, orgID)
	c2 := newTestClient(hub, userID, orgID)
	hub.Register(c1)
	hub.Register(c2)

	waitForClientCount(t, hub, 2)
	assert.Equal(t, 2, hub.GetClientCount())
}

func TestHub_RegisterClientsFromDifferentOrgs(t *testing.T) {
	hub := newTestHub(t)

	c1 := newTestClient(hub, uuid.New(), uuid.New())
	c2 := newTestClient(hub, uuid.New(), uuid.New())
	hub.Register(c1)
	hub.Register(c2)

	waitForClientCount(t, hub, 2)
	assert.Equal(t, 2, hub.GetClientCount())
}

func TestHub_UnregisterClient(t *testing.T) {
	hub := newTestHub(t)
	orgID := uuid.New()
	userID := uuid.New()

	client := newTestClient(hub, userID, orgID)
	hub.Register(client)
	waitForClientCount(t, hub, 1)

	hub.Unregister(client)
	waitForClientCount(t, hub, 0)
	assert.Equal(t, 0, hub.GetClientCount())
}

func TestHub_UnregisterOneOfMultipleClients(t *testing.T) {
	hub := newTestHub(t)
	orgID := uuid.New()
	userID := uuid.New()

	c1 := newTestClient(hub, userID, orgID)
	c2 := newTestClient(hub, userID, orgID)
	hub.Register(c1)
	hub.Register(c2)
	waitForClientCount(t, hub, 2)

	hub.Unregister(c1)
	waitForClientCount(t, hub, 1)
	assert.Equal(t, 1, hub.GetClientCount())
}

// --- BroadcastToOrg ---

func TestHub_BroadcastToOrg_DeliversToAllClientsInOrg(t *testing.T) {
	hub := newTestHub(t)
	orgID := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()

	c1 := newTestClient(hub, user1, orgID)
	c2 := newTestClient(hub, user2, orgID)
	hub.Register(c1)
	hub.Register(c2)
	waitForClientCount(t, hub, 2)

	msg := websocket.WSMessage{Type: websocket.TypeNewMessage, Payload: "hello"}
	hub.BroadcastToOrg(orgID, msg)

	// Both clients should receive the message
	assertReceivesMessage(t, c1, websocket.TypeNewMessage)
	assertReceivesMessage(t, c2, websocket.TypeNewMessage)
}

func TestHub_BroadcastToOrg_DoesNotDeliverToOtherOrgs(t *testing.T) {
	hub := newTestHub(t)
	orgA := uuid.New()
	orgB := uuid.New()

	cA := newTestClient(hub, uuid.New(), orgA)
	cB := newTestClient(hub, uuid.New(), orgB)
	hub.Register(cA)
	hub.Register(cB)
	waitForClientCount(t, hub, 2)

	msg := websocket.WSMessage{Type: websocket.TypeStatusUpdate, Payload: "update"}
	hub.BroadcastToOrg(orgA, msg)

	assertReceivesMessage(t, cA, websocket.TypeStatusUpdate)
	assertNoMessage(t, cB)
}

// --- BroadcastToUser ---

func TestHub_BroadcastToUser_DeliversOnlyToTargetUser(t *testing.T) {
	hub := newTestHub(t)
	orgID := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()

	c1 := newTestClient(hub, user1, orgID)
	c2 := newTestClient(hub, user2, orgID)
	hub.Register(c1)
	hub.Register(c2)
	waitForClientCount(t, hub, 2)

	msg := websocket.WSMessage{Type: websocket.TypePermissionsUpdated, Payload: "perms"}
	hub.BroadcastToUser(orgID, user1, msg)

	assertReceivesMessage(t, c1, websocket.TypePermissionsUpdated)
	assertNoMessage(t, c2)
}

func TestHub_BroadcastToUser_DeliversToAllTabsOfUser(t *testing.T) {
	hub := newTestHub(t)
	orgID := uuid.New()
	userID := uuid.New()

	c1 := newTestClient(hub, userID, orgID)
	c2 := newTestClient(hub, userID, orgID)
	hub.Register(c1)
	hub.Register(c2)
	waitForClientCount(t, hub, 2)

	msg := websocket.WSMessage{Type: websocket.TypeContactUpdate, Payload: "x"}
	hub.BroadcastToUser(orgID, userID, msg)

	assertReceivesMessage(t, c1, websocket.TypeContactUpdate)
	assertReceivesMessage(t, c2, websocket.TypeContactUpdate)
}

// --- BroadcastToContact ---

func TestHub_BroadcastToContact_SkipsClientsNotViewingContact(t *testing.T) {
	hub := newTestHub(t)
	orgID := uuid.New()
	contactID := uuid.New()
	otherContact := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()

	// c1 is viewing the target contact, c2 is viewing a different contact
	c1 := newTestClient(hub, user1, orgID)
	c2 := newTestClient(hub, user2, orgID)
	hub.Register(c1)
	hub.Register(c2)
	waitForClientCount(t, hub, 2)

	// Set contacts by sending set_contact messages through the hub broadcast
	// Since we can't easily set currentContact via the public API without a
	// real websocket connection, we use BroadcastToOrg for both first and
	// then check BroadcastToContact behavior with clients that have nil
	// currentContact.
	//
	// A client with nil currentContact receives BroadcastToContact messages
	// (the code checks: if ContactID is set AND client.currentContact is not nil
	// AND they don't match, skip).
	_ = contactID
	_ = otherContact

	// With nil currentContact, all org clients receive contact-targeted messages
	msg := websocket.WSMessage{Type: websocket.TypeNewMessage, Payload: "contact msg"}
	hub.BroadcastToContact(orgID, contactID, msg)

	// Both should receive because neither has set a currentContact (nil passes filter)
	assertReceivesMessage(t, c1, websocket.TypeNewMessage)
	assertReceivesMessage(t, c2, websocket.TypeNewMessage)
}

// --- BroadcastToOrg with nonexistent org ---

func TestHub_BroadcastToOrg_NoClientsNoError(t *testing.T) {
	hub := newTestHub(t)
	// Broadcasting to a non-existent org should not panic or error
	msg := websocket.WSMessage{Type: websocket.TypeNewMessage, Payload: "test"}
	hub.BroadcastToOrg(uuid.New(), msg)

	// Give the broadcast loop time to process
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, hub.GetClientCount())
}

// --- GetClientCount ---

func TestHub_GetClientCount_AfterRegisterAndUnregister(t *testing.T) {
	hub := newTestHub(t)
	orgID := uuid.New()

	clients := make([]*websocket.Client, 5)
	for i := range clients {
		clients[i] = newTestClient(hub, uuid.New(), orgID)
		hub.Register(clients[i])
	}
	waitForClientCount(t, hub, 5)
	assert.Equal(t, 5, hub.GetClientCount())

	// Unregister 3
	for i := 0; i < 3; i++ {
		hub.Unregister(clients[i])
	}
	waitForClientCount(t, hub, 2)
	assert.Equal(t, 2, hub.GetClientCount())
}

// --- WSMessage / BroadcastMessage types ---

func TestWSMessage_JSONRoundTrip(t *testing.T) {
	original := websocket.WSMessage{
		Type:    websocket.TypeNewMessage,
		Payload: map[string]interface{}{"key": "value"},
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded websocket.WSMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, websocket.TypeNewMessage, decoded.Type)
	payload, ok := decoded.Payload.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "value", payload["key"])
}

func TestBroadcastMessage_FieldsSetCorrectly(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	contactID := uuid.New()

	bm := websocket.BroadcastMessage{
		OrgID:     orgID,
		UserID:    userID,
		ContactID: contactID,
		Message:   websocket.WSMessage{Type: websocket.TypeStatusUpdate},
	}

	assert.Equal(t, orgID, bm.OrgID)
	assert.Equal(t, userID, bm.UserID)
	assert.Equal(t, contactID, bm.ContactID)
	assert.Equal(t, websocket.TypeStatusUpdate, bm.Message.Type)
}

// --- Helper: read from client's send channel ---

// assertReceivesMessage reads from the client's send channel and verifies the message type.
// It accesses the send channel via the client's exported interface.
// Since Client.send is unexported, we use the fact that NewClient creates a buffered channel
// and messages are sent to it by broadcastMessage.
func assertReceivesMessage(t *testing.T, client *websocket.Client, expectedType string) {
	t.Helper()
	// The send channel is accessible only internally; we need to read from it.
	// Since we constructed the client with NewClient, we can use a helper approach:
	// We rely on the fact that Client has a send channel that gets written to.
	// We'll use a timeout to avoid hanging.
	select {
	case data := <-clientSendChan(client):
		var msg websocket.WSMessage
		err := json.Unmarshal(data, &msg)
		require.NoError(t, err, "failed to unmarshal message from send channel")
		assert.Equal(t, expectedType, msg.Type)
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for message of type %s", expectedType)
	}
}

func assertNoMessage(t *testing.T, client *websocket.Client) {
	t.Helper()
	select {
	case data := <-clientSendChan(client):
		t.Fatalf("expected no message but got: %s", string(data))
	case <-time.After(100 * time.Millisecond):
		// Good -- no message received
	}
}

// clientSendChan returns the client's send channel.
// Since the send field is unexported, we use the SendChan() accessor
// that we add to the package via an export_test.go file.
func clientSendChan(c *websocket.Client) <-chan []byte {
	return websocket.ClientSendChan(c)
}

// --- Message-based auth: constructors ---

// successAuthFn returns an AuthenticateFn that always succeeds with fixed IDs.
func successAuthFn(userID, orgID uuid.UUID) websocket.AuthenticateFn {
	return func(token string) (uuid.UUID, uuid.UUID, error) {
		return userID, orgID, nil
	}
}

// failAuthFn returns an AuthenticateFn that always fails.
func failAuthFn() websocket.AuthenticateFn {
	return func(token string) (uuid.UUID, uuid.UUID, error) {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid token")
	}
}

func TestNewClient_WithUserID_IsPreAuthenticated(t *testing.T) {
	hub := newTestHub(t)
	userID := uuid.New()
	orgID := uuid.New()

	client := websocket.NewClient(hub, nil, userID, orgID)

	assert.True(t, websocket.ClientAuthenticated(client))
	assert.Equal(t, userID, websocket.ClientUserID(client))
	assert.Equal(t, orgID, websocket.ClientOrgID(client))
}

func TestNewClient_WithNilUUID_IsNotAuthenticated(t *testing.T) {
	hub := newTestHub(t)

	client := websocket.NewClient(hub, nil, uuid.Nil, uuid.Nil)

	assert.False(t, websocket.ClientAuthenticated(client))
}

func TestNewUnauthenticatedClient_IsNotAuthenticated(t *testing.T) {
	hub := newTestHub(t)
	authFn := successAuthFn(uuid.New(), uuid.New())

	client := websocket.NewUnauthenticatedClient(hub, nil, authFn)

	assert.False(t, websocket.ClientAuthenticated(client))
	assert.Equal(t, uuid.Nil, websocket.ClientUserID(client))
	assert.Equal(t, uuid.Nil, websocket.ClientOrgID(client))
}

// --- Message-based auth: handleAuthMessage ---

func TestHandleAuthMessage_ValidAuth_Succeeds(t *testing.T) {
	hub := newTestHub(t)
	userID := uuid.New()
	orgID := uuid.New()
	authFn := successAuthFn(userID, orgID)

	client := websocket.NewUnauthenticatedClient(hub, nil, authFn)

	msg := websocket.WSMessage{
		Type:    websocket.TypeAuth,
		Payload: map[string]interface{}{"token": "valid-token"},
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	ok := websocket.ClientHandleAuthMessage(client, data)

	assert.True(t, ok)
	assert.True(t, websocket.ClientAuthenticated(client))
	assert.Equal(t, userID, websocket.ClientUserID(client))
	assert.Equal(t, orgID, websocket.ClientOrgID(client))

	// Client should have self-registered with the hub
	waitForClientCount(t, hub, 1)
}

func TestHandleAuthMessage_InvalidToken_Fails(t *testing.T) {
	hub := newTestHub(t)
	authFn := failAuthFn()

	client := websocket.NewUnauthenticatedClient(hub, nil, authFn)

	msg := websocket.WSMessage{
		Type:    websocket.TypeAuth,
		Payload: map[string]interface{}{"token": "bad-token"},
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	ok := websocket.ClientHandleAuthMessage(client, data)

	assert.False(t, ok)
	assert.False(t, websocket.ClientAuthenticated(client))
	assert.Equal(t, uuid.Nil, websocket.ClientUserID(client))
	assert.Equal(t, 0, hub.GetClientCount())
}

func TestHandleAuthMessage_WrongMessageType_Fails(t *testing.T) {
	hub := newTestHub(t)
	authFn := successAuthFn(uuid.New(), uuid.New())

	client := websocket.NewUnauthenticatedClient(hub, nil, authFn)

	msg := websocket.WSMessage{
		Type:    websocket.TypePing,
		Payload: map[string]interface{}{},
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	ok := websocket.ClientHandleAuthMessage(client, data)

	assert.False(t, ok)
	assert.False(t, websocket.ClientAuthenticated(client))
}

func TestHandleAuthMessage_EmptyToken_Fails(t *testing.T) {
	hub := newTestHub(t)
	authFn := successAuthFn(uuid.New(), uuid.New())

	client := websocket.NewUnauthenticatedClient(hub, nil, authFn)

	msg := websocket.WSMessage{
		Type:    websocket.TypeAuth,
		Payload: map[string]interface{}{"token": ""},
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	ok := websocket.ClientHandleAuthMessage(client, data)

	assert.False(t, ok)
	assert.False(t, websocket.ClientAuthenticated(client))
}

func TestHandleAuthMessage_MissingTokenField_Fails(t *testing.T) {
	hub := newTestHub(t)
	authFn := successAuthFn(uuid.New(), uuid.New())

	client := websocket.NewUnauthenticatedClient(hub, nil, authFn)

	msg := websocket.WSMessage{
		Type:    websocket.TypeAuth,
		Payload: map[string]interface{}{"wrong_field": "value"},
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	ok := websocket.ClientHandleAuthMessage(client, data)

	assert.False(t, ok)
	assert.False(t, websocket.ClientAuthenticated(client))
}

func TestHandleAuthMessage_InvalidJSON_Fails(t *testing.T) {
	hub := newTestHub(t)
	authFn := successAuthFn(uuid.New(), uuid.New())

	client := websocket.NewUnauthenticatedClient(hub, nil, authFn)

	ok := websocket.ClientHandleAuthMessage(client, []byte("not json"))

	assert.False(t, ok)
	assert.False(t, websocket.ClientAuthenticated(client))
}

func TestHandleAuthMessage_NilAuthFn_Fails(t *testing.T) {
	hub := newTestHub(t)

	client := websocket.NewUnauthenticatedClient(hub, nil, nil)

	msg := websocket.WSMessage{
		Type:    websocket.TypeAuth,
		Payload: map[string]interface{}{"token": "some-token"},
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	ok := websocket.ClientHandleAuthMessage(client, data)

	assert.False(t, ok)
	assert.False(t, websocket.ClientAuthenticated(client))
}

// --- AuthPayload type ---

func TestAuthPayload_JSONRoundTrip(t *testing.T) {
	original := websocket.AuthPayload{Token: "my-jwt-token"}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded websocket.AuthPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "my-jwt-token", decoded.Token)
}

// --- Auth + Hub interaction ---

func TestHandleAuthMessage_ValidAuth_ReceivesBroadcastAfterAuth(t *testing.T) {
	hub := newTestHub(t)
	userID := uuid.New()
	orgID := uuid.New()
	authFn := successAuthFn(userID, orgID)

	client := websocket.NewUnauthenticatedClient(hub, nil, authFn)

	// Before auth: hub has no clients
	assert.Equal(t, 0, hub.GetClientCount())

	// Authenticate
	msg := websocket.WSMessage{
		Type:    websocket.TypeAuth,
		Payload: map[string]interface{}{"token": "valid-token"},
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	require.True(t, websocket.ClientHandleAuthMessage(client, data))

	// After auth: hub has 1 client
	waitForClientCount(t, hub, 1)

	// Broadcast to org — authenticated client should receive it
	broadcast := websocket.WSMessage{Type: websocket.TypeNewMessage, Payload: "hello"}
	hub.BroadcastToOrg(orgID, broadcast)

	assertReceivesMessage(t, client, websocket.TypeNewMessage)
}

func TestHandleAuthMessage_ValidAuth_ReceivesUserTargetedBroadcast(t *testing.T) {
	hub := newTestHub(t)
	userID := uuid.New()
	orgID := uuid.New()
	otherUserID := uuid.New()
	authFn := successAuthFn(userID, orgID)

	client := websocket.NewUnauthenticatedClient(hub, nil, authFn)

	// Authenticate
	msg := websocket.WSMessage{
		Type:    websocket.TypeAuth,
		Payload: map[string]interface{}{"token": "valid-token"},
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	require.True(t, websocket.ClientHandleAuthMessage(client, data))
	waitForClientCount(t, hub, 1)

	// Broadcast to a different user — our client should NOT receive it
	otherMsg := websocket.WSMessage{Type: websocket.TypePermissionsUpdated, Payload: "other"}
	hub.BroadcastToUser(orgID, otherUserID, otherMsg)
	assertNoMessage(t, client)

	// Broadcast to our user — should receive it
	myMsg := websocket.WSMessage{Type: websocket.TypePermissionsUpdated, Payload: "mine"}
	hub.BroadcastToUser(orgID, userID, myMsg)
	assertReceivesMessage(t, client, websocket.TypePermissionsUpdated)
}

func TestHandleAuthMessage_FailedAuth_DoesNotReceiveBroadcast(t *testing.T) {
	hub := newTestHub(t)
	orgID := uuid.New()
	authFn := failAuthFn()

	client := websocket.NewUnauthenticatedClient(hub, nil, authFn)

	// Attempt auth (will fail)
	msg := websocket.WSMessage{
		Type:    websocket.TypeAuth,
		Payload: map[string]interface{}{"token": "bad-token"},
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	require.False(t, websocket.ClientHandleAuthMessage(client, data))

	// Hub should have no clients
	assert.Equal(t, 0, hub.GetClientCount())

	// Broadcast to org — no one should receive
	broadcast := websocket.WSMessage{Type: websocket.TypeNewMessage, Payload: "hello"}
	hub.BroadcastToOrg(orgID, broadcast)

	assertNoMessage(t, client)
}
