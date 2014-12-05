package networkwebsockets

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/richtr/websocket"
)

type NetworkWebSocketClient struct {
	// Underlying websocket connection object
	conn *websocket.Conn

	// incoming message channels
	Status     chan *NetworkWebSocketWireMessage
	Connect    chan *NetworkWebSocketWireMessage
	Disconnect chan *NetworkWebSocketWireMessage
	Message    chan *NetworkWebSocketWireMessage
	Broadcast  chan *NetworkWebSocketWireMessage
}

func NewNetworkWebSocketClient(wsConn *websocket.Conn) *NetworkWebSocketClient {
	client := &NetworkWebSocketClient{
		conn: wsConn,

		Status:     make(chan *NetworkWebSocketWireMessage),
		Connect:    make(chan *NetworkWebSocketWireMessage),
		Disconnect: make(chan *NetworkWebSocketWireMessage),
		Message:    make(chan *NetworkWebSocketWireMessage),
		Broadcast:  make(chan *NetworkWebSocketWireMessage),
	}

	return client
}

func (client *NetworkWebSocketClient) SendBroadcastData(data string) {
	client.send("broadcast", "", data)
}

func (client *NetworkWebSocketClient) SendMessageData(data string, targetId string) {
	if targetId == "" {
		return
	}

	client.send("message", targetId, data)
}

// Send a message to the target websocket connection
func (client *NetworkWebSocketClient) send(action string, target string, payload string) {
	// Construct proxy wire message
	m := NetworkWebSocketWireMessage{
		Action:  action,
		Target:  target,
		Payload: payload,
	}

	wireMsg, err := json.Marshal(m)
	if err != nil {
		return
	}

	// TOOO: This is wrong!
	client.conn.SetWriteDeadline(time.Now().Add(writeWait))
	client.conn.WriteMessage(websocket.TextMessage, wireMsg)
}

// readConnectionPump pumps messages from an individual websocket connection to the dispatcher
func (client *NetworkWebSocketClient) readPump() {
	defer func() {
		//client.Stop()
	}()
	client.conn.SetReadLimit(maxMessageSize)
	client.conn.SetReadDeadline(time.Now().Add(pongWait))
	client.conn.SetPongHandler(func(string) error { client.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		opCode, buf, err := client.conn.ReadMessage()
		if err != nil || opCode != websocket.TextMessage {
			break
		}

		var message NetworkWebSocketWireMessage
		if err := json.Unmarshal(buf, &message); err != nil {
			continue // ignore unrecognized message format
		}

		switch message.Action {
		case "connect":
			client.Connect <- &message
			break
		case "disconnect":
			client.Disconnect <- &message
			break
		case "status":
			client.Status <- &message
			break
		case "broadcast":
			client.Broadcast <- &message
			break
		case "message":
			client.Message <- &message
			break
		}
	}
}

// writeConnectionPump keeps an individual websocket connection alive
func (client *NetworkWebSocketClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
	}()
	for {
		select {
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func Dial(urlStr string) (*NetworkWebSocketClient, *http.Response, error) {
	d := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   8192,
		WriteBufferSize:  8192,
	}

	wsConn, httpResp, err := d.Dial(urlStr, nil)
	if err != nil {
		return nil, nil, err
	}

	client := NewNetworkWebSocketClient(wsConn)

	go client.readPump()
	go client.writePump()

	return client, httpResp, nil
}
