package networkwebsockets

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	tls "github.com/richtr/go-tls-srp"
	"github.com/richtr/websocket"
)

func encodeWireMessage(action, source, target, payload string) ([]byte, error) {
	// Construct proxy wire message
	m := WireMessage{
		Action:  action,
		Source:  source,
		Target:  target,
		Payload: payload,
	}

	return json.Marshal(m) // returns ([]byte, error)
}

func decodeWireMessage(msg []byte) (WireMessage, error) {
	var message WireMessage
	err := json.Unmarshal(msg, &message)

	return message, err
}

func upgradeHTTPToWebSocket(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	// Chose a subprotocol from those offered in the client request
	selectedSubprotocol := ""
	if subprotocolsStr := strings.TrimSpace(r.Header.Get("Sec-Websocket-Protocol")); subprotocolsStr != "" {
		// Choose the first subprotocol requested in 'Sec-Websocket-Protocol' header
		selectedSubprotocol = strings.Split(subprotocolsStr, ",")[0]
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  8192,
		WriteBufferSize: 8192,
		CheckOrigin: func(r *http.Request) bool {
			return true // allow all cross-origin access
		},
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

func dialProxyFromDNSRecord(record *DNSRecord, channel *Channel) error {

	hosts := [...]string{record.AddrV4.String(), record.AddrV6.String()}

	for i := 0; i < len(hosts); i++ {

		if hosts[i] == "<nil>" {
			continue
		}

		addr := fmt.Sprintf("%s:%d", hosts[i], record.Port)

		// Build URL
		remoteWSUrl := url.URL{
			Scheme: "wss",
			Host:   addr,
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
				SRPPassword: channel.serviceName,
			},
		}

		ws, _, nErr := tlsSrpDialer.Dial(remoteWSUrl, map[string][]string{
			"Origin":                 []string{"localhost"},
			"Sec-WebSocket-Protocol": []string{"nws-proxy-draft-01"},
		})
		if nErr != nil {
			errStr := fmt.Sprintf("Proxy named web socket connection to wss://%s%s failed: %s", remoteWSUrl.Host, remoteWSUrl.Path, nErr)
			return errors.New(errStr)
		}

		log.Printf("Established proxy named web socket connection to wss://%s%s", remoteWSUrl.Host, remoteWSUrl.Path)

		// Create, bind and start a new proxy connection
		proxyConn := NewProxy(ws, false)
		proxyConn.setHash_Base64(record.Hash_Base64)
		proxyConn.Start(channel)

		return nil

	}

	return errors.New("Could not establish proxy named web socket connection")

}
