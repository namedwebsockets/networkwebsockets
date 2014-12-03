package networkwebsockets

import (
	"time"

	"github.com/richtr/websocket"
)

type PeerConnection struct {
	// Unique identifier for this peer connection
	id string

	// The Network Web Socket channel to which this peer connection belongs
	channel *NetworkWebSocket

	// WebSocket connection object
	conn *websocket.Conn
}

type Message struct {
	// The source peer connection of the message
	source string

	// The destination peer connection id target
	target string

	// The message payload
	payload string

	// Whether this message originated from a ProxyConnection object
	fromProxy bool
}

func NewPeerConnection(channel *NetworkWebSocket, id string, conn *websocket.Conn) *PeerConnection {
	peerConn := &PeerConnection{
		id:      id,
		channel: channel,
		conn:    conn,
	}

	// Start websocket read/write pumps
	peerConn.Start()

	return peerConn
}

func (peer *PeerConnection) Start() {
	// Start connection read/write pumps
	go peer.writeConnectionPump()
	go peer.readConnectionPump()

	// Add reference to this peer connection to channel
	peer.addConnection()
}

// Send a message to the target websocket connection
func (peer *PeerConnection) send(payload string) {
	peer.conn.SetWriteDeadline(time.Now().Add(writeWait))
	peer.conn.WriteMessage(websocket.TextMessage, []byte(payload))
}

// readConnectionPump pumps messages from an individual websocket connection to the dispatcher
func (peer *PeerConnection) readConnectionPump() {
	defer func() {
		peer.Stop()
	}()
	peer.conn.SetReadLimit(maxMessageSize)
	peer.conn.SetReadDeadline(time.Now().Add(pongWait))
	peer.conn.SetPongHandler(func(string) error { peer.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		opCode, message, err := peer.conn.ReadMessage()
		if err != nil || opCode != websocket.TextMessage {
			break
		}

		wsBroadcast := &Message{
			source:    peer.id,
			target:    "", // target all connections
			payload:   string(message),
			fromProxy: false,
		}

		peer.channel.broadcastBuffer <- wsBroadcast
	}
}

// writeConnectionPump keeps an individual websocket connection alive
func (peer *PeerConnection) writeConnectionPump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		peer.Stop()
	}()
	for {
		select {
		case <-ticker.C:
			peer.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := peer.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// Set up a new NetworkWebSocket connection instance
func (peer *PeerConnection) addConnection() {
	// Add this websocket instance to Named WebSocket broadcast list
	peer.channel.peers = append(peer.channel.peers, peer)

	// Inform all control connections that we now own this peer connection
	for _, control := range peer.channel.controllers {
		// don't notify controller if its id matches the peer's id
		if control.base.id != peer.id {
			control.send("connect", control.base.id, peer.id, "")
		}
	}

	// Inform all proxy connections that we now own this peer connection
	for _, proxy := range peer.channel.proxies {
		if proxy.writeable {
			proxy.send("connect", proxy.base.id, peer.id, "")
		}
	}
}

// Tear down an existing NetworkWebSocket connection instance
func (peer *PeerConnection) removeConnection() {
	for i, conn := range peer.channel.peers {
		if conn.id == peer.id {
			peer.channel.peers = append(peer.channel.peers[:i], peer.channel.peers[i+1:]...)
			break
		}
	}

	peer.conn.Close()

	// Find associated control connection and close also
	for _, control := range peer.channel.controllers {
		if control.base.id == peer.id {
			control.Stop()
			break
		}
	}

	// Inform all control connections that we no longer own this peer connection
	for _, control := range peer.channel.controllers {
		// don't notify controller if its id matches the peer's id
		if control.base.id != peer.id {
			control.send("disconnect", control.base.id, peer.id, "")
		}
	}

	// Inform all proxy connections that we no longer own this peer connection
	for _, proxy := range peer.channel.proxies {
		if proxy.writeable {
			proxy.send("disconnect", proxy.base.id, peer.id, "")
		}
	}

	// If no more local peers are connected then remove the current Named Web Socket service
	if len(peer.channel.peers) == 0 {
		peer.channel.Stop()
	}
}

func (peer *PeerConnection) Stop() {
	// Remove references to this control connection from channel
	peer.removeConnection()
}
