package main

func main() {
	// Start Bonjour discovery service
	NewDiscoveryServer()

	// Start HTTP/WebSocket endpoint server
	SetupHTTP()
}
