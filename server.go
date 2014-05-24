package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"path"
	"regexp"
)

var ValidServiceName = regexp.MustCompile("^[A-Za-z0-9\\._-]{1,255}$")

var namedWebSockets = map[string]*NamedWebSocket{}

func SetupHTTP(listener net.Listener) {
	// Serve the test console
	http.HandleFunc("/", serveConsoleTemplate)

	// Serve the web socket creation endpoints
	http.HandleFunc("/local/", serveWSCreator)
	http.HandleFunc("/broadcast/", serveWSCreator)

	if err := http.Serve(listener, nil); err != nil {
		log.Fatal("Could not serve proxy. ", err)
	}
}

func serveConsoleTemplate(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Only allow access from localhost
	if r.Host != fmt.Sprintf("localhost:%d", LocalPort) && r.Host != fmt.Sprintf("127.0.0.1:%d", LocalPort) {
		http.Error(w, "Permission denied. Named WebSockets Test Console is only accessible from localhost", 403)
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
	}

	w.Write(consoleHTML)
}

func serveWSCreator(w http.ResponseWriter, r *http.Request) {
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
		sock = NewNamedWebSocket(serviceName, isBroadcast)
		namedWebSockets[r.URL.Path] = sock
	}

	// Handle websocket connection
	sock.serve(w, r)
}
