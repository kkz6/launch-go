package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
)

type staffLiveEvents struct {
	broadcast.NopBroadcaster
	changes []broadcast.AdminResourceChange
}

func (b *staffLiveEvents) Broadcast(channel, event string, data any) {
	if channel == broadcast.AdminResourcesChannel && event == broadcast.AdminResourceChanged {
		b.changes = append(b.changes, data.(broadcast.AdminResourceChange))
	}
}

func TestStaffUserMutationsPublishOnlyAfterPersistence(t *testing.T) {
	svc, _ := staffRoleFixture(t)
	events := &staffLiveEvents{}
	svc.SetBroadcaster(events)
	ctx := context.Background()
	_, err := svc.UpdateStaffRole(ctx, "admin-a", "admin-a", nil)
	require.Error(t, err)
	require.Empty(t, events.changes)
	_, err = svc.UpdateStaffRole(ctx, "admin-a", "target", nil)
	require.NoError(t, err)
	require.NoError(t, svc.SetUserStatus(ctx, "admin-a", "target", authtypes.UserStatusSuspended))
	require.NoError(t, svc.DeleteUser(ctx, "admin-a", "target"))
	require.Equal(t, []broadcast.AdminResourceChange{
		{Resource: "user", Action: "staff_role_updated", UserID: "target"},
		{Resource: "user", Action: "updated", UserID: "target"},
		{Resource: "user", Action: "deleted", UserID: "target"},
	}, events.changes)
}

func TestInvitationPersistedBeforeEmailFailureStillInvalidates(t *testing.T) {
	svc, _, sender := setupInvitationService(t)
	events := &staffLiveEvents{}
	svc.SetBroadcaster(events)
	sender.shouldFail = true
	invite, err := svc.InviteUser(context.Background(), "invitee@example.test", "hobby", time.Now().Add(24*time.Hour), "admin-a")
	require.Error(t, err)
	require.NotNil(t, invite)
	require.Equal(t, []broadcast.AdminResourceChange{{Resource: "invitation", Action: "created"}}, events.changes)
	require.NoError(t, svc.RevokeInvitation(context.Background(), invite.ID))
	require.Equal(t, broadcast.AdminResourceChange{Resource: "invitation", Action: "deleted"}, events.changes[1])
}

func TestAcceptedTrialPublishesAfterCommitAndNotOnRejectedRetry(t *testing.T) {
	svc, db := setupTrialService(t)
	events := &staffLiveEvents{}
	svc.SetBroadcaster(events)
	invite := seedInvitation(t, db, nil)
	require.NoError(t, svc.AcceptPlatformInviteWithTrial(context.Background(), invite.Token, "team-a"))
	require.Equal(t, []broadcast.AdminResourceChange{
		{Resource: "invitation", Action: "accepted", TeamID: "team-a"},
		{Resource: "subscription", Action: "created", TeamID: "team-a"},
	}, events.changes)
	require.Error(t, svc.AcceptPlatformInviteWithTrial(context.Background(), invite.Token, "team-a"))
	require.Len(t, events.changes, 2)
}
