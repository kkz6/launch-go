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
	databasemodule "github.com/kkz6/launch-go/internal/modules/database"
	"github.com/kkz6/launch-go/internal/modules/dns"
	"github.com/kkz6/launch-go/internal/modules/server"
	"github.com/kkz6/launch-go/internal/modules/site"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/logger"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Application holds all application dependencies
type Application struct {
	config      *config.Config
	logger      *zerolog.Logger
	db          *gorm.DB
	queueClient *queue.Client
	wsHub       *websocket.Hub
	dispatcher  *taskrunner.Dispatcher
	fiber       *fiber.App
	kernel      *app.Kernel
}

func main() {
	application := bootstrap()
	application.registerMiddleware()
	application.registerModules()
	application.run()
}

// bootstrap initializes all application dependencies
func bootstrap() *Application {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	appLogger := logger.NewWithConfig(cfg.App.Environment, cfg.App.Debug)

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

	dispatcher := taskrunner.NewDispatcher(appLogger, wsHub)

	fiberApp := fiber.New(fiber.Config{
		AppName:      "Launch API",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		ErrorHandler: middleware.ErrorHandler,
	})

	return &Application{
		config:      cfg,
		logger:      appLogger,
		db:          db,
		queueClient: queueClient,
		wsHub:       wsHub,
		dispatcher:  dispatcher,
		fiber:       fiberApp,
	}
}

// registerMiddleware sets up global middleware
func (a *Application) registerMiddleware() {
	a.fiber.Use(recover.New())
	a.fiber.Use(cors.New(cors.Config{
		AllowOrigins:     a.config.Cors.AllowedOrigins,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
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
	)

	// Create application kernel
	a.kernel = app.NewKernel(a.logger)

	// Register all modules with the kernel
	a.kernel.
		Register(auth.NewModuleFromContext(ctx)).
		Register(server.NewModuleFromContext(ctx)).
		Register(databasemodule.NewModuleFromContext(ctx)).
		Register(site.NewModuleFromContext(ctx)).
		Register(dns.NewModuleFromContext(ctx))

	// Set up routes
	a.registerWebSocketRoutes()

	api := a.fiber.Group("/api")
	api.Get("/health", a.healthCheck)

	authMiddleware := middleware.Auth(a.config.JWT.Secret)

	// Boot all HTTP routes through the kernel
	a.kernel.BootHTTP(api, authMiddleware)

	// Boot webhook routes (at root level, no /api prefix)
	a.kernel.BootWebhooks(a.fiber)
}

// registerWebSocketRoutes sets up WebSocket endpoints
func (a *Application) registerWebSocketRoutes() {
	a.fiber.Get("/ws", websocket.Handler(a.wsHub, a.config.JWT.Secret))

	terminalHandler := websocket.NewTerminalHandler(a.db, a.config.JWT.Secret, *a.logger)
	a.fiber.Get("/terminal/ws", terminalHandler.Handler())

	logsHandler := websocket.NewLogsHandler(a.db, a.config.JWT.Secret, *a.logger)
	a.fiber.Get("/terminal/logs", logsHandler.Handler())
}

// healthCheck handles the health check endpoint
func (a *Application) healthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
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

	// Shutdown modules through kernel
	a.kernel.Shutdown()

	a.wsHub.Shutdown()
	a.queueClient.Close()

	a.logger.Info().Msg("Server stopped")
}
