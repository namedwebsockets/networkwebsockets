package namedwebsockets

import (
	"encoding/base64"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jameskeane/bcrypt"
	"github.com/richtr/mdns"
	tls "bitbucket.org/mjl/go-tls-srp"
)

/** DISCOVERYCLIENT interface **/

type DiscoveryClient struct {
	ServiceHash string
	Port        int
	server      *mdns.Server
}

func NewDiscoveryClient(serviceHash string, port int) *DiscoveryClient {
	discoveryClient := &DiscoveryClient{
		ServiceHash: serviceHash,
		Port:        port,
	}

	discoveryClient.Register("local")

	return discoveryClient
}

func (dc *DiscoveryClient) Register(domain string) {

	rand.Seed(time.Now().UTC().UnixNano())

	dnssdServiceId := fmt.Sprintf("%d", rand.Int())

	s := &mdns.MDNSService{
		Instance: dnssdServiceId,
		Service:  "_nws._tcp",
		Domain:   domain,
		Port:     dc.Port,
		Info:     fmt.Sprintf("path=/%s", dc.ServiceHash),
	}
	if err := s.Init(); err != nil {
		log.Fatalf("err: %v", err)
	}

	serv, err := mdns.NewServer(&mdns.Config{Zone: s})
	if err != nil {
		log.Fatalf("err: %v", err)
	}

	dc.server = serv

	log.Printf("Network websocket advertised as '%s' in %s network", fmt.Sprintf("%s._nws._tcp", dnssdServiceId), domain)
}

func (dc *DiscoveryClient) Shutdown() {
	if dc.server != nil {
		dc.server.Shutdown()
	}
}

/** DISCOVERYSERVER interface **/

type DiscoveryServer struct {
	Host   string
	Port   int
	closed bool
}

func (ds *DiscoveryServer) Browse(service *NamedWebSocket_Service) {

	entries := make(chan *mdns.ServiceEntry, 255)

	timeout := 20 * time.Second

	params := &mdns.QueryParam{
		Service: "_nws._tcp",
		Domain:  "local",
		Timeout: timeout,
		Entries: entries,
	}

	go func() {
		complete := false
		finish := time.After(timeout)

		// Wait for responses until timeout
		RecordCheck:
			for !complete {
				select {
				case e, ok := <-entries:

					if !ok {
						continue
					}

					// DEBUG
					//log.Printf("Found proxy web socket [%s] @ [%s:%d] TXT[%s]", shortName, e.Host, e.Port, e.Info)

					// Build websocket data from returned information
					servicePath := "/"
					serviceHash := ""

					serviceParts := strings.FieldsFunc(e.Info, func(r rune) bool {
						return r == '=' || r == ',' || r == ';' || r == ' '
					})
					if len(serviceParts) > 1 {
						for i := 0; i < len(serviceParts); i += 2 {
							if strings.ToLower(serviceParts[i]) == "path" {
								pathStr := serviceParts[i+1]
								serviceHashB, _ := base64.StdEncoding.DecodeString( path.Base(pathStr) ) // strip leading '/'
								serviceHash = string(serviceHashB[:])
								servicePath = pathStr
								break
							}
						}
					}

					shortName := ""
					serviceName := ""

					// Resolve service hash provided against advertised services
					for knownServiceName := range service.knownServiceNames {
						if bcrypt.Match(knownServiceName, serviceHash) {

							serviceName = knownServiceName

							shortName = fmt.Sprintf("/network/%s/", knownServiceName)

							// Ignore our own NetworkWebSocket services
							if isOwned := service.advertisedServiceNames[serviceName]; isOwned {
								continue RecordCheck
							}

							// Ignore previously discovered NetworkWebSocket services
							if isRegistered := service.registeredServiceNames[serviceName]; isRegistered {
								continue RecordCheck
							}

							break
						}
					}

					// Generate unique id for connection
					rand.Seed(time.Now().UTC().UnixNano())
					newPeerId := rand.Int()

					// Resolve websocket connection
					sock := service.namedWebSockets[shortName]
					if sock == nil {
						sock = NewNamedWebSocket(service, serviceName, true, ds.Port)
						service.namedWebSockets[shortName] = sock
					}

					hosts := [...]string{e.Host, e.AddrV4.String(), e.AddrV6.String()}

					for i := 0; i < len(hosts); i++ {

						// Build URL
						remoteWSUrl := &url.URL{
							Scheme: "wss",
							Host:   fmt.Sprintf("%s:%d", hosts[i], e.Port),
							Path:   fmt.Sprintf("%s/%d", servicePath, newPeerId),
						}

						log.Printf("Establishing proxy network websocket connection to wss://%s%s", remoteWSUrl.Host, remoteWSUrl.Path)

						// Establish Proxy WebSocket connection over TLS-SRP

						tlsSrpConfig := new(tls.Config)
						tlsSrpConfig.SRPUser = serviceHash
						tlsSrpConfig.SRPPassword = serviceName

						tlsSrpDialer := TLSSRPDialer{}

						ws, _, nErr := tlsSrpDialer.Dial(remoteWSUrl, tlsSrpConfig, map[string][]string{
							"Origin":                   []string{ds.Host},
							"X-NetworkWebSocket-Proxy": []string{"true"},
						})
						if nErr != nil {
							log.Printf("Proxy network websocket connection failed: %s", nErr)
							continue
						}

						proxyConn := NewProxyConnection(newPeerId, ws, false)

						proxyConn.addConnection(sock)

						service.registeredServiceNames[serviceName] = true

						break

					}

				case <-finish:
					complete = true
				}
			}
	}()

	// Run the mDNS query
	err := mdns.Query(params)
	if err != nil {
		log.Fatalf("err: %v", err)
	}
}

func (ds *DiscoveryServer) Shutdown() {
	ds.closed = true
}


type TLSSRPDialer struct {
	*websocket.Dialer
}

// Dial creates a new TLS-SRP based client connection. Use requestHeader to specify the
// origin (Origin), subprotocols (Sec-WebSocket-Protocol) and cookies (Cookie).
// Use the response.Header to get the selected subprotocol
// (Sec-WebSocket-Protocol) and cookies (Set-Cookie).
//
// If the WebSocket handshake fails, ErrBadHandshake is returned along with a
// non-nil *http.Response so that callers can handle redirects, authentication,
// etc.
func (d *TLSSRPDialer) Dial(url *url.URL, tlsSrpConfig *tls.Config, requestHeader http.Header) (*websocket.Conn, *http.Response, error) {
	if d == nil {
		d = &TLSSRPDialer{}
	}

	var deadline time.Time
	if d.HandshakeTimeout != 0 {
		deadline = time.Now().Add(d.HandshakeTimeout)
	}

	netConn, err := tls.Dial("tcp", url.Host, tlsSrpConfig)
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

	conn, resp, err := websocket.NewClient(netConn, url, requestHeader, d.ReadBufferSize, d.WriteBufferSize)
	if err != nil {
		return nil, resp, err
	}

	netConn.SetDeadline(time.Time{})
	netConn = nil // to avoid close in defer.
	return conn, resp, nil
}

