package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"path"
	"regexp"
	"time"

	"github.com/andrewtj/dnssd"
	"github.com/gorilla/websocket"
)

var advertisedServiceNames = map[string]bool{}

var NetworkServiceMatcher = regexp.MustCompile("^[0-9]+\\..*\\._bws( \\([0-9]+\\))?$")

func NewDiscoveryClient(serviceType string) *DiscoveryClient {
	discoveryClient := &DiscoveryClient{
		serviceType: serviceType,
	}

	// Register new service in network
	discoveryClient.registerOp = discoveryClient.register()

	if err := discoveryClient.registerOp.Start(); err != nil {
		log.Printf("Failed to register proxy web socket: %s", err)
		return nil
	}

	return discoveryClient
}

func NewDiscoveryServer() *DiscoveryServer {
	discoveryServer := &DiscoveryServer{}

	// Register new service in network
	discoveryServer.browseOp = discoveryServer.browse()

	if err := discoveryServer.browseOp.Start(); err != nil {
		log.Printf("Failed to start proxy web socket network services: %s", err)
		return nil
	}

	log.Print("Listening for BroadcastWebSocket proxies in network...")

	return discoveryServer
}

/** DISCOVERYCLIENT interface **/

type DiscoveryClient struct {
	serviceType string
	registerOp  *dnssd.RegisterOp
}

func (dc *DiscoveryClient) register() *dnssd.RegisterOp {

	rand.Seed(time.Now().UTC().UnixNano())

	dnssdServiceName := fmt.Sprintf("%d.%s._bws", rand.Int(), dc.serviceType)

	op := dnssd.NewRegisterOp(dnssdServiceName, "_ws._tcp", LocalPort, dc.registerCallback)

	// Add TXT record to DNS-SD registration record
	key, value := "path", fmt.Sprintf("/broadcast/%s", dc.serviceType)
	if err := op.SetTXTPair(key, value); err != nil {
		log.Printf(`Unexpected error setting proxy web socket key "%s", value "%s": %v`, key, value, err)
		return nil
	}

	return op
}

func (dc *DiscoveryClient) registerCallback(op *dnssd.RegisterOp, err error, add bool, name, serviceType, domain string) {
	if err != nil {
		// op is now inactive
		log.Printf("Proxy web socket advertisement failed: %s", err)
		return
	}

	if add {
		advertisedServiceNames[name] = true
		log.Printf("Proxy web socket advertised as “%s“ in %s", name, domain)
	} else {
		delete(advertisedServiceNames, name)
		log.Printf("Proxy web socket “%s” removed from %s", name, domain)
	}
}

/** DISCOVERYSERVER interface **/

type DiscoveryServer struct {
	browseOp *dnssd.BrowseOp
}

func (ds *DiscoveryServer) browse() *dnssd.BrowseOp {
	op := dnssd.NewBrowseOp("_ws._tcp", ds.browseCallback)

	// Set local DNS-SD search domain
	op.SetDomain("local.")

	return op
}

func (ds *DiscoveryServer) browseCallback(op *dnssd.BrowseOp, err error, add bool, interfaceIndex int, name string, serviceType string, domain string) {
	if err != nil {
		// op is now inactive
		log.Printf("Proxy web socket browse operation failed: %s", err)
		return
	}

	// Discard our own BroadcastWebSocket services
	if isOwnedService := advertisedServiceNames[name]; isOwnedService {
		return
	}

	// Is this a BroadcastWebSocket service?
	if isBroadcastWebSocketService := NetworkServiceMatcher.FindString(name); isBroadcastWebSocketService == "" {
		log.Printf("Ignoring non broadcast web socket advertisement: %s", name)
		return
	}

	if add {
		// Resolve discovered service and connect to service
		op := dnssd.NewResolveOp(interfaceIndex, name, serviceType, domain, ds.resolveCallback)
		if err := op.Start(); err != nil {
			log.Printf("Failed to start proxy web socket resolve operation: %s", err)
			return
		}
	}
}

func (ds *DiscoveryServer) resolveCallback(op *dnssd.ResolveOp, err error, host string, port int, txt map[string]string) {
	if err != nil {
		// op is now inactive
		log.Printf("Resolve operation failed: %s", err)
		return
	}

	// Build websocket data from returned information

	servicePath := txt["path"] // can be an empty string

	// Build URL
	remoteWSUrl := &url.URL{
		Scheme: "ws",
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   servicePath,
	}

	serviceName := path.Base(servicePath)

	// Resolve websocket connection
	sock := namedWebSockets[servicePath]
	if sock == nil {
		sock = NewNamedWebSocket(serviceName, true)
		namedWebSockets[servicePath] = sock
	}

	log.Printf("Establishing proxy web socket connection to ws://%s:%d%s", host, port, servicePath)

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
}
