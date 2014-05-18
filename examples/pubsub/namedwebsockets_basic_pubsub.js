/***
 *
 * Basic Publish/Subscribe library for Named WebSockets
 * ------------------------------------------------------------------------
 * ------------------------------------------------------------------------
 * https://github.com/richtr/namedwebsockets//tree/master/examples/pubsub
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

	this.nodeId = Math.floor( Math.random() * 1e16);

	this.topicSubscriptions = [];

	this.sendQueue = [];

	this.ws.onmessage = function(messageEvent) {
		// Distribute received message to subscriber
		try {
			var msg = JSON.parse(messageEvent.data);

			if (msg.action && msg.action == "publish") {
				for (var subscribedNodeId in this.topicSubscriptions[msg.topicURI]) {
					for (var callback in this.topicSubscriptions[msg.topicURI][subscribedNodeId]) {
						(this.topicSubscriptions[msg.topicURI][subscribedNodeId][callback]).call(this, msg.payload);
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
	var topicSubscriptionsContainer = this.topicSubscriptions[topicURI] || {};
	var topicSubscriptionsContainerNode = topicSubscriptionsContainer[this.nodeId] || [];

	topicSubscriptionsContainerNode.push(successCallback);

	topicSubscriptionsContainer[this.nodeId] = topicSubscriptionsContainerNode;
	this.topicSubscriptions[topicURI] = topicSubscriptionsContainer;
};

NamedWebSockets_Basic_PubSub.prototype.unsubscribe = function(topicURI, successCallback)	{
	var topicSubscriptionsContainer = this.topicSubscriptions[topicURI] || {};
	var topicSubscriptionsContainerNode = topicSubscriptionsContainer[this.nodeId] || [];

	for (var i in topicSubscriptionsContainerNode) {
		if (successCallbackBack == topicSubscriptionsContainerNode[i]) {
			topicSubscriptionsContainerNode.splice(i, 1);
			break;
		}
	}

	topicSubscriptionsContainer[this.nodeId] = topicSubscriptionsContainerNode;
	this.topicSubscriptions[topicURI] = topicSubscriptionsContainer;
}

NamedWebSockets_Basic_PubSub.prototype.publish = function(topicURI, payload, successCallback, advancedOptions)	{
	// Send over websocket
	var publishMsg = {
		action: "publish",
		topicURI: topicURI || "",
		payload: payload || {}
	};

	this.send(publishMsg);
};

NamedWebSockets_Basic_PubSub.prototype.send = function(json) {
	var msg = JSON.stringify(json)

	if (this.ws.readyState != 1) {
		this.sendQueue.push(msg);
	} else {
		this.ws.send(msg);
	}
};
