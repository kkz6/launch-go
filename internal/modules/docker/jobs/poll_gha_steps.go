package jobs

import (
	"context"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/hibiken/asynq"

	dockermodels "github.com/kkz6/launch-go/internal/modules/docker/models"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// TypePollGHASteps drives the live GitHub Actions step timeline (#87).
// Dispatched once when a GHA deploy is triggered, it re-enqueues itself
// every pollGHAStepsInterval until the run completes (or a deadline),
// broadcasting deployment.gha_steps on the team channel each tick so the
// deployment view can render a live timeline.
const TypePollGHASteps = "docker:poll_gha_steps"

const (
	// pollGHAStepsInterval is the cadence between step refreshes. 5s is
	// granular enough to feel live without burning the installation's
	// REST quota (one or two calls per tick).
	pollGHAStepsInterval = 5 * time.Second
	// pollGHAStepsDiscoveryDeadline bounds how long we hunt for the
	// just-dispatched run before giving up — if GitHub never shows a run
	// (dispatch raced a repo change, workflow errored at parse time) we
	// stop rather than poll forever.
	pollGHAStepsDiscoveryDeadline = 4 * time.Minute
	// pollGHAStepsHardDeadline caps the whole poll loop. Sits just past
	// the workflow's own 30-minute timeout so a wedged run can't keep us
	// polling indefinitely.
	pollGHAStepsHardDeadline = 35 * time.Minute
)

// PollGHAStepsPayload carries just enough to resume the poll across
// re-enqueues. RunID is empty on the first tick and filled once the run
// is discovered (or read off the deployment row's webhook-set value).
type PollGHAStepsPayload struct {
	TargetType       string `json:"target_type"` // "application" | "compose"
	TargetID         string `json:"target_id"`
	DeploymentID     string `json:"deployment_id"`
	RunID            string `json:"run_id,omitempty"`
	DispatchedAtUnix int64  `json:"dispatched_at_unix"`
	Attempt          int    `json:"attempt"`
}

// NewPollGHAStepsTask builds the asynq task. Dispatched by the deploy
// service right after it fires workflow_dispatch.
func NewPollGHAStepsTask(p PollGHAStepsPayload) (*asynq.Task, error) {
	return pkgjobs.Task(TypePollGHASteps, p)
}

// PollGHAStepsJob is the handler.
type PollGHAStepsJob struct {
	Deps    *JobDeps
	Payload PollGHAStepsPayload
}

func NewPollGHAStepsJob(p PollGHAStepsPayload) pkgjobs.Handler {
	return &PollGHAStepsJob{Deps: deps, Payload: p}
}

// Handle runs one poll tick: resolve the run, fetch its jobs/steps,
// broadcast them, and either stop (run done / deadline) or re-enqueue.
//
// The job is deliberately forgiving: any resolution/transient error
// ends the tick quietly (re-enqueuing while within the deadline) rather
// than failing the asynq task, because a live step timeline is a
// best-effort nicety — it must never spam retries or wedge the queue.
func (j *PollGHAStepsJob) Handle(ctx context.Context) error {
	if j.Deps == nil || j.Deps.GitProviders == nil {
		return nil
	}
	p := j.Payload
	dispatchedAt := time.Unix(p.DispatchedAtUnix, 0)

	if time.Since(dispatchedAt) > pollGHAStepsHardDeadline {
		return nil
	}

	var dep dockermodels.Deployment
	if err := j.Deps.DB.WithContext(ctx).Where("id = ?", p.DeploymentID).First(&dep).Error; err != nil {
		// Row deleted (or never persisted) → nothing to stream to.
		return nil
	}

	gh, gctx, ok := j.resolveContext(ctx, p.TargetType, p.TargetID)
	if !ok {
		return nil
	}

	runID := p.RunID
	if runID == "" {
		// The webhook may have already recorded the run id; prefer it.
		if dep.GHARunID != nil && *dep.GHARunID != "" {
			runID = *dep.GHARunID
		} else {
			run, err := gh.FindLatestWorkflowRun(ctx, gctx.installationID, gctx.owner, gctx.repo, gctx.workflowFile, gctx.branch, dispatchedAt)
			if err != nil || run == nil {
				// Not visible yet (or transient) — keep hunting until the
				// discovery deadline, then give up.
				if time.Since(dispatchedAt) <= pollGHAStepsDiscoveryDeadline {
					j.reschedule(p, "")
				}
				return nil
			}
			runID = fmt.Sprintf("%d", run.ID)
			// Surface the GitHub run link early (the webhook will set the
			// same value later). We intentionally do NOT write gha_run_id
			// here — that stays the webhook's single source of truth so a
			// mis-discovered run can never strand the webhook's row (#92).
			if dep.GHARunURL == nil || *dep.GHARunURL == "" {
				if err := j.Deps.DB.WithContext(ctx).Model(&dep).Update("gha_run_url", run.HTMLURL).Error; err != nil {
					j.Deps.Logger.Warn().Err(err).Str("deployment_id", dep.ID).Msg("failed to persist GitHub Actions run URL")
				}
			}
		}
	}

	wfJobs, err := gh.ListWorkflowRunJobs(ctx, gctx.installationID, gctx.owner, gctx.repo, runID)
	if err != nil {
		// Transient — try again within the deadline.
		j.reschedule(p, runID)
		return nil
	}

	j.broadcastSteps(&dep, runID, wfJobs)

	if jobsAllCompleted(wfJobs) {
		return nil
	}
	j.reschedule(p, runID)
	return nil
}

// reschedule re-enqueues the next tick, carrying the (possibly newly
// discovered) run id forward so subsequent ticks skip discovery.
func (j *PollGHAStepsJob) reschedule(p PollGHAStepsPayload, runID string) {
	p.RunID = runID
	p.Attempt++
	if err := j.Deps.DispatchIn(TypePollGHASteps, p, pollGHAStepsInterval); err != nil && j.Deps.Logger != nil {
		j.Deps.Logger.Warn().Err(err).Str("deployment_id", p.DeploymentID).
			Msg("poll_gha_steps: failed to reschedule next tick")
	}
}

// ghaPollContext is the resolved GitHub coordinates for a workload.
type ghaPollContext struct {
	owner          string
	repo           string
	workflowFile   string
	branch         string
	installationID string
}

// resolveContext loads the workload's source_config and resolves the
// GitHub provider + owner/repo/workflow/branch/installation. Returns
// ok=false on any failure (caller stops quietly).
func (j *PollGHAStepsJob) resolveContext(ctx context.Context, targetType, targetID string) (*gitproviders.GitHubProvider, ghaPollContext, bool) {
	var sc map[string]any
	switch targetType {
	case "application":
		var app dockermodels.Application
		if err := j.Deps.DB.WithContext(ctx).Where("id = ?", targetID).First(&app).Error; err != nil {
			return nil, ghaPollContext{}, false
		}
		sc = map[string]any(app.SourceConfig)
	case "compose":
		var c dockermodels.Compose
		if err := j.Deps.DB.WithContext(ctx).Where("id = ?", targetID).First(&c).Error; err != nil {
			return nil, ghaPollContext{}, false
		}
		sc = map[string]any(c.SourceConfig)
	default:
		return nil, ghaPollContext{}, false
	}

	cfg, err := parseGHASourceConfig(sc)
	if err != nil || cfg.Owner == "" || cfg.Repo == "" {
		return nil, ghaPollContext{}, false
	}

	installationID, err := j.resolvePollInstallationID(ctx, cfg.SourceControlID)
	if err != nil {
		return nil, ghaPollContext{}, false
	}

	provider, err := j.Deps.GitProviders.GetProvider(gitproviders.GitProviderType(gittypes.GitProviderGitHub))
	if err != nil {
		return nil, ghaPollContext{}, false
	}
	gh, ok := provider.(*gitproviders.GitHubProvider)
	if !ok {
		return nil, ghaPollContext{}, false
	}

	branch := cfg.Branch
	if branch == "" {
		branch = "main"
	}
	return gh, ghaPollContext{
		owner:          cfg.Owner,
		repo:           cfg.Repo,
		workflowFile:   path.Base(cfg.WorkflowPath),
		branch:         branch,
		installationID: installationID,
	}, true
}

func (j *PollGHAStepsJob) resolvePollInstallationID(ctx context.Context, sourceControlID string) (string, error) {
	if sourceControlID == "" {
		return "", errors.New("source_control_id missing")
	}
	var sc gitmodels.SourceControl
	if err := j.Deps.DB.WithContext(ctx).Where("id = ?", sourceControlID).First(&sc).Error; err != nil {
		return "", err
	}
	if sc.InstallationID == nil || *sc.InstallationID == "" {
		return "", errors.New("source_control has no installation_id")
	}
	return *sc.InstallationID, nil
}

// broadcastSteps emits the current job/step snapshot on the team channel.
// Docker deploy events ride the team channel with a deployment_id in the
// payload (the frontend filters on it), so the step stream follows suit.
func (j *PollGHAStepsJob) broadcastSteps(dep *dockermodels.Deployment, runID string, wfJobs []gitproviders.WorkflowJob) {
	if j.Deps.Broadcaster == nil || dep.TeamID == "" {
		return
	}
	j.Deps.Broadcaster.BroadcastToTeam(dep.TeamID, "deployment.gha_steps", map[string]any{
		"deployment_id": dep.ID,
		"target_type":   dep.TargetType,
		"target_id":     dep.TargetID,
		"team_id":       dep.TeamID,
		"run_id":        runID,
		"run_status":    overallRunStatus(wfJobs),
		"jobs":          ghaJobsToPayload(wfJobs),
	})
}

// ghaJobsToPayload flattens provider jobs/steps into JSON-friendly maps.
func ghaJobsToPayload(wfJobs []gitproviders.WorkflowJob) []map[string]any {
	out := make([]map[string]any, 0, len(wfJobs))
	for _, jb := range wfJobs {
		steps := make([]map[string]any, 0, len(jb.Steps))
		for _, s := range jb.Steps {
			steps = append(steps, map[string]any{
				"name":         s.Name,
				"status":       s.Status,
				"conclusion":   s.Conclusion,
				"number":       s.Number,
				"started_at":   s.StartedAt,
				"completed_at": s.CompletedAt,
			})
		}
		out = append(out, map[string]any{
			"name":       jb.Name,
			"status":     jb.Status,
			"conclusion": jb.Conclusion,
			"html_url":   jb.HTMLURL,
			"steps":      steps,
		})
	}
	return out
}

// jobsAllCompleted is true when there's at least one job and every job
// has reached the "completed" status — i.e. the run is done and we can
// stop polling.
func jobsAllCompleted(wfJobs []gitproviders.WorkflowJob) bool {
	if len(wfJobs) == 0 {
		return false
	}
	for _, jb := range wfJobs {
		if jb.Status != "completed" {
			return false
		}
	}
	return true
}

// overallRunStatus rolls the per-job statuses into one run-level value
// for the UI header: "completed" only when all jobs are done, otherwise
// "in_progress" if any has started, else "queued".
func overallRunStatus(wfJobs []gitproviders.WorkflowJob) string {
	if len(wfJobs) == 0 {
		return "queued"
	}
	allCompleted := true
	anyInProgress := false
	for _, jb := range wfJobs {
		if jb.Status != "completed" {
			allCompleted = false
		}
		if jb.Status == "in_progress" {
			anyInProgress = true
		}
	}
	switch {
	case allCompleted:
		return "completed"
	case anyInProgress:
		return "in_progress"
	default:
		return "queued"
	}
}
