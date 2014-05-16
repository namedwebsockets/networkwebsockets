«Named» WebSockets
===

#### Dynamic WebSockets binding, management and service discovery ####

Named WebSockets is a simple and powerful way to dynamically create, bind and share WebSocket connections between multiple peers either on the local machine or over a local network.

This repository contains the proof-of-concept _Named WebSockets Proxy_, written in Go, that you can download and run on your own machine following the instructions provided below.

Once you have a Named WebSockets Proxy up and running, then any web page or application on the local device can bind themselves to a shared WebSocket connections simply by specifiying the same *service name* from two seperate invocations of the interfaces exposed. Please read on for further information on the interfaces available.

### Run a Named Websockets Proxy

To create and shared Named WebSockets currently requires a Named WebSockets Proxy to be running on each local machine that wants to participate in the network.

1. Ensure you have a suitable [DNS Service Discovery client](#set-up-a-dns-service-discovery-client) installed and running for your machine's architecture.

1. [Install go](http://golang.org/doc/install).

2. Download this repository using `go get`:

        go get github.com/richtr/namedwebsockets

3. Locate and change directory to the download repository:

        cd `go list -f '{{.Dir}}' github.com/richtr/namedwebsockets/src`

4. Run a NamedWebSockets Proxy:

        go run *.go

Note: Named WebSockets works on port `9009` (and port `9009` only!). Only change the port in the source code if you know what you are doing.

#### Set up a DNS Service Discovery Client

This project requires a DNS Service Discovery client to be running on your device.

Bonjour - a DNS Service Discovery client - comes pre-bundled with OS X. If you are on OS X you can skip the rest of this section.

If you are on Windows you can obtain Bonjour via [Bonjour Print Services for Windows](http://support.apple.com/kb/dl999) or the [Bonjour SDK for Windows](https://developer.apple.com/bonjour/). Bonjour also comes bundled with iTunes if you have that installed on Windows also.

For other POSIX platforms Apple offer [mDNSResponder](http://opensource.apple.com/tarballs/mDNSResponder/) as open-source, however the [Avahi](http://www.avahi.org/) project is the de facto choice on most Linux and BSD systems.

### API Interfaces

#### Local HTTP Test Console

Once a Named WebSockets Proxy is up and running, you can access its test console and play around with both `local` and `broadcast` Named WebSockets at `http:/localhost:9009/console`.

#### JavaScript Interfaces

Named WebSockets expose two new JavaScript interfaces on the root global object (e.g. `window`) as follows:

* `LocalWebSocket` for creating/binding named websockets to share on the local machine only.
* `BroadcastWebSocket` for creating/binding named websockets to share both on the local machine and the local network.

You can create a new `LocalWebSocket` connection as follows in JavaScript:

    var localWS = new LocalWebSocket("myServiceName");

		// Now do something with `localWS` (it is a WebSocket object so use accordingly)

You can create a new `BroadcastWebSocket` connection as follows in JavaScript:

    var broadcastWS = new BroadcastWebSocket("myServiceName");

		// Now do something with `broadcastWS` (it is a WebSocket object so use accordingly)

When any other client connects to a websocket endpoint named `myServiceName` then your websocket connections will be automatically linked to one another.

You now have a full-duplex WebSocket channel to use for communication between each service connected to the same service name with the same service type!

#### WebSocket Interfaces

Devices and services running on the local machine can register and use Named WebSockets without needing to use the JavaScript API. Thus, we can connect up other applications and devices sitting in the local network such as TVs, Set-Top Boxes, Fridges, Home Automation Systems (assuming they run their own Named WebSockets proxy client also).

To create a new `local` Named WebSocket service from anywhere on the local machine, simply open a new WebSocket to `http://localhost:9009/local/<serviceName>`, where `serviceName` is the name of the service you want to create and use.

To create a new `broadcast` Named WebSocket service from anywhere on the local machine, simply open a new WebSocket to `http://localhost:9009/broadcast/<serviceName>`, where `serviceName` is the name of the service you want to create and use. In addition to `local` Named WebSocket services, this type of Named WebSocket will be advertised in your local network and other Named WebSocket Proxies running in the network will connect to your broadcasted web socket interface.

### Feedback

If you find any bugs or issues please report them on the [NamedWebSockets Issue Tracker](https://github.com/richtr/namedwebsockets/issues).

If you would like to contribute to this project please consider [forking this repo](https://github.com/richtr/namedwebsockets/fork), making your changes and then creating a new [Pull Request](https://github.com/richtr/namedwebsockets/pulls) back to the main code repository.

### License

MIT.
