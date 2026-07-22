package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/pkg/health"
)

func (a *Application) healthCheck(c *fiber.Ctx) error {
	result := a.healthChecker.Check(c.Context())
	status := fiber.StatusOK
	if result.Status != health.StatusHealthy {
		status = fiber.StatusServiceUnavailable
	}

	return c.Status(status).JSON(result)
}

// run starts the API and waits for a termination signal.
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

func (a *Application) shutdown() {
	a.logger.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.fiber.ShutdownWithContext(ctx); err != nil {
		a.logger.Error().Err(err).Msg("Server forced to shutdown")
	}

	a.kernel.Shutdown()
	a.wsSubscriber.Stop()
	if err := a.broadcaster.Close(); err != nil {
		a.logger.Error().Err(err).Msg("Failed to close event broadcaster")
	}
	if err := a.redisCache.Close(); err != nil {
		a.logger.Error().Err(err).Msg("Failed to close Redis cache")
	}
	a.queueClient.Close()

	if a.sentryEnabled {
		a.logger.Info().Msg("Flushing Sentry events...")
		middleware.FlushSentry(5 * time.Second)
	}
	a.logger.Info().Msg("Server stopped")
}
