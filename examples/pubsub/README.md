Named WebSockets PubSub Demo
===

A basic [publish/subscribe](https://en.wikipedia.org/wiki/Publish–subscribe_pattern) demo on top of [Named WebSockets](https://github.com/namedwebsockets/namedwebsockets).

#### Running the example

1. Ensure the Named WebSockets Proxy has been downloaded and is currently running on your local machine. You can download pre-built Named WebSocket Proxy binaries [here](https://github.com/namedwebsockets/namedwebsockets/releases).

    NOTE: Ultimately, this step should not be required with all proxy functionality implemented within user agents.

2. Open [`pubsub.html`](http://namedwebsockets.github.io/namedwebsockets/examples/pubsub/pubsub.html) on your local machine.

3. Open [`pubsub.html`](http://namedwebsockets.github.io/namedwebsockets/examples/pubsub/pubsub.html) in another browser window *on your local machine* (in the same browser or in a different browser).

    NOTE: This service has been limiting our service to operate on the local machine only due to using a `LocalWebSocket` connection.

4. Log in and out on one window using the interface provided and watch the authorization state get applied to the other window (and vice-versa).
