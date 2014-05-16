package main

// Configured at runtime
var LocalHost = "localhost"
var LocalPort = 9009

func main() {
	// Start Bonjour discovery service
	setupDNSSD()

	// Start HTTP/WebSocket endpoint server
	setupHTTP()
}
