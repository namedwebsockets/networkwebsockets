package namedwebsockets

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/richtr/bcrypt"
	tls "github.com/richtr/go-tls-srp"
	"github.com/richtr/websocket"
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

	serviceHash string

	// 'network' or 'local' scoped named web socket service
	serviceScope string

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
		return true // allow all cross-origin access
	},
}

// Create a new NamedWebSocket instance (local or network-based) with a given service type
func NewNamedWebSocket(service *NamedWebSocket_Service, serviceName string, isNetwork bool, port int) *NamedWebSocket {
	scope := "network"
	if isNetwork == false {
		scope = "local"
	}

	serviceHash_Base64, _ := bcrypt.HashBytes([]byte(serviceName))
	serviceHash_BCrypt := base64.StdEncoding.EncodeToString(serviceHash_Base64)

	sock := &NamedWebSocket{
		serviceName:     serviceName,
		serviceHash:     serviceHash_BCrypt,
		serviceScope:    scope,
		controllers:     make([]*ControlConnection, 0),
		peers:           make([]*PeerConnection, 0),
		proxies:         make([]*ProxyConnection, 0),
		broadcastBuffer: make(chan *Message, 512),
	}

	go sock.messageDispatcher()

	log.Printf("New %s websocket '%s' created with hash[%s].", scope, sock.serviceName, sock.serviceHash)

	if isNetwork {
		service.knownServiceNames[sock.serviceName] = true

		// Add TLS-SRP credentials for access to this service to credentials store
		serviceTab[sock.serviceHash] = sock.serviceName

		// Mark this service as advertised (to ignore during mDNS/DNS-SD discovery process)
		service.AdvertisedServiceHashes[sock.serviceHash] = true

		go sock.advertise(port + 1)

		// Attempt to resolve discovered unknown service hashes with this service name
		unresolvedServiceRecords := make(map[string]*NamedWebSocket_DNSRecord)
		for _, record := range service.UnresolvedServiceRecords {

			if bcrypt.Match(sock.serviceName, record.Hash_BCrypt) {
				if _, dErr := sock.dialDNSRecord(record, sock.serviceName); dErr != nil {
					log.Printf("err: %v", dErr)
				}

				// Add to resolved entries
				service.ResolvedServiceRecords[record.Hash_BCrypt] = record
			} else {
				// Maintain as an unresolved entry
				unresolvedServiceRecords[record.Hash_BCrypt] = record
			}

		}

		// Replace unresolved entries cache
		service.UnresolvedServiceRecords = unresolvedServiceRecords
	}

	return sock
}

func (sock *NamedWebSocket) advertise(port int) {
	if sock.discoveryClient == nil {
		// Advertise new socket type on the local network
		sock.discoveryClient = NewDiscoveryClient(sock.serviceHash, port, fmt.Sprintf("/%s/%s", sock.serviceScope, sock.serviceHash))
		sock.discoveryClient.Register("local")
	}
}

// Set up a new web socket connection
func (sock *NamedWebSocket) servePeer(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	ws, err := sock.upgradeToWebSocket(w, r)
	if err != nil {
		http.Error(w, "Not found", 404)
	}

	peerConn := NewPeerConnection(id, ws)
	peerConn.addConnection(sock)
}

// Set up a new web socket connection
func (sock *NamedWebSocket) serveProxy(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	requestedWebSocketSubProtocols := r.Header.Get("Sec-Websocket-Protocol")
	if requestedWebSocketSubProtocols != "nws-proxy-draft-01" {
		http.Error(w, "Bad Request", 400)
		return
	}

	ws, err := sock.upgradeToWebSocket(w, r)
	if err != nil {
		http.Error(w, "Not found", 404)
	}

	proxyConn := NewProxyConnection(id, ws, true)
	proxyConn.addConnection(sock)
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

func (sock *NamedWebSocket) dialDNSRecord(record *NamedWebSocket_DNSRecord, serviceName string) (*ProxyConnection, error) {

	// Generate unique id for this new connection
	rand.Seed(time.Now().UTC().UnixNano())
	newPeerId := rand.Int()

	hosts := [...]string{record.AddrV4.String(), record.AddrV6.String()}

	for i := 0; i < len(hosts); i++ {

		if hosts[i] == "<nil>" {
			continue
		}

		// Build URL
		remoteWSUrl := url.URL{
			Scheme: "wss",
			Host:   fmt.Sprintf("%s:%d", hosts[i], record.Port),
			Path:   fmt.Sprintf("%s/%d", record.Path, newPeerId),
		}

		// Establish Proxy WebSocket connection over TLS-SRP

		tlsSrpDialer := &TLSSRPDialer{
			&websocket.Dialer{
				HandshakeTimeout: time.Duration(10) * time.Second,
				ReadBufferSize:   8192,
				WriteBufferSize:  8192,
			},
			&tls.Config{
				SRPUser:     record.Hash_Base64,
				SRPPassword: serviceName,
			},
		}

		ws, _, nErr := tlsSrpDialer.Dial(remoteWSUrl, map[string][]string{
			"Origin":                 []string{"localhost"},
			"Sec-WebSocket-Protocol": []string{"nws-proxy-draft-01"},
		})
		if nErr != nil {
			errStr := fmt.Sprintf("Proxy network websocket connection to wss://%s%s failed: %s", remoteWSUrl.Host, remoteWSUrl.Path, nErr)
			return nil, errors.New(errStr)
		}

		log.Printf("Established proxy network websocket connection to wss://%s%s", remoteWSUrl.Host, remoteWSUrl.Path)

		proxyConn := NewProxyConnection(newPeerId, ws, false)

		proxyConn.addConnection(sock)

		return proxyConn, nil

	}

	return nil, errors.New("Could not establish proxy network websocket connection")

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
