package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database"
	"github.com/kkz6/launch-go/internal/middleware"
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
	"github.com/kkz6/launch-go/internal/pkg/logger"
	"github.com/kkz6/launch-go/internal/pkg/mail"
	mailtemplates "github.com/kkz6/launch-go/internal/pkg/mail/templates"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
	"github.com/kkz6/launch-go/internal/pkg/websocket"
	"github.com/kkz6/launch-go/internal/schedule"
)

// WorkerVersion is the build-stamped version string (set via -ldflags in CI).
var WorkerVersion = "development"

func main() {
	// Register all script templates at startup
	templates.MustRegisterAll()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Email template logo / "open in app" links go to the user-facing
	// frontend, not the API.
	mailtemplates.Initialize(cfg.App.Name, cfg.App.Frontend())

	// Initialize logger
	appLogger := logger.New(cfg.App.Environment)

	// Initialize Sentry for error tracking (mirrors cmd/api). Without this the
	// worker — where provider/SSH/task failures actually surface — wouldn't
	// report anything to Sentry.
	sentryEnabled := middleware.InitSentry(cfg.Sentry, cfg.App.Name, WorkerVersion, appLogger)

	// Initialize encryption for encrypted fields
	if err := database.InitEncryption(cfg.App.Key); err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to initialize encryption")
	}

	// Initialize signed URL signer with app key and base URL
	signer := signedurl.NewSigner(cfg.App.Key).WithBaseURL(cfg.App.URL)
	signedurl.SetDefaultSigner(signer)
	// User-facing URLs (provision script, etc.) render on the frontend host;
	// Nuxt proxies them back to the API and the default signer verifies.
	signedurl.SetPublicSigner(signedurl.NewSigner(cfg.App.Key).WithBaseURL(cfg.App.Frontend()))

	// Initialize database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Initialize Redis broadcaster for cross-process WebSocket messaging
	// The worker publishes to Redis, and the API server subscribes and forwards to clients
	redisBroadcaster := websocket.NewRedisBroadcaster(cfg.Redis.Address, cfg.Redis.Password, cfg.Redis.DB, appLogger)

	// Initialize task dispatcher with SSH streaming
	taskLogger := logger.NewFileLogger(cfg.App.Debug)
	dispatcher := taskrunner.NewDispatcherWithConfig(appLogger, redisBroadcaster, &taskrunner.DispatcherConfig{
		BroadcastInterval: 2 * time.Second,
		TaskLogger:        taskLogger,
	})

	// Initialize queue client for dispatching jobs from within jobs
	queueClient := queue.NewClient(cfg.Redis)

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
				// Report task failures to Sentry with the task type as a tag so
				// they're filterable. No-op when Sentry isn't configured.
				if sentryEnabled {
					sentry.WithScope(func(scope *sentry.Scope) {
						scope.SetTag("task_type", task.Type())
						scope.SetLevel(sentry.LevelError)
						sentry.CaptureException(err)
					})
				}
			}),
		},
	)

	// Create email sender and notification module for notifier access
	emailSender := mail.NewEmailSender(cfg.Mail)
	notifBuilder := app.NewBuilder(app.Deps{
		DB:     db,
		Logger: appLogger,
		Config: cfg,
	})
	notificationModule := notification.NewModule(notifBuilder, emailSender)
	notifier := notificationModule.Notifier()

	// Create application context with all shared dependencies
	// Note: membershipCache is nil for worker since it's only needed for HTTP middleware
	ctx := app.NewContext(cfg, db, appLogger, queueClient, redisBroadcaster, dispatcher, notifier, nil)

	// Create application kernel for module registration
	kernel := app.NewKernel(appLogger)
	builder := app.NewBuilderFromContext(ctx)

	// Initialize modules
	serverModule := server.NewModule(builder)
	databaseModule := databasemodule.NewModule(builder)
	dockerModule := docker.NewModule(builder)
	gitModule := git.NewModule(builder)
	siteModule := site.NewModule(builder)

	// Worker-side host-key persister: same wiring as the API
	// process, registered so deploy/backup/site jobs that connect
	// over SSH pin the host key the first time and refuse on
	// mismatch afterwards. Boot-time registration only — no per-job
	// plumbing required downstream.
	taskrunner.RegisterHostKeyPersister(func(ctx context.Context, serverID, hostKey string) error {
		return serverModule.Repos().Server().UpdateHostKey(ctx, serverID, hostKey)
	})
	scriptModule := script.NewModule(builder)

	// Set up cross-module dependencies
	siteModule.SetProviderFactory(gitModule.ProviderFactory())
	dockerModule.SetProviderFactory(gitModule.ProviderFactory())
	siteModule.SetCronCreator(adapters.NewCronCreatorAdapter(serverModule.Service()))
	siteModule.SetDatabaseManager(adapters.NewDatabaseManagerAdapter(databaseModule.Service()))

	// Backup module needs the database module's repos so manual backup
	// runs can dump linked databases. Construct once and wire before
	// kernel registration so RegisterJobs sees a non-nil registry.
	backupModule := backup.NewModule(builder)
	backupModule.SetDatabaseRepos(databaseModule.Repos())

	// Register all modules with the kernel
	kernel.
		Register(serverModule).
		Register(databaseModule).
		Register(dockerModule).
		Register(backupModule).
		Register(gitModule).
		Register(siteModule).
		Register(scriptModule).
		Register(platform.NewModule(builder))

	// Boot task callbacks (for local mode SSH streaming callbacks)
	kernel.BootTaskCallbacks()

	// Recover orphaned tasks from previous worker session
	recoverer := servertasks.NewTaskRecoverer(db, appLogger, dispatcher, queueClient, redisBroadcaster, notifier)
	if err := recoverer.RecoverOrphanedTasks(context.Background()); err != nil {
		appLogger.Error().Err(err).Msg("Task recovery encountered errors")
	}

	// Boot all jobs through the kernel
	mux := asynq.NewServeMux()
	kernel.BootJobs(mux)

	// Initialize scheduler for periodic tasks
	scheduler := queue.NewScheduler(cfg.Redis)

	// Register scheduled tasks from centralized schedule kernel
	scheduledTasks := schedule.GetScheduledTasks()
	if err := scheduler.RegisterTasks(scheduledTasks); err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to register scheduled tasks")
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		appLogger.Info().Msg("Worker started")
		if err := srv.Run(mux); err != nil {
			appLogger.Fatal().Err(err).Msg("Failed to start worker")
		}
	}()

	go func() {
		appLogger.Info().
			Int("scheduled_tasks", len(scheduledTasks)).
			Msg("Scheduler started")
		if err := scheduler.Run(); err != nil {
			appLogger.Error().Err(err).Msg("Scheduler error")
		}
	}()

	<-quit
	appLogger.Info().Msg("Shutting down worker...")

	srv.Shutdown()
	scheduler.Shutdown()
	kernel.Shutdown()
	redisBroadcaster.Close()
	queueClient.Close()

	// Flush any buffered Sentry events before exiting; matches cmd/api.
	if sentryEnabled {
		middleware.FlushSentry(5 * time.Second)
	}

	appLogger.Info().Msg("Worker stopped")
}
