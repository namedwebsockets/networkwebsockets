Named WebSockets WebRTC Video/Audio Conferencing Demo
===

A *pure* P2P audio/video conferencing demo via WebRTC on top of [Named WebSockets](https://github.com/namedwebsockets/namedwebsockets).

This demo establishes a Named WebSocket to coordinate the set up of WebRTC PeerConnections between participating peers. As such, no STUN or TURN servers are required for establishment of WebRTC sessions through Named WebSockets and no centralized authorities are required to establish sessions before handing off to P2P.

#### Running the example

1. Ensure the Named WebSockets Proxy has been downloaded and is currently running on your local machine. You can download pre-built Named WebSocket Proxy binaries [here](https://github.com/namedwebsockets/namedwebsockets/releases).

    NOTE: Ultimately, this step should not be required with all proxy functionality implemented within user agents.

2. Open [webrtc.html](http://namedwebsockets.github.io/namedwebsockets/examples/webrtc/webrtc.html) on your local machine.

3. Open [webrtc.html](http://namedwebsockets.github.io/namedwebsockets/examples/webrtc/webrtc.html) in another browser window on your local machine (in the same browser or in a different browser) or on another device that is also running a Named WebSockets proxy in the local network.

4. When prompted, give the web page access to your web camera and microphone.

5. Welcome to a zero-config ad-hoc decentralized P2P WebRTC session. Other WebRTC peers can join and leave at any time by opening the same web page with all session signalling performed completely P2P.
