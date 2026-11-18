package jobs

import (
	"context"
	"errors"
	"github.com/hibiken/asynq"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"strings"
)

const TypeGHACleanup = "docker:gha_cleanup"

type GHACleanupPayload struct {
	CleanupID string `json:"cleanup_id"`
}
type GHACleanupJob struct {
	Deps    *JobDeps
	Payload GHACleanupPayload
}

func NewGHACleanupTask(id string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeGHACleanup, GHACleanupPayload{CleanupID: id})
}
func NewGHACleanupJob(payload GHACleanupPayload) pkgjobs.Handler {
	return &GHACleanupJob{Deps: deps, Payload: payload}
}
func (j *GHACleanupJob) Handle(ctx context.Context) error {
	capability, err := access.ResolveCleanup(ctx, j.Deps.DB, j.Deps.GitProviders, j.Payload.CleanupID)
	if err != nil {
		j.Failed(ctx, err)
		return err
	}
	gh, err := access.GitHubProvider(j.Deps.GitProviders)
	if err != nil {
		return err
	}
	names := strings.Split(capability.RepositoryFullName, "/")
	if len(names) != 2 {
		return errors.New("invalid cleanup repository")
	}
	scoped := capability.Context(ctx)
	var cleanupErrors []error
	for _, resource := range capability.Resources {
		switch resource.Kind {
		case "workflow":
			err = gh.DeleteContents(scoped, capability.InstallationID, names[0], names[1], resource.Name, resource.Branch, resource.ContentSHA256)
		case "secret":
			err = gh.DeleteActionsSecret(scoped, capability.InstallationID, names[0], names[1], resource.Name)
		case "variable":
			err = gh.DeleteActionsVariable(scoped, capability.InstallationID, names[0], names[1], resource.Name)
		default:
			err = errors.New("invalid cleanup resource")
		}
		if err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	if len(cleanupErrors) > 0 {
		_ = capability.Fail(context.WithoutCancel(ctx))
		return errors.Join(cleanupErrors...)
	}
	return capability.Complete(context.WithoutCancel(ctx))
}
func (j *GHACleanupJob) Failed(ctx context.Context, err error) {
	if j.Deps == nil || j.Deps.DB == nil {
		return
	}
	_ = j.Deps.DB.WithContext(ctx).Model(&gitmodels.GitHubCleanupManifest{}).Where("id = ? AND status IN ?", j.Payload.CleanupID, []string{"pending", "failed_cleanup"}).Update("status", "failed_cleanup").Error
}
