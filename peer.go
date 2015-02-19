package networkwebsockets

import (
	"errors"
	"time"

	"github.com/richtr/websocket"
)

type Peer struct {
	// Unique identifier for this peer connection
	id string

	// The Network Web Socket channel to which this peer connection belongs
	channel *Channel

	// Transport object
	transport *Transport

	active bool
}

type PeerMessageHandler struct {
	peer *Peer
}

func (handler *PeerMessageHandler) Read(buf []byte) error {
	peer := handler.peer
	if peer == nil {
		return errors.New("PeerMessageHandler requires an attached Peer object")
	}

	message, err := decodeWireMessage(buf)
	if err != nil {
		return err
	}

	switch message.Action {

	case "connect":
	case "disconnect":
		// 'connect' and 'disconnect' events are write-only so will not be handled here
		return nil

	case "status":

		// Echo peer id back to callee
		wireData, err := encodeWireMessage("status", peer.id, peer.id, "")

		if err != nil {
			return err
		}

		peer.transport.Write(wireData)

		return nil

	case "broadcast":

		wsBroadcast := &WireMessage{
			Action:    "broadcast",
			Source:    peer.id,
			Target:    "", // target all connections
			Payload:   message.Payload,
			fromProxy: false,
		}
		peer.channel.broadcastBuffer <- wsBroadcast

		return nil

	case "message":

		if message.Target == "" {
			return errors.New("Message must have a target identifier")
		}

		wireData, err := encodeWireMessage("message", peer.id, message.Target, message.Payload)

		if err != nil {
			return err
		}

		// Relay message to peer channel that matches target
		for _, _peer := range peer.channel.peers {
			if _peer.id == message.Target {
				_peer.transport.Write(wireData)
				return nil
			}
		}

		// If we have not delivered the message yet then hunt for a
		// proxy that owns target peer id in known proxies
		for _, proxy := range peer.channel.proxies {
			if proxy.peerIds[message.Target] {
				proxy.base.transport.Write(wireData)
				return nil
			}
		}

	}

	return errors.New("Could not find target for message")
}

func (handler *PeerMessageHandler) Write(buf []byte) error {
	peer := handler.peer
	if peer == nil {
		return errors.New("PeerMessageHandler requires an attached Peer object")
	}

	if !peer.active {
		return errors.New("Peer is not active")
	}

	peer.transport.conn.SetWriteDeadline(time.Now().Add(writeWait))
	peer.transport.conn.WriteMessage(websocket.TextMessage, buf)

	return nil
}

func NewPeer(conn *websocket.Conn) *Peer {
	peerConn := &Peer{
		id: GenerateId(),
	}

	// Create a new peer socket message handler
	handler := &PeerMessageHandler{peerConn}

	// Create a new peer socket transporter
	transport := NewTransport(conn, handler)

	// Attach peer transport to peer object
	peerConn.transport = transport

	return peerConn
}

func (peer *Peer) Start(channel *Channel) error {
	if channel == nil {
		return errors.New("Peer requires a channel to start")
	}

	if peer.active {
		return errors.New("Peer is already started")
	}

	// Start connection read/write pumps
	peer.transport.Start()
	go func() {
		<-peer.transport.StopNotify()
		peer.Stop()
	}()

	peer.channel = channel
	peer.active = true

	// Add reference to this peer connection to channel
	peer.addConnection()

	return nil
}

func (peer *Peer) Stop() error {
	if !peer.active {
		return errors.New("Peer cannot be stopped because it is not currently active")
	}

	// Remove references to this peer connection from channel
	peer.removeConnection()

	// Close websocket connection
	peer.transport.Stop()

	// If no more local peers are connected then remove the current Network Web Socket service
	if len(peer.channel.peers) == 0 {
		peer.channel.Stop()
	}

	peer.active = false

	return nil
}

// Set up a new Channel connection instance
func (peer *Peer) addConnection() {
	// Add this websocket instance to Network Web Socket broadcast list
	peer.channel.peers = append(peer.channel.peers, peer)

	for _, _peer := range peer.channel.peers {
		if _peer.id != peer.id {
			// Inform other local peer connections that we now own this peer
			if wireData, err := encodeWireMessage("connect", _peer.id, peer.id, ""); err == nil {
				_peer.transport.Write(wireData)
			}

			// Inform this peer of all the other peer connections we own
			if wireData, err := encodeWireMessage("connect", peer.id, _peer.id, ""); err == nil {
				peer.transport.Write(wireData)
			}
		}
	}

	for _, proxy := range peer.channel.proxies {
		// Inform all proxy connections that we now own this peer connection
		if proxy.writeable {
			if wireData, err := encodeWireMessage("connect", proxy.base.id, peer.id, ""); err == nil {
				proxy.base.transport.Write(wireData)
			}
		}
		// Inform current peer of all the peer connections other connected proxies own
		for peerId, _ := range proxy.peerIds {
			if wireData, err := encodeWireMessage("connect", proxy.base.id, peerId, ""); err == nil {
				peer.transport.Write(wireData)
			}
		}
	}
}

// Tear down an existing Channel connection instance
func (peer *Peer) removeConnection() {
	for i, conn := range peer.channel.peers {
		if conn.id == peer.id {
			peer.channel.peers = append(peer.channel.peers[:i], peer.channel.peers[i+1:]...)
			break
		}
	}

	// Inform all local peer connections that we no longer own this peer connection
	for _, _peer := range peer.channel.peers {
		// don't notify peer if its id matches the peer's id
		if _peer.id != peer.id {
			if wireData, err := encodeWireMessage("disconnect", _peer.id, peer.id, ""); err == nil {
				_peer.transport.Write(wireData)
			}
		}
	}

	// Inform all proxy connections that we no longer own this peer connection
	for _, proxy := range peer.channel.proxies {
		if proxy.writeable {
			if wireData, err := encodeWireMessage("disconnect", proxy.base.id, peer.id, ""); err == nil {
				proxy.base.transport.Write(wireData)
			}
		}
	}
}
