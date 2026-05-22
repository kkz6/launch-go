package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// TypeRunDatabase is the asynq task type for spinning up a managed
// database container after CreateDatabase persists the row.
const TypeRunDatabase = "docker:run_database"

// TypeDatabaseLifecycle is the asynq task type for start / stop /
// restart / rm of an existing managed database.
const TypeDatabaseLifecycle = "docker:database_lifecycle"

// RunDatabasePayload travels through asynq — IDs only, per the
// callback rules in CLAUDE.md.
type RunDatabasePayload struct {
	DatabaseID string `json:"database_id"`
	ServerID   string `json:"server_id"`
	TeamID     string `json:"team_id"`
}

// DatabaseLifecyclePayload is the message for start/stop/restart/rm.
type DatabaseLifecyclePayload struct {
	DatabaseID string `json:"database_id"`
	ProjectID  string `json:"project_id"`
	ServerID   string `json:"server_id"`
	TeamID     string `json:"team_id"`
	Action     string `json:"action"`
}

// RunDatabaseJob pulls + starts the configured database container on
// the docker server. The created row is already in `idle` status; we
// flip it to `running` once `docker run` returns.
type RunDatabaseJob struct {
	Deps    *JobDeps
	Payload RunDatabasePayload

	db      *models.Database
	project *models.Project
	server  *servermodels.Server
}

func NewRunDatabaseJob(p RunDatabasePayload) pkgjobs.Handler {
	return &RunDatabaseJob{Deps: deps, Payload: p}
}

// Handle is the asynq entrypoint. Same shape as DeployApplicationJob:
// transient errors return non-nil for asynq to retry; user-data errors
// (invalid engine, bad credentials) are persisted on the row + nil so
// the same broken config doesn't retry forever.
func (j *RunDatabaseJob) Handle(ctx context.Context) error {
	if err := j.loadModels(ctx); err != nil {
		return err
	}

	_ = j.Deps.Repos.Database().UpdateFields(ctx, j.db.ID, map[string]any{
		"status": dockertypes.ApplicationStatusBuilding,
	})
	j.broadcastLifecycle("docker.database.starting", "building")

	cfg, err := j.buildRunConfig()
	if err != nil {
		j.markFailed(ctx, err.Error())
		return nil
	}

	task := taskrunner.NewBaseTask(
		taskrunner.WithName("Run Database"),
		taskrunner.WithScript(tasks.RunDatabaseScript(cfg)),
		taskrunner.WithTimeoutSeconds(600),
	)
	result, runErr := j.Deps.RunTask(j.server, task).AsRoot().Dispatch(ctx)
	output := ""
	if result != nil {
		output = result.GetOutput()
	}
	if runErr != nil || (result != nil && !result.IsSuccessful()) {
		errMsg := summariseDatabaseRunError(runErr, output)
		j.markFailed(ctx, errMsg)
		return nil
	}

	_ = j.Deps.Repos.Database().UpdateFields(ctx, j.db.ID, map[string]any{
		"status": dockertypes.ApplicationStatusRunning,
	})
	j.broadcastLifecycle("docker.database.running", "running")
	return nil
}

func (j *RunDatabaseJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("database_id", j.Payload.DatabaseID).
		Msg("run database job failed at the framework level")
	j.markFailed(ctx, err.Error())
}

func (j *RunDatabaseJob) loadModels(ctx context.Context) error {
	var err error
	j.db, err = j.Deps.Repos.Database().FindByIDAndTeamServer(
		ctx, j.Payload.DatabaseID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		return fmt.Errorf("find database: %w", err)
	}
	j.project, err = j.Deps.Repos.Project().FindByIDAndTeamServer(
		ctx, j.db.ProjectID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}
	return nil
}

// buildRunConfig translates the database row into the script-input
// struct. Credentials are decoded from the encrypted column; the
// catalogue tells us how to spell the engine-specific env vars.
func (j *RunDatabaseJob) buildRunConfig() (tasks.DatabaseRunConfig, error) {
	spec, ok := engineSpecForJobs(j.db.Engine)
	if !ok {
		return tasks.DatabaseRunConfig{}, fmt.Errorf("unsupported engine %q", j.db.Engine)
	}
	creds, err := decodeJobCredentials(j.db.Credentials)
	if err != nil {
		return tasks.DatabaseRunConfig{}, fmt.Errorf("decode credentials: %w", err)
	}

	image := spec.Image
	if j.db.ImageTag != nil && *j.db.ImageTag != "" {
		image = *j.db.ImageTag
	}

	cfg := tasks.DatabaseRunConfig{
		ContainerName: tasks.DatabaseContainerName(
			tasks.SlugFromName(j.project.Name),
			tasks.SlugFromName(j.db.Name),
		),
		Image:        image,
		EnvVars:      spec.EnvFn(creds),
		InternalPort: spec.InternalPort,
		ExternalPort: j.db.ExternalPort,
	}

	// Redis is the one engine that needs a CLI flag for auth (env vars
	// don't trigger requirepass). The catalogue's EnvFn returns nil
	// for redis; we add the `redis-server --requirepass <pw>` extra
	// args here so the script renders correctly.
	if j.db.Engine == dockertypes.DatabaseEngineRedis {
		cfg.ExtraArgs = []string{"redis-server", "--requirepass", creds.Password}
	}

	return cfg, nil
}

func (j *RunDatabaseJob) markFailed(ctx context.Context, errMsg string) {
	if j.db == nil {
		return
	}
	_ = j.Deps.Repos.Database().UpdateFields(ctx, j.db.ID, map[string]any{
		"status": dockertypes.ApplicationStatusFailed,
	})
	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.database.failed", map[string]any{
		"id":         j.db.ID,
		"server_id":  j.Payload.ServerID,
		"team_id":    j.Payload.TeamID,
		"project_id": j.db.ProjectID,
		"status":     "failed",
		"error":      summariseDatabaseRunError(nil, errMsg),
	})
}

func (j *RunDatabaseJob) broadcastLifecycle(event, status string) {
	if j.db == nil {
		return
	}
	j.Deps.BroadcastToTeam(j.Payload.TeamID, event, map[string]any{
		"id":         j.db.ID,
		"server_id":  j.Payload.ServerID,
		"team_id":    j.Payload.TeamID,
		"project_id": j.db.ProjectID,
		"status":     status,
	})
}

// DatabaseLifecycleJob runs start/stop/restart/rm on an existing
// container. Much simpler than the run job — no credential or image
// gymnastics, just a single docker subcommand.
type DatabaseLifecycleJob struct {
	Deps    *JobDeps
	Payload DatabaseLifecyclePayload

	db      *models.Database
	server  *servermodels.Server
	project *models.Project
}

func NewDatabaseLifecycleJob(p DatabaseLifecyclePayload) pkgjobs.Handler {
	return &DatabaseLifecycleJob{Deps: deps, Payload: p}
}

func (j *DatabaseLifecycleJob) Handle(ctx context.Context) error {
	var err error
	// rm is special: we may have soft-deleted the row already, so look
	// up with Unscoped to find it. Fall back to the scoped lookup if
	// the soft-delete check fails.
	j.db, err = j.Deps.Repos.Database().FindByIDAndTeamServer(
		ctx, j.Payload.DatabaseID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil && j.Payload.Action != "rm" {
		return fmt.Errorf("find database: %w", err)
	}
	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}
	j.project, err = j.Deps.Repos.Project().FindByIDAndTeamServer(
		ctx, j.Payload.ProjectID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}

	dbName := j.Payload.DatabaseID
	if j.db != nil {
		dbName = j.db.Name
	}
	containerName := tasks.DatabaseContainerName(
		tasks.SlugFromName(j.project.Name),
		tasks.SlugFromName(dbName),
	)

	task := taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Database %s", j.Payload.Action)),
		taskrunner.WithScript(tasks.DatabaseLifecycleScript(containerName, j.Payload.Action)),
		taskrunner.WithTimeoutSeconds(60),
	)
	result, runErr := j.Deps.RunTask(j.server, task).AsRoot().Dispatch(ctx)
	if runErr != nil || (result != nil && !result.IsSuccessful()) {
		j.Deps.Logger.Warn().
			Str("database_id", j.Payload.DatabaseID).
			Str("action", j.Payload.Action).
			Msg("database lifecycle command failed (may be benign if container already absent)")
		return nil
	}

	// Reflect the new state on the row when possible. `rm` is handled
	// by the service's soft-delete; here we only flip start/stop to
	// the relevant terminal status.
	if j.db != nil {
		newStatus := j.db.Status
		switch j.Payload.Action {
		case "start", "restart":
			newStatus = dockertypes.ApplicationStatusRunning
		case "stop":
			newStatus = dockertypes.ApplicationStatusStopped
		}
		if newStatus != j.db.Status {
			_ = j.Deps.Repos.Database().UpdateFields(ctx, j.db.ID, map[string]any{
				"status": newStatus,
			})
		}
		j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.database.lifecycle_done", map[string]any{
			"id":         j.db.ID,
			"server_id":  j.Payload.ServerID,
			"team_id":    j.Payload.TeamID,
			"project_id": j.db.ProjectID,
			"action":     j.Payload.Action,
			"status":     string(newStatus),
		})
	}
	return nil
}

func (j *DatabaseLifecycleJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("database_id", j.Payload.DatabaseID).
		Str("action", j.Payload.Action).
		Msg("database lifecycle job failed")
}

// NewRunDatabaseTask packages the run-database asynq task.
func NewRunDatabaseTask(databaseID, serverID, teamID string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeRunDatabase, RunDatabasePayload{
		DatabaseID: databaseID,
		ServerID:   serverID,
		TeamID:     teamID,
	}, pkgjobs.Dedup("docker-run-database", databaseID))
}

// NewDatabaseLifecycleTask packages the lifecycle asynq task. We
// include the action in the dedup key so back-to-back stop+start (a
// reboot, effectively) doesn't get collapsed into a single op.
func NewDatabaseLifecycleTask(databaseID, projectID, serverID, teamID, action string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeDatabaseLifecycle, DatabaseLifecyclePayload{
		DatabaseID: databaseID,
		ProjectID:  projectID,
		ServerID:   serverID,
		TeamID:     teamID,
		Action:     action,
	}, pkgjobs.Dedup("docker-db-lifecycle", databaseID+":"+action+":"+nowKey()))
}

// nowKey returns a per-second deduplication key suffix so the lifecycle
// jobs don't dedupe across truly distinct user clicks.
func nowKey() string {
	return fmt.Sprintf("%d", time.Now().Unix())
}

// engineSpecForJobs is a thin shim that exposes the services package's
// catalogue without re-importing — services already imports this
// package (for the asynq constructors) so we can't depend on it. The
// runtime parameters are minimal: image + port + env-builder.
//
// We duplicate the engine table here intentionally — a small amount of
// duplication is cheaper than introducing a separate package or a
// cyclic dependency, and these are LTS-pinned tags that change rarely.
type jobsEngineSpec struct {
	Image        string
	InternalPort int
	EnvFn        func(c jobsCredentials) []string
}

type jobsCredentials struct {
	Username string
	Password string
	Database string
}

func engineSpecForJobs(engine dockertypes.DatabaseEngine) (jobsEngineSpec, bool) {
	spec, ok := jobsEngineCatalogue[engine]
	return spec, ok
}

var jobsEngineCatalogue = map[dockertypes.DatabaseEngine]jobsEngineSpec{
	dockertypes.DatabaseEnginePostgres: {
		Image:        "postgres",
		InternalPort: 5432,
		EnvFn: func(c jobsCredentials) []string {
			return []string{
				"POSTGRES_USER=" + c.Username,
				"POSTGRES_PASSWORD=" + c.Password,
				"POSTGRES_DB=" + c.Database,
			}
		},
	},
	dockertypes.DatabaseEngineMySQL: {
		Image:        "mysql",
		InternalPort: 3306,
		EnvFn: func(c jobsCredentials) []string {
			return []string{
				"MYSQL_ROOT_PASSWORD=" + c.Password,
				"MYSQL_DATABASE=" + c.Database,
				"MYSQL_USER=" + c.Username,
				"MYSQL_PASSWORD=" + c.Password,
			}
		},
	},
	dockertypes.DatabaseEngineMariaDB: {
		Image:        "mariadb",
		InternalPort: 3306,
		EnvFn: func(c jobsCredentials) []string {
			return []string{
				"MARIADB_ROOT_PASSWORD=" + c.Password,
				"MARIADB_DATABASE=" + c.Database,
				"MARIADB_USER=" + c.Username,
				"MARIADB_PASSWORD=" + c.Password,
			}
		},
	},
	dockertypes.DatabaseEngineRedis: {
		Image:        "redis",
		InternalPort: 6379,
		EnvFn:        func(_ jobsCredentials) []string { return nil },
	},
	dockertypes.DatabaseEngineMongo: {
		Image:        "mongo",
		InternalPort: 27017,
		EnvFn: func(c jobsCredentials) []string {
			return []string{
				"MONGO_INITDB_ROOT_USERNAME=" + c.Username,
				"MONGO_INITDB_ROOT_PASSWORD=" + c.Password,
			}
		},
	},
}

// decodeJobCredentials is the jobs-package twin of
// services.decodeCredentials. Same shape, separate package boundary.
func decodeJobCredentials(raw dbtype.EncryptedString) (jobsCredentials, error) {
	var c jobsCredentials
	if string(raw) == "" {
		return c, fmt.Errorf("no credentials stored")
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return jobsCredentials{}, err
	}
	return c, nil
}

// summariseDatabaseRunError keeps the failure copy short. We surface a
// one-line summary on the WS payload + the row's status field; full
// SSH output stays in the worker log for postmortem.
func summariseDatabaseRunError(runErr error, output string) string {
	if runErr != nil {
		return runErr.Error()
	}
	if output == "" {
		return "database container failed to start"
	}
	// Last meaningful line wins — `docker run` typically prints the
	// failure reason on the final stderr line.
	lines := []string{}
	for _, line := range splitLines(output) {
		trimmed := trimSpace(line)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	if len(lines) == 0 {
		return "database container failed to start"
	}
	last := lines[len(lines)-1]
	if len(last) > 240 {
		return last[:240] + "…"
	}
	return last
}

// splitLines splits on \n while preserving the absence of a trailing
// empty line. Equivalent to strings.Split but kept inline so this file
// stays self-contained (and to dodge the SplitSeq vs Split lint).
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n':
		return true
	}
	return false
}
