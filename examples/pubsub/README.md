Named WebSockets PubSub Demo
===

A basic [publish/subscribe](https://en.wikipedia.org/wiki/Publish–subscribe_pattern) demo on top of [Named WebSockets](https://github.com/namedwebsockets/namedwebsockets).

#### Running the example

1. Ensure a Named WebSockets Proxy is running as detailed in the root project [README](https://github.com/namedwebsockets/namedwebsockets/blob/master/README.md#run-a-named-websockets-proxy).

2. Open [`pubsub.html`](http://namedwebsockets.github.io/namedwebsockets/examples/pubsub/pubsub.html) on your local machine.

3. Open [`pubsub.html`](http://namedwebsockets.github.io/namedwebsockets/examples/pubsub/pubsub.html) in another browser window *on your local machine* (in the same browser or in a different browser).

    NOTE: This service has been limiting our service to operate on the local machine only due to using a `LocalWebSocket` connection.

4. Log in and out on one window using the interface provided and watch the authorization state get applied to the other window (and vice-versa).
