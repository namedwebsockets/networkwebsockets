package networkwebsockets

import (
	"log"
	"testing"
)

func createClient(t testing.TB, urlStr string) *NetworkWebSocketClient {
	client, _, err := Dial(urlStr)
	if err != nil {
		t.Fatalf("Dial: ", err)
	}
	return client
}

func getClientId(client *NetworkWebSocketClient) string {
	// Request client's peer id
	client.SendStatusRequest()
	// Wait for response
	message := <-client.Status
	// Return client's peer id
	return message.Target
}

func checkConnect(t testing.TB, message NetworkWebSocketWireMessage, expectedTarget string) {
	if message.Target != expectedTarget {
		t.Fatalf("connect=%s, want %s", message.Target, expectedTarget)
	}
}

func checkDisconnect(t testing.TB, message NetworkWebSocketWireMessage, expectedTarget string) {
	if message.Target != expectedTarget {
		t.Fatalf("disconnect=%s, want %s", message.Target, expectedTarget)
	}
}

func checkBroadcast(t testing.TB, payload string, sender *NetworkWebSocketClient, receivers []*NetworkWebSocketClient) {
	// send broadcast message from sender
	sender.SendBroadcastData(payload)

	// check broadcast message arrived at all receivers
	for _, receiver := range receivers {
		message := <-receiver.Broadcast
		if message.Payload != payload {
			t.Fatalf("broadcast=%s, want %s", message.Payload, payload)
		}
	}
}

func checkMessage(t testing.TB, payload string, targetId string, sender *NetworkWebSocketClient, receiver *NetworkWebSocketClient) {
	if targetId == "" {
		t.Fatalf("No target identifier provided")
	}

	// send broadcast message from sender
	sender.SendMessageData(payload, targetId)

	// check broadcast message arrived at all receivers
	message := <-receiver.Message
	if message.Payload != payload {
		t.Fatalf("message=%s, want %s", message.Payload, payload)
	}
}

// TEST CASES

func TestSameProxyClients(t *testing.T) {

	service := NewNetworkWebSocketService("localhost", 21000)
	_ = service.Start()

	// Create new Network Web Socket channel peers
	client1 := createClient(t, "ws://localhost:21000/testservice1")
	client2 := createClient(t, "ws://localhost:21000/testservice1")
	client3 := createClient(t, "ws://localhost:21000/testservice1")

	// Test status messaging (+ store client ids for future tests)
	client1Id := getClientId(client1)
	client2Id := getClientId(client2)
	client3Id := getClientId(client3)

	// Test connect messaging
	checkConnect(t, <-client1.Connect, client2Id)
	checkConnect(t, <-client1.Connect, client3Id)
	checkConnect(t, <-client2.Connect, client1Id)
	checkConnect(t, <-client2.Connect, client3Id)
	checkConnect(t, <-client3.Connect, client1Id)
	checkConnect(t, <-client3.Connect, client2Id)

	// Test broadcast messaging
	checkBroadcast(t, "hello world 1", client1, []*NetworkWebSocketClient{client2, client3})
	checkBroadcast(t, "hello world 2", client2, []*NetworkWebSocketClient{client1, client3})
	checkBroadcast(t, "hello world 3", client3, []*NetworkWebSocketClient{client1, client2})

	// Test direct messaging
	checkMessage(t, "direct message 1", client2Id, client1, client2)
	checkMessage(t, "direct message 2", client3Id, client1, client3)
	checkMessage(t, "direct message 3", client1Id, client2, client1)
	checkMessage(t, "direct message 4", client3Id, client2, client3)
	checkMessage(t, "direct message 5", client1Id, client3, client1)
	checkMessage(t, "direct message 6", client2Id, client3, client2)

	// Test disconnect messaging
	client1.Close()
	checkDisconnect(t, <-client2.Disconnect, client1Id)
	checkDisconnect(t, <-client3.Disconnect, client1Id)

	client2.Close()
	checkDisconnect(t, <-client3.Disconnect, client2Id)

	client3.Close()

	go service.Stop()

	<-service.StopNotify()
}

func TestMultipleProxyClients(t *testing.T) {

	service1 := NewNetworkWebSocketService("localhost", 21000)
	_ = service1.Start()

	service2 := NewNetworkWebSocketService("localhost", 21001)
	_ = service2.Start()

	// Create new Network Web Socket channel peers
	client1 := createClient(t, "ws://localhost:21000/testservice2")
	client2 := createClient(t, "ws://localhost:21001/testservice2")
	client3 := createClient(t, "ws://localhost:21001/testservice2")

	// Test status messaging (+ store client ids for future tests)
	client1Id := getClientId(client1)
	client2Id := getClientId(client2)
	client3Id := getClientId(client3)

	log.Println("Waiting for Network Web Socket proxies to discover and connect to each other...")

	// Test connect messaging
	checkConnect(t, <-client1.Connect, client2Id)
	checkConnect(t, <-client1.Connect, client3Id)
	checkConnect(t, <-client2.Connect, client3Id)
	checkConnect(t, <-client2.Connect, client1Id)
	checkConnect(t, <-client3.Connect, client2Id)
	checkConnect(t, <-client3.Connect, client1Id)

	// Test broadcast messaging
	checkBroadcast(t, "hello world 1", client1, []*NetworkWebSocketClient{client2, client3})
	checkBroadcast(t, "hello world 2", client2, []*NetworkWebSocketClient{client1, client3})
	checkBroadcast(t, "hello world 3", client3, []*NetworkWebSocketClient{client1, client2})

	// Test direct messaging
	checkMessage(t, "direct message 1", client2Id, client1, client2)
	checkMessage(t, "direct message 2", client3Id, client1, client3)
	checkMessage(t, "direct message 3", client1Id, client2, client1)
	checkMessage(t, "direct message 4", client3Id, client2, client3)
	checkMessage(t, "direct message 5", client1Id, client3, client1)
	checkMessage(t, "direct message 6", client2Id, client3, client2)

	// Test disconnect messaging
	client1.Close()
	checkDisconnect(t, <-client2.Disconnect, client1Id)
	checkDisconnect(t, <-client3.Disconnect, client1Id)

	client2.Close()
	checkDisconnect(t, <-client3.Disconnect, client2Id)

	client3.Close()

	go func() {
		service1.Stop()
		service2.Stop()
	}()

	<-service1.StopNotify()
	<-service2.StopNotify()
}

// BENCHMARKS

func BenchmarkSameProxyClientSetup(b *testing.B) {
	service := NewNetworkWebSocketService("localhost", 21000)
	_ = service.Start()

	// run the benchmark function b.N times
	for n := 0; n < b.N; n++ {
		// Create new Network Web Socket channel peers
		client := createClient(b, "ws://localhost:21000/benchmarkservice1")
		_ = getClientId(client) // wait for client connection to be established
		client.Close()
	}

	go service.Stop()

	<-service.StopNotify()
}

func BenchmarkSameProxyClientMessaging(b *testing.B) {
	service := NewNetworkWebSocketService("localhost", 21000)
	_ = service.Start()

	client1 := createClient(b, "ws://localhost:21000/benchmarkservice2")
	client2 := createClient(b, "ws://localhost:21000/benchmarkservice2")

	client2Id := getClientId(client2)

	// run the benchmark function b.N times
	for n := 0; n < b.N; n++ {
		checkMessage(b, "direct benchmark message", client2Id, client1, client2)
	}

	go func() {
		client1.Close()
		client2.Close()

		service.Stop()
	}()

	<-service.StopNotify()
}

func BenchmarkSameProxyClientBroadcast(b *testing.B) {
	service := NewNetworkWebSocketService("localhost", 21000)
	_ = service.Start()

	client1 := createClient(b, "ws://localhost:21000/benchmarkservice3")
	client2 := createClient(b, "ws://localhost:21000/benchmarkservice3")
	client3 := createClient(b, "ws://localhost:21000/benchmarkservice3")

	// run the benchmark function b.N times
	for n := 0; n < b.N; n++ {
		checkBroadcast(b, "benchmark test msg", client1, []*NetworkWebSocketClient{client2, client3})
	}

	go func() {
		client1.Close()
		client2.Close()
		client3.Close()

		service.Stop()
	}()

	<-service.StopNotify()
}
