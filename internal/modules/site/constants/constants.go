package constants

import "time"

// Site types
const (
	TypeLaravel    = "laravel"
	TypeWordPress  = "wordpress"
	TypeStaticHTML = "static"
	TypeGenericPHP = "php"
	TypeNodeJS     = "nodejs"
)

// AllSiteTypes returns all supported site types
var AllSiteTypes = []string{TypeLaravel, TypeWordPress, TypeStaticHTML, TypeGenericPHP, TypeNodeJS}

// Deployment statuses
const (
	DeploymentPending   = "pending"
	DeploymentRunning   = "running"
	DeploymentSucceeded = "succeeded"
	DeploymentFailed    = "failed"
	DeploymentCancelled = "cancelled"
)

// Deployment defaults
const (
	DefaultDeploymentRetention = 10
	MaxDeploymentRetention     = 50
	DefaultReleaseRetention    = 5
	MaxReleaseRetention        = 20
)

// Deployment settings
const (
	DeploymentTimeout    = 30 * time.Minute
	DeploymentRetries    = 0
	ZeroDowntimeEnabled  = true
	AutoDeploymentOnPush = true
)

// Queue settings
const (
	DefaultQueueWorkers    = 1
	MaxQueueWorkers        = 10
	DefaultQueueConnection = "redis"
	DefaultQueueName       = "default"
	QueueRestartDelay      = 5 * time.Second
)

// Scheduler settings
const (
	SchedulerEnabled   = true
	SchedulerRunAsUser = "www-data"
	SchedulerLogPath   = "/var/log/scheduler.log"
)

// Certificate types
const (
	CertTypeLetsEncrypt = "letsencrypt"
	CertTypeCustom      = "custom"
	CertTypeNone        = "none"
)

// Certificate settings
const (
	CertRenewalDays   = 30
	CertCheckInterval = 24 * time.Hour
)

// Environment types
const (
	EnvProduction  = "production"
	EnvStaging     = "staging"
	EnvDevelopment = "development"
)

// Directory structure
const (
	ReleasesDir = "releases"
	SharedDir   = "shared"
	CurrentLink = "current"
	StorageDir  = "storage"
	EnvFile     = ".env"
)

// Git settings
const (
	DefaultBranch = "main"
	GitCloneDepth = 1
	GitFetchTags  = false
)

// Build settings
const (
	DefaultNodeVersion = "20"
	DefaultNPMClient   = "npm"
	BuildTimeout       = 15 * time.Minute
)

// NPM clients
const (
	NPMClientNPM  = "npm"
	NPMClientYarn = "yarn"
	NPMClientPNPM = "pnpm"
	NPMClientBun  = "bun"
)

// AllNPMClients returns all supported NPM clients
var AllNPMClients = []string{NPMClientNPM, NPMClientYarn, NPMClientPNPM, NPMClientBun}

// PHP optimization
const (
	OpcacheEnabled        = true
	OpcacheMemory         = 256
	OpcacheMaxFiles       = 20000
	OpcacheRevalidateFreq = 0
)

// Redirect types
const (
	RedirectPermanent = 301
	RedirectTemporary = 302
)

// Command types
const (
	CommandArtisan  = "artisan"
	CommandComposer = "composer"
	CommandNPM      = "npm"
	CommandCustom   = "custom"
)

// Webhook events
const (
	WebhookPush        = "push"
	WebhookPullRequest = "pull_request"
	WebhookTag         = "tag"
)
