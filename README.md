«Named» WebSockets
===

#### Dynamic WebSocket binding, management and service discovery ####

Named WebSockets is a simple and powerful way to dynamically create, bind and connect WebSocket peer connections together that share the same *service name* on a local machine or over the local network. This is useful for a variety of peer-to-peer applications such as file sharing or multi-player gaming.

Any web page or application on the local device can bind themselves to shared WebSocket connections by simply requesting the same *service type* (`local` or `broadcast`) and *service name* (any alphanumeric name you'd like).

This repository contains the proof-of-concept _Named WebSockets Proxy_, written in Go, required to manage Named WebSockets. You can [download and run this proxy](#getting-started) on your own machine to experiment with Named WebSockets following the instructions provided here.

Once you have a Named WebSockets Proxy up and running on the local machine then you are ready to [create and share your own local/broadcast Named WebSockets](#named-websocket-interfaces)!

### Getting started

To create and share Named WebSockets currently requires a Named WebSockets Proxy to be running on each local machine that wants to participate in the network. You can download and run the latest precompiled Named WebSockets Proxy for your current platform from our [releases page](https://github.com/richtr/namedwebsockets/releases).

[Go to the latest downloads page](https://github.com/richtr/namedwebsockets/releases)

#### Building from source

Optionally you can build this project from the source files contained in this repository with the following instructions:

1. [Install go](http://golang.org/doc/install).

2. Download this repository and its dependencies using `go get`:

        go get github.com/richtr/namedwebsockets

3. Locate and change directory to the download repository:

        cd `go list -f '{{.Dir}}' github.com/richtr/namedwebsockets`

4. Run your Named WebSockets Proxy:

        go run *.go

At this point your Named WebSockets Proxy should be up and ready for usage at `localhost:9009`*!

You can now start using your Named WebSockets Proxy via any of the [Named WebSocket Proxy Interfaces](#named-websocket-interfaces) described below.

\* Named WebSocket Proxies should run on port `9009` and port `9009` only. Changing the port number is likely to break things.

### Named WebSocket Interfaces

#### Local HTTP Test Console

Once a Named WebSockets Proxy is up and running, you can access its test console and play around with both `local` and `broadcast` Named WebSockets at `http://localhost:9009/console`.

#### JavaScript Interfaces

The [Named WebSockets JavaScript polyfill library](https://github.com/richtr/namedwebsockets/blob/master/lib/namedwebsockets.js) exposes two new JavaScript interfaces on the root global object (e.g. `window`) for your convenience as follows:

* `LocalWebSocket` for creating/binding named websockets to share on the local machine only.
* `BroadcastWebSocket` for creating/binding named websockets to share both on the local machine and the local network.

You must include the polyfill file in your own projects to create these JavaScript interfaces. Assuming your [Named WebSockets JavaScript polyfill](https://github.com/richtr/namedwebsockets/blob/master/lib/namedwebsockets.js) is located at `lib/namedwebsockets.js` then that can be done in an HTML document as follows:

    <script src="lib/namedwebsockets.js"></script>

You can create a new `LocalWebSocket` connection object via the JavaScript polyfill as follows:

    var localWS = new LocalWebSocket("myServiceName");
    // Now do something with `localWS` (it is a WebSocket object so use accordingly)

You can create a new `BroadcastWebSocket` connection object via the JavaScript polyfill as follows:

    var broadcastWS = new BroadcastWebSocket("myServiceName");
    // Now do something with `broadcastWS` (it is a WebSocket object so use accordingly)

When any other client connects to a `local` or `broadcast` websocket endpoint named `myServiceName` then your websocket connections will be automatically linked to one another. Note that `local` and `broadcast` based websocket connections are entirely seperate entities even if they happen to share the same service name.

You now have a full-duplex WebSocket channel to use for communication between each service connected to the same service name with the same service type!

#### WebSocket Interfaces

Devices and services running on the local machine can register and use Named WebSockets without needing to use the JavaScript API. Thus, we can connect up other applications and devices sitting in the local network such as TVs, Set-Top Boxes, Fridges, Home Automation Systems (assuming they run their own Named WebSockets proxy client also).

To create a new `local` Named WebSocket service from anywhere on the local machine, simply open a new WebSocket to `ws://localhost:9009/local/<serviceName>`, where `serviceName` is the name of the service you want to create and use.

To create a new `broadcast` Named WebSocket service from anywhere on the local machine, simply open a new WebSocket to `ws://localhost:9009/broadcast/<serviceName>`, where `serviceName` is the name of the service you want to create and use. In addition to `local` Named WebSocket services, this type of Named WebSocket will be advertised in your local network and other Named WebSocket Proxies running in the network will connect to your broadcasted web socket interface.

### Examples

* [Chat example](https://github.com/richtr/namedwebsockets/tree/master/examples/chat)
* [PubSub example](https://github.com/richtr/namedwebsockets/tree/master/examples/pubsub)
* [WebRTC example](https://github.com/richtr/namedwebsockets/tree/master/examples/webrtc)

### Feedback

If you find any bugs or issues please report them on the [Named WebSockets Issue Tracker](https://github.com/richtr/namedwebsockets/issues).

If you would like to contribute to this project please consider [forking this repo](https://github.com/richtr/namedwebsockets/fork), making your changes and then creating a new [Pull Request](https://github.com/richtr/namedwebsockets/pulls) back to the main code repository.

### License

MIT.
