package namedwebsockets

import (
	"math/rand"
	"time"

	"github.com/gorilla/websocket"
)

type PeerConnection struct {
	// Unique identifier for this peer connection
	id int

	// WebSocket connection object
	ws *websocket.Conn
}

type Message struct {
	// The source peer connection of the message
	source *PeerConnection

	// The destination peer connection id targets
	targets []int

	// The message payload
	payload []byte

	// Whether this message originated from a ProxyConnection object
	fromProxy bool
}

func NewPeerConnection(socket *websocket.Conn) *PeerConnection {
	// Generate unique id for connection
	rand.Seed(time.Now().UTC().UnixNano())
	connId := rand.Int()

	peerConn := &PeerConnection{
		id: connId,
		ws: socket,
	}

	return peerConn
}

// Send a message to the target websocket connection
func (conn *PeerConnection) write(mt int, payload []byte) {
	conn.ws.SetWriteDeadline(time.Now().Add(writeWait))
	conn.ws.WriteMessage(mt, payload)
}

// readConnectionPump pumps messages from an individual websocket connection to the dispatcher
func (peer *PeerConnection) readConnectionPump(sock *NamedWebSocket) {
	defer func() {
		peer.removeConnection(sock)
	}()
	peer.ws.SetReadLimit(maxMessageSize)
	peer.ws.SetReadDeadline(time.Now().Add(pongWait))
	peer.ws.SetPongHandler(func(string) error { peer.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := peer.ws.ReadMessage()
		if err != nil {
			break
		}

		wsBroadcast := &Message{
			source:    peer,
			targets:   []int{-1}, // target all connections
			payload:   message,
			fromProxy: false,
		}

		sock.broadcastBuffer <- wsBroadcast
	}
}

// writeConnectionPump keeps an individual websocket connection alive
func (peer *PeerConnection) writeConnectionPump(sock *NamedWebSocket) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		peer.removeConnection(sock)
	}()
	for {
		select {
		case <-ticker.C:
			peer.write(websocket.PingMessage, []byte{})
		}
	}
}

// Set up a new NamedWebSocket connection instance
func (peer *PeerConnection) addConnection(sock *NamedWebSocket) {
	// Add this websocket instance to Named WebSocket broadcast list
	sock.peers = append(sock.peers, peer)

	// Inform all proxy connections that we own this peer connection
	for _, proxy := range sock.proxies {
		if proxy.writeable {
			proxy.write(websocket.TextMessage, "connect", []int{peer.id}, []byte{})
		}
	}

	// Start connection read/write pumps
	go peer.writeConnectionPump(sock)
	go peer.readConnectionPump(sock)
}

// Tear down an existing NamedWebSocket connection instance
func (peer *PeerConnection) removeConnection(sock *NamedWebSocket) {
	for i, conn := range sock.peers {
		if conn.id == peer.id {
			sock.peers = append(sock.peers[:i], sock.peers[i+1:]...)
			break
		}
	}

	// Inform all proxy connections that we no longer own this peer connection
	for _, proxy := range sock.proxies {
		if proxy.writeable {
			proxy.write(websocket.TextMessage, "disconnect", []int{peer.id}, []byte{})
		}
	}

	peer.ws.Close()
}
