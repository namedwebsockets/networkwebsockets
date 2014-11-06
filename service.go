package namedwebsockets

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/richtr/bcrypt"
	tls "github.com/richtr/go-tls-srp"
)

var (
	// Regular expression matchers

	serviceNameRegexStr = "[A-Za-z0-9\\._-]{1,255}"

	peerIdRegexStr = "[0-9]{4,}"

	isNetworkRequest = regexp.MustCompile(fmt.Sprintf("^(.*/network/%s/%s)$", serviceNameRegexStr, peerIdRegexStr))

	isControlRequest = regexp.MustCompile(fmt.Sprintf("(/control/(network|local)/%s/%s)", serviceNameRegexStr, peerIdRegexStr))

	isValidServiceName = regexp.MustCompile(fmt.Sprintf("^%s$", serviceNameRegexStr))

	// TLS-SRP configuration components

	// We deliberately only use a weak salt because we don't persistently store TLS-SRP credential data
	Salt = []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}

	serviceTab = SRPCredentialsStore(map[string]string{})

)

// Simple in-memory storage table for TLS-SRP usernames/passwords
type SRPCredentialsStore map[string]string

func (cs SRPCredentialsStore) Lookup(user string) (v, s []byte, grp tls.SRPGroup, err error) {
	grp = tls.SRPGroup4096

	log.Println("Lookup for", user)
	p := cs[user]
	if p == "" {
		return nil, nil, grp, nil
	}

	v = tls.SRPVerifier(user, p, Salt, grp)
	return v, Salt, grp, nil
}

type NamedWebSocket_Service struct {
	Host string
	Port int

	// All Named WebSocket services (local or network) that this service manages
	namedWebSockets map[string]*NamedWebSocket

	// Discovery related trackers for services advertised and registered
	knownServiceNames       map[string]bool
	advertisedServiceHashes map[string]bool
	registeredServiceHashes map[string]bool
}

func NewNamedWebSocketService(host string, port int) *NamedWebSocket_Service {
	service := &NamedWebSocket_Service{
		Host: host,
		Port: port,

		namedWebSockets:         make(map[string]*NamedWebSocket),
		knownServiceNames:       make(map[string]bool),
		advertisedServiceHashes: make(map[string]bool),
		registeredServiceHashes: make(map[string]bool),
	}
	return service
}

func (service *NamedWebSocket_Service) StartHTTPServer() {
	// Create a new custom http server multiplexer
	serveMux := http.NewServeMux()

	// Serve the test console
	serveMux.HandleFunc("/", service.serveConsoleTemplate)

	// Serve websocket creation endpoints for localhost clients
	serveMux.HandleFunc("/local/", service.serveLocalWSCreator)
	serveMux.HandleFunc("/network/", service.serveLocalWSCreator)
	serveMux.HandleFunc("/control/", service.serveLocalWSCreator)

	log.Printf("Serving Named WebSockets Proxy at http://localhost:%d/", service.Port)

	// Listen and serve on all ports (public + loopback addresses)
	if err := http.ListenAndServe(fmt.Sprintf("localhost:%d", service.Port), serveMux); err != nil {
		log.Fatal("Could not serve proxy. ", err)
	}
}

func (service *NamedWebSocket_Service) StartNamedWebSocketServer() {
	// Create a new custom http server multiplexer
	serveMux := http.NewServeMux()

	// Serve secure websocket creation endpoints for network clients (network-only wss endpoints)
	serveMux.HandleFunc("/", service.serveProxyWSCreator)

	// Generate random server salt for use in TLS-SRP data storage
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, 32)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	srpSaltKey := string(b)

	tlsServerConfig := &tls.Config{
		SRPLookup: serviceTab,
		SRPSaltKey: srpSaltKey,
		SRPSaltSize: len(Salt),
	}

	tlsSrpListener, err := tls.Listen("tcp", fmt.Sprintf(":%d", service.Port + 1), tlsServerConfig)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Serving Named WebSockets Federation Server at wss://%s:%d/", service.Host, service.Port + 1)

	http.Serve(tlsSrpListener, serveMux)


}

func (service *NamedWebSocket_Service) StartDiscoveryServer() {
	discoveryServer := &DiscoveryServer{
		Host: service.Host,
		Port: service.Port,
	}

	defer discoveryServer.Shutdown()

	log.Print("Listening for network websocket advertisements in local network...")

	for !discoveryServer.closed {
		discoveryServer.Browse(service)
	}
}

func (service *NamedWebSocket_Service) serveConsoleTemplate(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	if r.URL.Path == "/" {
		fmt.Fprint(w, "<h2>A Named WebSockets Proxy is running on this host</h2>")
		return
	}

	if r.URL.Path != "/console" {
		http.Error(w, "Not found", 404)
		return
	}

	// Only allow console access from localhost
	if r.Host != fmt.Sprintf("localhost:%d", service.Port) && r.Host != fmt.Sprintf("127.0.0.1:%d", service.Port) {
		http.Error(w, fmt.Sprintf("Named WebSockets Test Console is only accessible from the local machine (i.e http://localhost:%d/console)", service.Port), 403)
		return
	}

	consoleHTML, err := Asset("_templates/console.html")
	if err != nil {
		// Asset was not found.
		http.Error(w, "Not found", 404)
		return
	}

	t := template.Must(template.New("console").Parse(string(consoleHTML)))
	if t == nil {
		http.Error(w, "Internal server error", 501)
		return
	}

	t.Execute(w, service.Port)
}

func (service *NamedWebSocket_Service) serveLocalWSCreator(w http.ResponseWriter, r *http.Request) {
	// Only allow access from localhost
	if r.Host != fmt.Sprintf("localhost:%d", service.Port) && r.Host != fmt.Sprintf("127.0.0.1:%d", service.Port) {
		http.Error(w, fmt.Sprintf("Named WebSocket Endpoints are only accessible from the local machine on this port (%d)", service.Port), 403)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	if isValidWSUpgradeRequest := strings.ToLower(r.Header.Get("Upgrade")); isValidWSUpgradeRequest != "websocket" {
		http.Error(w, "Not found", 404)
		return
	}

	isNetwork := isNetworkRequest.MatchString(r.URL.Path)
	isControl := isControlRequest.MatchString(r.URL.Path)

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
		http.Error(w, "Not found", 404)
		return
	}

	// Resolve websocket connection (also, split Local and Network types with the same name)
	sock := service.namedWebSockets[servicePath]
	if sock == nil {
		sock = NewNamedWebSocket(service, serviceName, isNetwork, service.Port)
		service.namedWebSockets[servicePath] = sock
	}

	peerId, _ := strconv.Atoi(peerIdStr)

	// Handle websocket connection
	if isControl {
		sock.serveControl(w, r, peerId)
	} else {
		sock.serveService(w, r, peerId)
	}
}

func (service *NamedWebSocket_Service) serveProxyWSCreator(w http.ResponseWriter, r *http.Request) {

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	if isValidWSUpgradeRequest := strings.ToLower(r.Header.Get("Upgrade")); isValidWSUpgradeRequest != "websocket" {
		http.Error(w, "Not found", 404)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")

	peerIdStr := pathParts[len(pathParts)-1]
	serviceHash := pathParts[len(pathParts)-2]

	// Resolve serviceHash to an active named websocket service name
	serviceBCryptHash, _ := base64.StdEncoding.DecodeString(serviceHash)
	serviceBCryptHashStr := string(serviceBCryptHash)

	for serviceName := range service.knownServiceNames {
		if bcrypt.Match(serviceName, serviceBCryptHashStr) {

			sock := service.namedWebSockets[fmt.Sprintf("/network/%s", serviceName)]
			if sock == nil {
				log.Fatal("Could not find matching NamedWebSocket_Service object for service")
			}

			peerId, _ := strconv.Atoi(peerIdStr)
			sock.serveProxy(w, r, peerId)
			break
		}
	}
}
