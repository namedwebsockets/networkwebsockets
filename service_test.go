package namedwebsockets

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Make a new Named WebSocket server
func makeService(host string, port int) *NamedWebSocket_Service {
	return NewNamedWebSocketService(host, port)
}

type WSClient struct {
	*websocket.Conn
}

// Make a new WebSocket client connection
func makeClient(t *testing.T, host, path string, peerId int) *WSClient {
	if peerId == 0 {
		// Generate unique id for connection
		rand.Seed(time.Now().UTC().UnixNano())
		peerId = rand.Int()
	}
	url := fmt.Sprintf("ws://%s%s/%d", host, path, peerId)
	ws, _, err := websocket.DefaultDialer.Dial(url, map[string][]string{
		"Origin": []string{"localhost"},
	})
	if err != nil {
		t.Fatalf("Websocket client connection failed: %s", err)
	}
	wsClient := &WSClient{ws}
	return wsClient
}

// Send messages to broadcast channel
func (ws *WSClient) send(t *testing.T, message string) {
	if err := ws.SetWriteDeadline(time.Now().Add(time.Second * 10)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	if err := ws.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
}

// Read messages from broadcast channel
func (ws *WSClient) recv(t *testing.T, message string) {
	if err := ws.SetReadDeadline(time.Now().Add(time.Second * 10)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	_, p, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if string(p) != message {
		t.Fatalf("message=%s, want %s", p, message)
	}
}

// Send message to control channel
func (ws *WSClient) sendDirect(t *testing.T, action string, source, target int, payload string) {
	m := ControlWireMessage{
		Action:  action,
		Source:  source,
		Target:  target,
		Payload: payload,
	}
	messagePayload, err := json.Marshal(m)
	if err != nil {
		return
	}

	ws.send(t, string(messagePayload))
}

// Receive message from control channel
func (ws *WSClient) recvDirect(t *testing.T, action string, source, target int, payload string) {
	if err := ws.SetReadDeadline(time.Now().Add(time.Second * 10)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	_, p, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var message ControlWireMessage
	if err := json.Unmarshal(p, &message); err != nil {
		t.Fatalf("ControlWireMessage JSON Unmarshaling: %s", err)
	}

	if message.Action != action {
		t.Fatalf("action=%s, want %s", message.Action, action)
	}

	if message.Source != source {
		t.Fatalf("source=%d, want %d", message.Source, source)
	}
	// It is tricky to determine the correct target order during network service discovery
	// as different clients will connect to each other at different times. Thus, we shall
	// only check that the target != source. Also, we check that we receive the correct
	// number of 'connect', 'message' and 'disconnect' messages in the individual tests.
	if message.Target == source {
		t.Fatalf("target=%d, don't want %d", message.Target, source)
	}
	if string(message.Payload) != payload {
		t.Fatalf("message=%s, want %s", message.Payload, payload)
	}
}

func TestLocalConnection_Broadcast(t *testing.T) {
	// Make named websocket test server
	s1 := makeService("localhost", 9021)
	// go s1.StartNamedWebSocketServer() // port: 9022
	go s1.StartHTTPServer()

	// Define connection identifiers
	const (
		c1_Id = 11111
		c2_Id = 22221
		c3_Id = 33331
		c4_Id = 44441
	)

	// Make named websocket test clients
	c1 := makeClient(t, "localhost:9021", "/local/testservice_A", c1_Id)
	c2 := makeClient(t, "localhost:9021", "/local/testservice_A", c2_Id)
	c3 := makeClient(t, "localhost:9021", "/local/testservice_A", c3_Id)

	// Make named websocket test client controllers
	c1_control := makeClient(t, "localhost:9021", "/control/local/testservice_A", c1_Id)
	c2_control := makeClient(t, "localhost:9021", "/control/local/testservice_A", c2_Id)
	c3_control := makeClient(t, "localhost:9021", "/control/local/testservice_A", c3_Id)

	defer func() {
		c1_control.Close()
		c2_control.Close()
		c3_control.Close()
	}()

	// Test connect control messages
	c1_control.recvDirect(t, "connect", c1_Id, c2_Id, "")
	c1_control.recvDirect(t, "connect", c1_Id, c3_Id, "")
	c2_control.recvDirect(t, "connect", c2_Id, c1_Id, "")
	c2_control.recvDirect(t, "connect", c2_Id, c3_Id, "")
	c3_control.recvDirect(t, "connect", c3_Id, c1_Id, "")
	c3_control.recvDirect(t, "connect", c3_Id, c2_Id, "")

	// Test broadcast ( c1 -> [c2, c3] )
	c1.send(t, "A_HelloFrom1")
	c2.recv(t, "A_HelloFrom1")
	c3.recv(t, "A_HelloFrom1")

	// Test broadcast ( c2 -> [c1, c3] )
	c2.send(t, "A_HelloFrom2")
	c1.recv(t, "A_HelloFrom2")
	c3.recv(t, "A_HelloFrom2")

	// Test broadcast ( c3 -> [c1, c2] )
	c3.send(t, "A_HelloFrom3")
	c1.recv(t, "A_HelloFrom3")
	c2.recv(t, "A_HelloFrom3")

	// Close connection 1 and test disconnect control messages against not-yet-closed connections
	c1.Close()
	c2_control.recvDirect(t, "disconnect", c2_Id, c1_Id, "")
	c3_control.recvDirect(t, "disconnect", c3_Id, c1_Id, "")

	// Close connection 2 and test disconnect control messages against not-yet-closed connections
	c2.Close()
	c3_control.recvDirect(t, "disconnect", c3_Id, c2_Id, "")

	// Close connection 3
	c3.Close()
}

func TestNetworkConnection_Broadcast(t *testing.T) {
	// Make named websocket test servers
	s1 := makeService("localhost", 9023)
	go s1.StartNamedWebSocketServer() // port: 9024
	go s1.StartDiscoveryServer()
	go s1.StartHTTPServer()

	s2 := makeService("localhost", 9025)
	go s2.StartNamedWebSocketServer() // port: 9026
	go s2.StartDiscoveryServer()
	go s2.StartHTTPServer()

	s3 := makeService("localhost", 9027)
	go s3.StartNamedWebSocketServer() // port: 9028
	go s3.StartDiscoveryServer()
	go s3.StartHTTPServer()

	// Define connection identifiers
	const (
		c1_Id = 11112
		c2_Id = 22222
		c3_Id = 33332
		c4_Id = 44442
	)

	// Make named websocket test clients
	c1 := makeClient(t, "localhost:9023", "/network/testservice_B", c1_Id)
	c2 := makeClient(t, "localhost:9023", "/network/testservice_B", c2_Id)
	c3 := makeClient(t, "localhost:9025", "/network/testservice_B", c3_Id)
	c4 := makeClient(t, "localhost:9027", "/network/testservice_B", c4_Id)

	// Make named websocket test client controllers
	c1_control := makeClient(t, "localhost:9023", "/control/network/testservice_B", c1_Id)
	c2_control := makeClient(t, "localhost:9023", "/control/network/testservice_B", c2_Id)
	c3_control := makeClient(t, "localhost:9025", "/control/network/testservice_B", c3_Id)
	c4_control := makeClient(t, "localhost:9027", "/control/network/testservice_B", c4_Id)

	defer func() {
		c1_control.Close()
		c2_control.Close()
		c3_control.Close()
		c4_control.Close()
	}()

	// Test connect control messages
	c1_control.recvDirect(t, "connect", c1_Id, c2_Id, "")
	c1_control.recvDirect(t, "connect", c1_Id, c3_Id, "")
	c1_control.recvDirect(t, "connect", c1_Id, c4_Id, "")
	c2_control.recvDirect(t, "connect", c2_Id, c1_Id, "")
	c2_control.recvDirect(t, "connect", c2_Id, c3_Id, "")
	c2_control.recvDirect(t, "connect", c2_Id, c4_Id, "")
	c3_control.recvDirect(t, "connect", c3_Id, c1_Id, "")
	c3_control.recvDirect(t, "connect", c3_Id, c2_Id, "")
	c3_control.recvDirect(t, "connect", c3_Id, c4_Id, "")
	c4_control.recvDirect(t, "connect", c4_Id, c1_Id, "")
	c4_control.recvDirect(t, "connect", c4_Id, c2_Id, "")
	c4_control.recvDirect(t, "connect", c4_Id, c3_Id, "")

	// Test broadcast -> receive ( c1 -> [c2, c3, c4] )
	c1.send(t, "B_HelloFrom1")
	c2.recv(t, "B_HelloFrom1")
	c3.recv(t, "B_HelloFrom1")
	c4.recv(t, "B_HelloFrom1")

	// Test broadcast -> receive ( c2 -> [c1, c3, c4] )
	c2.send(t, "B_HelloFrom2")
	c1.recv(t, "B_HelloFrom2")
	c3.recv(t, "B_HelloFrom2")
	c4.recv(t, "B_HelloFrom2")

	// Test broadcast -> receive ( c3 -> [c1, c2, c4] )
	c3.send(t, "B_HelloFrom3")
	c1.recv(t, "B_HelloFrom3")
	c2.recv(t, "B_HelloFrom3")
	c4.recv(t, "B_HelloFrom3")

	// Test broadcast -> receive ( c4 -> [c1, c2, c3] )
	c4.send(t, "B_HelloFrom4")
	c1.recv(t, "B_HelloFrom4")
	c2.recv(t, "B_HelloFrom4")
	c3.recv(t, "B_HelloFrom4")

	// Close connection 1 and test disconnect control messages against not-yet-closed connections
	c1.Close()
	c2_control.recvDirect(t, "disconnect", c2_Id, c1_Id, "")
	c3_control.recvDirect(t, "disconnect", c3_Id, c1_Id, "")
	c4_control.recvDirect(t, "disconnect", c4_Id, c1_Id, "")

	// Close connection 2 and test disconnect control messages against not-yet-closed connections
	c2.Close()
	c3_control.recvDirect(t, "disconnect", c3_Id, c2_Id, "")
	c4_control.recvDirect(t, "disconnect", c4_Id, c2_Id, "")

	// Close connection 3 and test disconnect control messages against not-yet-closed connections
	c3.Close()
	c4_control.recvDirect(t, "disconnect", c4_Id, c3_Id, "")

	// Close connection 4
	c4.Close()
}

func TestNetworkConnection_DirectMessaging(t *testing.T) {
	// Make named websocket test servers
	s1 := makeService("localhost", 9029)
	go s1.StartNamedWebSocketServer() // port: 9030
	go s1.StartDiscoveryServer()
	go s1.StartHTTPServer()

	s2 := makeService("localhost", 9031)
	go s2.StartNamedWebSocketServer() // port: 9032
	go s2.StartDiscoveryServer()
	go s2.StartHTTPServer()

	s3 := makeService("localhost", 9033)
	go s3.StartNamedWebSocketServer() // port: 9034
	go s3.StartDiscoveryServer()
	go s3.StartHTTPServer()

	// Define connection identifiers
	const (
		c1_Id = 11113
		c2_Id = 22223
		c3_Id = 33333
		c4_Id = 44443
	)

	// Make named websocket test clients
	c1 := makeClient(t, "localhost:9029", "/network/testservice_C", c1_Id)
	c2 := makeClient(t, "localhost:9031", "/network/testservice_C", c2_Id)
	c3 := makeClient(t, "localhost:9031", "/network/testservice_C", c3_Id)
	c4 := makeClient(t, "localhost:9033", "/network/testservice_C", c4_Id)

	// Make named websocket test client controllers
	c1_control := makeClient(t, "localhost:9029", "/control/network/testservice_C", c1_Id)
	c2_control := makeClient(t, "localhost:9031", "/control/network/testservice_C", c2_Id)
	c3_control := makeClient(t, "localhost:9031", "/control/network/testservice_C", c3_Id)
	c4_control := makeClient(t, "localhost:9033", "/control/network/testservice_C", c4_Id)

	defer func() {
		c1_control.Close()
		c2_control.Close()
		c3_control.Close()
		c4_control.Close()
	}()

	// Test connect control messages
	c1_control.recvDirect(t, "connect", c1_Id, c2_Id, "")
	c1_control.recvDirect(t, "connect", c1_Id, c3_Id, "")
	c1_control.recvDirect(t, "connect", c1_Id, c4_Id, "")
	c2_control.recvDirect(t, "connect", c2_Id, c1_Id, "")
	c2_control.recvDirect(t, "connect", c2_Id, c3_Id, "")
	c2_control.recvDirect(t, "connect", c2_Id, c4_Id, "")
	c3_control.recvDirect(t, "connect", c3_Id, c1_Id, "")
	c3_control.recvDirect(t, "connect", c3_Id, c2_Id, "")
	c3_control.recvDirect(t, "connect", c3_Id, c4_Id, "")
	c4_control.recvDirect(t, "connect", c4_Id, c1_Id, "")
	c4_control.recvDirect(t, "connect", c4_Id, c2_Id, "")
	c4_control.recvDirect(t, "connect", c4_Id, c3_Id, "")

	// Test direct message ( c1 -> c2 )
	c1_control.sendDirect(t, "message", c1_Id, c2_Id, "C_HelloFrom1To2")
	c2_control.recvDirect(t, "message", c1_Id, c2_Id, "C_HelloFrom1To2")

	// Test direct message ( c1 -> c3 )
	c1_control.sendDirect(t, "message", c1_Id, c3_Id, "C_HelloFrom1To3")
	c3_control.recvDirect(t, "message", c1_Id, c3_Id, "C_HelloFrom1To3")

	// Test direct message ( c1 -> c4 )
	c1_control.sendDirect(t, "message", c1_Id, c4_Id, "C_HelloFrom1To4")
	c4_control.recvDirect(t, "message", c1_Id, c4_Id, "C_HelloFrom1To4")

	// Test direct message ( c2 -> c1 )
	c2_control.sendDirect(t, "message", c2_Id, c1_Id, "C_HelloFrom2To1")
	c1_control.recvDirect(t, "message", c2_Id, c1_Id, "C_HelloFrom2To1")

	// Test direct message ( c2 -> c3 )
	c2_control.sendDirect(t, "message", c2_Id, c3_Id, "C_HelloFrom2To3")
	c3_control.recvDirect(t, "message", c2_Id, c3_Id, "C_HelloFrom2To3")

	// Test direct message ( c2 -> c4 )
	c2_control.sendDirect(t, "message", c2_Id, c4_Id, "C_HelloFrom2To4")
	c4_control.recvDirect(t, "message", c2_Id, c4_Id, "C_HelloFrom2To4")

	// Test direct message ( c3 -> c1 )
	c3_control.sendDirect(t, "message", c3_Id, c1_Id, "C_HelloFrom3To1")
	c1_control.recvDirect(t, "message", c3_Id, c1_Id, "C_HelloFrom3To1")

	// Test direct message ( c3 -> c2 )
	c3_control.sendDirect(t, "message", c3_Id, c2_Id, "C_HelloFrom3To2")
	c2_control.recvDirect(t, "message", c3_Id, c2_Id, "C_HelloFrom3To2")

	// Test direct message ( c3 -> c4 )
	c3_control.sendDirect(t, "message", c3_Id, c4_Id, "C_HelloFrom3To4")
	c4_control.recvDirect(t, "message", c3_Id, c4_Id, "C_HelloFrom3To4")

	// Test direct message ( c4 -> c1 )
	c4_control.sendDirect(t, "message", c4_Id, c1_Id, "C_HelloFrom4To1")
	c1_control.recvDirect(t, "message", c4_Id, c1_Id, "C_HelloFrom4To1")

	// Test direct message ( c4 -> c2 )
	c4_control.sendDirect(t, "message", c4_Id, c2_Id, "C_HelloFrom4To2")
	c2_control.recvDirect(t, "message", c4_Id, c2_Id, "C_HelloFrom4To2")

	// Test direct message ( c4 -> c3 )
	c4_control.sendDirect(t, "message", c4_Id, c3_Id, "C_HelloFrom4To3")
	c3_control.recvDirect(t, "message", c4_Id, c3_Id, "C_HelloFrom4To3")

	// Close connection 1 and test disconnect control messages against not-yet-closed connections
	c1.Close()
	c2_control.recvDirect(t, "disconnect", c2_Id, c1_Id, "")
	c3_control.recvDirect(t, "disconnect", c3_Id, c1_Id, "")
	c4_control.recvDirect(t, "disconnect", c4_Id, c1_Id, "")

	// Close connection 2 and test disconnect control messages against not-yet-closed connections
	c2.Close()
	c3_control.recvDirect(t, "disconnect", c3_Id, c2_Id, "")
	c4_control.recvDirect(t, "disconnect", c4_Id, c2_Id, "")

	// Close connection 3 and test disconnect control messages against not-yet-closed connections
	c3.Close()
	c4_control.recvDirect(t, "disconnect", c4_Id, c3_Id, "")

	// Close connection 4
	c4.Close()
}
