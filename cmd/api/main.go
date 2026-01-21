package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database"
	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/auth"
	"github.com/kkz6/launch-go/internal/modules/backup"
	"github.com/kkz6/launch-go/internal/modules/billing"
	"github.com/kkz6/launch-go/internal/modules/dashboard"
	databasemodule "github.com/kkz6/launch-go/internal/modules/database"
	"github.com/kkz6/launch-go/internal/modules/dns"
	"github.com/kkz6/launch-go/internal/modules/git"
	"github.com/kkz6/launch-go/internal/modules/notification"
	"github.com/kkz6/launch-go/internal/modules/script"
	"github.com/kkz6/launch-go/internal/modules/server"
	"github.com/kkz6/launch-go/internal/modules/site"
	wsmodule "github.com/kkz6/launch-go/internal/modules/websocket"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	"github.com/kkz6/launch-go/internal/pkg/health"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/logger"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/websocket"
)

// Application holds all application dependencies
type Application struct {
	config          *config.Config
	logger          *zerolog.Logger
	db              *gorm.DB
	queueClient     *queue.Client
	wsHub           *websocket.Hub
	wsSubscriber    *websocket.RedisSubscriber
	dispatcher      *taskrunner.Dispatcher
	fiber           *fiber.App
	kernel          *app.Kernel
	redisCache      *cache.RedisCache
	membershipCache *launchcache.TeamMembershipCache
	healthChecker   *health.Aggregator
	sentryEnabled   bool
}

func main() {
	application := bootstrap()
	application.registerMiddleware()
	application.registerModules()
	application.run()
}

// Version can be set via ldflags during build
var Version = "development"

// bootstrap initializes all application dependencies
func bootstrap() *Application {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	appLogger := logger.NewWithConfig(cfg.App.Environment, cfg.App.Debug)

	// Initialize Sentry for error tracking (only in production)
	sentryEnabled := middleware.InitSentry(cfg.Sentry, cfg.App.Name, Version, appLogger)

	if err := database.InitEncryption(cfg.App.Key); err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to initialize encryption")
	}

	// Initialize signed URL signer with app key and base URL
	signer := signedurl.NewSigner(cfg.App.Key).WithBaseURL(cfg.App.URL)
	signedurl.SetDefaultSigner(signer)

	db, err := database.ConnectWithLogger(cfg.Database, appLogger)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to connect to database")
	}

	queueClient := queue.NewClient(cfg.Redis)

	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Start Redis subscriber to receive broadcasts from the worker process
	wsSubscriber := websocket.NewRedisSubscriber(cfg.Redis.Address, cfg.Redis.Password, cfg.Redis.DB, wsHub, appLogger)
	wsSubscriber.Start()

	dispatcher := taskrunner.NewDispatcher(appLogger, wsHub)

	// Initialize Redis cache for team membership
	redisCache := cache.NewRedisCache(cfg.Redis)
	membershipCache := launchcache.NewTeamMembershipCache(redisCache, db)

	// Initialize health check aggregator
	healthChecker := health.NewAggregator().
		Add(&health.DatabaseChecker{DB: db}).
		Add(&health.RedisChecker{Client: redisCache.Client()})

	fiberApp := fiber.New(fiber.Config{
		AppName:      "Launch API",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		ErrorHandler: middleware.ErrorHandler,
	})

	return &Application{
		config:          cfg,
		logger:          appLogger,
		db:              db,
		queueClient:     queueClient,
		wsHub:           wsHub,
		wsSubscriber:    wsSubscriber,
		dispatcher:      dispatcher,
		fiber:           fiberApp,
		redisCache:      redisCache,
		membershipCache: membershipCache,
		healthChecker:   healthChecker,
		sentryEnabled:   sentryEnabled,
	}
}

// registerMiddleware sets up global middleware
func (a *Application) registerMiddleware() {
	// Sentry must be first to capture all panics and errors
	if a.sentryEnabled {
		a.fiber.Use(middleware.SentryHandler())
		a.fiber.Use(middleware.EnhanceSentryScope)
	}

	a.fiber.Use(recover.New())
	a.fiber.Use(cors.New(cors.Config{
		AllowOrigins:     a.config.Cors.AllowedOrigins,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Team-ID",
		AllowCredentials: true,
	}))
	a.fiber.Use(middleware.RequestLogger(a.logger))
}

// registerModules sets up all application modules using the kernel
func (a *Application) registerModules() {
	// Create application context with all shared dependencies
	ctx := app.NewContext(
		a.config,
		a.db,
		a.logger,
		a.queueClient,
		a.wsHub,
		a.dispatcher,
		a.membershipCache,
	)

	// Create application kernel
	a.kernel = app.NewKernel(a.logger)

	// Create module builder
	builder := app.NewBuilderFromContext(ctx)

	// Create modules using builder pattern
	authModule := auth.NewModule(builder)
	serverModule := server.NewModule(builder)
	databaseModule := databasemodule.NewModule(builder)
	siteModule := site.NewModule(builder)
	dnsModule := dns.NewModule(builder)
	backupModule := backup.NewModule(builder)
	billingModule := billing.NewModule(builder)
	gitModule := git.NewModule(builder)
	notificationModule := notification.NewModule(builder)
	scriptModule := script.NewModule(builder)
	dashboardModule := dashboard.NewModule(builder)
	wsModule := wsmodule.NewModule(builder)

	// Wire cross-module dependencies
	siteModule.SetDomainRepository(dnsModule.Repos().Domain())
	siteModule.SetProviderFactory(gitModule.ProviderFactory())

	// Register all modules with the kernel
	a.kernel.
		Register(authModule).
		Register(serverModule).
		Register(databaseModule).
		Register(siteModule).
		Register(dnsModule).
		Register(backupModule).
		Register(billingModule).
		Register(gitModule).
		Register(notificationModule).
		Register(scriptModule).
		Register(dashboardModule).
		Register(wsModule)

	// Boot task callbacks (needed for webhook handlers in production mode)
	a.kernel.BootTaskCallbacks()

	api := a.fiber.Group("/api")
	api.Get("/health", a.healthCheck)

	authMiddleware := middleware.Auth(a.config.JWT.Secret)

	// Initialize team middleware with membership cache
	middleware.InitTeamMiddleware(a.membershipCache)
	teamContextMiddleware := middleware.TeamScope()

	// Initialize subscription middleware
	middleware.InitSubscriptionMiddleware(a.db, a.config.Billing.SubscriptionsEnabled)

	// Boot all HTTP routes through the kernel
	// Note: TeamContext middleware is applied at the route level where team scope is required
	a.kernel.BootHTTP(api, authMiddleware, teamContextMiddleware)

	// Register server routes with cross-module dependencies
	serverModule.RegisterRoutes(api, authMiddleware, siteModule.SiteRepository())

	// Boot webhook routes (at root level, no /api prefix)
	a.kernel.BootWebhooks(a.fiber)

	// Boot WebSocket routes (at root level, no /api prefix)
	a.kernel.BootWebSocket(a.fiber)
}

// healthCheck handles the health check endpoint
func (a *Application) healthCheck(c *fiber.Ctx) error {
	result := a.healthChecker.Check(c.Context())

	status := fiber.StatusOK
	if result.Status != health.StatusHealthy {
		status = fiber.StatusServiceUnavailable
	}

	return c.Status(status).JSON(result)
}

// run starts the server and handles graceful shutdown
func (a *Application) run() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := a.fiber.Listen(":" + a.config.App.Port); err != nil {
			a.logger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	a.logger.Info().Str("port", a.config.App.Port).Msg("Server started")

	<-quit
	a.shutdown()
}

// shutdown gracefully shuts down the application
func (a *Application) shutdown() {
	a.logger.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := a.fiber.ShutdownWithContext(ctx); err != nil {
		a.logger.Error().Err(err).Msg("Server forced to shutdown")
	}

	// Shutdown modules through kernel (includes WebSocket hub)
	a.kernel.Shutdown()

	// Stop Redis subscriber
	a.wsSubscriber.Stop()

	// Close Redis cache
	if err := a.redisCache.Close(); err != nil {
		a.logger.Error().Err(err).Msg("Failed to close Redis cache")
	}

	a.queueClient.Close()

	// Flush Sentry events before exiting
	if a.sentryEnabled {
		a.logger.Info().Msg("Flushing Sentry events...")
		middleware.FlushSentry(5 * time.Second)
	}

	a.logger.Info().Msg("Server stopped")
}
