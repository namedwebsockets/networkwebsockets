package networkwebsockets

import (
	"testing"
	"time"
)

func Test_ClientServer(t *testing.T) {

	service := NewNetworkWebSocketService("localhost", 20100)
	_ = service.Start()

	time.Sleep(time.Second * 3) // sleep just long enough to let the all the Network Web Socket services start

	// Set up Network Web Socket clients

	client1, _, err := Dial("ws://localhost:20100/myexampleservice")
	if err != nil {
		t.Fatalf("Dial1: ", err)
	}

	client2, _, err := Dial("ws://localhost:20100/myexampleservice")
	if err != nil {
		t.Fatalf("Dial2: ", err)
	}

	client3, _, err := Dial("ws://localhost:20100/myexampleservice")
	if err != nil {
		t.Fatalf("Dial2: ", err)
	}

	// Wait for all peers to connect to each other before continuing test
	<-client1.Connect
	<-client1.Connect
	<-client2.Connect
	<-client2.Connect
	<-client3.Connect
	<-client3.Connect

	// Send a broadcast message from client 1
	client1.SendBroadcastData("hello world 1")

	// Receive broadcast message from client 1
	if message := <-client2.Broadcast; message.Payload != "hello world 1" {
		t.Fatalf("broadcast=%s, want %s", message.Payload, "hello world 1")
	}
	if message := <-client3.Broadcast; message.Payload != "hello world 1" {
		t.Fatalf("broadcast=%s, want %s", message.Payload, "hello world 1")
	}

	// Send a broadcast message from client 2
	client2.SendBroadcastData("hello world 2")

	// Receive broadcast message from client 2
	if message := <-client1.Broadcast; message.Payload != "hello world 2" {
		t.Fatalf("broadcast=%s, want %s", message.Payload, "hello world 2")
	}
	if message := <-client3.Broadcast; message.Payload != "hello world 2" {
		t.Fatalf("broadcast=%s, want %s", message.Payload, "hello world 2")
	}

	// Send a broadcast message from client 3
	client3.SendBroadcastData("hello world 3")

	// Receive broadcast message from client 2
	if message := <-client1.Broadcast; message.Payload != "hello world 3" {
		t.Fatalf("broadcast=%s, want %s", message.Payload, "hello world 3")
	}
	if message := <-client2.Broadcast; message.Payload != "hello world 3" {
		t.Fatalf("broadcast=%s, want %s", message.Payload, "hello world 3")
	}

}
