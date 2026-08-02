package services

import (
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

func TestQueueServiceStoresSitePHPBinaryInCommand(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   gormlogger.Default.LogMode(gormlogger.Silent),
		},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Site{}, &models.Queue{}))

	phpVersion := sitetypes.PhpVersion84
	site := &models.Site{
		BaseModel:  basemodels.BaseModel{ID: "site-1"},
		Address:    "example.test",
		Type:       sitetypes.SiteTypeLaravel,
		TLSSetting: sitetypes.TLSSettingAuto,
		User:       "launch",
		Path:       "/home/launch/example.test",
		WebFolder:  "public",
		PhpVersion: &phpVersion,
	}
	site.ServerID = "server-1"
	site.TeamID = "team-1"
	site.UserID = "user-1"
	require.NoError(t, db.Create(site).Error)

	logger := zerolog.Nop()
	deps := &ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: service.Dependencies{
				DB:     db,
				Logger: &logger,
			},
			Repos: repositories.NewRegistry(db),
		},
	}
	queueService := NewQueueService(deps)
	_, err = queueService.Create(
		context.Background(),
		site.ID,
		site.ServerID,
		site.TeamID,
		"user-1",
		&dto.CreateQueueRequest{
			QueueConnection:       "redis",
			Queue:                 "default",
			RestSecondsOnEmpty:    3,
			MaxSecondsPerJob:      60,
			FailedJobDelaySeconds: 3,
		},
	)
	require.NoError(t, err)

	var queue models.Queue
	require.NoError(t, db.First(&queue).Error)
	require.Equal(
		t,
		"php8.4 /home/launch/example.test/repository/artisan queue:work",
		queue.Command,
	)
}
