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

	function isValidServiceName(serviceName) {
		return /^[A-Za-z0-9\=\/\+\._-]{1,255}$/.test(serviceName);
	}

	function isJson(json) {
	    try {
	        JSON.parse(json);
	    } catch (e) {
	        return false;
	    }
	    return true;
	}

	function createServicePath(serviceName) {
		return "network/" + serviceName + "/";
	}

/**** START WEBSOCKET SHIM ****/

	var P2PWebSocket = function(peerId, controlWebSocket) {
		this.id     = peerId;
		this.socket = controlWebSocket;

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

		this.__doClose(code || 3001, reason || "Closed by local peer", this.socket);
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
		var events = this.__events[event.type] || [];
		for (var i = 0; i < events.length; ++i) {
			events[i].call(this, event);
		}
		var handler = this["on" + event.type];
		if (handler) handler.call(this, event);
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

	P2PWebSocket.prototype.__doClose = function(code, reason, rootWebSocket) {
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
						// Fire 'disconnect' event at root websocket object
						var disconnectEvt = new CustomEvent('disconnect', {
							"bubbles": false,
							"cancelable": false,
							"detail": {
									"target": this
							}
						});
						rootWebSocket.dispatchEvent(disconnectEvt);

						if (rootWebSocket["ondisconnect"])
							rootWebSocket["ondisconnect"].call(this, disconnectEvt);
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

	var NamedWebSocket = function (serviceName, subprotocols) {
		if (!isValidServiceName(serviceName)) {
			throw "Invalid Service Name: " + serviceName;
		}

		var peerId = Math.floor( Math.random() * 1e16);

		var path = createServicePath(serviceName)

		var rootWebSocket = new WebSocket(endpointUrlBase + path + peerId, subprotocols);

		// New custom attributes on NamedWebSocket objects
		rootWebSocket.id    = peerId
		rootWebSocket.peers = [];

		var controlWebSocket = new WebSocket(endpointUrlBase + "control/" + path + peerId);

		var p2pWebSockets = {};

		// Process control websocket messages
		controlWebSocket.onmessage = function(evt) {
			var data = JSON.parse(evt.data);

			switch (data.action) {
				case "connect":
					// Create a new WebSocket shim object
					var ws = new P2PWebSocket(data.target, controlWebSocket);
					p2pWebSockets[data.target] = ws;

					// Add to root web sockets p2p sockets enumeration
					rootWebSocket.peers.push(ws);

					// Fire 'connect' event at root websocket object
					// **then** fire p2p websocket 'open' event (see above)
					var connectEvt = new CustomEvent('connect', {
						"bubbles": false,
						"cancelable": false,
						"detail": {
								"target": ws
						}
					});
					rootWebSocket.dispatchEvent(connectEvt);

					if (rootWebSocket["onconnect"])
						rootWebSocket["onconnect"].call(this, connectEvt);

					window.setTimeout(function() {
						// Fire 'open' event at new websocket shim object
						ws.__handleEvent({
							type: "open",
							readyState: P2PWebSocket.prototype.OPEN
						});
					}, 200);

					break;
				case "disconnect":
					var ws = p2pWebSockets[data.target];
					if (ws) {

						// Remove from root web sockets p2p sockets enumeration
						for (var i = 0; i < rootWebSocket.peers.length; i++) {
							if (rootWebSocket.peers[i].id == data.target) {
								rootWebSocket.peers.splice(i, 1);
							}
						}

						// Create and fire events:
						//   - 'close' on p2p websocket object
						//   - 'disconnect' on root websocket object
						ws.__doClose(3000, "Closed by remote peer", rootWebSocket);
						delete p2pWebSockets[data.target];
					}

					break;
				case "message":
					// Use source address to match up to target
					var ws = p2pWebSockets[data.source];
					if (ws) {
						// Re-encode data payload as string
						var payload = data.data;
						if (Object.prototype.toString.call(payload) != '[object String]') {
							payload = JSON.stringify(payload);
						}

						// TODO: Check shim websocket readyState and queue or fire immediately
						ws.__handleEvent({
							type: "message",
							message: payload,
							senderId: data.source
						});
					}

					break;
			}
		}

		return rootWebSocket;
	};

	var NetworkWebSocket = function (serviceName, subprotocols) {
		return new NamedWebSocket(serviceName, subprotocols);
	};

	// Expose global functions

	if (!global.NetworkWebSocket) {
		global.NetworkWebSocket = (global.module || {}).exports = NetworkWebSocket;
	}

})(this);