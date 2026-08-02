package dto

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

func TestToServiceResponseIncludesPersistedPatchDiagnostics(t *testing.T) {
	taskID := "task-1"
	service := &models.InstalledService{
		TaskID: &taskID,
		TypeData: dbtype.JSONMap{
			"patch_status": "failed",
			"patch_error":  "apt repository unavailable",
		},
	}

	response := ToServiceResponse(service)

	require.Equal(t, &taskID, response.TaskID)
	require.NotNil(t, response.PatchStatus)
	require.Equal(t, "failed", *response.PatchStatus)
	require.NotNil(t, response.PatchError)
	require.Equal(t, "apt repository unavailable", *response.PatchError)
}
