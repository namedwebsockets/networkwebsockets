package namedwebsockets

import (
	"encoding/json"
	"math/rand"
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
	// Proxy message type: "connect", "disconnect", "ping", "message"
	Action string

	// Recipients' id list (currently on ever -1 === send to all peers)
	To []int

	// Raw message contents
	Payload []byte
}

func NewProxyConnection(socket *websocket.Conn, isWriteable bool) *ProxyConnection {
	// Generate unique id for connection
	rand.Seed(time.Now().UTC().UnixNano())
	connId := rand.Int()

	proxyConn := &ProxyConnection{
		PeerConnection: PeerConnection{
			id: connId,
			ws: socket,
		},
		writeable: isWriteable,
		peers:     make(map[int]bool),
	}

	return proxyConn
}

// Send a message to the target websocket connection
func (proxy *ProxyConnection) write(mt int, action string, targets []int, payload []byte) {
	// Construct proxy wire message
	m := ProxyWireMessage{
		Action:  action,
		To:      targets,
		Payload: payload,
	}
	messagePayload, err := json.Marshal(m)
	if err != nil {
		return
	}

	proxy.ws.SetWriteDeadline(time.Now().Add(writeWait))
	proxy.ws.WriteMessage(mt, messagePayload)
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
		_, buf, err := proxy.ws.ReadMessage()
		if err != nil {
			break
		}

		// Writeable proxies should not be receiving or relaying messages onwards
		if proxy.writeable {
			continue
		}

		var message ProxyWireMessage
		err = json.Unmarshal(buf, &message)
		if err != nil {
			continue
		}

		switch message.Action {

		case "connect":

			proxy.peers[message.To[0]] = true

		case "disconnect":

			delete(proxy.peers, message.To[0])

		case "message":

			// Broadcast message on to given targets
			wsBroadcast := &Message{
				source:    &proxy.PeerConnection,
				targets:   message.To,
				payload:   message.Payload,
				fromProxy: true,
			}

			sock.broadcastBuffer <- wsBroadcast

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
			proxy.write(websocket.PingMessage, "ping", []int{-1}, []byte{})
		}
	}
}

// Set up a new NamedWebSocket connection instance
func (proxy *ProxyConnection) addConnection(sock *NamedWebSocket) {
	sock.proxies = append(sock.proxies, proxy)

	if proxy.writeable {
		// Inform this proxy of all the peer connections we own
		for _, peer := range sock.peers {
			proxy.write(websocket.TextMessage, "connect", []int{peer.id}, []byte{})
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
			proxy.write(websocket.TextMessage, "disconnect", []int{peer.id}, []byte{})
		}
	}

	proxy.ws.Close()
}
