package networkwebsockets

import (
	"encoding/base64"
	"fmt"
	"log"

	"github.com/richtr/bcrypt"
)

type Channel struct {
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

	// Attached DNS-SD discovery registration and browser for this Network Web Socket
	discoveryService *DiscoveryService

	done chan int // blocks until .Stop() is called
}

// Create a new Channel instance with a given service type
func NewChannel(service *Service, serviceName string) *Channel {
	serviceHash_BCrypt, _ := bcrypt.HashBytes([]byte(serviceName))
	serviceHash_Base64 := base64.StdEncoding.EncodeToString(serviceHash_BCrypt)

	channel := &Channel{
		serviceName: serviceName,
		serviceHash: serviceHash_Base64,

		servicePath: fmt.Sprintf("/%s", serviceName),

		peers:           make([]*Peer, 0),
		proxies:         make([]*Proxy, 0),
		broadcastBuffer: make(chan *WireMessage, 512),

		done: make(chan int, 1),
	}

	channel.proxyPath = fmt.Sprintf("/%s", GenerateId())

	go channel.messageDispatcher()

	log.Printf("New '%s' channel peer created.", channel.serviceName)

	service.Channels[channel.servicePath] = channel

	// Terminate channel when it is closed
	go func() {
		<-channel.stopNotify()
		delete(service.Channels, channel.servicePath)
	}()

	// Add TLS-SRP credentials for access to this service to credentials store
	// TODO isolate this per socket
	serviceTab[channel.serviceHash] = channel.serviceName

	go channel.advertise(service.ProxyPort)

	if service.discoveryBrowser != nil {

		// Attempt to resolve discovered unknown service hashes with this service name
		recordsCache := make(map[string]*DNSRecord)
		for _, cachedRecord := range service.discoveryBrowser.cachedDNSRecords {
			if bcrypt.Match(channel.serviceName, cachedRecord.Hash_BCrypt) {
				if dErr := dialProxyFromDNSRecord(cachedRecord, channel); dErr != nil {
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

	return channel
}

func (channel *Channel) advertise(port int) {
	if channel.discoveryService == nil {
		// Advertise new socket type on the network
		channel.discoveryService = NewDiscoveryService(channel.serviceName, channel.serviceHash, channel.proxyPath, port)
		channel.discoveryService.Register("local")
	}
}

// Send service broadcast messages on Channel connections
func (channel *Channel) messageDispatcher() {
	for {
		select {
		case wsBroadcast, ok := <-channel.broadcastBuffer:
			if !ok {
				return
			}
			// Send message to local peers
			channel.localBroadcast(wsBroadcast)
			// Send message to remote proxies
			channel.remoteBroadcast(wsBroadcast)
		}
	}
}

// Broadcast a message to all peer connections for this Channel
// instance (except to the src websocket connection)
func (channel *Channel) localBroadcast(broadcast *WireMessage) {
	// Write to peer connections
	for _, peer := range channel.peers {
		// don't send back to self
		if peer.id == broadcast.Source {
			continue
		}
		if wireData, err := encodeWireMessage("broadcast", broadcast.Source, "", broadcast.Payload); err == nil {
			peer.transport.Write(wireData)
		}
	}
}

// Broadcast a message to all proxy connections for this Channel
// instance (except to the src websocket connection)
func (channel *Channel) remoteBroadcast(broadcast *WireMessage) {
	// Only send to remote proxies if this message was not received from a proxy itself
	if broadcast.fromProxy {
		return
	}

	// Write to proxy connections
	for _, proxy := range channel.proxies {
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
func (channel *Channel) Stop() {
	// Close discovery browser
	if channel.discoveryService != nil {
		channel.discoveryService.Shutdown()
	}

	for _, peer := range channel.peers {
		peer.Stop()
	}

	for _, proxy := range channel.proxies {
		proxy.Stop()
	}

	// Indicate object is closed
	channel.done <- 1
}

// StopNotify returns a channel that receives a empty integer
// when the channel service is terminated.
func (channel *Channel) stopNotify() <-chan int { return channel.done }
