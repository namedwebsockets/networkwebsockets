package networkwebsockets

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/richtr/websocket"
)

// Make a new Named WebSocket server
func makeService(host string, port int) *NetworkWebSocket_Service {
	return NewNetworkWebSocketService(host, port)
}

type WSClient struct {
	*websocket.Conn
}

// Make a new WebSocket client connection
func makeClient(t *testing.T, host, path string) *WSClient {
	url := fmt.Sprintf("ws://%s%s", host, path)
	ws, _, err := websocket.DefaultDialer.Dial(url, map[string][]string{
		"Origin": []string{"localhost"},
	})
	if err != nil {
		t.Fatalf("Websocket client connection failed: %s", err)
	}
	wsClient := &WSClient{ws}
	return wsClient
}

func (ws *WSClient) getSelfId(t *testing.T) string {

	ws.send(t, "status", "", "", "")

	ws.SetReadDeadline(time.Now().Add(time.Second * 10))
	_, p, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var message NetworkWebSocketWireMessage
	if err := json.Unmarshal(p, &message); err != nil {
		t.Fatalf("NetworkWebSocketWireMessage JSON Unmarshaling: %s", err)
	}

	if message.Action != "status" {
		t.Fatalf("Expected a status response: %s", err)
	}

	return message.Target
}

// Send message to Network Web Socket channel peer connection
func (ws *WSClient) send(t *testing.T, action string, source, target string, payload string) {
	m := NetworkWebSocketWireMessage{
		Action:  action,
		Source:  source,
		Target:  target,
		Payload: payload,
	}
	messagePayload, err := json.Marshal(m)
	if err != nil {
		return
	}

	if err := ws.SetWriteDeadline(time.Now().Add(time.Second * 10)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	if err := ws.WriteMessage(websocket.TextMessage, messagePayload); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
}

func (ws *WSClient) recvBasic(t *testing.T, expectedAction string) {
	if err := ws.SetReadDeadline(time.Now().Add(time.Second * 10)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

	_, p, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var message NetworkWebSocketWireMessage
	if err := json.Unmarshal(p, &message); err != nil {
		t.Fatalf("NetworkWebSocketWireMessage JSON Unmarshaling: %s", err)
	}

	if message.Action != expectedAction {
		t.Fatalf("action=%s, want %s", message.Action, expectedAction)
	}
}

// Receive message from Network Web Socket channel peer connection
func (ws *WSClient) recv(t *testing.T, expectedAction string, expectedSource, expectedTarget string, expectedPayload string) {
	if err := ws.SetReadDeadline(time.Now().Add(time.Second * 10)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

	_, p, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var message NetworkWebSocketWireMessage
	if err := json.Unmarshal(p, &message); err != nil {
		t.Fatalf("NetworkWebSocketWireMessage JSON Unmarshaling: %s", err)
	}

	if message.Action != expectedAction {
		t.Fatalf("action=%s, want %s", message.Action, expectedAction)
	}

	if message.Source != expectedSource {
		t.Fatalf("source=%s, want %s", message.Source, expectedSource)
	}
	// It is tricky to determine the correct target order during network service discovery
	// as different clients will connect to each other at different times. Thus, we shall
	// only check that the target != source. Also, we check that we receive the correct
	// number of 'connect', 'message' and 'disconnect' messages in the individual tests.
	if message.Target == expectedSource {
		t.Fatalf("target=%s, don't want %s", message.Target, expectedSource)
	}
	if string(message.Payload) != expectedPayload {
		t.Fatalf("message=%s, want %s", message.Payload, expectedPayload)
	}
}

func TestConnection_SameOrigin_Broadcast(t *testing.T) {
	// Make named websocket test server
	s1 := makeService("localhost", 9023)

	_ = s1.Start()

	// Make named websocket test clients
	c1 := makeClient(t, "localhost:9023", "/testservice_C")
	c2 := makeClient(t, "localhost:9023", "/testservice_C")
	c3 := makeClient(t, "localhost:9023", "/testservice_C")

	// Test connect control messages
	c1.recvBasic(t, "connect")
	c1.recvBasic(t, "connect")
	c2.recvBasic(t, "connect")
	c2.recvBasic(t, "connect")
	c3.recvBasic(t, "connect")
	c3.recvBasic(t, "connect")

	// Obtain connection identifiers
	c1_Id := c1.getSelfId(t)
	c2_Id := c2.getSelfId(t)
	c3_Id := c3.getSelfId(t)

	// Test broadcast ( c1 -> [c2, c3] )
	c1.send(t, "broadcast", "", "", "A_HelloFrom1")
	c2.recv(t, "broadcast", c1_Id, "", "A_HelloFrom1")
	c3.recv(t, "broadcast", c1_Id, "", "A_HelloFrom1")

	// Test broadcast ( c2 -> [c1, c3] )
	c2.send(t, "broadcast", "", "", "A_HelloFrom2")
	c1.recv(t, "broadcast", c2_Id, "", "A_HelloFrom2")
	c3.recv(t, "broadcast", c2_Id, "", "A_HelloFrom2")

	// Test broadcast ( c3 -> [c1, c2] )
	c3.send(t, "broadcast", "", "", "A_HelloFrom3")
	c1.recv(t, "broadcast", c3_Id, "", "A_HelloFrom3")
	c2.recv(t, "broadcast", c3_Id, "", "A_HelloFrom3")

	// Close connection 1 and test disconnect control messages against not-yet-closed connections
	c1.Close()
	c2.recv(t, "disconnect", c2_Id, c1_Id, "")
	c3.recv(t, "disconnect", c3_Id, c1_Id, "")

	// Close connection 2 and test disconnect control messages against not-yet-closed connections
	c2.Close()
	c3.recv(t, "disconnect", c3_Id, c2_Id, "")

	// Close connection 3
	c3.Close()
}

func TestConnection_SameOrigin_DirectMessaging(t *testing.T) {
	// Make named websocket test servers
	s1 := makeService("localhost", 9024)

	_ = s1.Start()

	// Make named websocket test clients
	c1 := makeClient(t, "localhost:9024", "/testservice_D")
	c2 := makeClient(t, "localhost:9024", "/testservice_D")
	c3 := makeClient(t, "localhost:9024", "/testservice_D")
	c4 := makeClient(t, "localhost:9024", "/testservice_D")

	// Test connect control messages
	c1.recvBasic(t, "connect")
	c1.recvBasic(t, "connect")
	c1.recvBasic(t, "connect")
	c2.recvBasic(t, "connect")
	c2.recvBasic(t, "connect")
	c2.recvBasic(t, "connect")
	c3.recvBasic(t, "connect")
	c3.recvBasic(t, "connect")
	c3.recvBasic(t, "connect")
	c4.recvBasic(t, "connect")
	c4.recvBasic(t, "connect")
	c4.recvBasic(t, "connect")

	// Obtain connection identifiers
	c1_Id := c1.getSelfId(t)
	c2_Id := c2.getSelfId(t)
	c3_Id := c3.getSelfId(t)
	c4_Id := c4.getSelfId(t)

	// Test direct message ( c1 -> c2 )
	c1.send(t, "message", c1_Id, c2_Id, "C_HelloFrom1To2")
	c2.recv(t, "message", c1_Id, c2_Id, "C_HelloFrom1To2")

	// Test direct message ( c1 -> c3 )
	c1.send(t, "message", c1_Id, c3_Id, "C_HelloFrom1To3")
	c3.recv(t, "message", c1_Id, c3_Id, "C_HelloFrom1To3")

	// Test direct message ( c1 -> c4 )
	c1.send(t, "message", c1_Id, c4_Id, "C_HelloFrom1To4")
	c4.recv(t, "message", c1_Id, c4_Id, "C_HelloFrom1To4")

	// Test direct message ( c2 -> c1 )
	c2.send(t, "message", c2_Id, c1_Id, "C_HelloFrom2To1")
	c1.recv(t, "message", c2_Id, c1_Id, "C_HelloFrom2To1")

	// Test direct message ( c2 -> c3 )
	c2.send(t, "message", c2_Id, c3_Id, "C_HelloFrom2To3")
	c3.recv(t, "message", c2_Id, c3_Id, "C_HelloFrom2To3")

	// Test direct message ( c2 -> c4 )
	c2.send(t, "message", c2_Id, c4_Id, "C_HelloFrom2To4")
	c4.recv(t, "message", c2_Id, c4_Id, "C_HelloFrom2To4")

	// Test direct message ( c3 -> c1 )
	c3.send(t, "message", c3_Id, c1_Id, "C_HelloFrom3To1")
	c1.recv(t, "message", c3_Id, c1_Id, "C_HelloFrom3To1")

	// Test direct message ( c3 -> c2 )
	c3.send(t, "message", c3_Id, c2_Id, "C_HelloFrom3To2")
	c2.recv(t, "message", c3_Id, c2_Id, "C_HelloFrom3To2")

	// Test direct message ( c3 -> c4 )
	c3.send(t, "message", c3_Id, c4_Id, "C_HelloFrom3To4")
	c4.recv(t, "message", c3_Id, c4_Id, "C_HelloFrom3To4")

	// Test direct message ( c4 -> c1 )
	c4.send(t, "message", c4_Id, c1_Id, "C_HelloFrom4To1")
	c1.recv(t, "message", c4_Id, c1_Id, "C_HelloFrom4To1")

	// Test direct message ( c4 -> c2 )
	c4.send(t, "message", c4_Id, c2_Id, "C_HelloFrom4To2")
	c2.recv(t, "message", c4_Id, c2_Id, "C_HelloFrom4To2")

	// Test direct message ( c4 -> c3 )
	c4.send(t, "message", c4_Id, c3_Id, "C_HelloFrom4To3")
	c3.recv(t, "message", c4_Id, c3_Id, "C_HelloFrom4To3")

	// Close connection 1 and test disconnect control messages against not-yet-closed connections
	c1.Close()
	c2.recv(t, "disconnect", c2_Id, c1_Id, "")
	c3.recv(t, "disconnect", c3_Id, c1_Id, "")
	c4.recv(t, "disconnect", c4_Id, c1_Id, "")

	// Close connection 2 and test disconnect control messages against not-yet-closed connections
	c2.Close()
	c3.recv(t, "disconnect", c3_Id, c2_Id, "")
	c4.recv(t, "disconnect", c4_Id, c2_Id, "")

	// Close connection 3 and test disconnect control messages against not-yet-closed connections
	c3.Close()
	c4.recv(t, "disconnect", c4_Id, c3_Id, "")

	// Close connection 4
	c4.Close()
}
