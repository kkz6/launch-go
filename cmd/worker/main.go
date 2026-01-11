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
	serverjobs "github.com/kkz6/launch-go/internal/modules/server/jobs"
	sitejobs "github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/pkg/logger"
	"github.com/kkz6/launch-go/internal/taskrunner"
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

	// Initialize database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Initialize WebSocket hub for broadcasting job progress
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Initialize task dispatcher with local mode support
	// In local mode, tasks use SSH streaming instead of HTTP callbacks
	dispatcher := taskrunner.NewDispatcherWithConfig(appLogger, wsHub, &taskrunner.DispatcherConfig{
		LocalMode:         cfg.App.IsLocal(),
		BroadcastInterval: 2 * time.Second,
	})

	if cfg.App.IsLocal() {
		appLogger.Info().Msg("Running in local mode - using SSH streaming for task output")
	}

	// Make dispatcher available for job handlers (can be used via dependency injection)
	_ = dispatcher

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

	// Create job registries
	serverJobRegistry := serverjobs.NewRegistry(db, wsHub, appLogger)
	siteJobHandler := sitejobs.NewHandler(db, wsHub, appLogger)

	// Register handlers
	mux := asynq.NewServeMux()

	// Server jobs - use registry pattern
	serverJobRegistry.RegisterHandlers(mux)

	// Site jobs
	mux.HandleFunc(sitejobs.TypeDeploy, siteJobHandler.HandleDeploy)
	mux.HandleFunc(sitejobs.TypeDeployZeroDowntime, siteJobHandler.HandleDeployZeroDowntime)
	mux.HandleFunc(sitejobs.TypeRollback, siteJobHandler.HandleRollback)
	mux.HandleFunc(sitejobs.TypeInstallSSL, siteJobHandler.HandleInstallSSL)

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
	wsHub.Shutdown()

	_ = db

	appLogger.Info().Msg("Worker stopped")
}
