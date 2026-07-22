package main

import (
	"context"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/bootstrap/tasktemplates"
	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database"
	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/logger"
	mailtemplates "github.com/kkz6/launch-go/internal/pkg/mail/templates"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/websocket"
	"github.com/kkz6/launch-go/internal/schedule"
)

type workerRuntime struct {
	logger             *zerolog.Logger
	sentryEnabled      bool
	server             *asynq.Server
	scheduler          *queue.Scheduler
	kernel             *app.Kernel
	broadcaster        *websocket.RedisBroadcaster
	queueClient        *queue.Client
	scheduledTaskCount int
}

func bootstrapWorker() (*workerRuntime, error) {
	tasktemplates.MustRegisterAll()

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	mailtemplates.Initialize(cfg.App.Name, cfg.App.Frontend())

	appLogger := logger.New(cfg.App.Environment)
	sentryEnabled := middleware.InitSentry(cfg.Sentry, cfg.App.Name, Version, appLogger)
	if err := database.InitEncryption(cfg.App.Key); err != nil {
		return nil, fmt.Errorf("initialize encryption: %w", err)
	}

	configureWorkerSigners(cfg)
	db, err := database.Connect(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	broadcaster := websocket.NewRedisBroadcaster(cfg.Redis.Address, cfg.Redis.Password, cfg.Redis.DB, appLogger)
	queueClient := queue.NewClient(cfg.Redis)
	dispatcher := taskrunner.NewDispatcherWithConfig(appLogger, broadcaster, &taskrunner.DispatcherConfig{
		BroadcastInterval: 2 * time.Second,
		TaskLogger:        logger.NewFileLogger(cfg.App.Debug),
	})

	kernel := newWorkerKernel(cfg, db, appLogger, queueClient, broadcaster, dispatcher)

	scheduledTasks := schedule.GetScheduledTasks()
	scheduler := queue.NewScheduler(cfg.Redis)
	if err := scheduler.RegisterTasks(scheduledTasks); err != nil {
		kernel.Shutdown()
		broadcaster.Close()
		queueClient.Close()
		return nil, fmt.Errorf("register scheduled tasks: %w", err)
	}

	return &workerRuntime{
		logger:             appLogger,
		sentryEnabled:      sentryEnabled,
		server:             newAsynqServer(cfg, appLogger, sentryEnabled),
		scheduler:          scheduler,
		kernel:             kernel,
		broadcaster:        broadcaster,
		queueClient:        queueClient,
		scheduledTaskCount: len(scheduledTasks),
	}, nil
}

func configureWorkerSigners(cfg *config.Config) {
	signedurl.SetDefaultSigner(signedurl.NewSigner(cfg.App.Key).WithBaseURL(cfg.App.URL))
	signedurl.SetPublicSigner(signedurl.NewSigner(cfg.App.Key).WithBaseURL(cfg.App.Frontend()))
}

func newAsynqServer(cfg *config.Config, appLogger *zerolog.Logger, sentryEnabled bool) *asynq.Server {
	return asynq.NewServer(asynq.RedisClientOpt{Addr: cfg.Redis.Address, Password: cfg.Redis.Password, DB: cfg.Redis.DB}, asynq.Config{
		Concurrency: cfg.Queue.Concurrency,
		Queues:      map[string]int{"critical": 6, "default": 3, "low": 1},
		ErrorHandler: asynq.ErrorHandlerFunc(func(_ context.Context, task *asynq.Task, err error) {
			appLogger.Error().Str("task", task.Type()).Err(err).Msg("Task failed")
			if sentryEnabled {
				sentry.WithScope(func(scope *sentry.Scope) {
					scope.SetTag("task_type", task.Type())
					scope.SetLevel(sentry.LevelError)
					sentry.CaptureException(err)
				})
			}
		}),
	})
}
