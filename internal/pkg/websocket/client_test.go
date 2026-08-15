package websocket

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	fastwebsocket "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
)

type websocketTestCache struct{}

func (websocketTestCache) Get(context.Context, string) (string, error) {
	return `{"team_id":"team-a","user_id":"user-a","role":"owner","is_member":true}`, nil
}

func (websocketTestCache) Set(context.Context, string, string, time.Duration) error { return nil }
func (websocketTestCache) Delete(context.Context, string) error                     { return nil }
func (websocketTestCache) Exists(context.Context, string) (bool, error)             { return true, nil }

type websocketTestAuthorizer struct {
	mu       sync.Mutex
	channels []string
}

func (a *websocketTestAuthorizer) AuthorizeChannel(_, _, channel string) bool {
	a.mu.Lock()
	a.channels = append(a.channels, channel)
	a.mu.Unlock()
	return channel == "server.srv-a"
}

func TestClientSubscriptionProtocol(t *testing.T) {
	const jwtSecret = "websocket-test-secret"

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-a",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signedToken, err := token.SignedString([]byte(jwtSecret))
	require.NoError(t, err)

	hub := NewHub()
	go hub.Run()

	authorizer := &websocketTestAuthorizer{}
	membershipCache := launchcache.NewTeamMembershipCache(websocketTestCache{}, nil)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/ws", Handler(hub, jwtSecret, membershipCache, authorizer))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- app.Listener(listener)
	}()
	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		select {
		case err := <-serveErr:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Error("websocket test server did not stop")
		}
	})
	t.Cleanup(hub.Shutdown)

	endpoint := url.URL{
		Scheme: "ws",
		Host:   listener.Addr().String(),
		Path:   "/ws",
		RawQuery: url.Values{
			"token":   []string{signedToken},
			"team_id": []string{"team-a"},
		}.Encode(),
	}
	conn, _, err := fastwebsocket.DefaultDialer.Dial(endpoint.String(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))

	require.NoError(t, conn.WriteJSON(map[string]string{
		"action":  "subscribe",
		"channel": "  server.srv-a  ",
	}))
	var response Message
	require.NoError(t, conn.ReadJSON(&response))
	require.Equal(t, "subscription.succeeded", response.Event)
	require.Equal(t, "server.srv-a", response.Channel)

	require.NoError(t, conn.WriteJSON(map[string]string{
		"action":  "subscribe",
		"channel": "server.srv-b",
	}))
	require.NoError(t, conn.ReadJSON(&response))
	require.Equal(t, "subscription.error", response.Event)
	require.Equal(t, "server.srv-b", response.Channel)
	data, err := json.Marshal(response.Data)
	require.NoError(t, err)
	require.JSONEq(t, `{"message":"channel access denied"}`, string(data))

	require.NoError(t, conn.WriteJSON(map[string]string{
		"action":  "unsubscribe",
		"channel": "  server.srv-a  ",
	}))
	require.Eventually(t, func() bool {
		authorizer.mu.Lock()
		defer authorizer.mu.Unlock()
		return len(authorizer.channels) == 2
	}, time.Second, 10*time.Millisecond)
}

func TestClientRejectsSubscriptionWithoutAuthorizer(t *testing.T) {
	client := NewClient(NewHub(), nil, "user-a", "team-a")
	client.sendProtocolMessage("subscription.error", "server.srv-a", map[string]string{
		"message": "channel access denied",
	})

	select {
	case payload := <-client.Send:
		var message Message
		require.NoError(t, json.Unmarshal(payload, &message))
		require.Equal(t, "subscription.error", message.Event)
	case <-time.After(time.Second):
		t.Fatal("expected protocol message")
	}
}

func TestClientCloseIsIdempotentAndStopsSending(t *testing.T) {
	client := &Client{Send: make(chan []byte, 1)}

	require.True(t, client.SafeSend([]byte("event")))
	client.Close()
	client.Close()

	require.True(t, client.IsClosing())
	require.False(t, client.SafeSend([]byte("late event")))
}

func TestClientCloseAndSafeSendAreConcurrentSafe(t *testing.T) {
	client := &Client{Send: make(chan []byte, 256)}
	var workers sync.WaitGroup

	for range 20 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 100 {
				client.SafeSend([]byte("event"))
			}
		}()
	}
	client.Close()
	workers.Wait()
}

func TestClientSafeSendReturnsFalseWhenQueueIsFull(t *testing.T) {
	client := NewClient(NewHub(), nil, "user-a", "team-a")
	for i := 0; i < cap(client.Send); i++ {
		require.True(t, client.SafeSend([]byte("queued")))
	}
	require.False(t, client.SafeSend([]byte("full")))
}
