package websocket

import (
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
