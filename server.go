package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"text/template"
)

var ValidServiceName = regexp.MustCompile("^[A-Za-z0-9_-]{1,255}$")

var namedWebSockets = map[string]*NamedWebSocket{}

var consoleTempl = template.Must(template.ParseFiles("console.html"))

func setupHTTP() {
	name, err := os.Hostname()
	if err != nil {
		fmt.Printf("Could not determine device hostname: %v\n", err)
		return
	}
	LocalHost = name

	// Serve the test console
	http.HandleFunc("/", serveConsoleTemplate)

	// Serve the web socket creation endpoints
	http.HandleFunc("/local/", serveWSCreator)
	http.HandleFunc("/broadcast/", serveWSCreator)

	portStr := strconv.Itoa(LocalPort)

	// Bind and serve on device's public interface
	go http.ListenAndServe(LocalHost+":"+portStr, nil)

	// Bind and serve also on device's loopback address
	http.ListenAndServe("localhost:"+portStr, nil)
}

func serveConsoleTemplate(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	portSuffix := ":" + strconv.Itoa(LocalPort)

	// Only allow access from localhost
	if r.Host != "localhost"+portSuffix && r.Host != "127.0.0.1"+portSuffix {
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

	consoleTempl.Execute(w, LocalPort)
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

	if isValidServiceName := ValidServiceName.FindString(serviceName); isValidServiceName == "" {
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
