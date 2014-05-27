package namedwebsockets

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"path"
	"regexp"
	"text/template"
)

var ValidServiceName = regexp.MustCompile("^[A-Za-z0-9\\._-]{1,255}$")

var namedWebSockets = map[string]*NamedWebSocket{}

type NamedWebSocket_Service struct {
	Host string
	Port int

	listener *net.Listener
}

func (service *NamedWebSocket_Service) StartHTTPServer() {

	// Listen on all ports (public + loopback addresses)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", service.Port))
	if err != nil {
		log.Fatal("Could not bind address. ", err)
	}

	service.listener = &listener

	// Serve the test console
	http.HandleFunc("/", service.serveConsoleTemplate)

	// Serve the web socket creation endpoints
	http.HandleFunc("/local/", service.serveWSCreator)
	http.HandleFunc("/broadcast/", service.serveWSCreator)

	log.Printf("Serving Named WebSockets Proxy at http://%s:%d/", service.Host, service.Port)
	log.Printf("(test console available @ http://localhost:%d/console)", service.Port)

	if err := http.Serve(*service.listener, nil); err != nil {
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

	// Only allow access from localhost
	if r.Host != fmt.Sprintf("localhost:%d", service.Port) && r.Host != fmt.Sprintf("127.0.0.1:%d", service.Port) {
		http.Error(w, fmt.Sprintf("<h2>Permission denied.</h2>\n\n<p>Named WebSockets Test Console is only accessible from <a href=\"http://localhost:%d/console\">http://localhost:%d/console</a></p>", service.Port, service.Port), 403)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	if r.URL.Path == "/" {
		fmt.Fprint(w, "<h2>Named WebSockets Proxy is running!</h2>")
		return
	}

	if r.URL.Path != "/console" {
		http.Error(w, "Not found", 404)
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

	isBroadcast, err := path.Match("/broadcast/*", r.URL.Path)
	if err != nil {
		http.Error(w, "Internal Server Error", 501)
		return
	}

	// Create a new websocket service at URL
	serviceName := path.Base(r.URL.Path)

	if isValid := ValidServiceName.MatchString(serviceName); !isValid {
		http.Error(w, "Not found", 404)
		return
	}

	// Resolve websocket connection (also, split Local and Broadcast types with the same name)
	sock := namedWebSockets[r.URL.Path]
	if sock == nil {
		sock = NewNamedWebSocket(serviceName, isBroadcast, service.Port)
		namedWebSockets[r.URL.Path] = sock
	}

	// Handle websocket connection
	sock.serve(w, r)
}
