Named WebSockets PubSub Demo
===

A basic [publish/subscribe](https://en.wikipedia.org/wiki/Publish–subscribe_pattern) demo on top of [Named WebSockets](https://github.com/richtr/namedwebsockets).

#### Running the example

1. Ensure a Named WebSockets Proxy is running as detailed in the root project [README](https://github.com/richtr/namedwebsockets/blob/master/README.md#run-a-named-websockets-proxy).

2. Open `pubsub.html` in a browser window.

3. Open `pubsub.html` in another browser window on the *local machine only* (i.e. we are limiting our service to operate on the local machine only via a `LocalWebSocket` connection).

4. Log in and out on one window using the interface provided and watch the authorization state get applied to the other window (and vice-versa).
