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
	// ApplicationStatusDeleting marks the in-flight window between
	// "user clicked Delete" and "container is actually gone". The row
	// stays visible during this period so the UI can show a spinner
	// and the operator knows the delete is real but not done. On the
	// success path the row is then soft-deleted and the
	// docker.application.deleted event removes it from the listing.
	// On failure (docker stop / rm errors, SSH dies, asynq retries
	// exhaust) the status reverts to whatever it was before the
	// delete so the row stays usable.
	ApplicationStatusDeleting ApplicationStatus = "deleting"
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

// BuildLocation chooses where the customer's source gets turned into a
// runnable docker image. "server" is today's behaviour — the worker
// SSHes onto the target host, clones, and runs docker build there.
// "github_actions" hands the build to a GitHub Actions workflow that
// Launch commits + manages; the workflow pushes to GHCR and calls
// back to a webhook on success.
type BuildLocation string

const (
	BuildLocationServer        BuildLocation = "server"
	BuildLocationGitHubActions BuildLocation = "github_actions"
)

func (b BuildLocation) String() string { return string(b) }

// IsValid is convenience for handler validation when the frontend
// sends a raw string. Anything other than the two declared values is
// rejected — we never want a typo silently dropping a workload into
// an unknown build mode.
func (b BuildLocation) IsValid() bool {
	switch b {
	case BuildLocationServer, BuildLocationGitHubActions:
		return true
	}
	return false
}

// DeploymentTriggerSource tags how a deployment was initiated.
// Pre-existing rows are implicitly "manual"; the column default in
// migration 0050_05_28_000004 fills that in for the backfill.
type DeploymentTriggerSource string

const (
	DeploymentTriggerManual        DeploymentTriggerSource = "manual"
	DeploymentTriggerAuto          DeploymentTriggerSource = "auto"
	DeploymentTriggerGitHubActions DeploymentTriggerSource = "github_actions"
)

func (t DeploymentTriggerSource) String() string { return string(t) }
