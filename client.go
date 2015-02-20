package networkwebsockets

import (
	"errors"
	"net/http"
	"time"

	"github.com/richtr/websocket"
)

type ClientMessageHandler struct {
	client *Client
}

func (handler *ClientMessageHandler) Read(buf []byte) error {
	client := handler.client
	if client == nil {
		return errors.New("ClientMessageHandler requires an attached Client object")
	}

	message, err := decodeWireMessage(buf)
	if err != nil {
		return err
	}

	switch message.Action {
	case "connect":
		client.Connect <- message
	case "disconnect":
		client.Disconnect <- message
	case "status":
		client.Status <- message
	case "broadcast":
		client.Broadcast <- message
	case "message":
		client.Message <- message
	}

	return nil
}

func (handler *ClientMessageHandler) Write(buf []byte) error {
	client := handler.client
	if client == nil {
		return errors.New("ClientMessageHandler requires an attached Client object")
	}

	if !client.transport.open {
		return errors.New("Client is not active")
	}

	client.transport.conn.SetWriteDeadline(time.Now().Add(writeWait))
	client.transport.conn.WriteMessage(websocket.TextMessage, buf)

	return nil
}

func Dial(urlStr string, handler MessageHandler) (*Client, *http.Response, error) {
	d := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   8192,
		WriteBufferSize:  8192,
	}

	wsConn, httpResp, err := d.Dial(urlStr, nil)
	if err != nil {
		return nil, nil, err
	}

	transport := NewTransport(wsConn, handler)

	client := NewClient(transport)

	// Setup default client message handler if one has not been provided
	if client.transport.handler == nil {
		client.transport.handler = &ClientMessageHandler{client}
	}

	// Start read/write pumps
	client.Start()

	return client, httpResp, nil
}

// Client interface

type Client struct {
	// Underlying transport object
	transport *Transport

	// incoming message channels
	Status     chan WireMessage
	Connect    chan WireMessage
	Disconnect chan WireMessage
	Message    chan WireMessage
	Broadcast  chan WireMessage
}

func NewClient(transport *Transport) *Client {
	client := &Client{
		transport: transport,

		Status:     make(chan WireMessage, 255),
		Connect:    make(chan WireMessage, 255),
		Disconnect: make(chan WireMessage, 255),
		Message:    make(chan WireMessage, 255),
		Broadcast:  make(chan WireMessage, 255),
	}

	return client
}

func (client *Client) Start() {
	// Start read/write pumps
	client.transport.Start()
}

func (client *Client) Stop() {
	// Stop read/write pumps
	client.transport.Stop()
}

// Default Client Message Handler Helper functions

func (client *Client) SendBroadcastData(data string) {
	if wireData, err := encodeWireMessage("broadcast", "", "", data); err == nil {
		client.transport.Write(wireData)
	}
}

func (client *Client) SendMessageData(data string, targetId string) {
	if targetId == "" {
		return
	}

	if wireData, err := encodeWireMessage("message", "", targetId, data); err == nil {
		client.transport.Write(wireData)
	}
}

func (client *Client) SendStatusRequest() {
	if wireData, err := encodeWireMessage("status", "", "", ""); err == nil {
		client.transport.Write(wireData)
	}
}
