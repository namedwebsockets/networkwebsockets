/***
 *
 * Basic Publish/Subscribe library for Named WebSockets
 * ------------------------------------------------------------------------
 * ------------------------------------------------------------------------
 * https://github.com/namedwebsockets/proxy/tree/master/examples/pubsub
 * ------------------------------------------------------------------------
 *
 * For an example of usage, please see `pubsub.html`.
 *
 */
var NamedWS_PubSubHub = function(namedWebSocketObj) {
	this.ws = namedWebSocketObj;
	this.ws.addEventListener("open", function() {
		for(var msg in this.sendQueue) {
			this.ws.send(this.sendQueue[msg]);
			this.sendQueue = [];
		}
	}.bind(this));

	this.sendQueue = [];

	this.topicSubscriptions = [];

	this.ws.onmessage = function(messageEvent) {
		// Distribute received message to subscriber
		try {
			var msg = JSON.parse(messageEvent.data);

			if (msg.action && msg.action == "publish") {
				var subscriptions = this.topicSubscriptions[msg.topicURI];
				for (var callback in subscriptions) {
					(subscriptions[callback]).call(this, msg.payload);
				}
			}
		} catch (e) {
			console.error(e);
		}
	}.bind(this);
};

NamedWS_PubSubHub.prototype.constructor = NamedWS_PubSubHub;

NamedWS_PubSubHub.prototype.subscribe = function(topicURI, successCallback) {
	if (!successCallback) return;
	var subscriptions = this.topicSubscriptions[topicURI] || [];
	subscriptions.push(successCallback);
	this.topicSubscriptions[topicURI] = subscriptions;
};

NamedWS_PubSubHub.prototype.unsubscribe = function(topicURI, successCallback) {
	if (!successCallback) return;
	var subscriptions = this.topicSubscriptions[topicURI] || [];
	for (var i in subscriptions) {
		if (successCallback == subscriptions[i]) {
			subscriptions.splice(i, 1);
			break;
		}
	}
	this.topicSubscriptions[topicURI] = subscriptions;
}

NamedWS_PubSubHub.prototype.publish = function(topicURI, payload, successCallback) {
	// Send over websocket
	var publishMsg = {
		action: "publish",
		topicURI: topicURI || "",
		payload: payload || {}
	};

	var msg = JSON.stringify(publishMsg)

	if (this.ws.readyState != 1) {
		this.sendQueue.push(msg);
	} else {
		this.ws.send(msg);
	}

	if (successCallback) successCallback.call(this);
};
