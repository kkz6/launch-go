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

	"github.com/kkz6/launch-go/internal/bootstrap/tasktemplates"
	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database"
	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/auth"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/modules/backup"
	backuppolicies "github.com/kkz6/launch-go/internal/modules/backup/policies"
	"github.com/kkz6/launch-go/internal/modules/billing"
	billingpolicies "github.com/kkz6/launch-go/internal/modules/billing/policies"
	"github.com/kkz6/launch-go/internal/modules/certificate"
	certificatepolicies "github.com/kkz6/launch-go/internal/modules/certificate/policies"
	"github.com/kkz6/launch-go/internal/modules/dashboard"
	databasemodule "github.com/kkz6/launch-go/internal/modules/database"
	databasepolicies "github.com/kkz6/launch-go/internal/modules/database/policies"
	"github.com/kkz6/launch-go/internal/modules/dns"
	dnspolicies "github.com/kkz6/launch-go/internal/modules/dns/policies"
	"github.com/kkz6/launch-go/internal/modules/docker"
	dockerpolicies "github.com/kkz6/launch-go/internal/modules/docker/policies"
	"github.com/kkz6/launch-go/internal/modules/git"
	gitpolicies "github.com/kkz6/launch-go/internal/modules/git/policies"
	"github.com/kkz6/launch-go/internal/modules/notification"
	notificationpolicies "github.com/kkz6/launch-go/internal/modules/notification/policies"
	"github.com/kkz6/launch-go/internal/modules/platform"
	platformpolicies "github.com/kkz6/launch-go/internal/modules/platform/policies"
	"github.com/kkz6/launch-go/internal/modules/script"
	scriptpolicies "github.com/kkz6/launch-go/internal/modules/script/policies"
	"github.com/kkz6/launch-go/internal/modules/server"
	serverpolicies "github.com/kkz6/launch-go/internal/modules/server/policies"
	"github.com/kkz6/launch-go/internal/modules/site"
	"github.com/kkz6/launch-go/internal/modules/site/adapters"
	sitedto "github.com/kkz6/launch-go/internal/modules/site/dto"
	sitepolicies "github.com/kkz6/launch-go/internal/modules/site/policies"
	"github.com/kkz6/launch-go/internal/modules/staff"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
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
	tasktemplates.MustRegisterAll()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Email template logo / "open in app" links go to the user-facing
	// frontend, not the API.
	mailtemplates.Initialize(cfg.App.Name, cfg.App.Frontend())

	// Webhook callback URLs are API-self-referencing — keep cfg.App.URL.
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

	// Public-facing signer renders URLs on the frontend host (e.g. the
	// provision-script command a user copies into their server). Nuxt
	// proxies the path back to this API, which validates against the
	// default signer — they share the secret key so signatures match
	// regardless of host.
	signedurl.SetPublicSigner(signedurl.NewSigner(cfg.App.Key).WithBaseURL(cfg.App.Frontend()))

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

	// Global rate limit per client IP to throttle scanners/bots. Health/liveness
	// probes and loopback traffic are exempt (see middleware.isRateLimitExempt).
	// The ceiling is generous because the SPA dashboard fires many parallel
	// requests per page; it's an anti-abuse bound, not a per-user quota.
	a.fiber.Use(middleware.GlobalRateLimit(600, time.Minute))

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

	// Wire the SSH host-key TOFU persister so the taskrunner can
	// pin a server's host key the first time we connect to it.
	// Subsequent connections refuse the handshake if the live host
	// key doesn't match — the only path that still falls back to
	// the legacy "ignore host keys" behaviour is ad-hoc CLI usage
	// where no ServerID is supplied. Registered at boot so every
	// downstream caller (terminal WS, deploy jobs, backup jobs, …)
	// gets the same persister without explicit plumbing.
	taskrunner.RegisterHostKeyPersister(func(ctx context.Context, serverID, hostKey string) error {
		return serverModule.Repos().Server().UpdateHostKey(ctx, serverID, hostKey)
	})
	// Manual backup runs in the worker need the database module's repos
	// to dump linked databases — same wiring lives in cmd/worker/main.go
	// for the actual job execution. The API doesn't run backup jobs but
	// wires the same setter so the dependency graph stays consistent.
	backupModule.SetDatabaseRepos(databaseModule.Repos())
	certificateModule := certificate.NewModule(builder)
	billingModule := billing.NewModule(builder)
	gitModule := git.NewModule(builder)
	scriptModule := script.NewModule(builder)
	platformModule := platform.NewModule(builder)
	dashboardModule := dashboard.NewModule(builder)
	staffModule := staff.NewModule(builder)
	wsModule := wsmodule.NewModule(builder)

	// Wire cross-module dependencies
	siteModule.SetDomainRepository(dnsModule.Repos().Domain())
	siteModule.SetProviderFactory(gitModule.ProviderFactory())
	dockerModule.SetProviderFactory(gitModule.ProviderFactory())
	siteModule.SetCronCreator(adapters.NewCronCreatorAdapter(serverModule.Service()))
	siteModule.SetDatabaseManager(adapters.NewDatabaseManagerAdapter(databaseModule.Service()))
	siteModule.SetStoredCertificateRepository(certificateModule.Repos().StoredCertificates)
	dockerModule.SetCertificateRepository(certificateModule.Repos())
	serverModule.SetSiteReader(siteModule.SiteReader())
	gitModule.SetSiteChecker(siteModule.SiteChecker())
	staffModule.SetServerLogReader(serverModule.Repos().Task())
	staffModule.Service().SetInvitationDeps(emailSender, a.config.App.Frontend())
	// Registration consumes a platform invite by delegating to the staff service
	// (which owns invitations + billing models). Auth defines the interface; staff
	// implements it; the direction stays staff -> auth, so no import cycle.
	authModule.Service().Auth.SetPlatformInviteReader(staffModule.Service())

	// Register all modules with the kernel
	a.kernel.
		Register(authModule).
		Register(serverModule).
		Register(databaseModule).
		Register(dockerModule).
		Register(siteModule).
		Register(dnsModule).
		Register(backupModule).
		Register(certificateModule).
		Register(billingModule).
		Register(gitModule).
		Register(notificationModule).
		Register(scriptModule).
		Register(platformModule).
		Register(dashboardModule).
		Register(staffModule).
		Register(wsModule)

	// Boot task callbacks (needed for webhook handlers in production mode)
	a.kernel.BootTaskCallbacks()

	// Boot modules (runs seeders, initialization, etc.)
	if err := a.kernel.Boot(); err != nil {
		a.logger.Fatal().Err(err).Msg("Failed to boot modules")
	}

	// Routes are served at the root path. The API has its own subdomain
	// (api.<domain>) and WebSockets have theirs (ws.<domain>), so a `/api`
	// path prefix on top of that would be redundant.
	api := a.fiber.Group("")
	api.Get("/health", a.healthCheck)

	authMiddleware := middleware.Auth(a.config.JWT.Secret, a.db)

	// Initialize team middleware with membership cache
	middleware.InitTeamMiddleware(a.membershipCache)
	teamContextMiddleware := middleware.TeamScope()

	// Initialize subscription middleware
	middleware.InitSubscriptionMiddleware(a.db, a.config.Billing.SubscriptionsEnabled)

	// Initialize server provisioned middleware
	middleware.InitServerProvisionedMiddleware(a.db)

	// Wire the staff-role lookup so RequireStaff can gate /admin routes.
	// Must run before BootHTTP registers the staff module's routes.
	middleware.InitStaffMiddleware(func(userID string) *stafftypes.StaffRole {
		return staffModule.Service().StaffRoleForUser(context.Background(), userID)
	})

	// Wire the account-status lookup so the Auth chokepoint can freeze
	// suspended users on every authenticated request. Must run before
	// BootHTTP registers routes guarded by the auth middleware.
	middleware.InitUserStatus(func(userID string) authtypes.UserStatus {
		return staffModule.Service().UserStatusForUser(context.Background(), userID)
	})

	// Wire session-liveness so the Auth chokepoint rejects access tokens whose
	// backing session has been revoked (logout, "log out other devices",
	// session revocation) immediately, instead of letting them linger until the
	// token's TTL expires. Mirrors the revocation check RefreshToken already
	// performs on the refresh path.
	middleware.InitSessionValidator(func(sessionID string) bool {
		exists, err := authModule.Repos().Session().Exists(context.Background(), sessionID)
		return err == nil && exists
	})

	// Wire the authorization gate and register module policies into it. The
	// auth module owns the gate (with its read-only freeze hook and team
	// abilities); each module registers its own abilities. The Can(ability)
	// route middleware authorizes against this gate.
	gate := authModule.Gate()
	serverpolicies.Register(gate)
	sitepolicies.Register(gate)
	databasepolicies.Register(gate)
	backuppolicies.Register(gate)
	certificatepolicies.Register(gate)
	dnspolicies.Register(gate)
	dockerpolicies.Register(gate)
	gitpolicies.Register(gate)
	scriptpolicies.Register(gate)
	notificationpolicies.Register(gate)
	platformpolicies.Register(gate)
	billingpolicies.Register(gate)
	middleware.InitGate(gate)

	// Boot all HTTP routes through the kernel
	// Note: TeamContext middleware is applied at the route level where team scope is required
	a.kernel.BootHTTP(app.BootHTTPOptions{
		Router:                api,
		AuthMiddleware:        authMiddleware,
		TeamContextMiddleware: teamContextMiddleware,
	})

	// Register server routes with cross-module dependencies
	serverModule.RegisterRoutes(api, authMiddleware, siteModule.SiteRepository())

	// Boot webhook routes at the root level (uses fiber app directly so it
	// doesn't pick up any group-level middleware).
	a.kernel.BootWebhooks(a.fiber)

	// Boot WebSocket routes. Served at root paths (e.g. /ws) — intended to
	// be reached via the dedicated ws.<domain> subdomain, though they also
	// resolve from api.<domain> since both hostnames hit the same process.
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
