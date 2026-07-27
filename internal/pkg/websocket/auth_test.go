package websocket

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateTeamMembershipFailsClosedWithoutCache(t *testing.T) {
	err := validateTeamMembership(context.Background(), "user-1", "team-1", nil)
	require.ErrorIs(t, err, ErrMembershipUnavailable)
}
