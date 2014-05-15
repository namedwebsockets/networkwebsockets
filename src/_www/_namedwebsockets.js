/***
 * THIS IS A TEMPLATE FILE!!
 *
 * Nothing to see here...
 *
 * Please use the file `/js/namedwebsocket.js` instead in your own projects
**/
(function(global) {

	if (global.NamedWebSocket) return;

	// *Always* connect to our own localhost-based proxy
	var endpointUrlBase = "ws://localhost:{{$}}/";

	function isValidServiceName(serviceName) {
		return /^[A-Za-z0-9_-]{1,255}$/.test(serviceName);
	}

	function dispatchSimpleEvent (target, eventName) {
		var simpleEvent = new Event(eventName);
		target.dispatchEvent(simpleEvent);
		if (target["on" + eventName]) {
			target["on" + eventName].call(target, simpleEvent);
		}
	}

	var NamedWebSocket = function (serviceName, isBroadcast) {

		if (!isValidServiceName(serviceName)) {
			throw "Invalid Service Name: " + serviceName;
		}

		var ws = new WebSocket(endpointUrlBase + (isBroadcast ? "broadcast" : "local") + "/" + serviceName);

		// Custom NamedWebSocket functionality

		ws.peerCount = 0;

		var messageFn;

		ws.onmessage = function (evt) {
			if (evt.data && evt.data == "____connect") {
				// Increment peer count
				ws.peerCount++;

				// Dispatch new simple 'connect' event on WS object
				dispatchSimpleEvent(ws, 'connect');

				return; // discard message
			} else if (evt.data && evt.data == "____disconnect") {
				// Decrement peer count
				ws.peerCount--;

				// Dispatch new simple 'disconnect' event on WS object
				dispatchSimpleEvent(ws, 'disconnect');

				return; // discard message
			}

			// Handle normally
			if (messageFn) messageFn.call(ws, evt);
		};

		ws.__defineSetter__("onmessage", function (fn) {
			messageFn = fn;
		});

		return ws;
	};

	var BroadcastWebSocket = function (serviceName) {
		return new NamedWebSocket(serviceName, true);
	};

	var LocalWebSocket = function (serviceName) {
		return new NamedWebSocket(serviceName, false);
	};

	// Expose global functions
	global.BroadcastWebSocket = (global.module || {}).exports = BroadcastWebSocket;
	global.LocalWebSocket = (global.module || {}).exports = LocalWebSocket;

})(this);