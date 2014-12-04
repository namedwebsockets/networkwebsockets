package networkwebsockets

import (
	"encoding/json"
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

func NewPeerConnection(channel *NetworkWebSocket, conn *websocket.Conn) *PeerConnection {
	peerConn := &PeerConnection{
		id:      GenerateId(),
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
func (peer *PeerConnection) send(action string, source string, target string, payload string) {
	// Construct proxy wire message
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

	peer.conn.SetWriteDeadline(time.Now().Add(writeWait))
	peer.conn.WriteMessage(websocket.TextMessage, messagePayload)
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
		opCode, buf, err := peer.conn.ReadMessage()
		if err != nil || opCode != websocket.TextMessage {
			break
		}

		var message NetworkWebSocketWireMessage
		if err := json.Unmarshal(buf, &message); err != nil {
			continue // ignore unrecognized message format
		}

		switch message.Action {
		// 'connect' and 'disconnect' events are write-only so will not be handled here

		case "status":

			// Echo peer id back to callee
			peer.send("status", peer.id, peer.id, "")

		case "broadcast":

			wsBroadcast := &NetworkWebSocketWireMessage{
				Action:    "broadcast",
				Source:    peer.id,
				Target:    "", // target all connections
				Payload:   string(message.Payload),
				fromProxy: false,
			}
			peer.channel.broadcastBuffer <- wsBroadcast

		case "message":

			messageSent := false

			// Relay message to peer channel that matches target
			for _, _peer := range peer.channel.peers {
				if _peer.id == message.Target {
					_peer.send("message", peer.id, message.Target, message.Payload)
					messageSent = true
					break
				}
			}

			if !messageSent {
				// Hunt for proxy that owns target peer id in known proxies
				for _, proxy := range peer.channel.proxies {
					if proxy.peerIds[message.Target] {
						proxy.send("message", peer.id, message.Target, message.Payload)
						messageSent = true
						break
					}
				}
			}
		}

	}
}

// writeConnectionPump keeps an individual websocket connection alive
func (peer *PeerConnection) writeConnectionPump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
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

	for _, _peer := range peer.channel.peers {
		if _peer.id != peer.id {
			// Inform other local peer connections that we now own this peer
			_peer.send("connect", _peer.id, peer.id, "")

			// Inform this peer of all the other peer connections we own
			peer.send("connect", peer.id, _peer.id, "")
		}
	}

	for _, proxy := range peer.channel.proxies {
		// Inform all proxy connections that we now own this peer connection
		if proxy.writeable {
			proxy.send("connect", proxy.base.id, peer.id, "")
		}
		// Inform current peer of all the peer connections other connected proxies own
		for peerId, _ := range proxy.peerIds {
			peer.send("connect", proxy.base.id, peerId, "")
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

	// Inform all local peer connections that we no longer own this peer connection
	for _, _peer := range peer.channel.peers {
		// don't notify peer if its id matches the peer's id
		if _peer.id != peer.id {
			_peer.send("disconnect", _peer.id, peer.id, "")
		}
	}

	// Inform all proxy connections that we no longer own this peer connection
	for _, proxy := range peer.channel.proxies {
		if proxy.writeable {
			proxy.send("disconnect", proxy.base.id, peer.id, "")
		}
	}

	peer.conn.Close()

	// If no more local peers are connected then remove the current Named Web Socket service
	if len(peer.channel.peers) == 0 {
		peer.channel.Stop()
	}
}

func (peer *PeerConnection) Stop() {
	// Remove references to this peer connection from channel
	peer.removeConnection()
}
