package namedwebsockets

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

type ProxyConnection struct {
	// Inherit attributes from PeerConnection struct
	PeerConnection

	// List of connection ids that this proxy connection 'owns'
	peers map[int]bool

	// Whether this proxy connection is writeable
	writeable bool
}

type ProxyWireMessage struct {
	// Proxy message type: "connect", "disconnect", "message", "directmessage"
	Action string

	Source int

	// Recipients' id list (0 === send to all peers)
	Target int

	// Raw message contents
	Payload string
}

func NewProxyConnection(id int, socket *websocket.Conn, isWriteable bool) *ProxyConnection {
	proxyConn := &ProxyConnection{
		PeerConnection: PeerConnection{
			id: id,
			ws: socket,
		},
		writeable: isWriteable,
		peers:     make(map[int]bool),
	}

	return proxyConn
}

// Send a message to the target websocket connection
func (proxy *ProxyConnection) send(action string, source int, target int, payload string) {
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

	proxy.ws.SetWriteDeadline(time.Now().Add(writeWait))
	proxy.ws.WriteMessage(websocket.TextMessage, messagePayload)
}

// readConnectionPump pumps messages from an individual websocket connection to the dispatcher
func (proxy *ProxyConnection) readConnectionPump(sock *NamedWebSocket) {
	defer func() {
		proxy.removeConnection(sock)
	}()
	proxy.ws.SetReadLimit(maxMessageSize)
	proxy.ws.SetReadDeadline(time.Now().Add(pongWait))
	proxy.ws.SetPongHandler(func(string) error { proxy.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		opCode, buf, err := proxy.ws.ReadMessage()
		if err != nil || opCode != websocket.TextMessage {
			break
		}

		var message ProxyWireMessage
		err = json.Unmarshal(buf, &message)
		if err != nil {
			continue
		}

		switch message.Action {

		case "connect":

			proxy.peers[message.Target] = true

			// Inform all control connections that this proxy owns this peer connection
			for _, control := range sock.controllers {
				control.send("connect", control.id, message.Target, "")
			}

		case "disconnect":

			delete(proxy.peers, message.Target)

			// Inform all control connections that this proxy no longer owns this peer connection
			for _, control := range sock.controllers {
				control.send("disconnect", control.id, message.Target, "")
			}

		case "message":

			// Broadcast message on to given target
			wsBroadcast := &Message{
				source:    message.Source,
				target:    0, // target all connections
				payload:   message.Payload,
				fromProxy: true,
			}

			sock.broadcastBuffer <- wsBroadcast

		case "directmessage":

			messageSent := false

			// Relay message to control channel that matches target
			for _, control := range sock.controllers {
				if control.id == message.Target {
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
func (proxy *ProxyConnection) writeConnectionPump(sock *NamedWebSocket) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		proxy.removeConnection(sock)
	}()
	for {
		select {
		case <-ticker.C:
			proxy.ws.SetWriteDeadline(time.Now().Add(writeWait))
			proxy.ws.WriteMessage(websocket.PingMessage, []byte{})
		}
	}
}

// Set up a new NamedWebSocket connection instance
func (proxy *ProxyConnection) addConnection(sock *NamedWebSocket) {
	sock.proxies = append(sock.proxies, proxy)

	if proxy.writeable {
		// Inform this proxy of all the peer connections we own
		for _, peer := range sock.peers {
			proxy.send("connect", proxy.id, peer.id, "")
		}
	}

	// Start connection read/write pumps
	go proxy.writeConnectionPump(sock)
	go proxy.readConnectionPump(sock)
}

// Tear down an existing NamedWebSocket connection instance
func (proxy *ProxyConnection) removeConnection(sock *NamedWebSocket) {
	for i, conn := range sock.proxies {
		if conn.id == proxy.id {
			sock.proxies = append(sock.proxies[:i], sock.proxies[i+1:]...)
			break
		}
	}

	if proxy.writeable {
		// Inform this proxy of all the peer connections we no longer own
		for _, peer := range sock.peers {
			proxy.send("disconnect", proxy.id, peer.id, "")
		}
	}

	proxy.ws.Close()
}
