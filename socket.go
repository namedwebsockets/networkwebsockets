package networkwebsockets

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"

	"github.com/richtr/bcrypt"
)

type Socket struct {
	serviceName string

	serviceHash string

	servicePath string

	proxyPath string

	// The current websocket connection instances to this named websocket
	peers []*Peer

	// The current websocket proxy connection instances to this named websocket
	proxies []*Proxy

	// Buffered channel of outbound service messages.
	broadcastBuffer chan *WireMessage

	// Attached DNS-SD discovery registration and browser for this Named Web Socket
	discoveryService *DiscoveryService

	done chan int // blocks until .Stop() is called
}

// Create a new Socket instance with a given service type
func NewSocket(service *Service, serviceName string) *Socket {
	serviceHash_BCrypt, _ := bcrypt.HashBytes([]byte(serviceName))
	serviceHash_Base64 := base64.StdEncoding.EncodeToString(serviceHash_BCrypt)

	sock := &Socket{
		serviceName: serviceName,
		serviceHash: serviceHash_Base64,

		servicePath: fmt.Sprintf("/%s", serviceName),

		peers:           make([]*Peer, 0),
		proxies:         make([]*Proxy, 0),
		broadcastBuffer: make(chan *WireMessage, 512),

		done: make(chan int, 1),
	}

	sock.proxyPath = fmt.Sprintf("/%s", GenerateId())

	go sock.messageDispatcher()

	log.Printf("New '%s' channel peer created.", sock.serviceName)

	service.Channels[sock.servicePath] = sock

	// Terminate channel when it is closed
	go func() {
		<-sock.stopNotify()
		delete(service.Channels, sock.servicePath)
	}()

	// Add TLS-SRP credentials for access to this service to credentials store
	// TODO isolate this per socket
	serviceTab[sock.serviceHash] = sock.serviceName

	go sock.advertise(service.ProxyPort)

	if service.discoveryBrowser != nil {

		// Attempt to resolve discovered unknown service hashes with this service name
		recordsCache := make(map[string]*DNSRecord)
		for _, cachedRecord := range service.discoveryBrowser.cachedDNSRecords {
			if bcrypt.Match(sock.serviceName, cachedRecord.Hash_BCrypt) {
				if dErr := dialProxyFromDNSRecord(cachedRecord, sock); dErr != nil {
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

	return sock
}

func (sock *Socket) advertise(port int) {
	if sock.discoveryService == nil {
		// Advertise new socket type on the network
		sock.discoveryService = NewDiscoveryService(sock.serviceName, sock.serviceHash, sock.proxyPath, port)
		sock.discoveryService.Register("local")
	}
}

// Set up a new web socket connection
func (sock *Socket) ServePeer(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	ws, err := upgradeHTTPToWebSocket(w, r)
	if err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	// Create, bind and start a new peer connection
	peer := NewPeer(ws)
	peer.Start(sock)
}

// Set up a new web socket connection
func (sock *Socket) ServeProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	requestedWebSocketSubProtocols := r.Header.Get("Sec-Websocket-Protocol")
	if requestedWebSocketSubProtocols != "nws-proxy-draft-01" {
		http.Error(w, "Bad Request", 400)
		return
	}

	ws, err := upgradeHTTPToWebSocket(w, r)
	if err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	// Create, bind and start a new proxy connection
	proxy := NewProxy(ws, false)
	proxy.Start(sock)
}

// Send service broadcast messages on Socket connections
func (sock *Socket) messageDispatcher() {
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

// Broadcast a message to all peer connections for this Socket
// instance (except to the src websocket connection)
func (sock *Socket) localBroadcast(broadcast *WireMessage) {
	// Write to peer connections
	for _, peer := range sock.peers {
		// don't send back to self
		if peer.id == broadcast.Source {
			continue
		}
		if wireData, err := encodeWireMessage("broadcast", broadcast.Source, "", broadcast.Payload); err == nil {
			peer.transport.Write(wireData)
		}
	}
}

// Broadcast a message to all proxy connections for this Socket
// instance (except to the src websocket connection)
func (sock *Socket) remoteBroadcast(broadcast *WireMessage) {
	// Only send to remote proxies if this message was not received from a proxy itself
	if broadcast.fromProxy {
		return
	}

	// Write to proxy connections
	for _, proxy := range sock.proxies {
		// don't send back to self
		// only write to *writeable* proxy connections
		if !proxy.writeable || proxy.base.id == broadcast.Source {
			continue
		}
		if wireData, err := encodeWireMessage("broadcast", broadcast.Source, "", broadcast.Payload); err == nil {
			proxy.base.transport.Write(wireData)
		}
	}
}

// Destroy this Network Web Socket service instance, close all
// peer and proxy connections.
func (sock *Socket) Stop() {
	// Close discovery browser
	if sock.discoveryService != nil {
		sock.discoveryService.Shutdown()
	}

	for _, peer := range sock.peers {
		peer.Stop()
	}

	for _, proxy := range sock.proxies {
		proxy.Stop()
	}

	// Indicate object is closed
	sock.done <- 1
}

// StopNotify returns a channel that receives a empty integer
// when the channel service is terminated.
func (sock *Socket) stopNotify() <-chan int { return sock.done }
