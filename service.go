package namedwebsockets

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

var (
	// Master list of all Named WebSocket services (local or broadcast) that we are aware of
	namedWebSockets = map[string]*NamedWebSocket{}

	// Regular expression matchers

	serviceNameRegexStr = "[A-Za-z0-9\\._-]{1,255}"

	peerIdRegexStr = "[0-9]{4,}"

	isBroadcastRequest = regexp.MustCompile(fmt.Sprintf("^(.*/broadcast/%s/%s)$", serviceNameRegexStr, peerIdRegexStr))

	isControlRequest = regexp.MustCompile(fmt.Sprintf("(/control/(broadcast|local)/%s/%s)", serviceNameRegexStr, peerIdRegexStr))

	isValidServiceName = regexp.MustCompile(fmt.Sprintf("^%s$", serviceNameRegexStr))
)

type NamedWebSocket_Service struct {
	Host string
	Port int
}

func (service *NamedWebSocket_Service) StartHTTPServer() {
	// Create a new custom http server multiplexer
	serveMux := http.NewServeMux()

	// Serve the test console
	serveMux.HandleFunc("/", service.serveConsoleTemplate)

	// Serve the web socket creation endpoints
	serveMux.HandleFunc("/local/", service.serveWSCreator)
	serveMux.HandleFunc("/broadcast/", service.serveWSCreator)
	serveMux.HandleFunc("/control/", service.serveWSCreator)

	log.Printf("Serving Named WebSockets Proxy at http://%s:%d/", service.Host, service.Port)
	log.Printf("(test console available @ http://localhost:%d/console)", service.Port)

	// Listen and serve on all ports (public + loopback addresses)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", service.Port), serveMux); err != nil {
		log.Fatal("Could not serve proxy. ", err)
	}
}

func (service *NamedWebSocket_Service) StartNewDiscoveryServer() {
	discoveryServer := &DiscoveryServer{
		Host: service.Host,
		Port: service.Port,
	}

	defer discoveryServer.Shutdown()

	log.Print("Listening for broadcast websocket advertisements in local network...")

	for !discoveryServer.closed {
		discoveryServer.Browse()
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

func (service *NamedWebSocket_Service) serveWSCreator(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	isBroadcast := isBroadcastRequest.MatchString(r.URL.Path)
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

	// Resolve websocket connection (also, split Local and Broadcast types with the same name)
	sock := namedWebSockets[servicePath]
	if sock == nil {
		sock = NewNamedWebSocket(serviceName, isBroadcast, service.Port)
		namedWebSockets[servicePath] = sock
	}

	peerId, _ := strconv.Atoi(peerIdStr)

	// Handle websocket connection
	if isControl {
		sock.serveControl(w, r, peerId)
	} else {
		sock.serveService(w, r, peerId)
	}
}
