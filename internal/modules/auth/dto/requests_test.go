package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTeamRequestNormalization(t *testing.T) {
	create := &CreateTeamRequest{Name: "  New team  "}
	update := &UpdateTeamRequest{Name: "  Renamed team  "}

	create.Normalize()
	update.Normalize()

	require.Equal(t, "New team", create.Name)
	require.Equal(t, "Renamed team", update.Name)
}
