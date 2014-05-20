Named WebSockets WebRTC Video/Audio Conferencing Demo
===

A *pure* P2P audio/video conferencing demo via WebRTC on top of [Named WebSockets](https://github.com/richtr/namedwebsockets).

This demo establishes a Named WebSocket to coordinate the set up of WebRTC PeerConnections between participating peers. As such, no STUN or TURN servers are required for establishment of WebRTC sessions through Named WebSockets and no centralized authorities are required to establish sessions before handing off to P2P.

#### Running the example

1. Ensure a Named WebSockets Proxy is running as detailed in the root project [README](https://github.com/richtr/namedwebsockets/blob/master/README.md#run-a-named-websockets-proxy).

2. Run this folder as a server on your local machine on e.g. `localhost`. Chrome/Opera do not allow access to the webcam from `file://` URLs so this step is important.

3. Open `index.html` in a browser window running on your host machine.

4. Open `index.html` in another browser window running on a host in the local machine or on another device that is also running a Named WebSockets proxy in the local network.

5. When prompted, give the web page access to your web camera and microphone.

6. Welcome to zero-config ad-hoc decentralized P2P WebRTC session(s)! :)
