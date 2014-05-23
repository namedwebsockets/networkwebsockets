package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 4096
)

type NamedWebSocket struct {
	serviceName string

	// The current websocket connection instances to this named websocket
	connections []*Connection

	// Buffered channel of outbound service messages.
	broadcastBuffer chan *Message

	// Attached DNS-SD discovery registration and browser for this Named Web Socket
	discoveryClient *DiscoveryClient
}

type Connection struct {
	ws      *websocket.Conn
	isProxy bool
}

// Send a message to the target websocket connection
func (conn *Connection) write(mt int, payload []byte) {
	conn.ws.SetWriteDeadline(time.Now().Add(writeWait))
	conn.ws.WriteMessage(mt, payload)
}

type Message struct {
	source  *Connection
	payload []byte
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins
	},
}

// Create a new NamedWebSocket instance (local or broadcast-based) with a given service type
func NewNamedWebSocket(serviceName string, isBroadcast bool) *NamedWebSocket {
	scope := "broadcast"
	if isBroadcast == false {
		scope = "local"
	}

	sock := &NamedWebSocket{
		serviceName:     serviceName,
		connections:     make([]*Connection, 0),
		broadcastBuffer: make(chan *Message, 512),
	}

	go sock.messageDispatcher()

	log.Printf("New %s web socket '%s' created.", scope, serviceName)

	if isBroadcast {
		go sock.advertise()
	}

	return sock
}

func (sock *NamedWebSocket) advertise() {
	if sock.discoveryClient == nil {
		// Advertise new socket type on the local network
		sock.discoveryClient = NewDiscoveryClient(sock.serviceName)
	}
}

// Set up a new web socket connection
func (sock *NamedWebSocket) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	isProxy := false
	proxyHeader := r.Header.Get("X-BroadcastWebSocket-Proxy")
	if proxyHeader == "true" {
		isProxy = true
	}

	// Chose a subprotocol from those offered in the client request
	selectedSubprotocol := ""
	if subprotocolsStr := strings.TrimSpace(r.Header.Get("Sec-Websocket-Protocol")); subprotocolsStr != "" {
		// Choose the first subprotocol requested in 'Sec-Websocket-Protocol' header
		selectedSubprotocol = strings.Split(subprotocolsStr, ",")[0]
	}

	ws, err := upgrader.Upgrade(w, r, map[string][]string{
		"Access-Control-Allow-Origin":      []string{"*"},
		"Access-Control-Allow-Credentials": []string{"true"},
		"Access-Control-Allow-Headers":     []string{"content-type"},
		// Return requested subprotocol(s) as supported assuming peers will be handle it
		"Sec-Websocket-Protocol": []string{selectedSubprotocol},
	})
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}

	conn := &Connection{
		ws:      ws,
		isProxy: isProxy,
	}

	sock.addConnection(conn, true)
}

// readConnectionPump pumps messages from an individual websocket connection to the dispatcher
func (sock *NamedWebSocket) readConnectionPump(conn *Connection) {
	defer func() {
		sock.removeConnection(conn)
	}()
	conn.ws.SetReadLimit(maxMessageSize)
	conn.ws.SetReadDeadline(time.Now().Add(pongWait))
	conn.ws.SetPongHandler(func(string) error { conn.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := conn.ws.ReadMessage()
		if err != nil {
			break
		}

		wsBroadcast := &Message{
			source:  conn,
			payload: message,
		}

		sock.broadcastBuffer <- wsBroadcast
	}
}

// writeConnectionPump keeps an individual websocket connection alive
func (sock *NamedWebSocket) writeConnectionPump(conn *Connection) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		sock.removeConnection(conn)
	}()
	for {
		select {
		case <-ticker.C:
			conn.write(websocket.PingMessage, []byte{})
		}
	}
}

// Send service broadcast messages on NamedWebSocket connections
func (sock *NamedWebSocket) messageDispatcher() {
	for {
		select {
		case wsBroadcast, ok := <-sock.broadcastBuffer:
			if !ok {
				wsBroadcast.source.write(websocket.CloseMessage, []byte{})
				return
			}
			sock.broadcast(wsBroadcast)
		}
	}
}

// Set up a new NamedWebSocket connection instance
func (sock *NamedWebSocket) addConnection(conn *Connection, writable bool) {
	// Add this websocket instance to Named WebSocket broadcast list
	if writable {
		sock.connections = append(sock.connections, conn)
	}

	// Start connection read/write pumps
	go sock.writeConnectionPump(conn)
	sock.readConnectionPump(conn)
}

// Broadcast a message to all websocket connections for this NamedWebSocket
// instance (except to the src websocket connection)
func (sock *NamedWebSocket) broadcast(broadcast *Message) {
	for _, conn := range sock.connections {
		if conn.ws != broadcast.source.ws {
			// don't relay broadcast messages infinitely between proxy connections
			if conn.isProxy && broadcast.source.isProxy {
				continue
			}
			conn.write(websocket.TextMessage, broadcast.payload)
		}
	}
}

// Tear down an existing NamedWebSocket connection instance
func (sock *NamedWebSocket) removeConnection(conn *Connection) {
	for i, oConn := range sock.connections {
		if oConn.ws == conn.ws {
			sock.connections = append(sock.connections[:i], sock.connections[i+1:]...)
			break
		}
	}

	conn.ws.Close()
}
