package main

import (
	"time"

	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/kkz6/launch-go/internal/middleware"
)

// registerMiddleware installs middleware that applies to every HTTP request.
func (a *Application) registerMiddleware() {
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
	a.fiber.Use(middleware.Trace())

	middleware.InitRateLimitMiddleware(a.redisCache)
	a.fiber.Use(middleware.GlobalRateLimit(600, time.Minute))
	a.fiber.Use(middleware.RequestLogger(a.logger, a.config.App.Environment == "development"))
}
