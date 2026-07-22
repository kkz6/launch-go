package main

import (
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/bootstrap/tasktemplates"
	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database"
	"github.com/kkz6/launch-go/internal/middleware"
	sitedto "github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	"github.com/kkz6/launch-go/internal/pkg/health"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/logger"
	mailtemplates "github.com/kkz6/launch-go/internal/pkg/mail/templates"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/websocket"
)

// bootstrap creates the infrastructure shared by the HTTP API and its modules.
func bootstrap() *Application {
	tasktemplates.MustRegisterAll()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	mailtemplates.Initialize(cfg.App.Name, cfg.App.Frontend())
	sitedto.SetWebhookBaseURL(cfg.Core.WebhookURL, cfg.App.URL)

	appLogger := logger.NewWithConfig(cfg.App.Environment, cfg.App.Debug)
	sentryEnabled := middleware.InitSentry(cfg.Sentry, cfg.App.Name, Version, appLogger)

	if err := database.InitEncryption(cfg.App.Key); err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to initialize encryption")
	}

	signedurl.SetDefaultSigner(signedurl.NewSigner(cfg.App.Key).WithBaseURL(cfg.App.URL))
	signedurl.SetPublicSigner(signedurl.NewSigner(cfg.App.Key).WithBaseURL(cfg.App.Frontend()))

	db, err := database.ConnectWithLogger(cfg.Database, appLogger)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to connect to database")
	}

	queueClient := queue.NewClient(cfg.Redis)
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// All business events, including API-originated ones, travel through Redis.
	// The subscriber below is the sole bridge from Redis to local socket clients.
	broadcaster := websocket.NewRedisBroadcaster(cfg.Redis.Address, cfg.Redis.Password, cfg.Redis.DB, appLogger)
	wsSubscriber := websocket.NewRedisSubscriber(cfg.Redis.Address, cfg.Redis.Password, cfg.Redis.DB, wsHub, appLogger)
	wsSubscriber.Start()

	dispatcher := taskrunner.NewDispatcherWithTaskLogger(appLogger, broadcaster, logger.NewFileLogger(cfg.App.Debug))
	redisCache := cache.NewRedisCache(cfg.Redis)
	membershipCache := launchcache.NewTeamMembershipCache(redisCache, db)
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

	return &Application{
		config: cfg, logger: appLogger, db: db, queueClient: queueClient,
		broadcaster: broadcaster, wsHub: wsHub, wsSubscriber: wsSubscriber, dispatcher: dispatcher,
		fiber: fiber.New(fiberCfg), redisCache: redisCache,
		membershipCache: membershipCache, healthChecker: healthChecker,
		sentryEnabled: sentryEnabled,
	}
}
