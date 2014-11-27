Network Web Sockets
===

#### Dynamic Web Socket binding, peer management, advertising and discovery over local networks ####

Network Web Sockets allow web pages, native applications and devices to create [*encrypted*](https://github.com/namedwebsockets/networkwebsockets/wiki/Introduction-to-Secure-DNS-based-Service-Discovery-\(DNS-SSD\)) Web Socket *networks within networks* by discovering, binding and connecting peers that share the same *channel name* in the local network. With this technology web pages, native applications and devices can create ad-hoc inter-applicaton communication bridges between and among themselves to fulfill a variety of uses:

* For discovering matching peer services on the local device and/or the local network.
* To create full-duplex, encrypted communications channels between network devices, native applications and web applications.
* To create full-duplex, encrypted communication channels between web pages on different domains.
* To import and export data between network devices, native applications and web applications.
* To create initial local session signalling channels for establishing P2P sessions (for e.g. [WebRTC](#examples) bootstrapping).
* To establish low latency local network multiplayer signalling channels for games.
* To enable collaborative editing, sharing and other forms of communication between different web pages and applications on a local device or a local network.

A web page or application can create a new Network Web Socket by choosing an appropriate *channel name* (any alphanumeric name) via any of the available [Network Web Socket interfaces](#network-web-socket-interfaces). When other web pages or applications create their own Network Web Socket connection with the same *channel name* they will join the matching Network Web Socket broadcast network.

You can read more about the network-level security used by Network Web Sockets on [this wiki page](https://github.com/namedwebsockets/networkwebsockets/wiki/Introduction-to-Secure-DNS-based-Service-Discovery-\(DNS-SSD\)).

### Getting started

This repository contains the proof-of-concept _Network Web Sockets Proxy_, written in Go, currently required to experiment with and use Network Web Sockets. You can either [download a pre-built Network Web Sockets binary](https://github.com/namedwebsockets/networkwebsockets/releases) or [build the Network Web Sockets Proxy from source](https://github.com/namedwebsockets/networkwebsockets/wiki/Building-a-Named-Web-Sockets-Proxy-from-Source) to get up and running.

Once you have a Network Web Sockets Proxy up and running on your local machine then you are ready to [create and share your own Network Web Sockets](#network-web-socket-interfaces). A number of [Network Web Socket client examples](#examples) are also provided to help get you started.

##### [Go to downloads page to get a pre-built platform binary](https://github.com/namedwebsockets/networkwebsockets/releases)

### Network Web Socket Interfaces

#### Local HTTP Test Console

Once a Network Web Sockets Proxy is up and running, you can access a test console and play around with _Network Web Sockets_ at `http://localhost:9009/console`.

#### JavaScript Interfaces

The [Network Web Sockets JavaScript polyfill library](https://github.com/namedwebsockets/networkwebsockets/blob/master/lib/namedwebsockets.js) exposes a new JavaScript interface on the root global object for your convenience as follows:

* `NetworkWebSocket` for creating/binding named websockets to share on the local network.

You must include the polyfill file in your own projects to create these JavaScript interfaces. Assuming your [Network Web Sockets JavaScript polyfill](https://github.com/namedwebsockets/networkwebsockets/blob/master/lib/namedwebsockets.js) is located at `lib/namedwebsockets.js` then that can be done in an HTML document as follows:

    <script src="lib/namedwebsockets.js"></script>

You can create a new `NetworkWebSocket` connection object via the JavaScript polyfill as follows:

    var networkWS = new NetworkWebSocket("myChannelName");
    // Now do something with `networkWS` (it is a WebSocket object so use accordingly)

When any other client connects to a network web socket endpoint named `myChannelName` then your web socket connections will be automatically linked with each other in a broadcast confiuration.

You now have a full-duplex Web Socket channel to use for communication between each peer connected to the same channel name!

#### Web Socket Interfaces

Devices and services running on the local machine can register Network Web Sockets without needing to use the JavaScript API. Thus, we can connect up other applications and devices sitting in the local network such as TVs, Set-Top Boxes, Fridges, Home Automation Systems (assuming they run their own Network Web Sockets proxy client also).

To create a new Network Web Socket channel from anywhere on the local machine, simply create a new Web Socket connection to `ws://localhost:9009/network/<channelName>/<peer_id>`, where `channelName` is the name of the channel you want to create and use and `peer_id` is a new random integer to identify the new peer you are registering on this Network Web Socket. Network Web Socket peers will be advertised in the local network and all other Network Web Socket Proxies running in the network will be able to connect to your broadcasted network web socket interface.

### Examples

Some example services built with Network Web Sockets:

* [Chat example](https://github.com/namedwebsockets/networkwebsockets/tree/master/examples/chat)
* [PubSub example](https://github.com/namedwebsockets/networkwebsockets/tree/master/examples/pubsub)
* [WebRTC example](https://github.com/namedwebsockets/networkwebsockets/tree/master/examples/webrtc)

### Feedback

If you find any bugs or issues please report them on the [Network Web Sockets Issue Tracker](https://github.com/namedwebsockets/networkwebsockets/issues).

If you would like to contribute to this project please consider [forking this repo](https://github.com/namedwebsockets/networkwebsockets/fork), making your changes and then creating a new [Pull Request](https://github.com/namedwebsockets/networkwebsockets/pulls) back to the main code repository.

### License

The MIT License (MIT) Copyright (c) 2014 Rich Tibbett.

See the [LICENSE](https://github.com/namedwebsockets/networkwebsockets/tree/master/LICENSE.txt) file for more information.
