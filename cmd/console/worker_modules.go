package main

import (
	"context"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/backup"
	databasemodule "github.com/kkz6/launch-go/internal/modules/database"
	"github.com/kkz6/launch-go/internal/modules/docker"
	"github.com/kkz6/launch-go/internal/modules/git"
	"github.com/kkz6/launch-go/internal/modules/notification"
	"github.com/kkz6/launch-go/internal/modules/platform"
	"github.com/kkz6/launch-go/internal/modules/script"
	"github.com/kkz6/launch-go/internal/modules/server"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site"
	"github.com/kkz6/launch-go/internal/modules/site/adapters"
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

	serverModule := server.NewModule(builder)
	databaseModule := databasemodule.NewModule(builder)
	dockerModule := docker.NewModule(builder)
	gitModule := git.NewModule(builder)
	siteModule := site.NewModule(builder)
	backupModule := backup.NewModule(builder)

	wireWorkerModules(serverModule, databaseModule, dockerModule, gitModule, siteModule, backupModule)

	kernel := app.NewKernel(appLogger)
	for _, module := range []app.Module{
		serverModule,
		databaseModule,
		dockerModule,
		backupModule,
		gitModule,
		siteModule,
		script.NewModule(builder),
		platform.NewModule(builder),
	} {
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

func wireWorkerModules(
	serverModule *server.Module,
	databaseModule *databasemodule.Module,
	dockerModule *docker.Module,
	gitModule *git.Module,
	siteModule *site.Module,
	backupModule *backup.Module,
) {
	taskrunner.RegisterHostKeyPersister(func(ctx context.Context, serverID, hostKey string) error {
		return serverModule.Repos().Server().UpdateHostKey(ctx, serverID, hostKey)
	})

	backupModule.SetDatabaseRepos(databaseModule.Repos())
	siteModule.SetProviderFactory(gitModule.ProviderFactory())
	dockerModule.SetProviderFactory(gitModule.ProviderFactory())
	siteModule.SetCronCreator(adapters.NewCronCreatorAdapter(serverModule.Service()))
	siteModule.SetDatabaseManager(adapters.NewDatabaseManagerAdapter(databaseModule.Service()))
}
