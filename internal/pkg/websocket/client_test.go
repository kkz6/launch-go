package websocket

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientCloseIsIdempotentAndStopsSending(t *testing.T) {
	client := &Client{Send: make(chan []byte, 1)}

	assert.True(t, client.SafeSend([]byte("event")))
	client.Close()
	client.Close()

	assert.True(t, client.IsClosing())
	assert.False(t, client.SafeSend([]byte("late event")))
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
