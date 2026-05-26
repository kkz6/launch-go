// Package types holds enum-like typed constants for the docker module.
// Mirrors the per-module convention used by server, site, etc.
package types

// ApplicationStatus is the lifecycle status of a docker application.
type ApplicationStatus string

const (
	ApplicationStatusIdle     ApplicationStatus = "idle"
	ApplicationStatusBuilding ApplicationStatus = "building"
	ApplicationStatusRunning  ApplicationStatus = "running"
	ApplicationStatusStopped  ApplicationStatus = "stopped"
	ApplicationStatusFailed   ApplicationStatus = "failed"
)

func (s ApplicationStatus) String() string { return string(s) }

// SourceType identifies where the application's image comes from.
type SourceType string

const (
	SourceTypeGit        SourceType = "git"
	SourceTypeImage      SourceType = "image"
	SourceTypeDockerfile SourceType = "dockerfile"
)

func (s SourceType) String() string { return string(s) }

// BuildType identifies the build pipeline applied to the source.
type BuildType string

const (
	BuildTypeNixpacks       BuildType = "nixpacks"
	BuildTypeDockerfile     BuildType = "dockerfile"
	BuildTypeHerokuBuildpck BuildType = "heroku-buildpack"
)

func (b BuildType) String() string { return string(b) }

// DatabaseEngine names the supported managed-database engines.
type DatabaseEngine string

const (
	DatabaseEnginePostgres DatabaseEngine = "postgres"
	DatabaseEngineMySQL    DatabaseEngine = "mysql"
	DatabaseEngineMariaDB  DatabaseEngine = "mariadb"
	DatabaseEngineRedis    DatabaseEngine = "redis"
	DatabaseEngineMongo    DatabaseEngine = "mongo"
)

func (e DatabaseEngine) String() string { return string(e) }

// DeploymentStatus is the lifecycle status of a single deploy attempt.
type DeploymentStatus string

const (
	DeploymentStatusPending   DeploymentStatus = "pending"
	DeploymentStatusBuilding  DeploymentStatus = "building"
	DeploymentStatusDeploying DeploymentStatus = "deploying"
	DeploymentStatusSuccess   DeploymentStatus = "success"
	DeploymentStatusFailed    DeploymentStatus = "failed"
	DeploymentStatusCancelled DeploymentStatus = "cancelled"
)

func (s DeploymentStatus) String() string { return string(s) }
