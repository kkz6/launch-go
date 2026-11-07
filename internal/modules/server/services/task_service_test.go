package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

func TestGetTaskScopesToServerAndTeam(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&models.Server{}, &models.InstalledService{}, &models.Task{}))

	for _, server := range []models.Server{
		{BaseModel: basemodels.BaseModel{ID: "server-1"}, TeamScoped: basemodels.TeamScoped{TeamID: "team-1"}},
		{BaseModel: basemodels.BaseModel{ID: "server-2"}, TeamScoped: basemodels.TeamScoped{TeamID: "team-1"}},
		{BaseModel: basemodels.BaseModel{ID: "server-3"}, TeamScoped: basemodels.TeamScoped{TeamID: "team-2"}},
	} {
		require.NoError(t, db.Create(&server).Error)
		task := models.Task{
			BaseModel:    basemodels.BaseModel{ID: "task-" + server.ID},
			ServerScoped: basemodels.ServerScoped{ServerID: server.ID},
			Name:         "Deploy site",
			Type:         "site:deploy",
			User:         "launcher",
			Output:       dbtype.EncryptedString("output for " + server.ID),
		}
		require.NoError(t, db.Create(&task).Error)
	}

	service := &Service{repos: repositories.NewRegistry(db)}
	for _, test := range []struct {
		name     string
		taskID   string
		serverID string
		teamID   string
		allowed  bool
	}{
		{name: "same server", taskID: "task-server-1", serverID: "server-1", teamID: "team-1", allowed: true},
		{name: "other server in same team", taskID: "task-server-2", serverID: "server-1", teamID: "team-1"},
		{name: "task from another team", taskID: "task-server-3", serverID: "server-1", teamID: "team-1"},
		{name: "server from another team", taskID: "task-server-3", serverID: "server-3", teamID: "team-1"},
		{name: "missing task", taskID: "missing", serverID: "server-1", teamID: "team-1"},
		{name: "missing server", taskID: "task-server-1", serverID: "missing", teamID: "team-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := service.GetTask(context.Background(), test.taskID, test.serverID, test.teamID)
			if !test.allowed {
				require.Error(t, err)
				require.True(t, fiberutil.IsNotFound(err), "expected not found, got %v", err)
				require.Equal(t, dto.TaskResponse{}, result)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.taskID, result.ID)
			require.Equal(t, test.serverID, result.ServerID)
			require.NotNil(t, result.Output)
			require.Equal(t, "output for "+test.serverID, *result.Output)
		})
	}
}
