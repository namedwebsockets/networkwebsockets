package networkwebsockets

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	tls "github.com/richtr/go-tls-srp"
	"github.com/richtr/websocket"
)

const (
	// Time allowed to write a message to any websocket.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from any websocket.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from any websocket.
	maxMessageSize = 8192
)

type MessageHandler interface {
	Read(buf []byte) error
	Write(buf []byte) error
}

// JSON structure to message sending
type WireMessage struct {
	// Proxy message type: "connect", "disconnect", "message", "broadcast"
	Action string `json:"action"`

	Source string `json:"source,omitempty"`

	Target string `json:"target,omitempty"`

	// Message contents
	Payload string `json:"data,omitempty"`

	// Whether this message originated from a Proxy object
	fromProxy bool `json:"-"`
}

type Transport struct {
	conn    *websocket.Conn
	handler MessageHandler
	open    bool
	done    chan int // blocks until .Stop() is called
}

func NewTransport(conn *websocket.Conn, handler MessageHandler) *Transport {
	transport := &Transport{
		conn:    conn,
		handler: handler,

		done: make(chan int, 1),
	}

	return transport
}

func (t *Transport) Start() {
	var wg sync.WaitGroup
	wg.Add(2)

	go t.writePump(&wg)
	go t.readPump(&wg)

	t.open = true

	wg.Wait()
}

func (t *Transport) Stop() {
	t.open = false

	t.conn.Close()
}

// StopNotify returns a channel that receives a empty integer
// when the transport is closed
func (t *Transport) StopNotify() <-chan int { return t.done }

func (t *Transport) Read(buf []byte) error {
	if !t.open {
		return errors.New("Transport is not currently active for reading")
	}

	if t.handler == nil {
		return errors.New("Cannot read message. Transport does not have a handler assigned")
	}

	return t.handler.Read(buf)
}

func (t *Transport) Write(buf []byte) error {
	if !t.open {
		return errors.New("Transport is not currently active for writing")
	}

	if t.handler == nil {
		return errors.New("Cannot write message. Transport does not have a handler assigned")
	}

	return t.handler.Write(buf)
}

// readPump pumps messages from an individual websocket connection to the dispatcher
func (t *Transport) readPump(wg *sync.WaitGroup) {
	t.conn.SetReadLimit(maxMessageSize)
	t.conn.SetReadDeadline(time.Now().Add(pongWait))
	t.conn.SetPongHandler(func(string) error {
		t.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	wg.Done()

	for {
		opCode, buf, err := t.conn.ReadMessage()
		if err != nil || opCode != websocket.TextMessage {
			break
		}

		// Pass incoming message to our assigned message handler
		if err := t.Read(buf); err != nil {
			log.Printf("err: %v", err)
		}
	}

	// Indicate object is closed
	t.done <- 1
}

// writePump keeps an individual websocket connection alive
func (t *Transport) writePump(wg *sync.WaitGroup) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
	}()

	wg.Done()

	for {
		select {
		case <-ticker.C:
			t.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := t.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

/** TLS-SRP Dialer interface **/

type TLSSRPDialer struct {
	*websocket.Dialer

	TLSClientConfig *tls.Config
}

// Dial creates a new TLS-SRP based client connection. Use requestHeader to specify the
// origin (Origin), subprotocols (Sec-WebSocket-Protocol) and cookies (Cookie).
// Use the response.Header to get the selected subprotocol
// (Sec-WebSocket-Protocol) and cookies (Set-Cookie).
//
// If the WebSocket handshake fails, ErrBadHandshake is returned along with a
// non-nil *http.Response so that callers can handle redirects, authentication,
// etc.
func (d *TLSSRPDialer) Dial(url url.URL, requestHeader http.Header) (*websocket.Conn, *http.Response, error) {
	var deadline time.Time

	if d.HandshakeTimeout != 0 {
		deadline = time.Now().Add(d.HandshakeTimeout)
	}

	netConn, err := tls.Dial("tcp", url.Host, d.TLSClientConfig)
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		if netConn != nil {
			netConn.Close()
		}
	}()

	if err := netConn.SetDeadline(deadline); err != nil {
		return nil, nil, err
	}

	if len(d.Subprotocols) > 0 {
		h := http.Header{}
		for k, v := range requestHeader {
			h[k] = v
		}
		h.Set("Sec-Websocket-Protocol", strings.Join(d.Subprotocols, ", "))
		requestHeader = h
	}

	conn, resp, err := websocket.NewClient(netConn, &url, requestHeader, d.ReadBufferSize, d.WriteBufferSize)
	if err != nil {
		return nil, resp, err
	}

	netConn.SetDeadline(time.Time{})
	netConn = nil // to avoid close in defer.
	return conn, resp, nil
}
