Named Web Sockets
===

#### Dynamic binding, peer management and local service discovery for Web Sockets ####

Named Web Sockets allow web pages, native applications and devices to create *encrypted* Web Socket *networks within networks* by discovering, binding and connecting peers that share the same *channel type* and *channel name* on local machines and local networks. With this technology web pages, native applications and devices can create ad-hoc inter-applicaton communication bridges between and among themselves to fulfill a variety of uses:

* For discovering matching peer services on the local device and/or the local network.
* To create full-duplex, encrypted communications channels between native applications and web applications.
* To create full-duplex, encrypted communication channels between web pages on different domains.
* To create initial local session signalling channels for establishing P2P sessions (for e.g. [WebRTC](#examples) bootstrapping).
* To establish low latency local network multiplayer signalling channels for games.
* To enable collaborative editing, sharing and other forms of communication between different web pages and applications on a local device or a local network.

A web page or application can create a new Named Web Socket by choosing an appropriate *channel type* (`local` or `network`) and *channel name* (any alphanumeric name) via any of the available [Named Web Socket interfaces](#named-web-socket-interfaces). When other web pages or applications create their own Named Web Socket connection with the same *type* and *name* they will join any matching Named Web Socket network.

![Named Web Sockets](https://raw.githubusercontent.com/namedwebsockets/namedwebsockets/images/networkwebsockets_diagram.png "Named Web Sockets")

### Getting started

This repository contains the proof-of-concept _Named Web Sockets Proxy_, written in Go, currently required to experiment with and use Named Web Sockets. You can either [download a pre-built Named Web Sockets binary](https://github.com/namedwebsockets/namedwebsockets/releases) or [build the Named Web Sockets Proxy from source](#build-from-source) to get up and running.

Once you have a Named Web Sockets Proxy up and running on your local machine then you are ready to [create and share your own Named Web Sockets](#named-websocket-interfaces). A number of [Named Web Socket examples](#examples) are also provided to help get you started.

##### [Go to Downloads Page](https://github.com/namedwebsockets/namedwebsockets/releases)

### Named Web Socket Interfaces

#### Local HTTP Test Console

Once a Named Web Sockets Proxy is up and running, you can access a test console and play around with both `local` and `network` Named Web Sockets at `http://localhost:9009/console`.

#### JavaScript Interfaces

The [Named Web Sockets JavaScript polyfill library](https://github.com/namedwebsockets/namedwebsockets/blob/master/lib/namedwebsockets.js) exposes two new JavaScript interfaces on the root global object (e.g. `window`) for your convenience as follows:

* `LocalWebSocket` for creating/binding named websockets to share on the local machine only.
* `NetworkWebSocket` for creating/binding named websockets to share both on the local machine and the local network.

You must include the polyfill file in your own projects to create these JavaScript interfaces. Assuming your [Named Web Sockets JavaScript polyfill](https://github.com/namedwebsockets/namedwebsockets/blob/master/lib/namedwebsockets.js) is located at `lib/namedwebsockets.js` then that can be done in an HTML document as follows:

    <script src="lib/namedwebsockets.js"></script>

You can create a new `LocalWebSocket` connection object via the JavaScript polyfill as follows:

    var localWS = new LocalWebSocket("myChannelName");
    // Now do something with `localWS` (it is a WebSocket object so use accordingly)

You can create a new `NetworkWebSocket` connection object via the JavaScript polyfill as follows:

    var networkWS = new NetworkWebSocket("myChannelName");
    // Now do something with `networkWS` (it is a WebSocket object so use accordingly)

When any other client connects to a `local` or `network` websocket endpoint named `myChannelName` then your websocket connections will be automatically linked to one another. Note that `local` and `network` based websocket connections are entirely seperate entities even if they happen to share the same channel name.

You now have a full-duplex Web Socket channel to use for communication between each peer connected to the same channel name with the same channel type!

#### Web Socket Interfaces

Devices and services running on the local machine can register and use Named Web Sockets without needing to use the JavaScript API. Thus, we can connect up other applications and devices sitting in the local network such as TVs, Set-Top Boxes, Fridges, Home Automation Systems (assuming they run their own Named Web Sockets proxy client also).

To create a new `local` Named Web Socket channel from anywhere on the local machine, simply open a new Web Socket to `ws://localhost:9009/local/<channelName>/<peer_id>`, where `channelName` is the name of the channel you want to create and use and `peer_id` is a new random integer to identify the new peer you are registering on this Named Web Socket.

To create a new `network` Named Web Socket channel from anywhere on the local machine, simply open a new Web Socket to `ws://localhost:9009/network/<channelName>/<peer_id>`, where `channelName` is the name of the channel you want to create and use and `peer_id` is a new random integer to identify the new peer you are registering on this Named Web Socket. In addition to `local` Named Web Socket channels, this type of Named Web Socket will be advertised in your local network and other Named Web Socket Proxies running in the network will connect to your broadcasted web socket interface.

### Examples

Some example services built with Named Web Sockets:

* [Chat example](https://github.com/namedwebsockets/namedwebsockets/tree/master/examples/chat)
* [PubSub example](https://github.com/namedwebsockets/namedwebsockets/tree/master/examples/pubsub)
* [WebRTC example](https://github.com/namedwebsockets/namedwebsockets/tree/master/examples/webrtc)

### Feedback

If you find any bugs or issues please report them on the [Named Web Sockets Issue Tracker](https://github.com/namedwebsockets/namedwebsockets/issues).

If you would like to contribute to this project please consider [forking this repo](https://github.com/namedwebsockets/namedwebsockets/fork), making your changes and then creating a new [Pull Request](https://github.com/namedwebsockets/namedwebsockets/pulls) back to the main code repository.

### License

The MIT License (MIT) Copyright (c) 2014 Rich Tibbett.

See the [LICENSE](https://github.com/namedwebsockets/namedwebsockets/tree/master/LICENSE.txt) file for more information.
