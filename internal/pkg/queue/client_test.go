package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientCloseHandlesNilClient(t *testing.T) {
	var client *Client
	require.NoError(t, client.Close())
}

func TestClientCloseHandlesEmptyClient(t *testing.T) {
	require.NoError(t, (&Client{}).Close())
}
