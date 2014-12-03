package networkwebsockets

import (
	"encoding/json"
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

type ProxyWireMessage struct {
	// Proxy message type: "connect", "disconnect", "message", "directmessage"
	Action string `json:"action"`

	Source string `json:"source,omitempty"`

	// Recipients' id list (empty string === send to all peers)
	Target string `json:"target,omitempty"`

	// Raw message contents
	Payload string `json:"data,omitempty"`
}

func NewProxyConnection(channel *NetworkWebSocket, id string, conn *websocket.Conn, isWriteable bool) *ProxyConnection {
	proxyConn := &ProxyConnection{
		base: PeerConnection{
			id:      id,
			channel: channel,
			conn:    conn,
		},
		Hash_Base64: "",
		writeable:   isWriteable,
		peerIds:     make(map[string]bool),
	}

	// Start websocket read/write pumps
	proxyConn.Start()

	return proxyConn
}

func (proxy *ProxyConnection) Start() {
	// Start connection read/write pumps
	go proxy.writeConnectionPump()
	go proxy.readConnectionPump()

	// Add reference to this proxy connection to channel
	proxy.addConnection()
}

// Send a message to the target websocket connection
func (proxy *ProxyConnection) send(action string, source string, target string, payload string) {
	// Construct proxy wire message
	m := ProxyWireMessage{
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

		var message ProxyWireMessage
		if err = json.Unmarshal(buf, &message); err != nil {
			continue // ignore unrecognized message format
		}

		switch message.Action {

		case "connect":

			proxy.peerIds[message.Target] = true

			// Inform all control connections that this proxy owns this peer connection
			for _, control := range proxy.base.channel.controllers {
				control.send("connect", control.base.id, message.Target, "")
			}

		case "disconnect":

			delete(proxy.peerIds, message.Target)

			// Inform all control connections that this proxy no longer owns this peer connection
			for _, control := range proxy.base.channel.controllers {
				control.send("disconnect", control.base.id, message.Target, "")
			}

		case "message":

			// broadcast message on to given target
			wsBroadcast := &Message{
				source:    message.Source,
				target:    "", // target all connections
				payload:   message.Payload,
				fromProxy: true,
			}

			proxy.base.channel.broadcastBuffer <- wsBroadcast

		case "directmessage":

			messageSent := false

			// Relay message to control channel that matches target
			for _, control := range proxy.base.channel.controllers {
				if control.base.id == message.Target {
					control.send("message", message.Source, message.Target, message.Payload)
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
		proxy.Stop()
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

	proxy.base.conn.Close()
}

func (proxy *ProxyConnection) Stop() {
	// Remove references to this control connection from channel
	proxy.removeConnection()
}
