package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	"github.com/kkz6/launch-go/internal/pkg/health"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/websocket"
)

// Application holds the shared services needed to configure and run the API.
type Application struct {
	config          *config.Config
	logger          *zerolog.Logger
	db              *gorm.DB
	queueClient     *queue.Client
	broadcaster     *websocket.RedisBroadcaster
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

// Version can be set via ldflags during build.
var Version = "development"
