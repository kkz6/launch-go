package backup

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	gofiber "github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/pkg/app"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func TestRegisterRoutesIncludesManualBackupRun(t *testing.T) {
	logger := zerolog.Nop()
	module := NewModule(app.NewBuilder(app.Deps{Logger: &logger}))
	router := gofiber.New()
	module.RegisterRoutes(router, func(c *gofiber.Ctx) error {
		return c.Next()
	})

	found := false
	for _, route := range router.GetRoutes() {
		if route.Method == gofiber.MethodPost && route.Path == "/servers/:serverId/backups/:id/run" {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestRunBackupHandler(t *testing.T) {
	tests := []struct {
		name       string
		withUser   bool
		run        runBackupFunc
		wantStatus int
	}{
		{
			name:       "requires request identity",
			wantStatus: gofiber.StatusUnauthorized,
		},
		{
			name:     "returns service failure",
			withUser: true,
			run: func(context.Context, string, string, string, string) (dto.BackupJobResponse, error) {
				return dto.BackupJobResponse{}, errors.New("dispatch failed")
			},
			wantStatus: gofiber.StatusInternalServerError,
		},
		{
			name:     "returns created job",
			withUser: true,
			run: func(_ context.Context, id, serverID, teamID, userID string) (dto.BackupJobResponse, error) {
				require.Equal(t, "backup-1", id)
				require.Equal(t, "server-1", serverID)
				require.Equal(t, "team-1", teamID)
				require.Equal(t, "user-1", userID)
				return dto.BackupJobResponse{ID: "job-1"}, nil
			},
			wantStatus: gofiber.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gofiber.New()
			if tt.withUser {
				app.Use(func(c *gofiber.Ctx) error {
					c.Locals(fiberutil.KeyTeamID, "team-1")
					c.Locals(fiberutil.KeyUserID, "user-1")
					return c.Next()
				})
			}
			run := tt.run
			if run == nil {
				run = func(context.Context, string, string, string, string) (dto.BackupJobResponse, error) {
					t.Fatal("backup service must not be called")
					return dto.BackupJobResponse{}, nil
				}
			}
			app.Post("/servers/:serverId/backups/:id/run", runBackupHandler(run))

			response, err := app.Test(httptest.NewRequest(
				gofiber.MethodPost,
				"/servers/server-1/backups/backup-1/run",
				nil,
			))

			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, response.StatusCode)
		})
	}
}
