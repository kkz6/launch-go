package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
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
	"github.com/kkz6/launch-go/internal/modules/docker"
	"github.com/kkz6/launch-go/internal/modules/git"
	"github.com/kkz6/launch-go/internal/modules/notification"
	"github.com/kkz6/launch-go/internal/modules/platform"
	"github.com/kkz6/launch-go/internal/modules/script"
	"github.com/kkz6/launch-go/internal/modules/server"
	"github.com/kkz6/launch-go/internal/modules/site"
	"github.com/kkz6/launch-go/internal/modules/site/adapters"
	sitedto "github.com/kkz6/launch-go/internal/modules/site/dto"
	wsmodule "github.com/kkz6/launch-go/internal/modules/websocket"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	"github.com/kkz6/launch-go/internal/pkg/health"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/logger"
	"github.com/kkz6/launch-go/internal/pkg/mail"
	mailtemplates "github.com/kkz6/launch-go/internal/pkg/mail/templates"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
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
	// Register all script templates at startup
	templates.MustRegisterAll()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize email templates with app name and URL
	mailtemplates.Initialize(cfg.App.Name, cfg.App.URL)

	// Initialize webhook base URL for deploy webhook URLs
	sitedto.SetWebhookBaseURL(cfg.Core.WebhookURL, cfg.App.URL)

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

	taskLogger := logger.NewFileLogger(cfg.App.Debug)
	dispatcher := taskrunner.NewDispatcherWithTaskLogger(appLogger, wsHub, taskLogger)

	// Initialize Redis cache for team membership
	redisCache := cache.NewRedisCache(cfg.Redis)
	membershipCache := launchcache.NewTeamMembershipCache(redisCache, db)

	// Initialize health check aggregator
	healthChecker := health.NewAggregator().
		Add(&health.DatabaseChecker{DB: db}).
		Add(&health.RedisChecker{Client: redisCache.Client()})

	fiberCfg := fiber.Config{
		AppName:      "Launch API",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		ErrorHandler: middleware.ErrorHandler,
	}

	if cfg.Proxy.TrustedProxies != "" {
		fiberCfg.EnableTrustedProxyCheck = true
		fiberCfg.TrustedProxies = strings.Split(cfg.Proxy.TrustedProxies, ",")
		fiberCfg.ProxyHeader = cfg.Proxy.ProxyHeader
	}

	fiberApp := fiber.New(fiberCfg)

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

	// Initialize rate limiting with Redis cache
	middleware.InitRateLimitMiddleware(a.redisCache)

	// Global rate limit: 120 requests per minute per IP to throttle scanners/bots
	a.fiber.Use(middleware.GlobalRateLimit(120, time.Minute))

	a.fiber.Use(middleware.RequestLogger(a.logger, a.config.App.Environment == "development"))
}

// registerModules sets up all application modules using the kernel
func (a *Application) registerModules() {
	// Create email sender and notifier before building context,
	// so the Notifier is available in Deps from the start (Deps is copied by value)
	emailSender := mail.NewEmailSender(a.config.Mail)
	if emailSender == nil {
		a.logger.Warn().Str("driver", a.config.Mail.Driver).Msg("Email sender not configured — team invitations, password resets, and notifications will not send emails. Set RESEND_API_KEY or SMTP_HOST to enable.")
	}
	notifBuilder := app.NewBuilder(app.Deps{
		DB:     a.db,
		Logger: a.logger,
		Config: a.config,
	})
	notificationModule := notification.NewModule(notifBuilder, emailSender)
	notifier := notificationModule.Notifier()

	// Create application context with all shared dependencies
	ctx := app.NewContext(
		a.config,
		a.db,
		a.logger,
		a.queueClient,
		a.wsHub,
		a.dispatcher,
		notifier,
		a.membershipCache,
	)

	// Create application kernel
	a.kernel = app.NewKernel(a.logger)

	// Create module builder
	builder := app.NewBuilderFromContext(ctx)

	// Create modules using builder pattern
	authModule, err := auth.NewModule(builder, emailSender, a.redisCache)
	if err != nil {
		a.logger.Fatal().Err(err).Msg("Failed to create auth module")
	}
	serverModule := server.NewModule(builder)
	databaseModule := databasemodule.NewModule(builder)
	dockerModule := docker.NewModule(builder)
	siteModule := site.NewModule(builder)
	dnsModule := dns.NewModule(builder)
	backupModule := backup.NewModule(builder)
	billingModule := billing.NewModule(builder)
	gitModule := git.NewModule(builder)
	scriptModule := script.NewModule(builder)
	platformModule := platform.NewModule(builder)
	dashboardModule := dashboard.NewModule(builder)
	wsModule := wsmodule.NewModule(builder)

	// Wire cross-module dependencies
	siteModule.SetDomainRepository(dnsModule.Repos().Domain())
	siteModule.SetProviderFactory(gitModule.ProviderFactory())
	siteModule.SetCronCreator(adapters.NewCronCreatorAdapter(serverModule.Service()))
	siteModule.SetDatabaseManager(adapters.NewDatabaseManagerAdapter(databaseModule.Service()))
	serverModule.SetSiteReader(siteModule.SiteReader())
	gitModule.SetSiteChecker(siteModule.SiteChecker())

	// Register all modules with the kernel
	a.kernel.
		Register(authModule).
		Register(serverModule).
		Register(databaseModule).
		Register(dockerModule).
		Register(siteModule).
		Register(dnsModule).
		Register(backupModule).
		Register(billingModule).
		Register(gitModule).
		Register(notificationModule).
		Register(scriptModule).
		Register(platformModule).
		Register(dashboardModule).
		Register(wsModule)

	// Boot task callbacks (needed for webhook handlers in production mode)
	a.kernel.BootTaskCallbacks()

	// Boot modules (runs seeders, initialization, etc.)
	if err := a.kernel.Boot(); err != nil {
		a.logger.Fatal().Err(err).Msg("Failed to boot modules")
	}

	api := a.fiber.Group("/api")
	api.Get("/health", a.healthCheck)

	authMiddleware := middleware.Auth(a.config.JWT.Secret, a.db)

	// Initialize team middleware with membership cache
	middleware.InitTeamMiddleware(a.membershipCache)
	teamContextMiddleware := middleware.TeamScope()

	// Initialize subscription middleware
	middleware.InitSubscriptionMiddleware(a.db, a.config.Billing.SubscriptionsEnabled)

	// Initialize server provisioned middleware
	middleware.InitServerProvisionedMiddleware(a.db)

	// Boot all HTTP routes through the kernel
	// Note: TeamContext middleware is applied at the route level where team scope is required
	a.kernel.BootHTTP(app.BootHTTPOptions{
		Router:                api,
		AuthMiddleware:        authMiddleware,
		TeamContextMiddleware: teamContextMiddleware,
	})

	// Register server routes with cross-module dependencies
	serverModule.RegisterRoutes(api, authMiddleware, siteModule.SiteRepository())

	// Boot webhook routes (at root level, no /api prefix)
	a.kernel.BootWebhooks(a.fiber)

	// Boot WebSocket routes (under /api prefix)
	a.kernel.BootWebSocket(api)
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
