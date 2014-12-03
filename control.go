package networkwebsockets

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/richtr/websocket"
)

type ControlConnection struct {
	// Inherit attributes from PeerConnection struct
	base PeerConnection
}

type ControlWireMessage struct {
	// Proxy message type: "connect", "disconnect", "message"
	Action string `json:"action"`

	Source string `json:"source,omitempty"`

	Target string `json:"target,omitempty"`

	// Message contents
	Payload string `json:"data,omitempty"`
}

func NewControlConnection(channel *NetworkWebSocket, id string, conn *websocket.Conn) *ControlConnection {
	controlConn := &ControlConnection{
		base: PeerConnection{
			id:      id,
			channel: channel,
			conn:    conn,
		},
	}

	// Start websocket read/write pumps
	controlConn.Start()

	return controlConn
}

func (control *ControlConnection) Start() {
	// Start connection read/write pumps
	go control.writeConnectionPump()
	go control.readConnectionPump()

	// Add reference to this control connection to channel
	control.addConnection()
}

// Send a message to the target websocket connection
func (control *ControlConnection) send(action string, source string, target string, payload string) {
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

	control.base.conn.SetWriteDeadline(time.Now().Add(writeWait))
	control.base.conn.WriteMessage(websocket.TextMessage, messagePayload)
}

// readConnectionPump pumps messages from an individual websocket connection to the dispatcher
func (control *ControlConnection) readConnectionPump() {
	defer func() {
		control.Stop()
	}()
	control.base.conn.SetReadLimit(maxMessageSize)
	control.base.conn.SetReadDeadline(time.Now().Add(pongWait))
	control.base.conn.SetPongHandler(func(string) error { control.base.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		opCode, buf, err := control.base.conn.ReadMessage()
		if err != nil || opCode != websocket.TextMessage {
			break
		}

		var message ControlWireMessage
		if err := json.Unmarshal(buf, &message); err != nil {
			continue // ignore unrecognized message format
		}

		switch message.Action {
		// Only read 'messages' (connect and disconnect events are write-only)
		case "message":

			messageSent := false

			// Relay message to control channel that matches target
			for _, _control := range control.base.channel.controllers {
				if _control.base.id == message.Target {
					_control.send("message", control.base.id, message.Target, message.Payload)
					messageSent = true
					break
				}
			}

			if !messageSent {
				// Hunt for target in known proxies
				for _, proxy := range control.base.channel.proxies {
					if proxy.peerIds[message.Target] {
						proxy.send("directmessage", control.base.id, message.Target, message.Payload)
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
func (control *ControlConnection) writeConnectionPump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		control.Stop()
	}()
	for {
		select {
		case <-ticker.C:
			control.base.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := control.base.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// Set up a new NetworkWebSocket control connection instance
func (control *ControlConnection) addConnection() {
	control.base.channel.controllers = append(control.base.channel.controllers, control)

	// Inform this control point of all the peer connections we own
	for _, peer := range control.base.channel.peers {
		// don't notify controller if its id matches the peer's id
		if control.base.id != peer.id {
			control.send("connect", control.base.id, peer.id, "")
		}
	}

	// Inform this control point of all the peer connections connected proxies own
	for _, proxy := range control.base.channel.proxies {
		for peerId, _ := range proxy.peerIds {
			control.send("connect", control.base.id, peerId, "")
		}
	}
}

// Tear down an existing NetworkWebSocket control connection instance
func (control *ControlConnection) removeConnection() {
	for i, conn := range control.base.channel.controllers {
		if control.base.id == conn.base.id {
			control.base.channel.controllers = append(control.base.channel.controllers[:i], control.base.channel.controllers[i+1:]...)
			break
		}
	}

	control.base.conn.Close()
}

func (control *ControlConnection) Stop() {
	// Remove reference to this control connection from channel
	control.removeConnection()
}
