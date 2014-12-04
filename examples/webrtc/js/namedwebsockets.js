/***
* NetworkWebSocket shim library
* ----------------------------------------------------------------
*
* API Usage:
* ----------
*
*     // Connect with other peers using the same service name in the current network
*     var ws = new NetworkWebSocket("myServiceName");
*
* ...then use the returned `ws` object just like a normal JavaScript WebSocket object.
*
**/
(function(global) {

// *Always* connect to our own localhost-based endpoint for _creating_ new Network Web Sockets
var endpointUrlBase = "ws://localhost:9009/";

function isValidServiceName(channelName) {
	return /^[A-Za-z0-9\=\+\._-]{1,255}$/.test(channelName);
}

function toJson(data) {
    try {
        return JSON.parse(data);
    } catch (e) {}
		return false;
}

var _NetworkWebSocket = function (channelName, subprotocols) {
	if (!isValidServiceName(channelName)) {
		throw "Invalid Service Name: " + channelName;
	}

	// *Actual* web socket connection to Network Web Socket proxy
	var webSocket = new WebSocket(endpointUrlBase + channelName, subprotocols);

	// Root NetworkWebSocket object
	var networkWebSocket = new P2PWebSocket(webSocket);

	function getPeerById(id) {
		for (var i = 0; i < networkWebSocket.peers.length; i++) {
			if (networkWebSocket.peers[i].id == id) {
				return networkWebSocket.peers[i];
			}
		}
	}

	// Peer NetworkWebSocket objects list
	networkWebSocket.peers = [];

	// override
	networkWebSocket.send = function(data) {
		if (this.readyState != P2PWebSocket.prototype.OPEN) {
			throw "message cannot be sent because the web socket is not open";
		}

		var message = {
			"action":  "broadcast",
			"data": data
		};
		this.socket.send(JSON.stringify(message));
	};

	// override
 	networkWebSocket.close = function(code, reason) {
 		if (this.readyState != P2PWebSocket.prototype.OPEN) {
 			throw "web socket cannot be closed because it is not open";
 		}

		webSocket.close();
 	};

	webSocket.onopen = function(event) {

		networkWebSocket.__handleEvent({
			type: "open",
			readyState: P2PWebSocket.prototype.OPEN
		});

	};

	// Incoming Network Web Socket message dispatcher
	webSocket.onmessage = function(event) {
		var json = toJson(event.data);

		if (!json) {
			return
		}

		switch(json.action) {

			case "connect":
				// fire connect event on root network web socket object

				// Create a new WebSocket shim object
				var peerWebSocket = new P2PWebSocket(webSocket, networkWebSocket, json.target);

				// Add to root web sockets p2p sockets enumeration
				networkWebSocket.peers.push(peerWebSocket);

				// Fire 'connect' event at root websocket object
				// **then** fire p2p websocket 'open' event (see above)
				var connectEvt = new CustomEvent('connect', {
					"bubbles": false,
					"cancelable": false,
					"detail": {
							"target": peerWebSocket
					}
				});
				networkWebSocket.dispatchEvent(connectEvt);

				window.setTimeout(function() {
					// Fire 'open' event at new websocket shim object
					peerWebSocket.__handleEvent({
						type: "open",
						readyState: P2PWebSocket.prototype.OPEN
					});
				}, 50);

				break;

			case "disconnect":
				// close peer network web socket object

				var peerWebSocket = getPeerById(json.target);
				if (!peerWebSocket) {
					return;
				}

				// Create and fire events:
				//   - 'close' on p2p websocket object
				//   - 'disconnect' on root websocket object
				peerWebSocket.__doClose(3000, "Closed", networkWebSocket);

				// Remove p2p websocket from root network web socket peers list
				for (var i = 0; i < networkWebSocket.peers.length; i++) {
					if (networkWebSocket.peers[i].id == json.target) {
						networkWebSocket.peers.splice(i,1);
						break;
					}
				}

				break;

			case "broadcast":
				// dispatch to root network web socket object

				// Re-encode data payload as string
				var payload = json.data;
				if (Object.prototype.toString.call(payload) != '[object String]') {
					payload = JSON.stringify(payload);
				}

				// TODO: Check shim websocket readyState and queue or fire immediately
				networkWebSocket.__handleEvent({
					type: "message",
					message: payload,
					senderId: json.source
				});

				break;

			case "message":
				// dispatch to peer network web socket object

				var peerWebSocket = getPeerById(json.source);
				if (!peerWebSocket) {
					return
				}

				// Re-encode data payload as string
				var payload = json.data;
				if (Object.prototype.toString.call(payload) != '[object String]') {
					payload = JSON.stringify(payload);
				}

				// TODO: Check shim websocket readyState and queue or fire immediately
				peerWebSocket.__handleEvent({
					type: "message",
					message: payload,
					senderId: json.source
				});

				break;

		}
	};

	webSocket.onclose = function(event) {

		// Close all peer connections
		for (var target in networkWebSocket.peers) {
			networkWebSocket.peers[target].__doClose(3000, "Closed", networkWebSocket);
		}
		networkWebSocket.peers = [];

		// Close root connection
		networkWebSocket.__doClose(3000, "Closed")

	};

	return networkWebSocket;

};

/**** START WEBSOCKET SHIM ****/

var P2PWebSocket = function(rootWebSocket, parentWebSocket, targetId) {
	this.id = targetId || "";
	this.socket = rootWebSocket;
	this.parent = parentWebSocket;

	// Setup dynamic WebSocket interface attributes
	this.url            = "#"; // no url (...that's kind of the whole point :)
	this.readyState     = P2PWebSocket.prototype.CONNECTING; // initial state
	this.bufferedAmount = 0;
	this.extensions     = this.socket.extensions; // inherit
	this.protocol       = this.socket.protocol;   // inherit
	this.binaryType     = "blob"; // as per WebSockets spec

	this.__events = {};
};

P2PWebSocket.prototype.send = function(data) {
	if (this.readyState != P2PWebSocket.prototype.OPEN) {
		throw "message cannot be sent because the web socket is not open";
	}

	var message = {
		"action":  "message",
		"target":  this.id,
		"data": data
	};
	this.socket.send(JSON.stringify(message));
};

P2PWebSocket.prototype.close = function(code, reason) {
	if (this.readyState != P2PWebSocket.prototype.OPEN) {
		throw "web socket cannot be closed because it is not open";
	}

	this.__doClose(code || 3001, reason || "Closed", this.parent);
};

P2PWebSocket.prototype.addEventListener = function(type, listener, useCapture) {
	if (!(type in this.__events)) {
		this.__events[type] = [];
	}
	this.__events[type].push(listener);
};

P2PWebSocket.prototype.removeEventListener = function(type, listener, useCapture) {
	if (!(type in this.__events)) return;
	var events = this.__events[type];
	for (var i = events.length - 1; i >= 0; --i) {
		if (events[i] === listener) {
			events.splice(i, 1);
			break;
		}
	}
};

P2PWebSocket.prototype.dispatchEvent = function(event) {

	// Delay until next run loop (like real events)
	window.setTimeout(function() {

 		var events = this.__events[event.type] || [];
 		for (var i = 0; i < events.length; ++i) {
 			events[i].call(this, event);
 		}
 		var handler = this["on" + event.type];
 		if (handler) handler.call(this, event);

	}.bind(this), 2);

};

P2PWebSocket.prototype.__handleEvent = function(flashEvent) {

	// Delay until next run loop (like real events)
	window.setTimeout(function() {

		if ("readyState" in flashEvent) {
			this.readyState = flashEvent.readyState;
		}

		var jsEvent;
		if (flashEvent.type == "open" || flashEvent.type == "closing" || flashEvent.type == "error") {
			jsEvent = this.__createSimpleEvent(flashEvent.type);
		} else if (flashEvent.type == "close") {
			jsEvent = this.__createSimpleEvent("close");
			jsEvent.code = flashEvent.code;
			jsEvent.reason = flashEvent.reason;
		} else if (flashEvent.type == "message") {
			jsEvent = this.__createMessageEvent(flashEvent.senderId, flashEvent.message);
		} else {
			throw "unknown event type: " + flashEvent.type;
		}

		this.dispatchEvent(jsEvent);

		// Fire callback (if any provided)
		if (flashEvent.callback) flashEvent.callback.call(this);

	}.bind(this), 1);
};

P2PWebSocket.prototype.__createSimpleEvent = function(type) {
	if (document.createEvent && window.Event) {
		var event = document.createEvent("Event");
		event.initEvent(type, false, false);
		return event;
	} else {
		return {type: type, bubbles: false, cancelable: false};
	}
};

P2PWebSocket.prototype.__createMessageEvent = function(senderId, data) {
	if (window.MessageEvent && typeof(MessageEvent) == "function") {
		return new MessageEvent("message", {
			"view": window,
			"bubbles": false,
			"cancelable": false,
			"senderId": senderId,
			"data": data
		});
	} else if (document.createEvent && window.MessageEvent) {
		var event = document.createEvent("MessageEvent");
		event.initMessageEvent("message", false, false, data, null, null, window, null);
		return event;
	} else {
		return {type: "message", data: data, bubbles: false, cancelable: false};
	}
};

P2PWebSocket.prototype.__doClose = function(code, reason, parentWebSocket) {
	// Fire 'open' event at new websocket shim object
	this.__handleEvent({
		type: "closing",
		readyState: P2PWebSocket.prototype.CLOSING,
		callback: function() {
			// Fire 'open' event at new websocket shim object
			this.__handleEvent({
				type: "close",
				readyState: P2PWebSocket.prototype.CLOSED,
				code: code,
				reason: reason,
				callback: function() {
					if (parentWebSocket) {
						// Fire 'disconnect' event at root websocket object
						var disconnectEvt = new CustomEvent('disconnect', {
							"bubbles": false,
							"cancelable": false,
							"detail": {
									"target": this
							}
						});
						parentWebSocket.dispatchEvent(disconnectEvt);
					}
				}
			});
		}
	});
}

/**
* Define the WebSocket readyState enumeration.
*/
P2PWebSocket.prototype.CONNECTING = 0;
P2PWebSocket.prototype.OPEN = 1;
P2PWebSocket.prototype.CLOSING = 2;
P2PWebSocket.prototype.CLOSED = 3;

/**** END WEBSOCKET SHIM ****/

var NetworkWebSocket = function (channelName, subprotocols) {
	return new _NetworkWebSocket(channelName, subprotocols);
};

// Expose global functions

if (!global.NetworkWebSocket) {
	global.NetworkWebSocket = (global.module || {}).exports = NetworkWebSocket;
}

})(this);