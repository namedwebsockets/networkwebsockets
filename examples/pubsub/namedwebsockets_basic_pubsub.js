/***
 *
 * Basic Publish/Subscribe library for Named WebSockets
 * ------------------------------------------------------------------------
 * ------------------------------------------------------------------------
 * https://github.com/richtr/namedwebsockets/tree/master/examples/pubsub
 * ------------------------------------------------------------------------
 *
 * For an example of usage, please see `pubsub.html`.
 *
 */
var NamedWebSockets_Basic_PubSub = function(namedWebSocketObj) {
	this.ws = namedWebSocketObj;
	this.ws.addEventListener("open", function() {
		for(var msg in this.sendQueue) {
			this.ws.send(this.sendQueue[msg]);
			this.sendQueue = [];
		}
	}.bind(this));

	this.nodeId = Math.floor(Math.random() * 1e16);

	this.topicSubscriptions = [];

	this.sendQueue = [];

	this.ws.onmessage = function(messageEvent) {
		// Distribute received message to subscriber
		try {
			var msg = JSON.parse(messageEvent.data);

			if (msg.action && msg.action == "publish") {
				var subscriptions = this.topicSubscriptions[msg.topicURI];
				for (var nodeId in subscriptions) {
					var nodeSubscriptions = subscriptions[nodeId];
					for (var callback in nodeSubscriptions) {
						(nodeSubscriptions[callback]).call(this, msg.payload);
					}
				}
			}
		} catch (e) {
			console.error("Could not process publish message");
		}
	}.bind(this);
};

NamedWebSockets_Basic_PubSub.prototype.constructor = NamedWebSockets_Basic_PubSub;

NamedWebSockets_Basic_PubSub.prototype.subscribe = function(topicURI, successCallback)	{
	var subscriptions = this.topicSubscriptions[topicURI] || {};
	var nodeSubscriptions = subscriptions[this.nodeId] || [];

	nodeSubscriptions.push(successCallback);

	subscriptions[this.nodeId] = nodeSubscriptions;
	this.topicSubscriptions[topicURI] = subscriptions;
};

NamedWebSockets_Basic_PubSub.prototype.unsubscribe = function(topicURI, successCallback)	{
	var subscriptions = this.topicSubscriptions[topicURI] || {};
	var nodeSubscriptions = subscriptions[this.nodeId] || [];

	for (var i in nodeSubscriptions) {
		if (successCallbackBack == nodeSubscriptions[i]) {
			nodeSubscriptions.splice(i, 1);
			break;
		}
	}

	subscriptions[this.nodeId] = nodeSubscriptions;
	this.topicSubscriptions[topicURI] = subscriptions;
}

NamedWebSockets_Basic_PubSub.prototype.publish = function(topicURI, payload, successCallback)	{
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

	if (successCallback) {
		successCallback.call(this);
	}
};
