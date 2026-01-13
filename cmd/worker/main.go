package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database"
	databasemodule "github.com/kkz6/launch-go/internal/modules/database"
	"github.com/kkz6/launch-go/internal/modules/server"
	"github.com/kkz6/launch-go/internal/modules/site"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/logger"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/websocket"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger := logger.New(cfg.App.Environment)

	// Initialize encryption for encrypted fields
	if err := database.InitEncryption(cfg.App.Key); err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to initialize encryption")
	}

	// Initialize database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Initialize WebSocket hub for broadcasting job progress
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Initialize task dispatcher with local mode support
	dispatcher := taskrunner.NewDispatcherWithConfig(appLogger, wsHub, &taskrunner.DispatcherConfig{
		LocalMode:         cfg.App.IsLocal(),
		BroadcastInterval: 2 * time.Second,
	})

	if cfg.App.IsLocal() {
		appLogger.Info().Msg("Running in local mode - using SSH streaming for task output")
	}

	// Create Asynq server
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.Redis.Address,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		},
		asynq.Config{
			Concurrency: cfg.Queue.Concurrency,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				appLogger.Error().
					Str("task", task.Type()).
					Err(err).
					Msg("Task failed")
			}),
		},
	)

	// Create application context with all shared dependencies
	ctx := app.NewContext(cfg, db, appLogger, nil, wsHub, dispatcher)

	// Create application kernel for module registration
	kernel := app.NewKernel(appLogger)

	// Register all modules with the kernel
	kernel.
		Register(server.NewModuleFromContext(ctx)).
		Register(databasemodule.NewModuleFromContext(ctx)).
		Register(site.NewModuleFromContext(ctx))

	// Boot all jobs through the kernel
	mux := asynq.NewServeMux()
	kernel.BootJobs(mux)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		appLogger.Info().Msg("Worker started")
		if err := srv.Run(mux); err != nil {
			appLogger.Fatal().Err(err).Msg("Failed to start worker")
		}
	}()

	<-quit
	appLogger.Info().Msg("Shutting down worker...")

	srv.Shutdown()
	kernel.Shutdown()
	wsHub.Shutdown()

	appLogger.Info().Msg("Worker stopped")
}
