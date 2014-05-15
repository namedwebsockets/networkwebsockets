Named WebSockets
================

#### Dynamic local and network service binding using DNS Service Discovery and Named WebSockets ####

*More details coming soon*

### Set up a DNS Service Discovery Client

This project requires a DNS Service Discovery client to be running on your device.

Bonjour - a DNS Service Discovery client - comes pre-bundled with OS X. If you are on OS X you can skip the rest of this section.

If you are on Windows you can obtain Bonjour via [Bonjour Print Services for Windows](http://support.apple.com/kb/dl999) or the [Bonjour SDK for Windows](https://developer.apple.com/bonjour/). Bonjour also comes bundled with iTunes if you have that installed on Windows also.

For other POSIX platforms Apple offer [mDNSResponder](http://opensource.apple.com/tarballs/mDNSResponder/) as open-source, however the [Avahi](http://www.avahi.org/) project is the de facto choice on most Linux and BSD systems.

### Running the NamedWebsockets Proxy

This project requires a DNS-SD client to be set up and installed on your device.

1. [Install go](http://golang.org/doc/install).

2. Download this repository using `go get`:

        go get github.com/richtr/namedwebsockets

3. Locate and change directory to the download repository:

        cd `go list -f '{{.Dir}}' github.com/richtr/namedwebsockets/src`

4. Run NamedWebSockets

        go run *.go

Note: Named WebSockets works on port `9009` (and port `9009` only!). Only change the port in the source code if you know what you are doing.

### Usage

`BroadcastWebSocket` Usage:

    // Broadcast and connect with other peers using the same service name in the current network
    var ws = new BroadcastWebSocket("myServiceName")

`LocalWebSocket` Usage:

    // Connect with other peers using the same service name on the local device
    var ws = new LocalWebSocket("myServiceName")

Then use the returned `ws` object like a normal `WebSocket` connection.

### Feedback

If you find any bugs or issues please report them on the [NamedWebSockets Issue Tracker](https://github.com/richtr/namedwebsockets/issues).

If you would like to contribute to this project please consider [forking this repo](https://github.com/richtr/namedwebsockets/fork), making your changes and then creating a new [Pull Request](https://github.com/richtr/namedwebsockets/pulls) back to the main code repository.

### License

MIT.
