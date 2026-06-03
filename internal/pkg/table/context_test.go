package table

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActorIDRoundTrip(t *testing.T) {
	ctx := WithActorID(context.Background(), "usr00000000000000000000001")
	require.Equal(t, "usr00000000000000000000001", ActorID(ctx))
}

func TestActorIDUnsetIsEmpty(t *testing.T) {
	require.Equal(t, "", ActorID(context.Background()))
}
