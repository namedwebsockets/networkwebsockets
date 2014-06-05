package namedwebsockets

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

	// The current websocket connection control instances to this named websocket
	controllers []*ControlConnection

	// The current websocket connection instances to this named websocket
	peers []*PeerConnection

	// The current websocket proxy connection instances to this named websocket
	proxies []*ProxyConnection

	// Buffered channel of outbound service messages.
	broadcastBuffer chan *Message

	// Attached DNS-SD discovery registration and browser for this Named Web Socket
	discoveryClient *DiscoveryClient
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins
	},
}

// Create a new NamedWebSocket instance (local or network-based) with a given service type
func NewNamedWebSocket(service *NamedWebSocket_Service, serviceName string, isNetwork bool, port int) *NamedWebSocket {
	scope := "network"
	if isNetwork == false {
		scope = "local"
	}

	sock := &NamedWebSocket{
		serviceName:     serviceName,
		controllers:     make([]*ControlConnection, 0),
		peers:           make([]*PeerConnection, 0),
		proxies:         make([]*ProxyConnection, 0),
		broadcastBuffer: make(chan *Message, 512),
	}

	go sock.messageDispatcher()

	log.Printf("New %s websocket '%s' created.", scope, serviceName)

	if isNetwork {
		go sock.advertise(service, port)
	}

	return sock
}

func (sock *NamedWebSocket) advertise(service *NamedWebSocket_Service, port int) {
	if sock.discoveryClient == nil {
		// Advertise new socket type on the local network
		sock.discoveryClient = NewDiscoveryClient(service, sock.serviceName, port)
	}
}

// Set up a new web socket connection
func (sock *NamedWebSocket) serveService(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	isProxy := false
	proxyHeader := r.Header.Get("X-NetworkWebSocket-Proxy")
	if proxyHeader == "true" {
		isProxy = true
	}

	ws, err := sock.upgradeToWebSocket(w, r)
	if err != nil {
		http.Error(w, "Not found", 404)
	}

	if isProxy {
		proxyConn := NewProxyConnection(id, ws, true)
		proxyConn.addConnection(sock)
	} else {
		peerConn := NewPeerConnection(id, ws)
		peerConn.addConnection(sock)
	}
}

// Set up a new web socket connection
func (sock *NamedWebSocket) serveControl(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	ws, err := sock.upgradeToWebSocket(w, r)
	if err != nil {
		http.Error(w, "Not found", 404)
	}

	controlConn := NewControlConnection(id, ws)
	controlConn.addConnection(sock)
}

func (sock *NamedWebSocket) upgradeToWebSocket(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
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
		// Return requested subprotocol(s) as supported so peers can handle it
		"Sec-Websocket-Protocol": []string{selectedSubprotocol},
	})
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return nil, err
	}

	return ws, nil
}

// Send service broadcast messages on NamedWebSocket connections
func (sock *NamedWebSocket) messageDispatcher() {
	for {
		select {
		case wsBroadcast, ok := <-sock.broadcastBuffer:
			if !ok {
				return
			}
			// Send message to local peers
			sock.localBroadcast(wsBroadcast)
			// Send message to remote proxies
			sock.remoteBroadcast(wsBroadcast)
		}
	}
}

// Broadcast a message to all peer connections for this NamedWebSocket
// instance (except to the src websocket connection)
func (sock *NamedWebSocket) localBroadcast(broadcast *Message) {
	// Write to peer connections
	for _, peer := range sock.peers {
		// don't send back to self
		if peer.id == broadcast.source {
			continue
		}
		peer.send(broadcast.payload)
	}
}

// Broadcast a message to all proxy connections for this NamedWebSocket
// instance (except to the src websocket connection)
func (sock *NamedWebSocket) remoteBroadcast(broadcast *Message) {
	// Only send to remote proxies if this message was not received from a proxy itself
	if broadcast.fromProxy {
		return
	}

	// Write to proxy connections
	for _, proxy := range sock.proxies {
		// don't send back to self
		// only write to *writeable* proxy connections
		if !proxy.writeable || proxy.id == broadcast.source {
			continue
		}
		proxy.send("message", broadcast.source, 0, broadcast.payload)
	}
}
