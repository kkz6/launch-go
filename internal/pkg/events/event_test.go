package events

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordingListener struct {
	name        string
	calls       int
	err         error
	shouldPanic bool
}

func (l *recordingListener) Name() string { return l.name }

func (l *recordingListener) Handle(_ context.Context, _ Event) error {
	l.calls++
	if l.shouldPanic {
		panic("listener panic")
	}
	return l.err
}

func TestDispatcherIsolatesListenerPanics(t *testing.T) {
	dispatcher := NewDispatcher()
	first := &recordingListener{name: "first", shouldPanic: true}
	second := &recordingListener{name: "second"}
	dispatcher.MustSubscribe("deployment.succeeded", first)
	dispatcher.MustSubscribe("deployment.succeeded", second)

	event, err := New("deployment-1", "deployment.succeeded", nil)
	require.NoError(t, err)

	err = dispatcher.Dispatch(context.Background(), event)
	require.ErrorContains(t, err, "listener panic")
	require.Equal(t, 1, first.calls)
	require.Equal(t, 1, second.calls)
}

func TestDispatcherRunsEveryListenerAndJoinsErrors(t *testing.T) {
	dispatcher := NewDispatcher()
	first := &recordingListener{name: "first", err: errors.New("first failed")}
	second := &recordingListener{name: "second"}
	dispatcher.MustSubscribe("deployment.succeeded", first)
	dispatcher.MustSubscribe("deployment.succeeded", second)

	event, err := New("deployment-1", "deployment.succeeded", map[string]string{"site_id": "site-1"})
	require.NoError(t, err)

	err = dispatcher.Dispatch(context.Background(), event)
	require.ErrorContains(t, err, `listener "first"`)
	require.Equal(t, 1, first.calls)
	require.Equal(t, 1, second.calls)
}

func TestDispatcherRejectsDuplicateListenerNames(t *testing.T) {
	dispatcher := NewDispatcher()
	dispatcher.MustSubscribe("deployment.succeeded", &recordingListener{name: "restart-queues"})

	err := dispatcher.Subscribe("deployment.succeeded", &recordingListener{name: "restart-queues"})
	require.ErrorContains(t, err, "already registered")
}

func TestEventTypedPayloadRoundTrip(t *testing.T) {
	type payload struct {
		SiteID string `json:"site_id"`
	}

	event, err := New("deployment-1", "deployment.succeeded", payload{SiteID: "site-1"})
	require.NoError(t, err)

	decoded, err := Decode[payload](event)
	require.NoError(t, err)
	require.Equal(t, "site-1", decoded.SiteID)
}
