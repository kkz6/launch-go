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

// GetDatabase returns a single database. If revealCreds is true the
// credentials are included — the handler sets this based on whether
// the caller passed ?reveal=true.
func (s *DatabaseService) GetDatabase(
	ctx context.Context, id, projectID, serverID, teamID string, revealCreds bool,
) (dto.DatabaseResponse, error) {
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
	resp := dto.ToDatabaseResponse(d, revealCreds)
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
	task, err := jobs.NewDatabaseLifecycleTask(d.ID, d.ProjectID, serverID, teamID, "rm")
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

	task, err := jobs.NewDatabaseLifecycleTask(d.ID, d.ProjectID, serverID, teamID, action)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	if err := s.EnqueueTask(task); err != nil {
		return dto.DatabaseResponse{}, err
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
