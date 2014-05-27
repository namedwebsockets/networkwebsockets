package main

import (
	"log"
	"os"

	"github.com/richtr/namedwebsockets"
)

func main() {
	name, err := os.Hostname()
	if err != nil {
		log.Printf("Could not determine device hostname: %v\n", err)
		return
	}

	localHost := name
	localPort := 9009

	service := &namedwebsockets.NamedWebSocket_Service{
		Port: localPort,
		Host: localHost,
	}

	// Start mDNS/DNS-SD discovery service
	go service.StartNewDiscoveryServer()

	// Start HTTP/WebSocket endpoint server (blocking call)
	service.StartHTTPServer()
}
