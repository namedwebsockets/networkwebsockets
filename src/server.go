package main

import (
	"os"
	"fmt"
	"net/http"
	"text/template"
	"regexp"
	"path"
	"strconv"
)

var homeTempl = template.Must(template.ParseFiles("_www/testclient.html"))
var scriptTempl = template.Must(template.ParseFiles("_www/_namedwebsockets.js"))

var ValidServiceName = regexp.MustCompile("^[A-Za-z0-9_-]{1,255}$")

var namedWebSockets = map[string]*NamedWebSocket{}

func setupHTTP() {
	name, err := os.Hostname()
	if err != nil {
		fmt.Printf("Could not determine device hostname: %v\n", err)
		return
	}
	LocalHost = name

	http.HandleFunc("/", serveHTTP)
	http.HandleFunc("/_namedwebsockets.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		scriptTempl.Execute(w, LocalPort)
  })

	http.HandleFunc("/local/", serveWSCreator)
	http.HandleFunc("/broadcast/", serveWSCreator)

	portStr := strconv.Itoa(LocalPort)

	go http.ListenAndServe(LocalHost + ":" + portStr, nil)

	// Bind and serve also on loopback address
	http.ListenAndServe("localhost:" + portStr, nil)
}

func serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	homeTempl.Execute(w, r.Host)
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
