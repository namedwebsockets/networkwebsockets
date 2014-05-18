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
		return /^[A-Za-z0-9\._-]{1,255}$/.test(serviceName);
	}

	var NamedWebSocket = function (serviceName, subprotocols, isBroadcast) {
		if (!isValidServiceName(serviceName)) {
			throw "Invalid Service Name: " + serviceName;
		}
		return new WebSocket(endpointUrlBase + (isBroadcast ? "broadcast" : "local") + "/" + serviceName, subprotocols);
	};

	var BroadcastWebSocket = function (serviceName, subprotocols) {
		return new NamedWebSocket(serviceName, subprotocols, true);
	};

	var LocalWebSocket = function (serviceName, subprotocols) {
		return new NamedWebSocket(serviceName, subprotocols, false);
	};

	// Expose global functions

	if (!global.BroadcastWebSocket) {
		global.BroadcastWebSocket = (global.module || {}).exports = BroadcastWebSocket;
	}

	if (!global.LocalWebSocket) {
		global.LocalWebSocket = (global.module || {}).exports = LocalWebSocket;
	}

})(this);