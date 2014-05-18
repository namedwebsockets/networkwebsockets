/***
 * BroadcastWebSocket + LocalWebSocket shim library
 * ----------------------------------------------------------------
 *
 * API Usage:
 * ----------
 *
 *     // Broadcast and connect with other peers using the same service name in the current network
 *     var ws = new BroadcastWebSocket("myServiceName");
 *
 * or:
 *
 *     // Connect with other peers using the same service name on the local device
 *     var ws = new LocalWebSocket("myServiceName");
 *
 * ...then use the returned `ws` object just like a normal JavaScript WebSocket object.
 *
**/
(function(global) {

	// *Always* connect to our own localhost-based proxy
	var endpointUrlBase = "ws://localhost:9009/";

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
		return new WebSocket(endpointUrlBase + (isBroadcast ? "broadcast" : "local") + "/" + serviceName);
	};

	var BroadcastWebSocket = function (serviceName) {
		return new NamedWebSocket(serviceName, true);
	};

	var LocalWebSocket = function (serviceName) {
		return new NamedWebSocket(serviceName, false);
	};

	// Expose global functions

	if (!global.BroadcastWebSocket) {
		global.BroadcastWebSocket = (global.module || {}).exports = BroadcastWebSocket;
	}

	if (!global.LocalWebSocket) {
		global.LocalWebSocket = (global.module || {}).exports = LocalWebSocket;
	}

})(this);