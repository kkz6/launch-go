package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DatabaseService owns CRUD + lifecycle for managed database
// containers (Postgres / MySQL / MariaDB / Redis / Mongo).
//
// Different from ApplicationService in two ways:
//  1. Credentials are auto-generated on create and persisted encrypted.
//  2. There's no "deploy" lifecycle — start / stop / restart are first-
//     class actions. Lifecycle calls dispatch an asynq job that runs
//     the corresponding `docker` subcommand via SSH.
type DatabaseService struct {
	*BaseService
}

func NewDatabaseService(deps *ServiceDeps) *DatabaseService {
	return &DatabaseService{BaseService: NewBaseService(deps)}
}

// ListDatabases returns databases in a project. Credentials are NOT
// returned in list responses — the user has to open the detail page to
// view them, which forces an explicit reveal action.
func (s *DatabaseService) ListDatabases(
	ctx context.Context, projectID, serverID, teamID string,
) ([]dto.DatabaseResponse, error) {
	if _, err := s.requireProjectForDB(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().Database().ListForProject(ctx, teamID, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.DatabaseResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToDatabaseResponse(&rows[i], false))
	}
	return out, nil
}

// ListDatabasesForServer returns every docker database on the server
// (across all projects on it) scoped to the caller's team. Used by
// the backup-restore dialog's "target database" picker, where the
// user might be redirecting a prod snapshot to a staging row that
// lives in a different project. Credentials are stripped — same
// rule as the project-scoped list.
func (s *DatabaseService) ListDatabasesForServer(
	ctx context.Context, serverID, teamID string,
) ([]dto.DatabaseResponse, error) {
	// Defence-in-depth: confirm the server belongs to the caller's
	// team before listing. The repo query already filters by team_id,
	// but a 404 on the server up-front is a cleaner error than an
	// empty list.
	if _, err := s.ServerRepos().Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().Database().ListForServer(ctx, teamID, serverID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.DatabaseResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToDatabaseResponse(&rows[i], false))
	}
	return out, nil
}

// GetDatabase returns a single database. If revealCreds is true the
// credentials are included — the handler sets this based on whether
// the caller passed ?reveal=true.
func (s *DatabaseService) GetDatabase(
	ctx context.Context, id, projectID, serverID, teamID string, revealCreds bool,
) (dto.DatabaseResponse, error) {
	// Load the project once here so we can stamp the deterministic
	// container name onto the response. The Terminal button on the
	// workload detail page reads `container_name` and forwards it to
	// the WS handler as `?container=...`, which makes the bottom
	// pane attach to the database container rather than the host
	// root shell.
	project, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	d, err := s.Repos().Database().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	if d.ProjectID != projectID {
		return dto.DatabaseResponse{}, fiberutil.NotFound()
	}
	resp := dto.ToDatabaseResponse(d, revealCreds)
	resp.ContainerName = tasks.DatabaseContainerName(
		tasks.SlugFromName(project.Name),
		tasks.SlugFromName(d.Name),
	)
	if revealCreds {
		creds, err := decodeCredentials(d.Credentials)
		if err == nil {
			resp.Credentials = &dto.DatabaseCredentials{
				Username: creds.Username,
				Password: creds.Password,
				Database: creds.Database,
			}
		}
	}
	return *resp, nil
}

// CreateDatabase generates credentials, persists the row, and dispatches
// the asynq job that actually `docker run`s the container.
func (s *DatabaseService) CreateDatabase(
	ctx context.Context, projectID, serverID, teamID, userID string,
	req *dto.CreateDatabaseRequest,
) (dto.DatabaseResponse, error) {
	_ = userID
	if _, err := s.requireProjectForDB(ctx, projectID, serverID, teamID); err != nil {
		return dto.DatabaseResponse{}, err
	}

	engine := dockertypes.DatabaseEngine(req.Engine)
	spec, ok := engineSpecFor(engine)
	if !ok {
		return dto.DatabaseResponse{}, fiberutil.BadRequest("Unsupported database engine")
	}

	name := strings.TrimSpace(req.Name)
	if !validDBNameRe.MatchString(name) {
		return dto.DatabaseResponse{}, fiberutil.BadRequest(
			"Database name must start with a letter and contain only letters, digits, underscores, or hyphens",
		)
	}
	taken, err := s.Repos().Database().ExistsByNameInProject(ctx, projectID, name, "")
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	if taken {
		return dto.DatabaseResponse{}, fiberutil.Conflict(
			"A database with that name already exists in this project",
		)
	}

	version := req.Version
	if version == "" {
		version = spec.DefaultVersion
	}
	if !contains(spec.SupportedVersions, version) {
		return dto.DatabaseResponse{}, fiberutil.BadRequest(
			fmt.Sprintf("Unsupported version %q for %s", version, engine),
		)
	}

	creds, err := generateCredentials(name)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	encoded, err := json.Marshal(creds)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}

	d := &models.Database{
		ProjectID:     projectID,
		Name:          name,
		Engine:        engine,
		EngineVersion: version,
		ImageTag:      strPtr(fmt.Sprintf("%s:%s", spec.Image, version)),
		ExternalPort:  req.ExternalPort,
		Credentials:   dbtype.EncryptedString(string(encoded)),
		Status:        dockertypes.ApplicationStatusIdle,
	}
	d.TeamID = teamID
	d.ServerID = serverID

	if err := s.Repos().Database().Create(ctx, d); err != nil {
		return dto.DatabaseResponse{}, err
	}

	task, err := jobs.NewRunDatabaseTask(d.ID, serverID, teamID)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	if err := s.EnqueueTask(task); err != nil {
		// Failure to enqueue → mark failed so the UI shows the dispatch
		// error rather than a forever-spinner.
		_ = s.Repos().Database().UpdateFields(ctx, d.ID, map[string]any{
			"status": dockertypes.ApplicationStatusFailed,
		})
		return dto.DatabaseResponse{}, err
	}

	resp := dto.ToDatabaseResponse(d, false)
	s.BroadcastToTeam(teamID, "docker.database.created", resp)
	return *resp, nil
}

// DeleteDatabase removes the docker container and soft-deletes the
// row. We dispatch the rm-container job and immediately soft-delete
// the row — if the rm fails the row is already gone from the UI; an
// operator can manually remove the container.
func (s *DatabaseService) DeleteDatabase(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
	removeVolume bool,
) error {
	_ = userID
	if _, err := s.requireProjectForDB(ctx, projectID, serverID, teamID); err != nil {
		return err
	}
	d, err := s.Repos().Database().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return err
	}
	if d.ProjectID != projectID {
		return fiberutil.NotFound()
	}
	if err := s.Repos().Database().Delete(ctx, id); err != nil {
		return err
	}
	// Pass removeVolume into the lifecycle job; when true the worker
	// will compute the deterministic launch-db-<id>-data volume name
	// and `docker volume rm` it after the container is gone.
	task, err := jobs.NewDatabaseLifecycleTask(d.ID, d.ProjectID, serverID, teamID, "rm", removeVolume)
	if err == nil {
		_ = s.EnqueueTask(task)
	}
	s.BroadcastToTeam(teamID, "docker.database.deleted", map[string]any{
		"id":         d.ID,
		"project_id": d.ProjectID,
		"server_id":  d.ServerID,
		"team_id":    d.TeamID,
	})
	return nil
}

// ListDeployments returns the lifecycle history for a database, most
// recent first. Reads from docker_deployments with target_type =
// "database" — same polymorphic table that powers application + compose
// deploy history. Action column on each row (create / start / restart /
// stop / rm) tells the UI what verb the entry represents.
func (s *DatabaseService) ListDeployments(
	ctx context.Context, databaseID, projectID, serverID, teamID string,
) ([]models.Deployment, error) {
	if _, err := s.requireProjectForDB(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	d, err := s.Repos().Database().FindByIDAndTeamServer(ctx, databaseID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if d.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	return s.Repos().Deployment().ListForTarget(ctx, "database", databaseID)
}

// Lifecycle dispatches a start/stop/restart job for the database.
// Returns the same database row so the UI can show the optimistic
// "transitioning" state immediately.
func (s *DatabaseService) Lifecycle(
	ctx context.Context, id, projectID, serverID, teamID, userID, action string,
) (dto.DatabaseResponse, error) {
	_ = userID
	if action != "start" && action != "stop" && action != "restart" {
		return dto.DatabaseResponse{}, fiberutil.BadRequest("Unsupported lifecycle action")
	}
	if _, err := s.requireProjectForDB(ctx, projectID, serverID, teamID); err != nil {
		return dto.DatabaseResponse{}, err
	}
	d, err := s.Repos().Database().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	if d.ProjectID != projectID {
		return dto.DatabaseResponse{}, fiberutil.NotFound()
	}

	// "restart" RECREATES the container from the existing image with the
	// current env (engine credentials + user-added env vars), preserving
	// the data volume — so saved runtime env changes are applied. A plain
	// `docker restart` keeps the env baked in at container-create time.
	// RunDatabaseJob (WipeVolume=false) is idempotent: stop + rm the old
	// container, keep the named volume, re-run.
	//
	// CAVEAT: engine credential env (e.g. POSTGRES_PASSWORD) is only
	// honoured on FIRST init against an empty data dir, so this does NOT
	// rotate an existing database's password — that needs a dedicated
	// change-credentials action (engine ALTER USER), tracked separately.
	switch action {
	case "restart":
		task, err := jobs.NewRunDatabaseTask(d.ID, serverID, teamID)
		if err != nil {
			return dto.DatabaseResponse{}, err
		}
		if err := s.EnqueueTask(task); err != nil {
			return dto.DatabaseResponse{}, err
		}
	default: // start, stop — quick docker start/stop, no recreate.
		task, err := jobs.NewDatabaseLifecycleTask(d.ID, d.ProjectID, serverID, teamID, action, false)
		if err != nil {
			return dto.DatabaseResponse{}, err
		}
		if err := s.EnqueueTask(task); err != nil {
			return dto.DatabaseResponse{}, err
		}
	}

	s.BroadcastToTeam(teamID, "docker.database.lifecycle", map[string]any{
		"id":         d.ID,
		"project_id": d.ProjectID,
		"server_id":  d.ServerID,
		"team_id":    d.TeamID,
		"action":     action,
	})
	return *dto.ToDatabaseResponse(d, false), nil
}

// UpdateDatabaseAdvanced applies the Advanced subtab's runtime knobs
// to a managed database container — restart policy + resource limits.
// Persists them into build_config so the values survive container
// recreation, then dispatches an asynq job that runs the bundled
// `docker update` over SSH.
//
// Empty resource strings clear that knob (docker treats absence of
// the flag as "leave unchanged", so the persisted nil acts like an
// opt-out for fresh `docker run` invocations).
func (s *DatabaseService) UpdateDatabaseAdvanced(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
	req *dto.UpdateDatabaseAdvancedRequest,
) (dto.DatabaseResponse, error) {
	_ = userID
	switch req.RestartPolicy {
	case "no", "on-failure", "always", "unless-stopped":
	default:
		return dto.DatabaseResponse{}, fiberutil.BadRequest("Unsupported restart policy")
	}
	if _, err := s.requireProjectForDB(ctx, projectID, serverID, teamID); err != nil {
		return dto.DatabaseResponse{}, err
	}
	db, err := s.Repos().Database().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	if db.ProjectID != projectID {
		return dto.DatabaseResponse{}, fiberutil.NotFound()
	}

	// Persist into build_config. Start from the existing map so we
	// don't clobber unrelated keys a future iteration might add.
	build := map[string]any(db.BuildConfig)
	if build == nil {
		build = map[string]any{}
	}
	build["restart_policy"] = req.RestartPolicy
	setOrClear(build, "cpu_limit", req.CPULimit)
	setOrClear(build, "memory_limit", req.MemoryLimit)
	setOrClear(build, "cpu_reservation", req.CPUReservation)
	setOrClear(build, "memory_reservation", req.MemoryReservation)

	if err := s.Repos().Database().UpdateFields(ctx, db.ID, map[string]any{
		"build_config": dbtype.JSONMap(build),
	}); err != nil {
		return dto.DatabaseResponse{}, err
	}

	// JSON-encode the advanced update + dispatch through the existing
	// lifecycle job. The task script reads `update-advanced:<json>` and
	// builds a single `docker update` invocation with the active flags.
	advBytes, err := json.Marshal(tasks.DatabaseAdvancedUpdate{
		RestartPolicy:     req.RestartPolicy,
		CPULimit:          req.CPULimit,
		MemoryLimit:       req.MemoryLimit,
		CPUReservation:    req.CPUReservation,
		MemoryReservation: req.MemoryReservation,
	})
	if err != nil {
		return dto.DatabaseResponse{}, err
	}

	task, err := jobs.NewDatabaseLifecycleTask(
		db.ID, db.ProjectID, serverID, teamID, "update-advanced:"+string(advBytes), false,
	)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	if err := s.EnqueueTask(task); err != nil {
		return dto.DatabaseResponse{}, err
	}

	s.BroadcastToTeam(teamID, "docker.database.advanced.updated", map[string]any{
		"id":             db.ID,
		"server_id":      db.ServerID,
		"team_id":        db.TeamID,
		"restart_policy": req.RestartPolicy,
	})

	// Re-pull so the response carries the new build_config.
	reloaded, err := s.Repos().Database().FindByIDAndTeamServer(ctx, db.ID, teamID, serverID)
	if err != nil {
		return *dto.ToDatabaseResponse(db, false), nil
	}
	return *dto.ToDatabaseResponse(reloaded, false), nil
}

// SetExposeExternal toggles the database's external port. When Enabled
// is true, Port (defaulting to the engine's standard port) is mapped
// onto the host so the database is reachable from the open internet.
// When false, the external_port is cleared and the container is
// recreated without `-p` — sibling containers on launch-network can
// still reach it by DNS.
//
// RunDatabaseScript is idempotent (stops + removes any prior container
// of the same name), so dispatching RunDatabaseJob re-creates the
// container with the new port mapping in a single SSH round-trip.
func (s *DatabaseService) SetExposeExternal(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
	req *dto.SetDatabaseExposeRequest,
) (dto.DatabaseResponse, error) {
	_ = userID
	if _, err := s.requireProjectForDB(ctx, projectID, serverID, teamID); err != nil {
		return dto.DatabaseResponse{}, err
	}
	db, err := s.Repos().Database().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	if db.ProjectID != projectID {
		return dto.DatabaseResponse{}, fiberutil.NotFound()
	}

	updates := map[string]any{}
	if req.Enabled {
		port := 0
		if req.Port != nil && *req.Port > 0 {
			port = *req.Port
		} else if spec, ok := engineSpecFor(db.Engine); ok {
			port = spec.InternalPort
		}
		if port <= 0 {
			return dto.DatabaseResponse{}, fiberutil.BadRequest(
				"Port is required to expose this database",
			)
		}
		updates["external_port"] = port
	} else {
		// Use a nil *int via gorm's column-clear convention so the
		// MySQL row's external_port goes back to NULL.
		updates["external_port"] = nil
	}

	if err := s.Repos().Database().UpdateFields(ctx, db.ID, updates); err != nil {
		return dto.DatabaseResponse{}, err
	}

	// Dispatch a recreate (RunDatabaseJob — idempotent). Worker picks
	// up the updated row, builds the run script with the new port (or
	// without -p when cleared), and the container comes back up with
	// the new mapping. WS broadcasts go out as the worker flips state.
	task, err := jobs.NewRunDatabaseTask(db.ID, serverID, teamID)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	if err := s.EnqueueTask(task); err != nil {
		return dto.DatabaseResponse{}, err
	}

	s.BroadcastToTeam(teamID, "docker.database.expose.updated", map[string]any{
		"id":            db.ID,
		"server_id":     db.ServerID,
		"team_id":       db.TeamID,
		"enabled":       req.Enabled,
		"external_port": updates["external_port"],
	})

	reloaded, err := s.Repos().Database().FindByIDAndTeamServer(ctx, db.ID, teamID, serverID)
	if err != nil {
		return *dto.ToDatabaseResponse(db, false), nil
	}
	return *dto.ToDatabaseResponse(reloaded, false), nil
}

// RebuildDatabase is the Danger Zone "wipe + recreate" action. Same
// container, same image, same credentials — fresh data volume. We
// dispatch a RunDatabaseJob with WipeVolume=true; the worker stops the
// container, `docker volume rm`s the named volume, then starts it
// again from scratch.
//
// All scoping (team / project / server) and the broadcast follow the
// SetExposeExternal shape so the deployments subtab and the navbar
// dot transition the same way.
func (s *DatabaseService) RebuildDatabase(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
) (dto.DatabaseResponse, error) {
	_ = userID
	if _, err := s.requireProjectForDB(ctx, projectID, serverID, teamID); err != nil {
		return dto.DatabaseResponse{}, err
	}
	db, err := s.Repos().Database().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	if db.ProjectID != projectID {
		return dto.DatabaseResponse{}, fiberutil.NotFound()
	}

	task, err := jobs.NewRebuildDatabaseTask(db.ID, serverID, teamID)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	if err := s.EnqueueTask(task); err != nil {
		return dto.DatabaseResponse{}, err
	}

	s.BroadcastToTeam(teamID, "docker.database.rebuild.queued", map[string]any{
		"id":         db.ID,
		"server_id":  db.ServerID,
		"team_id":    db.TeamID,
		"project_id": db.ProjectID,
	})

	return *dto.ToDatabaseResponse(db, false), nil
}

// setOrClear stores `value` under `key` when non-empty, deletes the
// key when empty. Lets the user toggle a knob off without leaving a
// stale string in the map.
func setOrClear(m map[string]any, key, value string) {
	if value == "" {
		delete(m, key)
		return
	}
	m[key] = value
}

// requireProjectForDB validates the project chain for database routes —
// kept named distinctly so the file's other helpers stay obvious in
// stack traces.
func (s *DatabaseService) requireProjectForDB(
	ctx context.Context, projectID, serverID, teamID string,
) (string, error) {
	p, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

// generateCredentials produces a username + password + database-name
// triple. The username defaults to "launch_user"; the password is 32
// bytes of crypto/rand encoded base64-url (no `+/` so it's safe in
// connection strings without escaping). Database name follows the
// docker-compose convention of matching the resource name.
func generateCredentials(dbName string) (Credentials, error) {
	pw := make([]byte, 24)
	if _, err := rand.Read(pw); err != nil {
		return Credentials{}, err
	}
	return Credentials{
		Username: "launch_user",
		Password: base64.RawURLEncoding.EncodeToString(pw),
		Database: dbName,
	}, nil
}

// decodeCredentials unmarshals the stored JSON blob. Returns a zero
// value + error if the blob is missing or corrupt — the caller treats
// either as "creds not available" rather than failing the whole
// request.
func decodeCredentials(raw dbtype.EncryptedString) (Credentials, error) {
	var c Credentials
	if string(raw) == "" {
		return c, fmt.Errorf("no credentials stored")
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return Credentials{}, err
	}
	return c, nil
}

// validDBNameRe constrains names to what works as both a docker
// container name suffix and a typical database identifier.
var validDBNameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,62}$`)

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func strPtr(s string) *string { return &s }
