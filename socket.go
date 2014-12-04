package networkwebsockets

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
	maxMessageSize = 8192
)

type NetworkWebSocket struct {
	serviceName string

	serviceHash string

	servicePath string

	proxyPath string

	// The current websocket connection control instances to this named websocket
	controllers []*ControlConnection

	// The current websocket connection instances to this named websocket
	peers []*PeerConnection

	// The current websocket proxy connection instances to this named websocket
	proxies []*ProxyConnection

	// Buffered channel of outbound service messages.
	broadcastBuffer chan *Message

	// Attached DNS-SD discovery registration and browser for this Named Web Socket
	discoveryService *DiscoveryService

	done chan int // blocks until .close() is called
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all cross-origin access
	},
}

// Create a new NetworkWebSocket instance with a given service type
func NewNetworkWebSocket(service *NetworkWebSocket_Service, serviceName string, isControl bool) *NetworkWebSocket {
	serviceHash_BCrypt, _ := bcrypt.HashBytes([]byte(serviceName))
	serviceHash_Base64 := base64.StdEncoding.EncodeToString(serviceHash_BCrypt)

	rand.Seed(time.Now().UTC().UnixNano())

	sock := &NetworkWebSocket{
		serviceName: serviceName,
		serviceHash: serviceHash_Base64,
		servicePath: fmt.Sprintf("/network/%s", serviceName),
		proxyPath:   fmt.Sprintf("/%d", rand.Int()),

		controllers:     make([]*ControlConnection, 0),
		peers:           make([]*PeerConnection, 0),
		proxies:         make([]*ProxyConnection, 0),
		broadcastBuffer: make(chan *Message, 512),

		done: make(chan int),
	}

	go sock.messageDispatcher()

	log.Printf("New '%s' channel peer created.", sock.serviceName)

	service.Channels[sock.servicePath] = sock

	// Terminate channel when it is closed
	go func() {
		<-sock.stopNotify()
		delete(service.Channels, sock.servicePath)
	}()

	if !isControl {

		// Add TLS-SRP credentials for access to this service to credentials store
		// TODO isolate this per socket
		serviceTab[sock.serviceHash] = sock.serviceName

		go sock.advertise(service.ProxyPort)

		if service.discoveryBrowser != nil {

			// Attempt to resolve discovered unknown service hashes with this service name
			recordsCache := make(map[string]*NetworkWebSocket_DNSRecord)
			for _, cachedRecord := range service.discoveryBrowser.cachedDNSRecords {
				if bcrypt.Match(sock.serviceName, cachedRecord.Hash_BCrypt) {
					if _, dErr := sock.dialFromDNSRecord(cachedRecord); dErr != nil {
						log.Printf("err: %v", dErr)
					}
				} else {
					// Maintain as an unresolved entry in cache
					recordsCache[cachedRecord.Hash_Base64] = cachedRecord
				}
			}

			// Replace unresolved DNS-SD service entries cache
			service.discoveryBrowser.cachedDNSRecords = recordsCache

		}
	}

	return sock
}

func (sock *NetworkWebSocket) advertise(port int) {
	if sock.discoveryService == nil {
		// Advertise new socket type on the network
		sock.discoveryService = NewDiscoveryService(sock.serviceName, sock.serviceHash, sock.proxyPath, port)
		sock.discoveryService.Register("local")
	}
}

// Set up a new web socket connection
func (sock *NetworkWebSocket) servePeer(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "GET" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	ws, err := sock.upgradeToWebSocket(w, r)
	if err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	_ = NewPeerConnection(sock, id, ws)

}

// Set up a new web socket connection
func (sock *NetworkWebSocket) serveProxy(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "GET" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	requestedWebSocketSubProtocols := r.Header.Get("Sec-Websocket-Protocol")
	if requestedWebSocketSubProtocols != "nws-proxy-draft-01" {
		http.Error(w, "Bad Request", 400)
		return
	}

	ws, err := sock.upgradeToWebSocket(w, r)
	if err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	_ = NewProxyConnection(sock, id, ws, true)
}

// Set up a new web socket connection
func (sock *NetworkWebSocket) serveControl(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "GET" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	ws, err := sock.upgradeToWebSocket(w, r)
	if err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	_ = NewControlConnection(sock, id, ws)
}

func (sock *NetworkWebSocket) upgradeToWebSocket(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
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

func (sock *NetworkWebSocket) dialFromDNSRecord(record *NetworkWebSocket_DNSRecord) (*ProxyConnection, error) {

	hosts := [...]string{record.AddrV4.String(), record.AddrV6.String()}

	for i := 0; i < len(hosts); i++ {

		if hosts[i] == "<nil>" {
			continue
		}

		// Build URL
		remoteWSUrl := url.URL{
			Scheme: "wss",
			Host:   fmt.Sprintf("%s:%d", hosts[i], record.Port),
			Path:   record.Path,
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
				SRPPassword: sock.serviceName,
			},
		}

		ws, _, nErr := tlsSrpDialer.Dial(remoteWSUrl, map[string][]string{
			"Origin":                 []string{"localhost"},
			"Sec-WebSocket-Protocol": []string{"nws-proxy-draft-01"},
		})
		if nErr != nil {
			errStr := fmt.Sprintf("Proxy named web socket connection to wss://%s%s failed: %s", remoteWSUrl.Host, remoteWSUrl.Path, nErr)
			return nil, errors.New(errStr)
		}

		log.Printf("Established proxy named web socket connection to wss://%s%s", remoteWSUrl.Host, remoteWSUrl.Path)

		// Generate a new id for this proxy connection
		rand.Seed(time.Now().UTC().UnixNano())
		newPeerId := fmt.Sprintf("%d", rand.Int())

		proxyConn := NewProxyConnection(sock, newPeerId, ws, false)
		proxyConn.setHash_Base64(record.Hash_Base64)

		return proxyConn, nil

	}

	return nil, errors.New("Could not establish proxy named web socket connection")

}

// Send service broadcast messages on NetworkWebSocket connections
func (sock *NetworkWebSocket) messageDispatcher() {
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

// Broadcast a message to all peer connections for this NetworkWebSocket
// instance (except to the src websocket connection)
func (sock *NetworkWebSocket) localBroadcast(broadcast *Message) {
	// Write to peer connections
	for _, peer := range sock.peers {
		// don't send back to self
		if peer.id == broadcast.source {
			continue
		}
		peer.send(broadcast.payload)
	}
}

// Broadcast a message to all proxy connections for this NetworkWebSocket
// instance (except to the src websocket connection)
func (sock *NetworkWebSocket) remoteBroadcast(broadcast *Message) {
	// Only send to remote proxies if this message was not received from a proxy itself
	if broadcast.fromProxy {
		return
	}

	// Write to proxy connections
	for _, proxy := range sock.proxies {
		// don't send back to self
		// only write to *writeable* proxy connections
		if !proxy.writeable || proxy.base.id == broadcast.source {
			continue
		}
		proxy.send("message", broadcast.source, "", broadcast.payload)
	}
}

// Destroy this Network Web Socket service instance, close all
// peer, control and proxy connections.
func (sock *NetworkWebSocket) Stop() {
	// Close discovery browser
	if sock.discoveryService != nil {
		sock.discoveryService.Shutdown()
	}

	for _, peer := range sock.peers {
		peer.Stop()
	}

	for _, control := range sock.controllers {
		control.Stop()
	}

	for _, proxy := range sock.proxies {
		proxy.Stop()
	}

	// Indicate object is closed
	sock.done <- 1
}

// StopNotify returns a channel that receives a empty integer
// when the channel service is terminated.
func (sock *NetworkWebSocket) stopNotify() <-chan int { return sock.done }

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
