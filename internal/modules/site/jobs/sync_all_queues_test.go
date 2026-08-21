package jobs

import (
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

func TestSyncAllQueuesHandlesEmptyAndEligibleSites(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   logger.Default.LogMode(logger.Silent),
		},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&servermodels.Server{}, &sitemodels.Queue{}))

	log := zerolog.Nop()
	job := &SyncAllQueuesJob{Deps: &JobDeps{Deps: &pkgjobs.Deps{DB: db, Logger: &log}}}
	require.NoError(t, job.Handle(context.Background()))

	server := &servermodels.Server{BaseModel: basemodels.BaseModel{ID: "server-1"}, Name: "Production", Connected: true}
	server.TeamID = "team-1"
	server.UserID = "user-1"
	require.NoError(t, db.Create(server).Error)
	queue := &sitemodels.Queue{BaseModel: basemodels.BaseModel{ID: "queue-1"}, QueueName: "default"}
	queue.SiteID = "site-1"
	queue.ServerID = server.ID
	queue.TeamID = server.TeamID
	queue.UserID = server.UserID
	require.NoError(t, db.Create(queue).Error)

	require.NoError(t, job.Handle(context.Background()))
}

func TestSyncAllQueuesConstructors(t *testing.T) {
	handler := NewSyncAllQueuesJob(SyncAllQueuesPayload{})
	require.IsType(t, &SyncAllQueuesJob{}, handler)

	task, err := NewSyncAllQueuesTask()
	require.NoError(t, err)
	require.Equal(t, TypeSyncAllQueues, task.Type())
}
