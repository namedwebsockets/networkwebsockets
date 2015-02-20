package networkwebsockets

import (
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
	"time"

	tls "github.com/richtr/go-tls-srp"
)

var (
	// Proxy path matchers
	serviceNameRegexStr  = "[A-Za-z0-9\\+=\\*\\._-]{1,255}"
	isValidCreateRequest = regexp.MustCompile(fmt.Sprintf("^/%s$", serviceNameRegexStr))
	isValidProxyRequest  = regexp.MustCompile(fmt.Sprintf("^/%s$", serviceNameRegexStr))

	// TLS-SRP configuration components
	Salt       = []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	serviceTab = CredentialsStore(map[string]string{})
)

// Generate a new random identifier
func GenerateId() string {
	rand.Seed(time.Now().UTC().UnixNano())
	return fmt.Sprintf("%d", rand.Int())
}

type Service struct {
	Host string
	Port int

	ProxyPort int

	// All Network Web Socket channels that this service manages
	Channels map[string]*Channel

	discoveryBrowser *DiscoveryBrowser

	done chan int // blocks until .Stop() is called on this service

	localListener net.Listener
	netListener   net.Listener
}

func NewService(host string, port int) *Service {
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

	service := &Service{
		Host: host,
		Port: port,

		ProxyPort: 0,

		Channels: make(map[string]*Channel),

		discoveryBrowser: NewDiscoveryBrowser(),

		done: make(chan int),
	}

	return service
}

func (service *Service) Start() <-chan int {
	// Start HTTP/Network Web Socket creation server
	service.StartHTTPServer()

	// Start TLS-SRP Network Web Socket (wss) proxy server
	service.StartProxyServer()

	// Start mDNS/DNS-SD Network Web Socket discovery service
	service.StartDiscoveryBrowser(10)

	return service.StopNotify()
}

func (service *Service) StartHTTPServer() {
	// Create a new custom http server multiplexer
	serveMux := http.NewServeMux()

	// Serve network web socket creation endpoints for localhost clients
	serveMux.HandleFunc("/", service.serveWSCreatorRequest)

	// Listen and on loopback address + port
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", service.Port))
	if err != nil {
		log.Fatal("Could not serve web server. ", err)
	}

	service.localListener = listener

	log.Printf("Serving Network Web Socket Creator Proxy at address [ ws://localhost:%d/ ]", service.Port)

	go http.Serve(listener, serveMux)
}

func (service *Service) StartProxyServer() {
	// Create a new custom http server multiplexer
	serveMux := http.NewServeMux()

	// Serve secure network web socket proxy endpoints for network clients
	serveMux.HandleFunc("/", service.serveWSProxyRequest)

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
		log.Fatal("Could not serve proxy server. ", err)
	}

	service.netListener = tlsSrpListener

	// Obtain and store the port of the proxy endpoint
	_, port, err := net.SplitHostPort(tlsSrpListener.Addr().String())
	if err != nil {
		log.Fatal("Could not determine bound port of proxy server. ", err)
	}

	service.ProxyPort, _ = strconv.Atoi(port)

	log.Printf("Serving Network Web Socket Network Proxy at address [ wss://%s:%d/ ]", service.Host, service.ProxyPort)

	go http.Serve(tlsSrpListener, serveMux)
}

func (service *Service) StartDiscoveryBrowser(timeoutSeconds int) {
	log.Printf("Listening for Network Web Socket services on the local network...")

	go func() {
		defer service.discoveryBrowser.Shutdown()

		for !service.discoveryBrowser.closed {
			service.discoveryBrowser.Browse(service, timeoutSeconds)
		}
	}()
}

func (service *Service) serveWSCreatorRequest(w http.ResponseWriter, r *http.Request) {
	// Only allow access from localhost to all services
	if isRequestFromLocalHost := service.checkRequestIsFromLocalHost(r.Host); !isRequestFromLocalHost {
		http.Error(w, fmt.Sprintln("This interface is only accessible from the local machine"), 403)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	serviceName := strings.TrimPrefix(r.URL.Path, "/")

	// Serve console page for use in web browser if no service name has been requested
	if serviceName == "" {

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

		return
	}

	if isValidRequest := isValidCreateRequest.MatchString(r.URL.Path); !isValidRequest {
		http.Error(w, "Not Found", 404)
		return
	}

	if isValidWSUpgradeRequest := strings.ToLower(r.Header.Get("Upgrade")); isValidWSUpgradeRequest != "websocket" {
		http.Error(w, "Bad Request", 400)
		return
	}

	// Resolve to network web socket channel
	channel := service.GetChannelByName(serviceName)
	if channel == nil {
		channel = NewChannel(service, serviceName)
	}

	// Serve network web socket channel peer
	channel.ServePeer(w, r)
}

func (service *Service) serveWSProxyRequest(w http.ResponseWriter, r *http.Request) {

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

	// Resolve servicePath to an active named websocket service
	for _, channel := range service.Channels {
		if channel.proxyPath == r.URL.Path {
			channel.ServeProxy(w, r)

			return
		}
	}

	http.Error(w, "Not Found", 404)
	return
}

// Check whether we know the given service name
func (service *Service) GetChannelByName(serviceName string) *Channel {
	for _, channel := range service.Channels {
		if channel.serviceName == serviceName {
			return channel
		}
	}
	return nil
}

// Check whether a DNS-SD derived Network Web Socket hash is owned by the current proxy instance
func (service *Service) isOwnProxyService(serviceRecord *DNSRecord) bool {
	for _, channel := range service.Channels {
		if channel.serviceHash == serviceRecord.Hash_Base64 {
			return true
		}
	}
	return false
}

// Check whether a DNS-SD derived Network Web Socket hash is currently connected as a service
func (service *Service) isActiveProxyService(serviceRecord *DNSRecord) bool {
	for _, channel := range service.Channels {
		for _, proxy := range channel.proxies {
			if proxy.Hash_Base64 == serviceRecord.Hash_Base64 {
				return true
			}
		}
	}
	return false
}

// Stop stops the server gracefully, and shuts down the running goroutine.
// Stop should be called after a Start(s), otherwise it will block forever.
func (service *Service) Stop() {
	if service.discoveryBrowser != nil {
		service.discoveryBrowser.closed = true
	}

	if service.localListener != nil {
		service.localListener.Close()
	}

	if service.netListener != nil {
		service.netListener.Close()
	}

	service.done <- 1
}

// StopNotify returns a channel that receives a empty integer
// when the server is stopped.
func (service *Service) StopNotify() <-chan int { return service.done }

//
// HELPER FUNCTIONS
//

func (service *Service) checkRequestIsFromLocalHost(host string) bool {
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
