package networkwebsockets

import (
	//"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/richtr/websocket"
)

type Proxy struct {
	// Inherit attributes from Peer struct
	base Peer

	// Discovered proxy connection's base64 hash value
	// empty unless set via .setHash_Base64()
	Hash_Base64 string

	// List of connection ids that this proxy connection 'owns'
	peerIds map[string]bool

	// Whether this proxy connection is writeable
	writeable bool
}

type ProxyMessageHandler struct {
	proxy *Proxy
}

func (handler *ProxyMessageHandler) Read(buf []byte) error {
	proxy := handler.proxy
	if proxy == nil {
		return errors.New("ProxyMessageHandler requires an attached Proxy object")
	}

	message, err := decodeWireMessage(buf)
	if err != nil {
		return err
	}

	switch message.Action {
	case "connect":

		proxy.peerIds[message.Target] = true

		// Inform all local peer connections that this proxy owns this peer connection
		for _, peer := range proxy.base.channel.peers {
			if wireData, err := encodeWireMessage("connect", peer.id, message.Target, ""); err == nil {
				peer.transport.Write(wireData)
			}
		}

		return nil

	case "disconnect":

		delete(proxy.peerIds, message.Target)

		// Inform all local peer connections that this proxy no longer owns this peer connection
		for _, peer := range proxy.base.channel.peers {
			if wireData, err := encodeWireMessage("disconnect", peer.id, message.Target, ""); err == nil {
				peer.transport.Write(wireData)
			}
		}

		return nil

	case "broadcast":

		// broadcast message on to given target
		wsBroadcast := &WireMessage{
			Action:    "broadcast",
			Source:    message.Source,
			Target:    "", // target all connections
			Payload:   message.Payload,
			fromProxy: true,
		}

		proxy.base.channel.broadcastBuffer <- wsBroadcast

		return nil

	case "message":

		messageSent := false

		// Relay message to channel peer that matches target
		for _, peer := range proxy.base.channel.peers {
			if peer.id == message.Target {
				if wireData, err := encodeWireMessage("message", message.Source, message.Target, message.Payload); err == nil {
					peer.transport.Write(wireData)
				}
				messageSent = true
				break
			}
		}

		if !messageSent {
			fmt.Errorf("P2P message target could not be found. Not sent.")
		}

		return nil
	}

	return errors.New("Could not find target for message")
}

func (handler *ProxyMessageHandler) Write(buf []byte) error {
	proxy := handler.proxy
	if proxy == nil {
		return errors.New("ProxyMessageHandler requires an attached Proxy object")
	}

	if !proxy.base.active {
		return errors.New("Proxy is not active")
	}

	proxy.base.transport.conn.SetWriteDeadline(time.Now().Add(writeWait))
	proxy.base.transport.conn.WriteMessage(websocket.TextMessage, buf)

	return nil
}

func NewProxy(conn *websocket.Conn, isWriteable bool) *Proxy {
	proxyConn := &Proxy{
		base: Peer{
			id: GenerateId(),
		},
		Hash_Base64: "",
		writeable:   isWriteable,
		peerIds:     make(map[string]bool),
	}

	// Create a new peer socket message handler
	handler := &ProxyMessageHandler{proxyConn}

	// Create a new peer socket transporter
	transport := NewTransport(conn, handler)

	// Attach peer transport to peer object
	proxyConn.base.transport = transport

	return proxyConn
}

func (proxy *Proxy) Start(channel *Channel) error {
	if channel == nil {
		return errors.New("Proxy requires a channel to start")
	}

	if proxy.base.active {
		return errors.New("Proxy is already started")
	}

	// Start connection read/write pumps
	proxy.base.transport.Start()
	go func() {
		<-proxy.base.transport.StopNotify()
		proxy.Stop()
	}()

	proxy.base.channel = channel
	proxy.base.active = true

	// Add reference to this proxy connection to channel
	proxy.addConnection()

	return nil
}

func (proxy *Proxy) Stop() error {
	if !proxy.base.active {
		return errors.New("Proxy cannot be stopped because it is not currently active")
	}

	// Remove references to this proxy connection from channel
	proxy.removeConnection()

	// Close underlying websocket connection
	proxy.base.transport.Stop()

	// If no more local peers are connected then remove the current Network Web Socket service
	if len(proxy.base.channel.peers) == 0 {
		proxy.base.channel.Stop()
	}

	proxy.base.active = false

	return nil
}

func (proxy *Proxy) setHash_Base64(hash string) {
	proxy.Hash_Base64 = hash
}

// Set up a new Channel connection instance
func (proxy *Proxy) addConnection() {
	proxy.base.channel.proxies = append(proxy.base.channel.proxies, proxy)

	if proxy.writeable {
		// Inform this proxy of all the peer connections we own
		for _, peer := range proxy.base.channel.peers {
			if wireData, err := encodeWireMessage("connect", proxy.base.id, peer.id, ""); err == nil {
				proxy.base.transport.Write(wireData)
			}
		}
	}
}

// Tear down an existing Channel connection instance
func (proxy *Proxy) removeConnection() {
	for i, conn := range proxy.base.channel.proxies {
		if proxy.base.id == conn.base.id {
			proxy.base.channel.proxies = append(proxy.base.channel.proxies[:i], proxy.base.channel.proxies[i+1:]...)
			break
		}
	}

	if proxy.writeable {
		// Inform this proxy of all the peer connections we no longer own
		for _, peer := range proxy.base.channel.peers {
			if wireData, err := encodeWireMessage("disconnect", proxy.base.id, peer.id, ""); err == nil {
				proxy.base.transport.Write(wireData)
			}
		}
	}
}
