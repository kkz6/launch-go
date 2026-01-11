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

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database"
	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/auth"
	"github.com/kkz6/launch-go/internal/modules/dns"
	"github.com/kkz6/launch-go/internal/modules/server"
	"github.com/kkz6/launch-go/internal/modules/site"
	"github.com/kkz6/launch-go/internal/modules/team"
	"github.com/kkz6/launch-go/internal/pkg/logger"
	"github.com/kkz6/launch-go/internal/queue"
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
	appLogger := logger.NewWithConfig(cfg.App.Environment, cfg.App.Debug)

	// Initialize encryption for Laravel-encrypted fields
	if err := database.InitEncryption(cfg.App.Key); err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to initialize encryption")
	}

	// Initialize database with custom logger
	db, err := database.ConnectWithLogger(cfg.Database, appLogger)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Initialize Redis client for queue
	queueClient := queue.NewClient(cfg.Redis)

	// Initialize WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Initialize task runner dispatcher
	dispatcher := taskrunner.NewDispatcher(appLogger, wsHub)

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "Launch API",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		ErrorHandler: middleware.ErrorHandler,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.Cors.AllowedOrigins,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: true,
	}))
	app.Use(middleware.RequestLogger(appLogger))

	// WebSocket endpoints
	app.Get("/ws", websocket.Handler(wsHub, cfg.JWT.Secret))

	// Terminal WebSocket endpoint
	terminalHandler := websocket.NewTerminalHandler(db, cfg.JWT.Secret, *appLogger)
	app.Get("/terminal/ws", terminalHandler.Handler())

	// API routes
	api := app.Group("/api")

	// Health check
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"time":   time.Now().UTC(),
		})
	})

	// Initialize modules
	authModule := auth.NewModule(db, cfg, appLogger)
	teamModule := team.NewModule(db, appLogger)
	serverModule := server.NewModule(db, queueClient, wsHub, dispatcher, appLogger)
	siteModule := site.NewModule(db, queueClient, wsHub, appLogger)
	dnsModule := dns.NewModule(db, appLogger)

	// Register routes
	authModule.RegisterRoutes(api)
	teamModule.RegisterRoutes(api, middleware.Auth(cfg.JWT.Secret))
	serverModule.RegisterRoutes(api, middleware.Auth(cfg.JWT.Secret))
	siteModule.RegisterRoutes(api, middleware.Auth(cfg.JWT.Secret))
	dnsModule.RegisterRoutes(api, middleware.Auth(cfg.JWT.Secret))

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.Listen(":" + cfg.App.Port); err != nil {
			appLogger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	appLogger.Info().Str("port", cfg.App.Port).Msg("Server started")

	<-quit
	appLogger.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		appLogger.Error().Err(err).Msg("Server forced to shutdown")
	}

	// Close connections
	wsHub.Shutdown()
	queueClient.Close()

	appLogger.Info().Msg("Server stopped")
}
