package dto

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/stretchr/testify/require"
)

func TestTeamDetailsRespectInvitationAndRemovalPermissions(t *testing.T) {
	team := &models.Team{UserID: "owner"}
	admin, editor, member := "admin", "editor", "member"
	members := []models.TeamMember{
		{UserID: admin, Role: &admin},
		{UserID: editor, Role: &editor},
		{UserID: member, Role: &member},
	}
	invitations := []models.TeamInvitation{{Email: "pending@example.test"}}

	for _, userID := range []string{"owner", admin, editor, member} {
		t.Run(userID, func(t *testing.T) {
			response := ToTeamDetailResponse(team, members, invitations, userID)
			require.Equal(t, userID == "owner", response.IsOwner)
			require.Equal(t, userID == "owner", response.Permissions.CanRemoveTeamMembers)
			if userID == "owner" || userID == admin {
				require.Len(t, response.Invitations, 1)
			} else {
				require.Empty(t, response.Invitations)
			}
		})
	}
}
