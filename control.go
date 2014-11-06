package namedwebsockets

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/richtr/websocket"
)

type ControlConnection struct {
	// Inherit attributes from PeerConnection struct
	PeerConnection
}

type ControlWireMessage struct {
	// Proxy message type: "connect", "disconnect", "message"
	Action string

	Source int

	Target int

	// Message contents
	Payload string
}

func NewControlConnection(id int, socket *websocket.Conn) *ControlConnection {
	controlConn := &ControlConnection{
		PeerConnection: PeerConnection{
			id: id,
			ws: socket,
		},
	}

	return controlConn
}

// Send a message to the target websocket connection
func (control *ControlConnection) send(action string, source int, target int, payload string) {
	// Construct proxy wire message
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

	control.ws.SetWriteDeadline(time.Now().Add(writeWait))
	control.ws.WriteMessage(websocket.TextMessage, messagePayload)
}

// readConnectionPump pumps messages from an individual websocket connection to the dispatcher
func (control *ControlConnection) readConnectionPump(sock *NamedWebSocket) {
	defer func() {
		control.removeConnection(sock)
	}()
	control.ws.SetReadLimit(maxMessageSize)
	control.ws.SetReadDeadline(time.Now().Add(pongWait))
	control.ws.SetPongHandler(func(string) error { control.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		opCode, buf, err := control.ws.ReadMessage()
		if err != nil || opCode != websocket.TextMessage {
			break
		}

		var message ControlWireMessage
		if err := json.Unmarshal(buf, &message); err != nil {
			log.Fatalf("%s", err)
			continue
		}

		switch message.Action {
		// Only read 'messages' (connect and disconnect events are write-only)
		case "message":

			messageSent := false

			// Relay message to control channel that matches target
			for _, _control := range sock.controllers {
				if _control.id == message.Target {
					_control.send("message", control.id, message.Target, message.Payload)
					messageSent = true
					break
				}
			}

			if !messageSent {
				// Hunt for target in known proxies
				for _, proxy := range sock.proxies {
					if proxy.peers[message.Target] {
						proxy.send("directmessage", control.id, message.Target, message.Payload)
						messageSent = true
						break
					}
				}
			}

			if !messageSent {
				fmt.Errorf("P2P message target could not be found. Not sent.")
			}

		}

	}
}

// writeConnectionPump keeps an individual websocket connection alive
func (control *ControlConnection) writeConnectionPump(sock *NamedWebSocket) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		control.removeConnection(sock)
	}()
	for {
		select {
		case <-ticker.C:
			control.ws.SetWriteDeadline(time.Now().Add(writeWait))
			control.ws.WriteMessage(websocket.PingMessage, []byte{})
		}
	}
}

// Set up a new NamedWebSocket control connection instance
func (control *ControlConnection) addConnection(sock *NamedWebSocket) {
	sock.controllers = append(sock.controllers, control)

	// Start connection read/write pumps
	go control.writeConnectionPump(sock)
	go control.readConnectionPump(sock)

	// Inform this control point of all the peer connections we own
	for _, peer := range sock.peers {
		// don't notify controller if its id matches the peer's id
		if control.id != peer.id {
			control.send("connect", control.id, peer.id, "")
		}
	}

	// Inform this control point of all the peer connections connected proxies own
	for _, proxy := range sock.proxies {
		for peerId, _ := range proxy.peers {
			control.send("connect", control.id, peerId, "")
		}
	}
}

// Tear down an existing NamedWebSocket control connection instance
func (control *ControlConnection) removeConnection(sock *NamedWebSocket) {
	for i, conn := range sock.controllers {
		if conn.id == control.id {
			sock.controllers = append(sock.controllers[:i], sock.controllers[i+1:]...)
			break
		}
	}

	control.ws.Close()
}
