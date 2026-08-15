package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHubBroadcastSerializedForwardsOriginalPayload(t *testing.T) {
	hub := NewHub()
	client := &Client{Send: make(chan []byte, 1), Channels: make(map[string]bool)}
	hub.Subscribe(client, "teams:team-1")
	go hub.Run()
	t.Cleanup(hub.Shutdown)

	payload := []byte(`{"event":"server.updated","channel":"teams:team-1","data":{"team_id":"team-1","status":"ready"}}`)
	hub.BroadcastSerialized("teams:team-1", payload)

	select {
	case received := <-client.Send:
		require.Equal(t, payload, received)
	case <-time.After(time.Second):
		t.Fatal("expected websocket payload")
	}
}

func TestRedisSubscriberForwardPreservesEncodedPayload(t *testing.T) {
	hub := NewHub()
	client := &Client{Send: make(chan []byte, 1), Channels: make(map[string]bool)}
	hub.Subscribe(client, "teams:team-1")
	go hub.Run()
	t.Cleanup(hub.Shutdown)

	payload := `{"event":"server.updated","channel":"teams:team-1","data":{"team_id":"team-1","status":"ready"}}`
	(&RedisSubscriber{hub: hub}).forward(payload)

	select {
	case received := <-client.Send:
		require.Equal(t, []byte(payload), received)
	case <-time.After(time.Second):
		t.Fatal("expected websocket payload")
	}
}

func TestHubBroadcastAndShutdown(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := NewClient(hub, nil, "user-a", "team-a")
	hub.Register(client)
	hub.Subscribe(client, "team.team-a")
	hub.Broadcast("team.team-a", "server.updated", map[string]string{"server_id": "srv-a"})

	select {
	case payload := <-client.Send:
		var message Message
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatal(err)
		}
		if message.Event != "server.updated" || message.Channel != "team.team-a" {
			t.Fatalf("unexpected message: %#v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast")
	}

	hub.Shutdown()
	hub.Shutdown()
	if !client.IsClosing() {
		t.Fatal("shutdown did not close registered client")
	}
	if client.SafeSend([]byte("after-close")) {
		t.Fatal("send succeeded after shutdown")
	}
}

func TestHubSubscribeUnsubscribe(t *testing.T) {
	hub := NewHub()
	client := NewClient(hub, nil, "user-a", "team-a")

	hub.Subscribe(client, "server.srv-a")
	if got := client.GetChannels(); len(got) != 1 || got[0] != "server.srv-a" {
		t.Fatalf("unexpected subscriptions: %v", got)
	}
	hub.Unsubscribe(client, "server.srv-a")
	if got := client.GetChannels(); len(got) != 0 {
		t.Fatalf("unsubscribe retained channels: %v", got)
	}
}

func TestHubRemovesRegisteredAndSlowClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Shutdown)

	client := NewClient(hub, nil, "user-a", "team-a")
	hub.Register(client)
	hub.Subscribe(client, "server.srv-a")
	hub.Unregister(client)
	require.Eventually(t, client.IsClosing, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		hub.mu.RLock()
		defer hub.mu.RUnlock()
		_, registered := hub.clients[client]
		_, subscribed := hub.channels["server.srv-a"]
		return !registered && !subscribed
	}, time.Second, 10*time.Millisecond)

	slowClient := NewClient(hub, nil, "user-b", "team-a")
	hub.Register(slowClient)
	hub.Subscribe(slowClient, "server.srv-a")
	for i := 0; i < cap(slowClient.Send); i++ {
		slowClient.Send <- []byte("queued")
	}
	hub.BroadcastSerialized("server.srv-a", []byte("new message"))
	require.Eventually(t, slowClient.IsClosing, time.Second, 10*time.Millisecond)
}

func TestHubLifecycleEdgeCases(t *testing.T) {
	hub := NewHub()

	unknownClient := NewClient(hub, nil, "unknown", "team-a")
	hub.removeClient(unknownClient)
	require.True(t, unknownClient.IsClosing())

	closedClient := NewClient(hub, nil, "closed", "team-a")
	closedClient.Close()
	hub.Subscribe(closedClient, "")
	hub.Subscribe(closedClient, "server.srv-a")
	require.Empty(t, closedClient.GetChannels())

	blockedClient := NewClient(hub, nil, "blocked", "team-a")
	hub.mu.Lock()
	subscribeDone := make(chan struct{})
	go func() {
		hub.Subscribe(blockedClient, "server.srv-a")
		close(subscribeDone)
	}()
	require.Eventually(t, func() bool {
		return len(blockedClient.GetChannels()) == 1
	}, time.Second, 10*time.Millisecond)
	blockedClient.Close()
	hub.mu.Unlock()
	select {
	case <-subscribeDone:
	case <-time.After(time.Second):
		t.Fatal("blocked subscription did not finish")
	}
	require.Empty(t, blockedClient.GetChannels())

	hub.mu.Lock()
	hub.clients[blockedClient] = true
	hub.mu.Unlock()
	hub.Shutdown()
	hub.Shutdown()
	hub.BroadcastSerialized("server.srv-a", []byte("after shutdown"))

	registeredAfterShutdown := NewClient(hub, nil, "late-register", "team-a")
	hub.Register(registeredAfterShutdown)
	require.True(t, registeredAfterShutdown.IsClosing())

	unregisteredAfterShutdown := NewClient(hub, nil, "late-unregister", "team-a")
	hub.Unregister(unregisteredAfterShutdown)
	require.True(t, unregisteredAfterShutdown.IsClosing())
}
