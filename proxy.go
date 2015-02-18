package networkwebsockets

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/richtr/websocket"
)

type ProxyConnection struct {
	// Inherit attributes from PeerConnection struct
	base PeerConnection

	// Discovered proxy connection's base64 hash value
	// empty unless set via .setHash_Base64()
	Hash_Base64 string

	// List of connection ids that this proxy connection 'owns'
	peerIds map[string]bool

	// Whether this proxy connection is writeable
	writeable bool
}

func NewProxyConnection(conn *websocket.Conn, isWriteable bool) *ProxyConnection {
	proxyConn := &ProxyConnection{
		base: PeerConnection{
			id:   GenerateId(),
			conn: conn,
		},
		Hash_Base64: "",
		writeable:   isWriteable,
		peerIds:     make(map[string]bool),
	}

	return proxyConn
}

func (proxy *ProxyConnection) JoinChannel(channel *NetworkWebSocket) {
	if channel == nil {
		return
	}

	proxy.base.channel = channel

	// Add reference to this proxy connection to channel
	proxy.addConnection()
}

func (proxy *ProxyConnection) LeaveChannel() {
	if proxy.base.channel == nil {
		return
	}

	// Add reference to this peer connection to channel
	proxy.removeConnection()

	proxy.base.channel = nil
}

func (proxy *ProxyConnection) Start() error {
	if proxy.base.channel == nil {
		return errors.New("ProxyConnection does not have a channel. You must invoke .JoinGroup before .Start")
	}

	if proxy.base.active {
		return errors.New("ProxyConnection is already started")
	}

	// Start connection read/write pumps
	go proxy.writeConnectionPump()
	go proxy.readConnectionPump()

	proxy.base.active = true

	return nil
}

func (proxy *ProxyConnection) Stop() error {
	if !proxy.base.active {
		return errors.New("ProxyConnection cannot be stopped because it is not currently active")
	}

	// Remove references to this proxy connection from channel
	proxy.removeConnection()

	// Close underlying websocket connection
	proxy.base.conn.Close()

	proxy.base.active = false

	return nil
}

// Send a message to the target websocket connection
func (proxy *ProxyConnection) send(action string, source string, target string, payload string) {
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

	proxy.base.conn.SetWriteDeadline(time.Now().Add(writeWait))
	proxy.base.conn.WriteMessage(websocket.TextMessage, messagePayload)
}

func (proxy *ProxyConnection) setHash_Base64(hash string) {
	proxy.Hash_Base64 = hash
}

// readConnectionPump pumps messages from an individual websocket connection to the dispatcher
func (proxy *ProxyConnection) readConnectionPump() {
	defer func() {
		proxy.Stop()
	}()
	proxy.base.conn.SetReadLimit(maxMessageSize)
	proxy.base.conn.SetReadDeadline(time.Now().Add(pongWait))
	proxy.base.conn.SetPongHandler(func(string) error { proxy.base.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		opCode, buf, err := proxy.base.conn.ReadMessage()
		if err != nil || opCode != websocket.TextMessage {
			break
		}

		var message NetworkWebSocketWireMessage
		if err = json.Unmarshal(buf, &message); err != nil {
			continue // ignore unrecognized message format
		}

		switch message.Action {
		case "connect":

			proxy.peerIds[message.Target] = true

			// Inform all local peer connections that this proxy owns this peer connection
			for _, peer := range proxy.base.channel.peers {
				peer.send("connect", peer.id, message.Target, "")
			}

		case "disconnect":

			delete(proxy.peerIds, message.Target)

			// Inform all local peer connections that this proxy no longer owns this peer connection
			for _, peer := range proxy.base.channel.peers {
				peer.send("disconnect", peer.id, message.Target, "")
			}

		case "broadcast":

			// broadcast message on to given target
			wsBroadcast := &NetworkWebSocketWireMessage{
				Action:    "broadcast",
				Source:    message.Source,
				Target:    "", // target all connections
				Payload:   message.Payload,
				fromProxy: true,
			}

			proxy.base.channel.broadcastBuffer <- wsBroadcast

		case "message":

			messageSent := false

			// Relay message to channel peer that matches target
			for _, peer := range proxy.base.channel.peers {
				if peer.id == message.Target {
					peer.send("message", message.Source, message.Target, message.Payload)
					messageSent = true
					break
				}
			}

			if !messageSent {
				fmt.Errorf("P2P message target could not be found. Not sent.")
			}
		}
	}
}

// writeConnectionPump keeps an individual websocket connection alive
func (proxy *ProxyConnection) writeConnectionPump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
	}()
	for {
		select {
		case <-ticker.C:
			proxy.base.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := proxy.base.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// Set up a new NetworkWebSocket connection instance
func (proxy *ProxyConnection) addConnection() {
	proxy.base.channel.proxies = append(proxy.base.channel.proxies, proxy)

	if proxy.writeable {
		// Inform this proxy of all the peer connections we own
		for _, peer := range proxy.base.channel.peers {
			proxy.send("connect", proxy.base.id, peer.id, "")
		}
	}
}

// Tear down an existing NetworkWebSocket connection instance
func (proxy *ProxyConnection) removeConnection() {
	for i, conn := range proxy.base.channel.proxies {
		if proxy.base.id == conn.base.id {
			proxy.base.channel.proxies = append(proxy.base.channel.proxies[:i], proxy.base.channel.proxies[i+1:]...)
			break
		}
	}

	if proxy.writeable {
		// Inform this proxy of all the peer connections we no longer own
		for _, peer := range proxy.base.channel.peers {
			proxy.send("disconnect", proxy.base.id, peer.id, "")
		}
	}
}
