package types

import (
	"database/sql/driver"
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// =============================================================================
// Source: where the application's image comes from.
// SourceImage pulls a pre-built image. SourceCompose runs a docker-compose
// stack defined inline. SourceGit lands in Phase 5.
// =============================================================================

// Source identifies how an application is built/pulled.
type Source string

const (
	// SourceImage pulls a pre-built image from a registry.
	SourceImage Source = "image"
	// SourceCompose runs a docker compose stack from inline YAML.
	SourceCompose Source = "compose"
)

var allSources = []Source{SourceImage, SourceCompose}

func (s Source) String() string    { return string(s) }
func (s Source) IsValid() bool     { return enumtypes.IsValid(s, allSources...) }
func (s *Source) Scan(v any) error { return enumtypes.ScanString(s, v) }
func (s Source) Value() (driver.Value, error) {
	return enumtypes.ValueString(s)
}

// ParseSource validates and returns a Source.
func ParseSource(in string) (Source, error) {
	s := Source(in)
	if !s.IsValid() {
		return "", fmt.Errorf("invalid source: %s", in)
	}
	return s, nil
}

// AllSources returns every source kind currently supported.
func AllSources() []Source { return allSources }

// =============================================================================
// Status: lifecycle state of an application.
// =============================================================================

// Status is the lifecycle state of an application.
type Status string

const (
	// StatusPending — row exists, deploy job has not started.
	StatusPending Status = "pending"
	// StatusDeploying — deploy job currently running.
	StatusDeploying Status = "deploying"
	// StatusRunning — container is up.
	StatusRunning Status = "running"
	// StatusStopped — container is intentionally stopped.
	StatusStopped Status = "stopped"
	// StatusFailed — last lifecycle op failed; LastError holds detail.
	StatusFailed Status = "failed"
)

var allStatuses = []Status{
	StatusPending, StatusDeploying, StatusRunning, StatusStopped, StatusFailed,
}

var statusLabels = map[Status]string{
	StatusPending:   "Pending",
	StatusDeploying: "Deploying",
	StatusRunning:   "Running",
	StatusStopped:   "Stopped",
	StatusFailed:    "Failed",
}

func (s Status) String() string    { return string(s) }
func (s Status) Label() string     { return enumtypes.Label(s, statusLabels, "Unknown") }
func (s Status) IsValid() bool     { return enumtypes.IsValid(s, allStatuses...) }
func (s *Status) Scan(v any) error { return enumtypes.ScanString(s, v) }
func (s Status) Value() (driver.Value, error) {
	return enumtypes.ValueString(s)
}

// IsTerminal reports whether the status represents a settled state.
func (s Status) IsTerminal() bool {
	return s == StatusRunning || s == StatusStopped || s == StatusFailed
}

// IsBusy reports whether a job is currently advancing the application.
func (s Status) IsBusy() bool {
	return s == StatusPending || s == StatusDeploying
}

// =============================================================================
// RestartPolicy: docker --restart values we expose.
// =============================================================================

// RestartPolicy is the docker --restart flag value.
type RestartPolicy string

const (
	RestartUnlessStopped RestartPolicy = "unless-stopped"
	RestartAlways        RestartPolicy = "always"
	RestartOnFailure     RestartPolicy = "on-failure"
	RestartNo            RestartPolicy = "no"
)

var allRestartPolicies = []RestartPolicy{
	RestartUnlessStopped, RestartAlways, RestartOnFailure, RestartNo,
}

func (p RestartPolicy) String() string    { return string(p) }
func (p RestartPolicy) IsValid() bool     { return enumtypes.IsValid(p, allRestartPolicies...) }
func (p *RestartPolicy) Scan(v any) error { return enumtypes.ScanString(p, v) }
func (p RestartPolicy) Value() (driver.Value, error) {
	return enumtypes.ValueString(p)
}

// AllRestartPolicies returns every supported restart policy.
func AllRestartPolicies() []RestartPolicy { return allRestartPolicies }

// =============================================================================
// Naming helpers
// =============================================================================

// DefaultContainerName returns the deterministic container name for an
// application named `name`. The "launch-app-" prefix keeps it from
// clashing with traefik / managed services on the same host.
func DefaultContainerName(name string) string {
	return "launch-app-" + name
}

// DefaultVolumeName returns the deterministic volume name for an
// application's named volume. App name plus the user-supplied volume
// suffix gives global uniqueness on the host.
func DefaultVolumeName(appName, volumeName string) string {
	return "launch-app-" + appName + "-" + volumeName
}
