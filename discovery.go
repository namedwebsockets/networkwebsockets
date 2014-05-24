package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/richtr/mdns"
)

var advertisedServiceNames = map[string]bool{}
var registeredServiceNames = map[string]bool{}

var NetworkServiceMatcher = regexp.MustCompile("^([A-Za-z0-9\\._-]{1,255})\\[[0-9]+\\]( \\([0-9]+\\))?$")

/** DISCOVERYCLIENT interface **/

type DiscoveryClient struct {
	serviceType string
	server      *mdns.Server
}

func NewDiscoveryClient(serviceType string) *DiscoveryClient {
	discoveryClient := &DiscoveryClient{
		serviceType: serviceType,
	}

	discoveryClient.Register("local")

	return discoveryClient
}

func (dc *DiscoveryClient) Register(domain string) {

	rand.Seed(time.Now().UTC().UnixNano())

	dnssdServiceName := fmt.Sprintf("%s[%d]", dc.serviceType, rand.Int())

	s := &mdns.MDNSService{
		Instance: dnssdServiceName,
		Service:  "_ws._tcp",
		Domain:   domain,
		Port:     LocalPort,
		Info:     fmt.Sprintf("path=/broadcast/%s", dc.serviceType),
	}
	if err := s.Init(); err != nil {
		log.Fatalf("err: %v", err)
	}

	serv, err := mdns.NewServer(&mdns.Config{Zone: s})
	if err != nil {
		log.Fatalf("err: %v", err)
	}

	dc.server = serv

	advertisedServiceNames[dnssdServiceName] = true

	log.Printf("Proxy web socket advertised as '%s' in %s", fmt.Sprintf("%s._ws._tcp", dnssdServiceName), domain)
}

func (dc *DiscoveryClient) Shutdown() {
	if dc.server != nil {
		dc.server.Shutdown()
	}
}

/** DISCOVERYSERVER interface **/

type DiscoveryServer struct {
	closed bool
}

func StartDiscoveryServer() {
	discoveryServer := &DiscoveryServer{}
	defer discoveryServer.Close()

	log.Print("Listening for BroadcastWebSocket proxies in network...")

	for !discoveryServer.closed {
		discoveryServer.Browse()
	}
}

func (ds *DiscoveryServer) Browse() {

	entries := make(chan *mdns.ServiceEntry, 255)

	timeout := 2 * time.Second

	params := &mdns.QueryParam{
		Service: "_ws._tcp",
		Domain:  "local",
		Timeout: timeout,
		Entries: entries,
	}

	go func() {
		complete := false
		finish := time.After(timeout)

		// Wait for responses until timeout
		for !complete {
			select {
			case e, ok := <-entries:

				if !ok {
					continue
				}

				nameComponents := strings.Split(e.Name, ".")
				shortName := ""

				for i := len(nameComponents) - 1; i >= 0; i-- {
					if nameComponents[i] == "_ws" {
						shortName = strings.Join(nameComponents[:i], ".")
						break
					}
				}

				// DEBUG
				//log.Printf("Found proxy web socket [%s] @ [%s:%d] TXT[%s]", shortName, e.Host, e.Port, e.Info)

				// Is this a BroadcastWebSocket service?
				if isValid := NetworkServiceMatcher.FindString(shortName); isValid == "" {
					continue
				}

				// Ignore our own BroadcastWebSocket services
				if isOwned := advertisedServiceNames[shortName]; isOwned {
					continue
				}

				// Ignore previously discovered BroadcastWebSocket services
				if isRegistered := registeredServiceNames[shortName]; isRegistered {
					continue
				}

				// Build websocket data from returned information

				servicePath := strings.Split(e.Info, "=")[1] // can be an empty string

				// Build URL
				remoteWSUrl := &url.URL{
					Scheme: "ws",
					Host:   fmt.Sprintf("%s:%d", e.Host, e.Port),
					Path:   servicePath,
				}

				serviceName := path.Base(servicePath)

				// Resolve websocket connection
				sock := namedWebSockets[servicePath]
				if sock == nil {
					sock = NewNamedWebSocket(serviceName, true)
					namedWebSockets[servicePath] = sock
				}

				log.Printf("Establishing proxy web socket connection to ws://%s%s", remoteWSUrl.Host, remoteWSUrl.Path)

				ws, _, nErr := websocket.DefaultDialer.Dial(remoteWSUrl.String(), map[string][]string{
					"Origin":                     []string{LocalHost},
					"X-BroadcastWebSocket-Proxy": []string{"true"},
				})
				if nErr != nil {
					log.Printf("Proxy web socket connection failed: %s", nErr)
					return
				}

				conn := &Connection{
					ws:      ws,
					isProxy: true,
				}

				// Don't block discovery process
				go sock.addConnection(conn, false)

				registeredServiceNames[shortName] = true

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

func (ds *DiscoveryServer) Close() {
	ds.closed = true
}
