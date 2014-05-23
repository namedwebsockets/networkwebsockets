package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

// Configured at runtime
var LocalHost string
var LocalPort int
var LocalAddr *net.TCPAddr

func main() {
	listener := SetupNetworking()

	// Start Bonjour discovery service
	go StartDiscoveryServer()

	// Start HTTP/WebSocket endpoint server
	SetupHTTP(listener)
}

func SetupNetworking() net.Listener {

	name, err := os.Hostname()
	if err != nil {
		log.Printf("Could not determine device hostname: %v\n", err)
		return nil
	}

	LocalHost = name
	LocalPort = 9009

	// Listen on all ports (public + loopback addresses)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", LocalPort))
	if err != nil {
		log.Fatal("Could not bind address. ", err)
	}

	LocalAddr = (listener.Addr()).(*net.TCPAddr)

	return listener
}