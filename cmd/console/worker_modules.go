package main

import (
	"context"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/bootstrap/modules"
	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/notification"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/mail"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/websocket"
)

func newWorkerKernel(
	cfg *config.Config,
	db *gorm.DB,
	appLogger *zerolog.Logger,
	queueClient *queue.Client,
	broadcaster *websocket.RedisBroadcaster,
	dispatcher *taskrunner.Dispatcher,
) *app.Kernel {
	builder, notifier := newWorkerModuleBuilder(cfg, db, appLogger, queueClient, broadcaster, dispatcher)

	kernel := app.NewKernel(appLogger)
	for _, module := range modules.NewWorkloads(builder).All() {
		kernel.Register(module)
	}
	kernel.BootTaskCallbacks()

	recoverer := servertasks.NewTaskRecoverer(db, appLogger, dispatcher, queueClient, broadcaster, notifier)
	if err := recoverer.RecoverOrphanedTasks(context.Background()); err != nil {
		appLogger.Error().Err(err).Msg("Task recovery encountered errors")
	}

	return kernel
}

func newWorkerModuleBuilder(
	cfg *config.Config,
	db *gorm.DB,
	appLogger *zerolog.Logger,
	queueClient *queue.Client,
	broadcaster *websocket.RedisBroadcaster,
	dispatcher *taskrunner.Dispatcher,
) (*app.Builder, taskrunner.NotifierService) {
	builder := app.NewBuilder(app.Deps{
		Config:     cfg,
		DB:         db,
		Logger:     appLogger,
		Queue:      queueClient,
		WebSocket:  broadcaster,
		Dispatcher: dispatcher,
	})
	notificationModule := notification.NewModule(builder, mail.NewEmailSender(cfg.Mail))
	notifier := notificationModule.Notifier()
	return builder.WithNotifier(notifier), notifier
}
