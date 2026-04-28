package services

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/dto"
	"github.com/kkz6/launch-go/internal/modules/dockerapp/jobs"
	"github.com/kkz6/launch-go/internal/modules/dockerapp/models"
	dockerapptasks "github.com/kkz6/launch-go/internal/modules/dockerapp/tasks"
	"github.com/kkz6/launch-go/internal/modules/dockerapp/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

// ListByServer returns every app for a server.
// Signature matches IndexNestedFunc: (ctx, serverID, teamID).
func (s *Service) ListByServer(ctx context.Context, serverID, teamID string) ([]dto.AppResponse, error) {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	apps, err := s.repos.App().FindByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return dto.ToAppResponseList(apps), nil
}

// Get returns a single app by ID with sub-resources preloaded.
// Signature matches ShowNestedFunc: (ctx, id, serverID, teamID).
func (s *Service) Get(ctx context.Context, id, serverID, teamID string) (dto.AppResponse, error) {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return dto.AppResponse{}, err
	}
	app, err := s.repos.App().FindByIDWithRelations(ctx, id)
	if err != nil {
		return dto.AppResponse{}, err
	}
	if app.ServerID != serverID {
		return dto.AppResponse{}, fmt.Errorf("app does not belong to server")
	}
	return dto.ToAppResponse(app), nil
}

// Create persists a new application and dispatches the deploy job.
// Signature matches CreateNestedFunc: (ctx, serverID, teamID, userID, req).
func (s *Service) Create(ctx context.Context, serverID, teamID, userID string, req *dto.CreateAppRequest) (dto.AppResponse, error) {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return dto.AppResponse{}, err
	}

	if existing, err := s.repos.App().FindByNameAndServer(ctx, req.Name, serverID); err != nil {
		return dto.AppResponse{}, err
	} else if existing != nil {
		return dto.AppResponse{}, ErrNameTaken
	}

	tag := req.Tag
	if tag == "" {
		tag = "latest"
	}
	policy := req.RestartPolicy
	if policy == "" {
		policy = types.RestartUnlessStopped
	}
	source := req.Source
	if source == "" {
		source = types.SourceImage
	}
	if !source.IsValid() {
		return dto.AppResponse{}, fmt.Errorf("invalid source: %s", source)
	}

	app := &models.App{
		Name:                 req.Name,
		Source:               source,
		Image:                req.Image,
		Tag:                  tag,
		RegistryCredentialID: req.RegistryCredentialID,
		RestartPolicy:        policy,
		Status:               types.StatusPending,
	}
	app.ServerID = serverID
	app.TeamID = teamID

	if source == types.SourceCompose {
		if req.ComposeYAML == nil || *req.ComposeYAML == "" {
			return dto.AppResponse{}, fmt.Errorf("compose_yaml is required for compose source")
		}
		yaml := *req.ComposeYAML
		app.ComposeYAML = &yaml
		if req.ComposeEnv != nil {
			env := dbtype.EncryptedString(*req.ComposeEnv)
			app.ComposeEnv = &env
		}
		// Image fields are optional for compose. Keep them blank.
		app.Image = ""
	}

	if err := s.repos.App().Create(ctx, app); err != nil {
		return dto.AppResponse{}, fmt.Errorf("failed to create app: %w", err)
	}

	activity.RecordEvent(ctx, "created", userID, app, fmt.Sprintf("Application %s created", app.Name))
	s.dispatchDeploy(app, userID)

	return dto.ToAppResponse(app), nil
}

// Update applies partial changes and (optionally) redeploys.
// Signature matches UpdateNestedFunc: (ctx, id, serverID, teamID, userID, req).
func (s *Service) Update(ctx context.Context, id, serverID, teamID, userID string, req *dto.UpdateAppRequest) (dto.AppResponse, error) {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return dto.AppResponse{}, err
	}
	app, err := s.repos.App().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return dto.AppResponse{}, err
	}

	updates := map[string]any{}
	if req.Image != nil {
		updates["image"] = *req.Image
	}
	if req.Tag != nil {
		updates["tag"] = *req.Tag
	}
	if req.RegistryCredentialID != nil {
		// An empty string means "clear" — store NULL.
		if *req.RegistryCredentialID == "" {
			updates["registry_credential_id"] = nil
		} else {
			updates["registry_credential_id"] = *req.RegistryCredentialID
		}
	}
	if req.RestartPolicy != nil {
		updates["restart_policy"] = *req.RestartPolicy
	}
	if req.ComposeYAML != nil {
		updates["compose_yaml"] = *req.ComposeYAML
	}
	if req.ComposeEnv != nil {
		// Encrypted at rest.
		env := dbtype.EncryptedString(*req.ComposeEnv)
		updates["compose_env"] = env
	}

	if len(updates) > 0 {
		if err := s.repos.App().Update(ctx, app.ID, updates); err != nil {
			return dto.AppResponse{}, fmt.Errorf("failed to update app: %w", err)
		}
	}

	app, err = s.repos.App().FindByIDWithRelations(ctx, app.ID)
	if err != nil {
		return dto.AppResponse{}, err
	}
	activity.RecordEvent(ctx, "updated", userID, app, fmt.Sprintf("Application %s settings updated", app.Name))
	return dto.ToAppResponse(app), nil
}

// Delete enqueues an uninstall.
// Signature matches DeleteNestedFunc: (ctx, id, serverID, teamID, userID).
func (s *Service) Delete(ctx context.Context, id, serverID, teamID, userID string) error {
	return s.UninstallWithOptions(ctx, id, serverID, teamID, userID, false)
}

// UninstallWithOptions removes the application, optionally dropping its
// named volumes. Used by the explicit uninstall handler so the UI can
// surface the "remove data" toggle.
func (s *Service) UninstallWithOptions(ctx context.Context, id, serverID, teamID, userID string, removeData bool) error {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return err
	}
	app, err := s.repos.App().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}
	if err := s.repos.App().Update(ctx, app.ID, map[string]any{
		"status": types.StatusDeploying, // reuse for "in-progress" UI badge
	}); err != nil {
		return err
	}
	activity.RecordEvent(ctx, "uninstalling", userID, app, fmt.Sprintf("Application %s uninstall requested", app.Name))
	s.dispatchUninstall(app, removeData, userID)
	return nil
}

// Deploy triggers a redeploy of the existing app.
// Signature matches ActionItemNestedFunc: (ctx, id, serverID, teamID, userID).
func (s *Service) Deploy(ctx context.Context, id, serverID, teamID, userID string) error {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return err
	}
	app, err := s.repos.App().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}
	if err := s.repos.App().Update(ctx, app.ID, map[string]any{"status": types.StatusPending}); err != nil {
		return err
	}
	activity.RecordEvent(ctx, "redeployed", userID, app, fmt.Sprintf("Application %s redeploy requested", app.Name))
	s.dispatchDeploy(app, userID)
	return nil
}

// Start runs `docker start`.
// Signature matches ActionItemNestedFunc.
func (s *Service) Start(ctx context.Context, id, serverID, teamID, userID string) error {
	return s.runLifecycle(ctx, id, serverID, teamID, userID, dockerapptasks.LifecycleStart, types.StatusRunning, "started")
}

// Stop runs `docker stop`.
func (s *Service) Stop(ctx context.Context, id, serverID, teamID, userID string) error {
	return s.runLifecycle(ctx, id, serverID, teamID, userID, dockerapptasks.LifecycleStop, types.StatusStopped, "stopped")
}

// Restart runs `docker restart`.
func (s *Service) Restart(ctx context.Context, id, serverID, teamID, userID string) error {
	return s.runLifecycle(ctx, id, serverID, teamID, userID, dockerapptasks.LifecycleRestart, types.StatusRunning, "restarted")
}

// Logs runs `docker logs --tail N` synchronously and returns the output.
func (s *Service) Logs(ctx context.Context, id, serverID, teamID string, tail int) (string, error) {
	server, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return "", err
	}
	app, err := s.repos.App().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return "", err
	}
	if app.Status.IsBusy() {
		return "", ErrBusy
	}

	if app.Source == types.SourceCompose {
		composeTask := dockerapptasks.ComposeLogs(dockerapptasks.ComposeLogsOptions{
			AppName: app.Name,
			Project: app.ComposeProject(),
			Tail:    tail,
		})
		result, err := s.runnerDeps.RunTask(ctx, server, composeTask, true)
		if err != nil {
			return "", fmt.Errorf("failed to fetch logs: %w", err)
		}
		return result.GetOutput(), nil
	}

	imageTask := dockerapptasks.Logs(dockerapptasks.LogsOptions{
		AppName:   app.Name,
		Container: app.Container(),
		Tail:      tail,
	})
	result, err := s.runnerDeps.RunTask(ctx, server, imageTask, true)
	if err != nil {
		return "", fmt.Errorf("failed to fetch logs: %w", err)
	}
	return result.GetOutput(), nil
}

func (s *Service) runLifecycle(
	ctx context.Context,
	id, serverID, teamID, userID string,
	action dockerapptasks.LifecycleAction,
	endStatus types.Status,
	verb string,
) error {
	server, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}
	app, err := s.repos.App().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}
	if app.Status.IsBusy() {
		return ErrBusy
	}
	if app.DeployedAt == nil {
		return ErrNotDeployed
	}

	var result interface {
		IsSuccessful() bool
		GetOutput() string
	}
	if app.Source == types.SourceCompose {
		composeTask := dockerapptasks.ComposeLifecycle(dockerapptasks.ComposeLifecycleOptions{
			AppName: app.Name,
			Project: app.ComposeProject(),
			Action:  action,
		})
		r, err := s.runnerDeps.RunTask(ctx, server, composeTask, true)
		if err != nil {
			return fmt.Errorf("failed to %s app: %w", action, err)
		}
		result = r
	} else {
		imageTask := dockerapptasks.Lifecycle(dockerapptasks.LifecycleOptions{
			AppName:   app.Name,
			Container: app.Container(),
			Action:    action,
		})
		r, err := s.runnerDeps.RunTask(ctx, server, imageTask, true)
		if err != nil {
			return fmt.Errorf("failed to %s app: %w", action, err)
		}
		result = r
	}
	if !result.IsSuccessful() {
		return fmt.Errorf("%s script failed: %s", action, result.GetOutput())
	}

	if err := s.repos.App().Update(ctx, app.ID, map[string]any{
		"status":     endStatus,
		"last_error": nil,
	}); err != nil {
		return fmt.Errorf("failed to update app status: %w", err)
	}
	activity.RecordEvent(ctx, verb, userID, app, fmt.Sprintf("Application %s %s", app.Name, verb))
	return nil
}

func (s *Service) dispatchDeploy(app *models.App, userID string) {
	uid := userIDPtr(userID)
	s.DispatchTask("DeployDockerApp", func() (*asynq.Task, error) {
		return jobs.NewDeployTask(app.ID, uid)
	}, "app_id", app.ID)
}

func (s *Service) dispatchUninstall(app *models.App, removeData bool, userID string) {
	uid := userIDPtr(userID)
	s.DispatchTask("UninstallDockerApp", func() (*asynq.Task, error) {
		return jobs.NewUninstallTask(app.ID, removeData, uid)
	}, "app_id", app.ID)
}

func userIDPtr(userID string) *string {
	if userID == "" {
		return nil
	}
	return &userID
}
