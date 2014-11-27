package namedwebsockets

import (
	"encoding/base64"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/richtr/bcrypt"
	tls "github.com/richtr/go-tls-srp"
)

var (
	// Regular expression matchers

	serviceNameRegexStr = "[A-Za-z0-9/\\+=\\*\\._-]{1,255}"

	peerIdRegexStr = "[0-9]{4,}"

	isValidLocalRequest = regexp.MustCompile(fmt.Sprintf("^((/control)?/network/%s/%s)$", serviceNameRegexStr, peerIdRegexStr))

	isValidControlRequest = regexp.MustCompile(fmt.Sprintf("^(/control/network/%s/%s)$", serviceNameRegexStr, peerIdRegexStr))

	isValidProxyRequest = regexp.MustCompile(fmt.Sprintf("^(/network/%s/%s)$", serviceNameRegexStr, peerIdRegexStr))

	isValidServiceName = regexp.MustCompile(fmt.Sprintf("^%s$", serviceNameRegexStr))

	// TLS-SRP configuration components

	// We deliberately only use a weak salt because we don't persistently store TLS-SRP credential data
	Salt = []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}

	serviceTab = CredentialsStore(map[string]string{})
)

type NamedWebSocket_Service struct {
	Host string
	Port int

	ProxyPort int

	networkSockets *NamedWebSocket_Service_Group
}

type NamedWebSocket_Service_Group struct {
	// All Named Web Socket services that this service manages
	Services map[string]*NamedWebSocket

	// Discovery related trackers for services advertised and registered
	knownServiceNames map[string]bool

	AdvertisedServiceHashes map[string]bool

	ResolvedServiceRecords   map[string]*NamedWebSocket_DNSRecord
	UnresolvedServiceRecords map[string]*NamedWebSocket_DNSRecord
}

func NewNamedWebSocketService(host string, port int) *NamedWebSocket_Service {
	if host == "" {
		hostname, err := os.Hostname()
		if err != nil {
			log.Printf("Could not determine device hostname: %v\n", err)
			return nil
		}
		host = hostname
	}

	if port <= 1024 || port >= 65534 {
		port = 9009
	}

	service := &NamedWebSocket_Service{
		Host: host,
		Port: port,

		ProxyPort: 0,

		networkSockets: NewNamedWebSocketServiceGroup(),
	}
	return service
}

func NewNamedWebSocketServiceGroup() *NamedWebSocket_Service_Group {
	return &NamedWebSocket_Service_Group{
		Services:                make(map[string]*NamedWebSocket),
		knownServiceNames:       make(map[string]bool),
		AdvertisedServiceHashes: make(map[string]bool),

		ResolvedServiceRecords:   make(map[string]*NamedWebSocket_DNSRecord),
		UnresolvedServiceRecords: make(map[string]*NamedWebSocket_DNSRecord),
	}
}

func (service *NamedWebSocket_Service) Start() {
	// Start HTTP/WebSocket endpoint server (blocking call)
	go service.StartHTTPServer(true)

	// Start TLS-SRP Named WebSocket (wss) server
	go service.StartProxyServer()

	// Start mDNS/DNS-SD discovery service (with 10 second network polling interval)
	service.StartDiscoveryServer(10)
}

func (service *NamedWebSocket_Service) StartHTTPServer(async bool) {
	// Create a new custom http server multiplexer
	serveMux := http.NewServeMux()

	// Serve the test console
	serveMux.HandleFunc("/", service.serveConsoleTemplate)

	// Serve websocket creation endpoint for localhost clients
	serveMux.HandleFunc("/network/", service.serveLocalWSCreator)

	// Serve websocket control endpoint for localhost clients
	serveMux.HandleFunc("/control/", service.serveLocalWSCreator)

	// Listen and on loopback address + port
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", service.Port))
	if err != nil {
		log.Fatal("Could not serve proxy. ", err)
	}

	log.Printf("Serving Named Web Socket Creator Proxy at address [ ws://localhost:%d/ ]", service.Port)

	if async {
		go http.Serve(listener, serveMux)
	} else {
		http.Serve(listener, serveMux)
	}
}

func (service *NamedWebSocket_Service) StartProxyServer() {
	// Create a new custom http server multiplexer
	serveMux := http.NewServeMux()

	// Serve secure websocket creation endpoints for network clients
	serveMux.HandleFunc("/network/", service.serveProxyWSCreator)

	// Generate random server salt for use in TLS-SRP data storage
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, 32)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	srpSaltKey := string(b)

	tlsServerConfig := &tls.Config{
		SRPLookup:   serviceTab,
		SRPSaltKey:  srpSaltKey,
		SRPSaltSize: len(Salt),
	}

	// Listen on all addresses + port
	tlsSrpListener, err := tls.Listen("tcp", ":0", tlsServerConfig)
	if err != nil {
		log.Fatal(err)
	}

	// Obtain and store the port of the proxy endpoint
	_, port, err := net.SplitHostPort(tlsSrpListener.Addr().String())
	if err != nil {
		log.Fatal(err)
	}

	service.ProxyPort, _ = strconv.Atoi(port)

	log.Printf("Serving Named Web Socket Network Proxy at address [ wss://%s:%d/ ]", service.Host, service.ProxyPort)

	http.Serve(tlsSrpListener, serveMux)
}

func (service *NamedWebSocket_Service) StartDiscoveryServer(timeoutSeconds int) {
	networkDiscoveryServer := NewDiscoveryServer()

	defer networkDiscoveryServer.Shutdown()

	log.Printf("Listening for Named Web Socket services on the local network...")

	for !networkDiscoveryServer.closed {
		networkDiscoveryServer.Browse(service, timeoutSeconds)
	}
}

func (service *NamedWebSocket_Service) serveConsoleTemplate(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Only allow console access from localhost
	if isRequestFromLocalHost := service.checkRequestIsFromLocalHost(r.Host); !isRequestFromLocalHost {
		http.Error(w, fmt.Sprintf("Named WebSockets Test Console is only accessible from the local machine (i.e http://localhost:%d/console)", service.Port), 403)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	if r.URL.Path == "/" {
		fmt.Fprint(w, "<h2>A Named WebSockets Proxy is running on this host</h2>")
		return
	}

	if r.URL.Path != "/console" {
		http.Error(w, "Not Found", 404)
		return
	}

	consoleHTML, err := Asset("_templates/console.html")
	if err != nil {
		// Asset was not found.
		http.Error(w, "Not Found", 404)
		return
	}

	t := template.Must(template.New("console").Parse(string(consoleHTML)))
	if t == nil {
		http.Error(w, "Internal Server Error", 501)
		return
	}

	t.Execute(w, service.Port)
}

func (service *NamedWebSocket_Service) serveLocalWSCreator(w http.ResponseWriter, r *http.Request) {
	// Only allow access from localhost to all services
	if isRequestFromLocalHost := service.checkRequestIsFromLocalHost(r.Host); !isRequestFromLocalHost {
		http.Error(w, fmt.Sprintf("This interface is only accessible from the local machine on this port (%d)", service.Port), 403)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	if isValidRequest := isValidLocalRequest.MatchString(r.URL.Path); !isValidRequest {
		http.Error(w, "Not Found", 404)
		return
	}

	if isValidWSUpgradeRequest := strings.ToLower(r.Header.Get("Upgrade")); isValidWSUpgradeRequest != "websocket" {
		http.Error(w, "Bad Request", 400)
		return
	}

	isControl := isValidControlRequest.MatchString(r.URL.Path)

	pathParts := strings.Split(r.URL.Path, "/")

	peerIdStr := pathParts[len(pathParts)-1]
	serviceName := pathParts[len(pathParts)-2]

	// Remove trailing peerId from service path
	servicePath := fmt.Sprintf("%s", strings.Join(pathParts[0:len(pathParts)-1], "/"))

	// Remove leading "/control" from service path if this is a control request
	if isControl {
		servicePath = fmt.Sprintf("/%s", strings.Join(pathParts[2:len(pathParts)-1], "/"))
	}

	if isValid := isValidServiceName.MatchString(serviceName); !isValid {
		http.Error(w, "Not Found", 404)
		return
	}

	group := service.networkSockets

	// Resolve websocket connection
	sock := group.Services[servicePath]
	if sock == nil {
		sock = NewNamedWebSocket(service, serviceName, service.Port, isControl)
		group.Services[servicePath] = sock
	}

	peerId, _ := strconv.Atoi(peerIdStr)

	// Handle websocket connection
	if isControl {
		sock.serveControl(w, r, peerId)
	} else {
		sock.servePeer(w, r, peerId)
	}
}

func (service *NamedWebSocket_Service) serveProxyWSCreator(w http.ResponseWriter, r *http.Request) {

	if r.Method != "GET" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	if isValidRequest := isValidProxyRequest.MatchString(r.URL.Path); !isValidRequest {
		http.Error(w, "Not Found", 404)
		return
	}

	if isValidWSUpgradeRequest := strings.ToLower(r.Header.Get("Upgrade")); isValidWSUpgradeRequest != "websocket" {
		http.Error(w, "Bad Request", 400)
		return
	}

	group := service.networkSockets

	pathParts := strings.Split(r.URL.Path, "/")

	peerIdStr := pathParts[len(pathParts)-1]
	serviceHash := pathParts[len(pathParts)-2]

	serviceScope := pathParts[len(pathParts)-3]

	// Resolve serviceHash to an active named websocket service name
	serviceBCryptHash, _ := base64.StdEncoding.DecodeString(serviceHash)
	serviceBCryptHashStr := string(serviceBCryptHash)

	for serviceName := range group.knownServiceNames {
		if bcrypt.Match(serviceName, serviceBCryptHashStr) {

			sock := group.Services[fmt.Sprintf("/%s/%s", serviceScope, serviceName)]
			if sock == nil {
				log.Printf("Could not find matching %s NamedWebSocket object for service %s", serviceScope, serviceName)
				continue
			}

			peerId, _ := strconv.Atoi(peerIdStr)
			sock.serveProxy(w, r, peerId)
			return
		}
	}

	http.Error(w, "Not Found", 404)
	return
}

func (service *NamedWebSocket_Service) checkRequestIsFromLocalHost(host string) bool {
	allowedLocalHosts := map[string]bool{
		fmt.Sprintf("localhost:%d", service.Port):        true,
		fmt.Sprintf("127.0.0.1:%d", service.Port):        true,
		fmt.Sprintf("::1:%d", service.Port):              true,
		fmt.Sprintf("%s:%d", service.Host, service.Port): true,
	}

	if allowedLocalHosts[host] {
		return true
	}

	return false
}

/** Simple in-memory storage table for TLS-SRP usernames/passwords **/

type CredentialsStore map[string]string

func (cs CredentialsStore) Lookup(user string) (v, s []byte, grp tls.SRPGroup, err error) {
	grp = tls.SRPGroup4096

	p := cs[user]
	if p == "" {
		return nil, nil, grp, nil
	}

	v = tls.SRPVerifier(user, p, Salt, grp)
	return v, Salt, grp, nil
}
