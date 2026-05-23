package dto

import (
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
)

// ProjectResponse is the API representation of a docker project.
type ProjectResponse struct {
	ID                string     `json:"id"`
	TeamID            string     `json:"team_id"`
	ServerID          string     `json:"server_id"`
	Name              string     `json:"name"`
	Description       *string    `json:"description,omitempty"`
	ApplicationsCount int64      `json:"applications_count"`
	ComposesCount     int64      `json:"composes_count"`
	DatabasesCount    int64      `json:"databases_count"`
	CreatedAt         *time.Time `json:"created_at,omitempty"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty"`
}

// ToProjectResponse maps a Project model to its API response shape.
func ToProjectResponse(p *models.Project) *ProjectResponse {
	return &ProjectResponse{
		ID:                p.ID,
		TeamID:            p.TeamID,
		ServerID:          p.ServerID,
		Name:              p.Name,
		Description:       p.Description,
		ApplicationsCount: p.ApplicationsCount,
		ComposesCount:     p.ComposesCount,
		DatabasesCount:    p.DatabasesCount,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}
}

// ApplicationResponse is the API representation of a docker application.
//
// SourceConfig and BuildConfig are returned as opaque JSON-encoded blobs —
// the frontend interprets them based on SourceType. Doing this here (rather
// than projecting each variant into its own field) keeps the API stable as
// we add fields to one source type without touching the others.
type ApplicationResponse struct {
	ID             string         `json:"id"`
	TeamID         string         `json:"team_id"`
	ServerID       string         `json:"server_id"`
	ProjectID      string         `json:"project_id"`
	Name           string         `json:"name"`
	InternalPort   int            `json:"internal_port"`
	SourceType     string         `json:"source_type"`
	SourceConfig   map[string]any `json:"source_config,omitempty"`
	BuildType      *string        `json:"build_type,omitempty"`
	BuildConfig    map[string]any `json:"build_config,omitempty"`
	Status         string         `json:"status"`
	ContainerID    *string        `json:"container_id,omitempty"`
	// ContainerName is the on-host docker name (e.g.
	// `launch-<project>-<app>`). Like DatabaseResponse.ContainerName
	// — fed to the navbar Terminal button so it can attach to the
	// application container instead of the host root shell.
	ContainerName  string         `json:"container_name,omitempty"`
	LastDeployedAt *time.Time     `json:"last_deployed_at,omitempty"`
	CreatedAt      *time.Time     `json:"created_at,omitempty"`
	UpdatedAt      *time.Time     `json:"updated_at,omitempty"`
}

// DeploymentResponse is the API representation of a deploy attempt or
// database-lifecycle action. Shared across all three workload kinds
// (application / compose / database) — see models.Deployment for the
// per-field semantics. LogPath is internal and never serialised; the
// frontend uses TaskID to subscribe to the live task-logs websocket
// instead.
type DeploymentResponse struct {
	ID         string  `json:"id"`
	TeamID     string  `json:"team_id"`
	ServerID   string  `json:"server_id"`
	TargetType string  `json:"target_type"`
	TargetID   string  `json:"target_id"`
	// Action is set on database rows (create/start/restart/stop/rm).
	// nil for application + compose rows (their implicit action is
	// "deploy"). Lets the same table row a unified Deployments tab UI
	// across all three workload kinds.
	Action *string `json:"action,omitempty"`
	Status string  `json:"status"`
	// TaskID is the server-tasks ID. Frontend uses this with
	// ServerLogViewer entity="task" :entity-id="task_id" to stream the
	// live SSH output — same pattern site deployments use.
	TaskID     *string    `json:"task_id,omitempty"`
	CommitSHA  *string    `json:"commit_sha,omitempty"`
	CommitMsg  *string    `json:"commit_msg,omitempty"`
	ImageRef   *string    `json:"image_ref,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      *string    `json:"error,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

// ComposeResponse is the API representation of a docker compose stack.
type ComposeResponse struct {
	ID                string         `json:"id"`
	TeamID            string         `json:"team_id"`
	ServerID          string         `json:"server_id"`
	ProjectID         string         `json:"project_id"`
	Name              string         `json:"name"`
	ComposeSourceType string         `json:"compose_source_type"`
	SourceConfig      map[string]any `json:"source_config,omitempty"`
	ComposeFilePath   *string        `json:"compose_file_path,omitempty"`
	// RawYAML is omitted from list responses to keep them light; pulled
	// in for single-compose Show responses where the user is editing.
	RawYAML *string `json:"raw_yaml,omitempty"`
	// EnvFile is the `.env` body the Environment subtab edits. Only
	// included on single-compose Show responses (same as RawYAML) so
	// list responses don't carry potentially-large bodies.
	EnvFile        *string    `json:"env_file,omitempty"`
	Status         string     `json:"status"`
	LastDeployedAt *time.Time `json:"last_deployed_at,omitempty"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
}

// ToComposeResponse converts a Compose model to the API shape. The
// includeRaw flag controls whether the raw YAML body (potentially KB-
// scale) is included; list endpoints should pass false.
func ToComposeResponse(c *models.Compose, includeRaw bool) *ComposeResponse {
	resp := &ComposeResponse{
		ID:                c.ID,
		TeamID:            c.TeamID,
		ServerID:          c.ServerID,
		ProjectID:         c.ProjectID,
		Name:              c.Name,
		ComposeSourceType: c.ComposeSourceType,
		SourceConfig:      map[string]any(c.SourceConfig),
		ComposeFilePath:   c.ComposeFilePath,
		Status:            string(c.Status),
		LastDeployedAt:    c.LastDeployedAt,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}
	if includeRaw {
		resp.RawYAML = c.RawYAML
		resp.EnvFile = c.EnvFile
	}
	return resp
}

// ScheduleResponse is the API shape for an application schedule.
type ScheduleResponse struct {
	ID            string `json:"id"`
	ApplicationID string `json:"application_id"`
	Cron          string `json:"cron"`
	Command       string `json:"command"`
	// Enabled = false means the worker won't arm this schedule on
	// boot. UI shows a paused-pill instead of a status pill.
	Enabled   bool   `json:"enabled"`
	ShellType string `json:"shell_type"`
	// LastTaskID is the server-tasks ULID for the most recent run.
	// Frontend wires it into <ServerLogViewer entity="task"> so
	// View Logs streams that single run's output (same surface site
	// deployments use).
	LastTaskID *string    `json:"last_task_id,omitempty"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	LastStatus *string    `json:"last_status,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

func ToScheduleResponse(s *models.ApplicationSchedule) *ScheduleResponse {
	return &ScheduleResponse{
		ID:            s.ID,
		ApplicationID: s.ApplicationID,
		Cron:          s.Cron,
		Command:       s.Command,
		Enabled:       s.Enabled,
		ShellType:     s.ShellType,
		LastTaskID:    s.LastTaskID,
		LastRunAt:     s.LastRunAt,
		LastStatus:    s.LastStatus,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
}

// ApplicationRedirectResponse mirrors the PHP-site redirect shape so
// the docker-app Redirects subtab can reuse the same DataTable +
// dialog the SitesRedirects subtab uses. Stored inside build_config
// (no table) but exposed via per-row CRUD endpoints for parity.
type ApplicationRedirectResponse struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Type      int    `json:"type"`
	CreatedAt string `json:"created_at,omitempty"`
}

// EnvVarResponse is the API shape for an application env var. Value is
// masked when IsSecret=true unless the caller explicitly reveals it.
type EnvVarResponse struct {
	ID            string     `json:"id"`
	ApplicationID string     `json:"application_id"`
	Key           string     `json:"key"`
	Value         string     `json:"value"`
	IsSecret      bool       `json:"is_secret"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

// ProjectEnvVarResponse is the API shape for a project-scoped env
// var. Same fields as EnvVarResponse, but scoped to project_id —
// kept distinct so the response is self-documenting (vs. nilable
// owner fields on a polymorphic shape).
type ProjectEnvVarResponse struct {
	ID        string     `json:"id"`
	ProjectID string     `json:"project_id"`
	Key       string     `json:"key"`
	Value     string     `json:"value"`
	IsSecret  bool       `json:"is_secret"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// ToProjectEnvVarResponse renders a project env-var. Same masking
// rule as ToEnvVarResponse — list endpoints pass reveal=false to
// keep secrets out of the response.
func ToProjectEnvVarResponse(v *models.ProjectEnvVar, revealSecret bool) *ProjectEnvVarResponse {
	value := string(v.Value)
	if v.IsSecret && !revealSecret {
		value = "********"
	}
	return &ProjectEnvVarResponse{
		ID:        v.ID,
		ProjectID: v.ProjectID,
		Key:       v.Key,
		Value:     value,
		IsSecret:  v.IsSecret,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

// DatabaseEnvVarResponse is the API shape for a database-scoped env
// var (user-added extras on top of the auto-generated engine
// credentials).
type DatabaseEnvVarResponse struct {
	ID         string     `json:"id"`
	DatabaseID string     `json:"database_id"`
	Key        string     `json:"key"`
	Value      string     `json:"value"`
	IsSecret   bool       `json:"is_secret"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

// ToDatabaseEnvVarResponse mirrors ToProjectEnvVarResponse — masks
// secrets unless reveal=true.
func ToDatabaseEnvVarResponse(v *models.DatabaseEnvVar, revealSecret bool) *DatabaseEnvVarResponse {
	value := string(v.Value)
	if v.IsSecret && !revealSecret {
		value = "********"
	}
	return &DatabaseEnvVarResponse{
		ID:         v.ID,
		DatabaseID: v.DatabaseID,
		Key:        v.Key,
		Value:      value,
		IsSecret:   v.IsSecret,
		CreatedAt:  v.CreatedAt,
		UpdatedAt:  v.UpdatedAt,
	}
}

// ToEnvVarResponse renders an env-var. revealSecret=false (default)
// masks the value when IsSecret is true so list endpoints never leak
// passwords — same defence pattern as database credentials.
func ToEnvVarResponse(v *models.ApplicationEnvVar, revealSecret bool) *EnvVarResponse {
	// Value is `dbtype.EncryptedString` on the model — convert here so
	// the response stays a plain string. GORM already decrypted the
	// column when hydrating the row, so `string(v.Value)` is plaintext.
	value := string(v.Value)
	if v.IsSecret && !revealSecret {
		value = "********"
	}
	return &EnvVarResponse{
		ID:            v.ID,
		ApplicationID: v.ApplicationID,
		Key:           v.Key,
		Value:         value,
		IsSecret:      v.IsSecret,
		CreatedAt:     v.CreatedAt,
		UpdatedAt:     v.UpdatedAt,
	}
}

// VolumeResponse is the API shape for an application volume / mount.
// Carries all three mount-kinds (bind / volume / file) in a single
// shape — fields not relevant to the row's `type` are omitted.
type VolumeResponse struct {
	ID            string     `json:"id"`
	ApplicationID string     `json:"application_id"`
	Name          string     `json:"name"`
	MountPath     string     `json:"mount_path"`
	Type          string     `json:"type"`
	HostPath      *string    `json:"host_path,omitempty"`
	// File-mount payload — content + on-host filename. The list
	// endpoint returns content as well (the editor on the frontend
	// needs it to render the existing body).
	Content   *string    `json:"content,omitempty"`
	FilePath  *string    `json:"file_path,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

func ToVolumeResponse(v *models.ApplicationVolume) *VolumeResponse {
	return &VolumeResponse{
		ID:            v.ID,
		ApplicationID: v.ApplicationID,
		Name:          v.Name,
		MountPath:     v.MountPath,
		Type:          v.Type,
		HostPath:      v.HostPath,
		Content:       v.Content,
		FilePath:      v.FilePath,
		CreatedAt:     v.CreatedAt,
		UpdatedAt:     v.UpdatedAt,
	}
}

// BackupResponse is the API shape for a database backup configuration.
// References the global storage_providers row by id — the actual S3
// credentials never travel back to the client.
type BackupResponse struct {
	ID                string     `json:"id"`
	DatabaseID        string     `json:"database_id"`
	StorageProviderID uint64     `json:"storage_provider_id"`
	Path              *string    `json:"path,omitempty"`
	Retention         int        `json:"retention"`
	NotifyOnSuccess   bool       `json:"notify_on_success"`
	NotifyOnFailure   bool       `json:"notify_on_failure"`
	CronSchedule      *string    `json:"cron_schedule,omitempty"`
	Enabled           bool       `json:"enabled"`
	CreatedAt         *time.Time `json:"created_at,omitempty"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty"`
}

// BackupRunResponse is one row in the run-history table.
type BackupRunResponse struct {
	ID         string     `json:"id"`
	BackupID   string     `json:"backup_id"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ObjectKey  *string    `json:"object_key,omitempty"`
	SizeBytes  *int64     `json:"size_bytes,omitempty"`
	Error      *string    `json:"error,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
}

// ToBackupResponse renders a backup config. Credentials live on the
// linked storage_providers row — never on this response.
func ToBackupResponse(b *models.DatabaseBackup) *BackupResponse {
	return &BackupResponse{
		ID:                b.ID,
		DatabaseID:        b.DatabaseID,
		StorageProviderID: b.StorageProviderID,
		Path:              b.Path,
		Retention:         b.Retention,
		NotifyOnSuccess:   b.NotifyOnSuccess,
		NotifyOnFailure:   b.NotifyOnFailure,
		CronSchedule:      b.CronSchedule,
		Enabled:           b.Enabled,
		CreatedAt:         b.CreatedAt,
		UpdatedAt:         b.UpdatedAt,
	}
}

// ToBackupRunResponse renders a single backup run.
func ToBackupRunResponse(r *models.DatabaseBackupRun) *BackupRunResponse {
	return &BackupRunResponse{
		ID:         r.ID,
		BackupID:   r.BackupID,
		Status:     r.Status,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		ObjectKey:  r.ObjectKey,
		SizeBytes:  r.SizeBytes,
		Error:      r.Error,
		CreatedAt:  r.CreatedAt,
	}
}

// DatabaseCredentials is the shape of the auto-generated DB secrets we
// reveal to the user when they explicitly ask. Mirrored from the
// services.Credentials struct.
type DatabaseCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// DatabaseResponse is the API representation of a managed database.
//
// Credentials are absent unless the caller asked to reveal them — the
// reveal flag controls whether the password is included so we don't
// leak it on list endpoints.
type DatabaseResponse struct {
	ID            string               `json:"id"`
	TeamID        string               `json:"team_id"`
	ServerID      string               `json:"server_id"`
	ProjectID     string               `json:"project_id"`
	Name          string               `json:"name"`
	Engine        string               `json:"engine"`
	EngineVersion string               `json:"engine_version"`
	ImageTag      *string              `json:"image_tag,omitempty"`
	ExternalPort  *int                 `json:"external_port,omitempty"`
	Status        string               `json:"status"`
	Credentials   *DatabaseCredentials `json:"credentials,omitempty"`
	// BuildConfig surfaces the Advanced subtab's runtime knobs
	// (restart_policy, cpu_limit, memory_limit, cpu_reservation,
	// memory_reservation). Same shape applications use.
	BuildConfig map[string]any `json:"build_config,omitempty"`
	// VolumeName + DataPath describe the named bind that persists the
	// database's on-disk state across container recreates. Both are
	// deterministic — derived from the database id + engine — so the
	// frontend can render the Volumes section in Advanced without an
	// extra round-trip.
	VolumeName string `json:"volume_name,omitempty"`
	DataPath   string `json:"data_path,omitempty"`
	// ContainerName is the on-host docker name (e.g.
	// `launch-db-<project>-<db>`). Surfaced so the navbar's Terminal
	// button can pass it to the WS handler and open a shell inside
	// the database container instead of the host root shell.
	ContainerName string     `json:"container_name,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

// databaseEngineDataPath mirrors services.engineCatalogue's DataPath
// values. Inlined here (instead of importing the services package) so
// dto stays a leaf of the package graph — services already imports
// dto, so the reverse would be a cycle.
func databaseEngineDataPath(engine string) string {
	switch engine {
	case "postgres":
		return "/var/lib/postgresql/data"
	case "mysql", "mariadb":
		return "/var/lib/mysql"
	case "redis":
		return "/data"
	case "mongo":
		return "/data/db"
	default:
		return ""
	}
}

// ToDatabaseResponse renders a Database model. The caller passes
// reveal=true only on the explicit-reveal show endpoint; list +
// default get omit the password entirely.
func ToDatabaseResponse(d *models.Database, _ bool) *DatabaseResponse {
	engine := string(d.Engine)
	return &DatabaseResponse{
		ID:            d.ID,
		TeamID:        d.TeamID,
		ServerID:      d.ServerID,
		ProjectID:     d.ProjectID,
		Name:          d.Name,
		Engine:        engine,
		EngineVersion: d.EngineVersion,
		ImageTag:      d.ImageTag,
		ExternalPort:  d.ExternalPort,
		Status:        string(d.Status),
		BuildConfig:   map[string]any(d.BuildConfig),
		// Deterministic — must stay in sync with tasks.DatabaseVolumeName.
		// Lowercasing the ULID matches docker's volume-name rules and
		// what the run script writes.
		VolumeName: "launch-db-" + strings.ToLower(d.ID) + "-data",
		DataPath:   databaseEngineDataPath(engine),
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

// ValidateDNSResponse is the result of the "Validate DNS" button in
// the Domains subtab. The frontend renders OK + Message as a toast;
// ResolvedIPs / ExpectedIP let a future Inspect panel show the full
// diff. Wildcard=true means we short-circuited (traefik.me etc).
type ValidateDNSResponse struct {
	Host        string   `json:"host"`
	OK          bool     `json:"ok"`
	Wildcard    bool     `json:"wildcard"`
	ExpectedIP  string   `json:"expected_ip,omitempty"`
	ResolvedIPs []string `json:"resolved_ips,omitempty"`
	Message     string   `json:"message"`
}

// DomainResponse is the API representation of an application domain.
type DomainResponse struct {
	ID                  string     `json:"id"`
	ApplicationID       string     `json:"application_id"`
	Host                string     `json:"host"`
	Path                *string    `json:"path,omitempty"`
	InternalPath        *string    `json:"internal_path,omitempty"`
	StripPath           bool       `json:"strip_path"`
	ContainerPort       *int       `json:"container_port,omitempty"`
	HTTPS               bool       `json:"https"`
	CertificateProvider string     `json:"certificate_provider"`
	CertificateID       *string    `json:"certificate_id,omitempty"`
	CreatedAt           *time.Time `json:"created_at,omitempty"`
	UpdatedAt           *time.Time `json:"updated_at,omitempty"`
}

// ToDomainResponse maps a domain model to the API response shape.
func ToDomainResponse(d *models.ApplicationDomain) *DomainResponse {
	return &DomainResponse{
		ID:                  d.ID,
		ApplicationID:       d.ApplicationID,
		Host:                d.Host,
		Path:                d.Path,
		InternalPath:        d.InternalPath,
		StripPath:           d.StripPath,
		ContainerPort:       d.ContainerPort,
		HTTPS:               d.HTTPS,
		CertificateProvider: d.CertificateProvider,
		CertificateID:       d.CertificateID,
		CreatedAt:           d.CreatedAt,
		UpdatedAt:           d.UpdatedAt,
	}
}

// ToDeploymentResponse maps a Deployment model to the API response.
func ToDeploymentResponse(d *models.Deployment) *DeploymentResponse {
	return &DeploymentResponse{
		ID:         d.ID,
		TeamID:     d.TeamID,
		ServerID:   d.ServerID,
		TargetType: d.TargetType,
		TargetID:   d.TargetID,
		Action:     d.Action,
		Status:     string(d.Status),
		TaskID:     d.TaskID,
		CommitSHA:  d.CommitSHA,
		CommitMsg:  d.CommitMsg,
		ImageRef:   d.ImageRef,
		StartedAt:  d.StartedAt,
		FinishedAt: d.FinishedAt,
		Error:      d.Error,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

// ToApplicationResponse maps an Application model to its API response shape.
func ToApplicationResponse(a *models.Application) *ApplicationResponse {
	var buildType *string
	if a.BuildType != nil {
		bt := string(*a.BuildType)
		buildType = &bt
	}
	return &ApplicationResponse{
		ID:             a.ID,
		TeamID:         a.TeamID,
		ServerID:       a.ServerID,
		ProjectID:      a.ProjectID,
		Name:           a.Name,
		InternalPort:   a.InternalPort,
		SourceType:     string(a.SourceType),
		SourceConfig:   map[string]any(a.SourceConfig),
		BuildType:      buildType,
		BuildConfig:    map[string]any(a.BuildConfig),
		Status:         string(a.Status),
		ContainerID:    a.ContainerID,
		LastDeployedAt: a.LastDeployedAt,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
	}
}
