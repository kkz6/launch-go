package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
// callback rules in CLAUDE.md. WipeVolume = true reroutes through the
// "Rebuild Database" path: the run script will `docker volume rm` the
// named data volume before starting the new container so the engine
// reinitialises from scratch. Defaulting to false keeps every existing
// caller (first create, expose-toggle recreate) data-preserving.
type RunDatabasePayload struct {
	DatabaseID string `json:"database_id"`
	ServerID   string `json:"server_id"`
	TeamID     string `json:"team_id"`
	WipeVolume bool   `json:"wipe_volume,omitempty"`
}

// DatabaseLifecyclePayload is the message for start/stop/restart/rm.
type DatabaseLifecyclePayload struct {
	DatabaseID string `json:"database_id"`
	ProjectID  string `json:"project_id"`
	ServerID   string `json:"server_id"`
	TeamID     string `json:"team_id"`
	Action     string `json:"action"`
	// RemoveVolume only applies when Action == "rm". When true the
	// script also runs `docker volume rm` against the database's
	// named data volume after the container is gone. Default false
	// — preserves data so a misclicked Delete is recoverable.
	RemoveVolume bool `json:"remove_volume,omitempty"`
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
//
// Writes a docker_deployments row with target_type="database" +
// action="create" so the Deployments subtab picks the entry up. The
// row's task_id wires into the existing /servers/:id/tasks/:taskId/logs
// websocket so the frontend can show the SSH output.
func (j *RunDatabaseJob) Handle(ctx context.Context) error {
	if err := j.loadModels(ctx); err != nil {
		return err
	}

	// Label the deployment row by intent so the Deployments subtab
	// distinguishes a first-create from a rebuild (data-wipe). Lifecycle
	// recreates (expose-toggle, advanced update) still funnel through
	// "create" — those preserve data.
	deploymentAction := "create"
	if j.Payload.WipeVolume {
		deploymentAction = "rebuild"
	}
	deployment, err := createDeploymentRow(
		ctx, j.Deps, "database", j.db.ID, deploymentAction, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		// Failure to create the audit row is bad but shouldn't block
		// the actual container start — log and proceed with a nil
		// deployment so downstream code uses no-op branches.
		j.Deps.Logger.Error().Err(err).Msg("failed to create database deployment row")
	}
	if deployment != nil {
		j.broadcastDeployment("docker.database.deployment.started", deployment, "")
	}

	_ = j.Deps.Repos.Database().UpdateFields(ctx, j.db.ID, map[string]any{
		"status": dockertypes.ApplicationStatusBuilding,
	})
	j.broadcastLifecycle("docker.database.starting", "building")

	cfg, err := j.buildRunConfig(ctx)
	if err != nil {
		if deployment != nil {
			finalizeDeploymentRow(ctx, j.Deps, deployment.ID, dockertypes.DeploymentStatusFailed, err.Error())
			j.broadcastDeployment("docker.database.deployment.failed", deployment, err.Error())
		}
		j.markFailed(ctx, err.Error())
		return nil
	}

	task := taskrunner.NewBaseTask(
		taskrunner.WithName("Run Database"),
		taskrunner.WithScript(tasks.RunDatabaseScript(cfg)),
		taskrunner.WithTimeoutSeconds(600),
	)
	// TrackInDB() persists a server-tasks row before SSH starts so the
	// frontend can stream the live output via ServerLogViewer.
	result, runErr := j.Deps.RunTask(j.server, task).AsRoot().TrackInDB().Dispatch(ctx)
	output := ""
	taskID := ""
	if result != nil {
		output = result.GetOutput()
		if result.TaskModel != nil {
			taskID = result.TaskModel.ID
		}
	}
	if deployment != nil && taskID != "" {
		attachDeploymentTaskID(ctx, j.Deps, deployment.ID, taskID)
		deployment.TaskID = &taskID
		j.broadcastDeployment("docker.database.deployment.running", deployment, "")
	}

	if runErr != nil || (result != nil && !result.IsSuccessful()) {
		errMsg := summariseDatabaseRunError(runErr, output)
		if deployment != nil {
			finalizeDeploymentRow(ctx, j.Deps, deployment.ID, dockertypes.DeploymentStatusFailed, errMsg)
			j.broadcastDeployment("docker.database.deployment.failed", deployment, errMsg)
		}
		j.markFailed(ctx, errMsg)
		return nil
	}

	_ = j.Deps.Repos.Database().UpdateFields(ctx, j.db.ID, map[string]any{
		"status": dockertypes.ApplicationStatusRunning,
	})
	if deployment != nil {
		finalizeDeploymentRow(ctx, j.Deps, deployment.ID, dockertypes.DeploymentStatusSuccess, "")
		j.broadcastDeployment("docker.database.deployment.succeeded", deployment, "")
	}
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
//
// Env composition (in this order so docker -e overrides go user-wins):
//  1. Engine-required creds  (POSTGRES_USER + ...)
//  2. User-added env vars    (docker_database_env_vars rows)
// Then every value is run through ResolveProjectRefs so
// `${{project.<KEY>}}` references expand against the project's
// current env-var set at run time.
func (j *RunDatabaseJob) buildRunConfig(ctx context.Context) (tasks.DatabaseRunConfig, error) {
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

	// Start with the engine-required env (creds), then append any
	// user-added rows. Errors here don't fail the run — we degrade
	// to "engine creds only" and log so an ops user can spot it.
	envVars := spec.EnvFn(creds)
	if j.Deps.Repos != nil {
		if extras, err := j.Deps.Repos.DatabaseEnvVar().ListForDatabase(ctx, j.db.ID); err == nil {
			for _, e := range extras {
				envVars = append(envVars, e.Key+"="+string(e.Value))
			}
		} else {
			j.Deps.Logger.Warn().Err(err).Str("database_id", j.db.ID).
				Msg("failed to load database env vars; running with engine creds only")
		}
	}

	// Resolve project-level references in the combined set. Loading
	// the map once + applying ResolveProjectRefsInPairs avoids a
	// per-pair lookup.
	projectEnv, perr := j.Deps.Repos.ProjectEnvVar().ListMapForProject(ctx, j.db.ProjectID)
	if perr != nil {
		j.Deps.Logger.Warn().Err(perr).Str("project_id", j.db.ProjectID).
			Msg("failed to load project env; ${{project.*}} refs will not resolve in database env")
		projectEnv = map[string]string{}
	}
	envVars = tasks.ResolveProjectRefsInPairs(envVars, projectEnv)

	cfg := tasks.DatabaseRunConfig{
		ContainerName: tasks.DatabaseContainerName(
			tasks.SlugFromName(j.project.Name),
			tasks.SlugFromName(j.db.Name),
		),
		Image:        image,
		EnvVars:      envVars,
		InternalPort: spec.InternalPort,
		ExternalPort: j.db.ExternalPort,
		// VolumeName is keyed off the database ID — that's the only
		// identifier that survives renames. DataPath comes from the
		// engine catalogue. Together they tell the run script to
		// bind-mount a persistent named volume at the engine's data
		// directory, so recreates (expose-toggle, advanced updates,
		// image bumps) don't wipe customer data.
		VolumeName: tasks.DatabaseVolumeName(j.db.ID),
		DataPath:   spec.DataPath,
		// Set by the "Rebuild Database" Danger Zone action — wipes
		// the volume between stop and start. All other lifecycle
		// flows pass false.
		WipeVolume: j.Payload.WipeVolume,
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

// broadcastDeployment is the per-job sugar for broadcastDeploymentEvent
// — keeps the call sites in Handle compact and injects the project ID
// (which the helper takes as a positional arg because the polymorphic
// Deployment row doesn't carry it directly).
func (j *RunDatabaseJob) broadcastDeployment(event string, d *models.Deployment, errMsg string) {
	projectID := ""
	if j.db != nil {
		projectID = j.db.ProjectID
	}
	broadcastDeploymentEvent(j.Deps, event, d, projectID, errMsg)
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
	// rm is special: the service soft-deletes the row BEFORE
	// dispatching the job, so by the time we hit this handler the
	// default-scoped lookup misses it. Try the scoped lookup first
	// (covers start/stop/restart); for rm, fall back to an unscoped
	// lookup so we can still read the row's name (needed to compose
	// the on-host container name).
	//
	// Previous bug: the scoped lookup returned NotFound on rm, j.db
	// stayed nil, and the container-name fallback at line below used
	// the database ID (a ULID like `01ks9gf8mg...`) as the slug. That
	// produced `docker rm launch-db-<project>-<ulid>` — a container
	// that has never existed — and the real container was left
	// running. The Containers tab on the server page then showed a
	// "deleted" database as a live container forever.
	j.db, err = j.Deps.Repos.Database().FindByIDAndTeamServer(
		ctx, j.Payload.DatabaseID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		if j.Payload.Action != "rm" {
			return fmt.Errorf("find database: %w", err)
		}
		// rm path: try the unscoped lookup so we can still read the
		// real name. Falling through with nil is OK as a last resort
		// — the script will run `docker rm <stale-name>` and exit
		// non-zero, but that's logged + non-fatal.
		if unscoped, uerr := j.Deps.Repos.Database().FindByIDUnscoped(ctx, j.Payload.DatabaseID); uerr == nil {
			j.db = unscoped
		}
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

	// Audit row in the shared docker_deployments table. Action carries
	// the verb (start/stop/restart/rm) so the Deployments tab UI can
	// render the right label without a separate enum.
	deployment, derr := createDeploymentRow(
		ctx, j.Deps, "database", j.Payload.DatabaseID, j.Payload.Action,
		j.Payload.TeamID, j.Payload.ServerID,
	)
	if derr != nil {
		j.Deps.Logger.Error().Err(derr).Msg("failed to create database lifecycle deployment row")
	}
	if deployment != nil {
		j.broadcastDeployment("docker.database.deployment.started", deployment, "")
	}

	dbName := j.Payload.DatabaseID
	if j.db != nil {
		dbName = j.db.Name
	}
	containerName := tasks.DatabaseContainerName(
		tasks.SlugFromName(j.project.Name),
		tasks.SlugFromName(dbName),
	)

	// On `rm` with RemoveVolume, ship the volume name into the script
	// so it can `docker volume rm` after the container is gone. We
	// compute the deterministic name here (not in the script) so the
	// host shell doesn't need to know the launch-db-<id>-data
	// convention.
	var volumeToRemove string
	if j.Payload.Action == "rm" && j.Payload.RemoveVolume {
		volumeToRemove = tasks.DatabaseVolumeName(j.Payload.DatabaseID)
	}

	task := taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Database %s", j.Payload.Action)),
		taskrunner.WithScript(tasks.DatabaseLifecycleScript(containerName, j.Payload.Action, volumeToRemove)),
		taskrunner.WithTimeoutSeconds(60),
	)
	result, runErr := j.Deps.RunTask(j.server, task).AsRoot().TrackInDB().Dispatch(ctx)
	taskID := ""
	output := ""
	if result != nil {
		output = result.GetOutput()
		if result.TaskModel != nil {
			taskID = result.TaskModel.ID
		}
	}
	if deployment != nil && taskID != "" {
		attachDeploymentTaskID(ctx, j.Deps, deployment.ID, taskID)
		deployment.TaskID = &taskID
		j.broadcastDeployment("docker.database.deployment.running", deployment, "")
	}

	if runErr != nil || (result != nil && !result.IsSuccessful()) {
		j.Deps.Logger.Warn().
			Str("database_id", j.Payload.DatabaseID).
			Str("action", j.Payload.Action).
			Msg("database lifecycle command failed (may be benign if container already absent)")
		errMsg := ""
		if runErr != nil {
			errMsg = runErr.Error()
		}
		if output != "" {
			if errMsg != "" {
				errMsg += "\n"
			}
			errMsg += output
		}
		if deployment != nil {
			finalizeDeploymentRow(ctx, j.Deps, deployment.ID, dockertypes.DeploymentStatusFailed, errMsg)
			j.broadcastDeployment("docker.database.deployment.failed", deployment, errMsg)
		}
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
	if deployment != nil {
		finalizeDeploymentRow(ctx, j.Deps, deployment.ID, dockertypes.DeploymentStatusSuccess, "")
		j.broadcastDeployment("docker.database.deployment.succeeded", deployment, "")
	}
	return nil
}

// broadcastDeployment is DatabaseLifecycleJob's sugar for
// broadcastDeploymentEvent — same shape as RunDatabaseJob's, just with
// the project id sourced from the payload (because the row may be
// soft-deleted by the time `rm` runs and we can't rely on j.db).
func (j *DatabaseLifecycleJob) broadcastDeployment(event string, d *models.Deployment, errMsg string) {
	broadcastDeploymentEvent(j.Deps, event, d, j.Payload.ProjectID, errMsg)
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

// NewRebuildDatabaseTask is the Danger-Zone rebuild variant of the
// run-database task: same job handler, but with `WipeVolume = true`
// so the script removes the named data volume between the stop and
// the start. The dedup key includes "rebuild" to keep it distinct from
// a regular recreate (expose-toggle) — back-to-back rebuild+recreate
// shouldn't get collapsed into one operation.
func NewRebuildDatabaseTask(databaseID, serverID, teamID string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeRunDatabase, RunDatabasePayload{
		DatabaseID: databaseID,
		ServerID:   serverID,
		TeamID:     teamID,
		WipeVolume: true,
	}, pkgjobs.Dedup("docker-rebuild-database", databaseID))
}

// NewDatabaseLifecycleTask packages the lifecycle asynq task. We
// include the action in the dedup key so back-to-back stop+start (a
// reboot, effectively) doesn't get collapsed into a single op.
//
// `removeVolume` only matters for action=="rm". Existing callers that
// don't care about the new flag should pass false; the volume cleanup
// is opt-in from the UI's Delete confirmation checkbox.
func NewDatabaseLifecycleTask(
	databaseID, projectID, serverID, teamID, action string,
	removeVolume bool,
) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeDatabaseLifecycle, DatabaseLifecyclePayload{
		DatabaseID:   databaseID,
		ProjectID:    projectID,
		ServerID:     serverID,
		TeamID:       teamID,
		Action:       action,
		RemoveVolume: removeVolume,
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
	// DataPath mirrors services.engineSpec.DataPath — the in-container
	// directory where the engine keeps its on-disk state. The run-task
	// bind-mounts a named volume here so data persists across recreates.
	DataPath string
	EnvFn    func(c jobsCredentials) []string
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
		DataPath:     "/var/lib/postgresql/data",
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
		DataPath:     "/var/lib/mysql",
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
		DataPath:     "/var/lib/mysql",
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
		DataPath:     "/data",
		EnvFn:        func(_ jobsCredentials) []string { return nil },
	},
	dockertypes.DatabaseEngineMongo: {
		Image:        "mongo",
		InternalPort: 27017,
		DataPath:     "/data/db",
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
	var last string
	for line := range strings.SplitSeq(output, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			last = trimmed
		}
	}
	if last == "" {
		return "database container failed to start"
	}
	if len(last) > 240 {
		return last[:240] + "…"
	}
	return last
}
