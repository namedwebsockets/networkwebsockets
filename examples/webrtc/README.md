Network Web Sockets WebRTC Conferencing Demo
===

A pure P2P A/V conferencing demo via WebRTC on top of [Network Web Sockets](https://github.com/namedwebsockets/networkwebsockets).

This demo establishes a Network Web Socket channel to initially transfer signalling data between WebRTC session peers. Using Network Web Sockets means no STUN or TURN servers are required for establishment of WebRTC sessions and no centralized servers are required to establish local WebRTC sessions.

#### Running the example

1. Ensure a Network Web Socket Proxy has been downloaded and is currently running on your local machine. You can download a pre-built Network Web Socket Proxy [here](https://github.com/namedwebsockets/networkwebsockets/releases).

2. Open [webrtc.html](http://namedwebsockets.github.io/networkwebsockets/examples/webrtc/webrtc.html) on your local machine.

3. Open [webrtc.html](http://namedwebsockets.github.io/networkwebsockets/examples/webrtc/webrtc.html) in another browser window on your local machine (in the same browser or in a different browser) or on another device that is also running a Named WebSockets proxy in the local network.

4. Give each web page access to your web camera and microphone to participate in the conference.

5. Enjoy your zero-config WebRTC session. Other WebRTC peers can join and leave at any time by opening [the same webrtc.html](http://namedwebsockets.github.io/networkwebsockets/examples/webrtc/webrtc.html).
