package websocket

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/pkg/broadcast"
)

type staffChannelTestAuthorizer struct{ allowed atomic.Bool }

func (a *staffChannelTestAuthorizer) AuthorizeChannel(_, _, channel string) bool {
	return channel == broadcast.AdminResourcesChannel && a.allowed.Load()
}

func TestStaffInvalidationDeliveryRechecksAccess(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Shutdown)
	auth := &staffChannelTestAuthorizer{}
	auth.allowed.Store(true)
	client := NewClient(hub, nil, "staff-a", "staff-team", auth)
	client.staffTokenExpiresAt = time.Now().Add(time.Hour)
	hub.Subscribe(client, broadcast.AdminResourcesChannel)

	// This owner's team is not subscribed. Staff still receives the ID-only hint.
	hub.BroadcastToTeam("customer-team", "site.deleted", map[string]any{"site_id": "site-a", "server_id": "server-a", "address": "private.example"})
	message := receiveAdminTestMessage(t, client)
	require.Equal(t, broadcast.AdminResourceChanged, message.Event)
	require.Equal(t, broadcast.AdminResourcesChannel, message.Channel)
	data := message.Data.(map[string]any)
	require.Equal(t, "site-a", data["site_id"])
	require.NotContains(t, data, "address")

	auth.allowed.Store(false)
	hub.BroadcastToTeam("customer-team", "site.deleted", map[string]any{"site_id": "site-b"})
	message = receiveAdminTestMessage(t, client)
	require.Equal(t, "subscription.error", message.Event)
	require.Empty(t, client.GetChannels())
}

func TestAdminChannelRejectsExpiredOrSpectateSessions(t *testing.T) {
	const secret = "admin-channel-test"
	valid := func(claims jwt.MapClaims) string {
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
		require.NoError(t, err)
		return signed
	}
	for _, tc := range []struct {
		name    string
		extra   jwt.MapClaims
		allowed bool
	}{
		{"access", jwt.MapClaims{}, true},
		{"refresh", jwt.MapClaims{"type": "refresh"}, false},
		{"spectate", jwt.MapClaims{"impersonator_id": "actor"}, false},
		{"spectate session", jwt.MapClaims{"impersonation_sid": "session"}, false},
		{"read only", jwt.MapClaims{"read_only": true}, false},
		{"expired", jwt.MapClaims{"exp": time.Now().Add(-time.Hour).Unix()}, false},
		{"no expiry", jwt.MapClaims{"exp": nil}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := jwt.MapClaims{"sub": "staff-a", "type": "access", "exp": time.Now().Add(time.Hour).Unix()}
			for key, value := range tc.extra {
				claims[key] = value
			}
			expires := staffTokenExpiry(valid(claims), secret)
			require.Equal(t, tc.allowed, !expires.IsZero())
			auth := &staffChannelTestAuthorizer{}
			auth.allowed.Store(true)
			client := NewClient(NewHub(), nil, "staff-a", "team-a", auth)
			client.staffTokenExpiresAt = expires
			require.Equal(t, tc.allowed, client.canSubscribe(broadcast.AdminResourcesChannel))
		})
	}
}

func TestRedisBroadcasterMirrorsWorkerLifecycleWithoutModelData(t *testing.T) {
	server := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, redisClient.Close()) })
	subscription := redisClient.Subscribe(context.Background(), wsChannel)
	t.Cleanup(func() { require.NoError(t, subscription.Close()) })
	_, err := subscription.Receive(context.Background())
	require.NoError(t, err)
	b := NewRedisBroadcaster(server.Addr(), "", 0, nil)
	t.Cleanup(func() { require.NoError(t, b.Close()) })
	b.BroadcastToTeam("team-a", "server.deletion_failed", map[string]any{"server_id": "server-a", "error": "private provider error"})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	team, err := subscription.ReceiveMessage(ctx)
	require.NoError(t, err)
	staff, err := subscription.ReceiveMessage(ctx)
	require.NoError(t, err)
	var teamMessage, staffMessage Message
	require.NoError(t, json.Unmarshal([]byte(team.Payload), &teamMessage))
	require.NoError(t, json.Unmarshal([]byte(staff.Payload), &staffMessage))
	require.Equal(t, "team.team-a", teamMessage.Channel)
	require.Equal(t, broadcast.AdminResourcesChannel, staffMessage.Channel)
	require.Equal(t, map[string]any{"resource": "server", "action": "deletion_failed", "team_id": "team-a", "server_id": "server-a"}, staffMessage.Data)
}

func receiveAdminTestMessage(t *testing.T, client *Client) Message {
	t.Helper()
	select {
	case payload := <-client.Send:
		var message Message
		require.NoError(t, json.Unmarshal(payload, &message))
		return message
	case <-time.After(time.Second):
		t.Fatal("expected staff channel message")
		return Message{}
	}
}

func TestAdminDeliveryChecksTokenExpiryForEveryConnection(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Shutdown)
	auth := &staffChannelTestAuthorizer{}
	auth.allowed.Store(true)
	valid := NewClient(hub, nil, "staff-a", "team-a", auth)
	valid.staffTokenExpiresAt = time.Now().Add(time.Hour)
	expired := NewClient(hub, nil, "staff-a", "team-a", auth)
	expired.staffTokenExpiresAt = time.Now().Add(-time.Second)
	hub.Subscribe(valid, broadcast.AdminResourcesChannel)
	hub.Subscribe(expired, broadcast.AdminResourcesChannel)
	hub.BroadcastToTeam("customer-team", "server.updated", map[string]string{"server_id": "server-a"})
	require.Equal(t, broadcast.AdminResourceChanged, receiveAdminTestMessage(t, valid).Event)
	require.Equal(t, "subscription.error", receiveAdminTestMessage(t, expired).Event)
	require.Empty(t, expired.GetChannels())
}
