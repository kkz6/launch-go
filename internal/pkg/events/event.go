// Package events provides a small typed domain-event envelope and dispatcher.
//
// Events are transport-agnostic: modules may enqueue the envelope for durable
// asynchronous delivery, then dispatch it to independently registered
// listeners inside a worker.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Event is the durable wire format for a domain event.
type Event struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

// New creates a domain event with a JSON-encoded typed payload.
func New(id, name string, payload any) (Event, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" {
		return Event{}, errors.New("event id is required")
	}
	if name == "" {
		return Event{}, errors.New("event name is required")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event %q payload: %w", name, err)
	}

	return Event{
		ID:         id,
		Name:       name,
		OccurredAt: time.Now().UTC(),
		Payload:    data,
	}, nil
}

// Decode unmarshals an event payload into its concrete type.
func Decode[T any](event Event) (T, error) {
	var payload T
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return payload, fmt.Errorf("decode event %q payload: %w", event.Name, err)
	}
	return payload, nil
}

// Listener handles one domain event type. Name must be stable because it is
// also suitable for idempotency keys when a listener enqueues follow-up work.
type Listener interface {
	Name() string
	Handle(ctx context.Context, event Event) error
}

// Dispatcher fans an event out to every registered listener.
type Dispatcher struct {
	mu        sync.RWMutex
	listeners map[string][]Listener
}

// NewDispatcher creates an empty dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{listeners: make(map[string][]Listener)}
}

// Subscribe registers one listener for an event name.
func (d *Dispatcher) Subscribe(eventName string, listener Listener) error {
	eventName = strings.TrimSpace(eventName)
	if eventName == "" {
		return errors.New("event name is required")
	}
	if listener == nil || strings.TrimSpace(listener.Name()) == "" {
		return errors.New("listener name is required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	for _, registered := range d.listeners[eventName] {
		if registered.Name() == listener.Name() {
			return fmt.Errorf("listener %q already registered for event %q", listener.Name(), eventName)
		}
	}
	d.listeners[eventName] = append(d.listeners[eventName], listener)
	return nil
}

// MustSubscribe registers a listener and panics on invalid static wiring.
func (d *Dispatcher) MustSubscribe(eventName string, listener Listener) {
	if err := d.Subscribe(eventName, listener); err != nil {
		panic(err)
	}
}

// Dispatch invokes every matching listener even when an earlier listener
// fails. The joined error lets the worker retry fan-out while listener-specific
// idempotency keys prevent duplicate follow-up work.
func (d *Dispatcher) Dispatch(ctx context.Context, event Event) error {
	d.mu.RLock()
	listeners := append([]Listener(nil), d.listeners[event.Name]...)
	d.mu.RUnlock()

	var dispatchErrors []error
	for _, listener := range listeners {
		if err := invokeListener(ctx, listener, event); err != nil {
			dispatchErrors = append(dispatchErrors, fmt.Errorf(
				"event %q listener %q: %w",
				event.Name,
				listener.Name(),
				err,
			))
		}
	}
	return errors.Join(dispatchErrors...)
}

func invokeListener(ctx context.Context, listener Listener, event Event) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
		}
	}()
	return listener.Handle(ctx, event)
}
