package main

import (
	"log"
	"os"
)

// Configured at runtime
var LocalHost string
var LocalPort int

func main() {
	name, err := os.Hostname()
	if err != nil {
		log.Printf("Could not determine device hostname: %v\n", err)
		return
	}

	LocalHost = name
	LocalPort = 9009

	// Start Bonjour discovery service
	NewDiscoveryServer()

	// Start HTTP/WebSocket endpoint server
	SetupHTTP()
}
