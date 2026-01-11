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
	"github.com/kkz6/launch-go/internal/modules/dns"
	"github.com/kkz6/launch-go/internal/modules/server"
	"github.com/kkz6/launch-go/internal/modules/site"
	"github.com/kkz6/launch-go/internal/pkg/logger"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Application holds all application dependencies
type Application struct {
	config       *config.Config
	logger       *zerolog.Logger
	db           *gorm.DB
	queueClient  *queue.Client
	wsHub        *websocket.Hub
	dispatcher   *taskrunner.Dispatcher
	fiber        *fiber.App
	serverModule *server.Module
}

func main() {
	app := bootstrap()
	app.registerMiddleware()
	app.registerRoutes()
	app.run()
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
func (app *Application) registerMiddleware() {
	app.fiber.Use(recover.New())
	app.fiber.Use(cors.New(cors.Config{
		AllowOrigins:     app.config.Cors.AllowedOrigins,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: true,
	}))
	app.fiber.Use(middleware.RequestLogger(app.logger))
}

// registerRoutes sets up all application routes
func (app *Application) registerRoutes() {
	app.registerWebSocketRoutes()
	app.registerAPIRoutes()
	// Register webhook routes after API routes since serverModule is initialized there
	app.registerWebhookRoutes()
}

// registerWebSocketRoutes sets up WebSocket endpoints
func (app *Application) registerWebSocketRoutes() {
	app.fiber.Get("/ws", websocket.Handler(app.wsHub, app.config.JWT.Secret))

	terminalHandler := websocket.NewTerminalHandler(app.db, app.config.JWT.Secret, *app.logger)
	app.fiber.Get("/terminal/ws", terminalHandler.Handler())

	logsHandler := websocket.NewLogsHandler(app.db, app.config.JWT.Secret, *app.logger)
	app.fiber.Get("/terminal/logs", logsHandler.Handler())
}

// registerWebhookRoutes sets up webhook endpoints (no auth required)
func (app *Application) registerWebhookRoutes() {
	if app.serverModule != nil {
		app.serverModule.RegisterWebhookRoutes(app.fiber)
	}
}

// registerAPIRoutes sets up API endpoints
func (app *Application) registerAPIRoutes() {
	api := app.fiber.Group("/api")

	api.Get("/health", app.healthCheck)

	authMiddleware := middleware.Auth(app.config.JWT.Secret)

	// Initialize and register modules
	authModule := auth.NewModule(app.db, app.config, app.logger)
	authModule.RegisterRoutes(api)

	app.serverModule = server.NewModule(app.db, app.queueClient, app.wsHub, app.dispatcher, app.logger, app.config.App.Key)
	app.serverModule.RegisterRoutes(api, authMiddleware)

	siteModule := site.NewModule(app.db, app.queueClient, app.wsHub, app.logger)
	siteModule.RegisterRoutes(api, authMiddleware)

	dnsModule := dns.NewModule(app.db, app.logger)
	dnsModule.RegisterRoutes(api, authMiddleware)
}

// healthCheck handles the health check endpoint
func (app *Application) healthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

// run starts the server and handles graceful shutdown
func (app *Application) run() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.fiber.Listen(":" + app.config.App.Port); err != nil {
			app.logger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	app.logger.Info().Str("port", app.config.App.Port).Msg("Server started")

	<-quit
	app.shutdown()
}

// shutdown gracefully shuts down the application
func (app *Application) shutdown() {
	app.logger.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.fiber.ShutdownWithContext(ctx); err != nil {
		app.logger.Error().Err(err).Msg("Server forced to shutdown")
	}

	app.wsHub.Shutdown()
	app.queueClient.Close()

	app.logger.Info().Msg("Server stopped")
}
