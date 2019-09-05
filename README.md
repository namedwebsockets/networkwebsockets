Network Web Sockets
===

#### Local Network broadcast channels with secure service discovery and encrypted proxy communication

Network Web Sockets allow web pages, native applications and devices to create [*encrypted*](https://github.com/namedwebsockets/networkwebsockets/wiki/Introduction-to-Secure-DNS-based-Service-Discovery-\(DNS-SSD\)) Web Socket networks by discovering, binding and connecting peers that share the same *channel name* in the local network.

_Channel names_ can either be:

* Common, memorable strings such as e.g. "webchat" to allow any service to connect to a _public_ channel, or:
* Pseudo-secure strings such as randomly-generated hashes e.g. "cJWHi8q7SvNWAiSerpfxW3inYjXiKNqR" that are known and shared out-of-band between two or more actors to connect to a _private_ channel.

Network Web Sockets prevents channel name discovery and channel message injection by snooping on the traffic in the local network. This is achieved using the mechanisms described in our [DNS-based Secure Service Discovery (DNS-SSD) draft](https://github.com/namedwebsockets/networkwebsockets/wiki/Introduction-to-Secure-DNS-based-Service-Discovery-(DNS-SSD)) and encrypting all communications between participating nodes within different channels.

Any channel within the Network Web Sockets ecosystem is considered as secure as its out-of-band key sharing mechanisms (whether that is on a public forum online or via other more secure sharing mechanisms). If a channel key is available to a user, then that user will be able to join that channel (although nothing stops channels performing _additional_ authentication over the channel whenever a new peer connects). Similarly, if a user is unaware of a channel name, they will have no way of discovering that channel name via any traffic flowing in the local network.

Web pages, native applications and devices can create ad-hoc inter-applicaton communication bridges between and among themselves for a variety of purposes:

* For discovering matching peer services on the local device and/or the local network.
* To create full-duplex, encrypted communications channels between network devices, native applications and web applications.
* To create full-duplex, encrypted communication channels between web pages on different domains.
* To import and export data between network devices, native applications and web applications.
* To create initial local session signalling channels for establishing P2P sessions (for e.g. [WebRTC](#examples) signalling channel bootstrapping).
* To establish low latency local network multiplayer signalling channels for games.
* To enable collaborative editing, sharing and other forms of communication between different web pages and applications on a local device or a local network.

A web page or application can create a new Network Web Socket by choosing a *channel name* (any alphanumeric name) via any of the available [Network Web Socket interfaces](#network-web-socket-interfaces). When other peers join the same *channel name* then they will join all other peers in the same Network Web Socket broadcast network.

You can read more about the secure discovery process and proxy-to-proxy encryption used by Network Web Sockets on [this wiki page](https://github.com/namedwebsockets/networkwebsockets/wiki/Introduction-to-Secure-DNS-based-Service-Discovery-\(DNS-SSD\)).

### Getting started

This repository contains an implementation of a _Network Web Socket Proxy_, written in Go, required to use Network Web Sockets.

You can either [download a pre-built Network Web Sockets binary](https://github.com/namedwebsockets/networkwebsockets/releases) or [build a Network Web Socket Proxy from source](https://github.com/namedwebsockets/networkwebsockets/wiki/Building-a-Network-Web-Socket-Proxy-from-Source) to get up and running.

Once you have a Network Web Socket Proxy up and running on your local machine then you are ready to [create and share your own Network Web Sockets](#network-web-socket-interfaces). A number of [Network Web Socket client examples](#examples) are also provided to help get you started.

##### [Download a pre-built platform binary](https://github.com/namedwebsockets/networkwebsockets/releases)

##### [How to build from source](https://github.com/namedwebsockets/networkwebsockets/wiki/Building-a-Network-Web-Socket-Proxy-from-Source)

### Network Web Socket Interfaces

#### Local HTTP Test Console

Once a Network Web Socket Proxy is up and running, you can access a test console in your web browser and play around with _Network Web Sockets_ at `http://localhost:9009`.

#### JavaScript Interfaces

The [Network Web Sockets JavaScript polyfill library](https://github.com/namedwebsockets/networkwebsockets/blob/master/lib/namedwebsockets.js) exposes a new JavaScript interface on the root global object for your convenience as follows:

* `NetworkWebSocket` for creating/binding named websockets to share on the local network.

You must include the polyfill file in your own projects to create these JavaScript interfaces. Assuming we have added the [Network Web Sockets JavaScript polyfill](https://github.com/namedwebsockets/networkwebsockets/blob/master/lib/namedwebsockets.js) to our page then we can create a new `NetworkWebSocket` connection object via the JavaScript polyfill as follows:

```javascript
// Create a new Network Web Socket peer in the network
var ws = new NetworkWebSocket("myChannelName");
```

We then wait for our peer to be successfully added to the network:

```javascript
ws.onopen = function() {
  console.log('Our channel peer is now connected to the `myChannelName` web socket network');
};
```

We can listen for incoming _broadcast_ messages from channel peers in the network as follows:

```javascript
ws.onmessage = function(event) {
  console.log("Broadcast message received: " + event.data);
};
```

We can send _broadcast_ messages to all the other _currently known_ channel peers in the network as follows:

```javascript
ws.send('This is a broadcast message to *all* other channel peers');
```

When we create a Network Web Socket connection object then the Network Web Socket Proxy will start to discover and connect to all other `myChannelName` channel peers that are being advertised in the local network.

Each time a new channel peer is discovered in the network a Web Socket proxy connection to that peer is established and a new `connect` event is queued and fired against our root Network Web Socket object:

```javascript
ws.onconnect = function(event) {
  console.log('Another peer has been discovered and connected to our `myChannelName` web socket network!');
};
```

In this `connect` event, we are provided with a direct, _peer-to-peer_ Web Socket connection object that can be used to communicate directly with this newly discovered and connected peer.

We can send a _direct message_ to a channel peer and listen for _direct messages_ from this channel peer as follows:

```javascript
// Wait for a new channel peer to connect to our `myChannelName` web socket network
ws.onconnect = function(event) {

  // Retrieve the new direct P2P Web Socket connection object with the newly connected channel peer
  var peerWS = evt.detail.target;

  // Wait for this new direct p2p channel connection to be opened
  peerWS.onopen = function() {

    // Listen for direct messages from this peer
    peerWS.onmessage = function(event) {
      console.log("Direct message received from [" + peerWS.id + "]: " + event.data);
    }

    // Send a direct message to this peer
    peerWS.send('This is a direct message to the new channel peer *only*'):

  };
};
```

With both broadcast and direct messaging capabilities it is possible to build advanced services on top of Network Web Sockets. We are excited to see what you come up with!

#### Web Socket Interfaces

Devices and services running on the local machine can register Network Web Sockets without needing to use the JavaScript API. Thus, we can connect up other applications and devices sitting in the local network such as TVs, Set-Top Boxes, Fridges, Home Automation Systems (assuming they run their own Network Web Socket Proxy client also).

To create a new Network Web Socket connection to a channel from anywhere _on the local machine_ (i.e. to become a 'channel peer') you can establish a Web Socket connection to a running Network Web Socket Proxy at the following URL:

```
ws://localhost:<port>/<channelName>
```

where:

* `port` is the port on which your Network Web Socket Proxy is running (by default, `9009`),
* `channelName` is the name of the channel you want to create, and;

Messages sent and received on this Web Socket connection have a well-defined data format.

This Web Socket connection will notify you when channel peers connect and disconnect from `<channelName>` and when broadcast or direct messages are sent to you from other connected channel peers. This Web Socket connection can also be used to send broadcast or direct messages toward all other connected channel peers.

When a new channel peer connects to `<channelName>` on the network a new message is sent to your connection as follows:

```javascript
{
  action: "connect", // a new channel peer has connected to <channelName>
  source: "<you>", // your channel peer's id
  target: "<newPeerId>" // the unique id of the new channel peer connection
}
```

Similarly when a channel peer disconnects from `<channelName>` on the network a new message is sent to your connection as follows:

```javascript
{
  action: "disconnect", // an existing channel peer has disconnected from <channelName>
  source: "<you>", // your channel peer's id
  target: "<existingPeerId>" // the unique id of the existing channel peer connection
}
```

To send a _broadcast message_ to all other connected channel peers you can send it over your connection as follows:

```javascript
{
  action: "broadcast", // this is a sent broadcast message
  data: "<data>" // the data you want to send to all other channel peers
}
```

When receiving a _broadcast message_ from another connected channel peer it is sent to you over your connection as follows:

```javascript
{
  action: "broadcast", // this is a received broadcast message
  source: "<peerId>", // the sending channel peer's id
  data: "<data>" // the data you want to send to all other channel peers
}
```

To send a _direct message_ to another channel peer, bypassing the broadcast channel, you can send it over your connection as follows:

```javascript
{
  action: "message", // this is a sent direct message
  target: "<recipient>", // the id of an existing channel peer you want to send a direct message to
  data: "<data>" // the data you want to send to <recipient>
}
```

When receiving a _direct message_ from another channel peer, that has bypassed the broadcast channel, it is sent to you over your connection as follows:

```javascript
{
  action: "message", // this is a received direct message
  source: "<sender>", // the id of the channel peer that sent you this direct message
  target: "<you>", // your channel peer's id
  data: "<data>" // the data sent to you by <sender>
}
```

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
