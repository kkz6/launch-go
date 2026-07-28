package events

import (
	"context"
	"errors"
	"testing"
	"time"

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

	before := time.Now().UTC()
	event, err := New(" deployment-1 ", " deployment.succeeded ", payload{SiteID: "site-1"})
	require.NoError(t, err)
	require.Equal(t, "deployment-1", event.ID)
	require.Equal(t, "deployment.succeeded", event.Name)
	require.False(t, event.OccurredAt.Before(before))

	decoded, err := Decode[payload](event)
	require.NoError(t, err)
	require.Equal(t, "site-1", decoded.SiteID)
}

func TestNewRejectsInvalidEvents(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		eventName   string
		payload     any
		expectedErr string
	}{
		{
			name:        "missing id",
			id:          " ",
			eventName:   "deployment.succeeded",
			expectedErr: "event id is required",
		},
		{
			name:        "missing name",
			id:          "deployment-1",
			eventName:   " ",
			expectedErr: "event name is required",
		},
		{
			name:        "payload cannot be marshaled",
			id:          "deployment-1",
			eventName:   "deployment.succeeded",
			payload:     make(chan struct{}),
			expectedErr: `marshal event "deployment.succeeded" payload`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.id, test.eventName, test.payload)
			require.ErrorContains(t, err, test.expectedErr)
		})
	}
}

func TestDecodeReportsInvalidPayload(t *testing.T) {
	event := Event{
		Name:    "deployment.succeeded",
		Payload: []byte(`{"site_id":`),
	}

	_, err := Decode[map[string]string](event)
	require.ErrorContains(t, err, `decode event "deployment.succeeded" payload`)
}

func TestSubscribeRejectsInvalidListenerConfiguration(t *testing.T) {
	dispatcher := NewDispatcher()

	require.ErrorContains(
		t,
		dispatcher.Subscribe(" ", &recordingListener{name: "listener"}),
		"event name is required",
	)
	require.ErrorContains(
		t,
		dispatcher.Subscribe("deployment.succeeded", nil),
		"listener name is required",
	)
	require.ErrorContains(
		t,
		dispatcher.Subscribe("deployment.succeeded", &recordingListener{name: " "}),
		"listener name is required",
	)
}

func TestMustSubscribePanicsForInvalidWiring(t *testing.T) {
	dispatcher := NewDispatcher()

	require.PanicsWithError(t, "event name is required", func() {
		dispatcher.MustSubscribe("", &recordingListener{name: "listener"})
	})
}

func TestDispatchWithoutListenersSucceeds(t *testing.T) {
	dispatcher := NewDispatcher()

	require.NoError(t, dispatcher.Dispatch(context.Background(), Event{Name: "unregistered"}))
}
