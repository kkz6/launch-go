package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// This file holds the read-only GitHub Actions endpoints Launch uses to
// surface a LIVE workflow-run step timeline in the deployment dashboard
// (#87). They mirror the write-side helpers in github_writes.go: resolve
// an installation token, hit the REST API, map the response into slim
// public structs the docker module consumes without importing GitHub's
// JSON shapes.

// WorkflowRunSummary is a slim view of a GitHub Actions run, used to
// discover the run a deploy dispatch kicked off (the workflow_dispatch
// API returns 204 with no body, so we find the run afterwards by
// workflow file + branch + creation time).
type WorkflowRunSummary struct {
	ID         int64
	Status     string // "queued" | "in_progress" | "completed"
	Conclusion string // "success" | "failure" | "cancelled" | … ("" while running)
	HTMLURL    string
	CreatedAt  time.Time
	Event      string
	HeadBranch string
}

// WorkflowStep is one step inside a job.
type WorkflowStep struct {
	Name        string
	Status      string // "queued" | "in_progress" | "completed"
	Conclusion  string // "success" | "failure" | "skipped" | "cancelled" | "" (not finished)
	Number      int
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// WorkflowJob is one job inside a run, carrying its ordered steps.
type WorkflowJob struct {
	ID          int64
	Name        string
	Status      string
	Conclusion  string
	StartedAt   *time.Time
	CompletedAt *time.Time
	HTMLURL     string
	Steps       []WorkflowStep
}

// --- private JSON decode shapes (named so revive's no-nested-structs
// rule is satisfied; mapped into the public structs above) ------------

type ghStepJSON struct {
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Conclusion  string     `json:"conclusion"`
	Number      int        `json:"number"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

type ghJobJSON struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name"`
	Status      string       `json:"status"`
	Conclusion  string       `json:"conclusion"`
	StartedAt   *time.Time   `json:"started_at"`
	CompletedAt *time.Time   `json:"completed_at"`
	HTMLURL     string       `json:"html_url"`
	Steps       []ghStepJSON `json:"steps"`
}

type ghJobsResponseJSON struct {
	Jobs []ghJobJSON `json:"jobs"`
}

type ghRunJSON struct {
	ID         int64     `json:"id"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	HTMLURL    string    `json:"html_url"`
	CreatedAt  time.Time `json:"created_at"`
	Event      string    `json:"event"`
	HeadBranch string    `json:"head_branch"`
}

type ghRunsResponseJSON struct {
	WorkflowRuns []ghRunJSON `json:"workflow_runs"`
}

// ListWorkflowRunJobs fetches the jobs (and their steps) for a workflow
// run, used to render the live step timeline. A 404 (run deleted or not
// yet visible) returns an empty slice + nil error so callers treat it as
// "nothing to show yet" rather than a hard failure.
//
// GitHub API: GET /repos/{owner}/{repo}/actions/runs/{run_id}/jobs
// docs: https://docs.github.com/en/rest/actions/workflow-jobs#list-jobs-for-a-workflow-run
// Requires the installation's `actions: read` permission (implied by the
// `actions: write` the deploy-dispatch flow already relies on).
func (p *GitHubProvider) ListWorkflowRunJobs(
	ctx context.Context,
	installationID, owner, repo, runID string,
) ([]WorkflowJob, error) {
	if runID == "" {
		return nil, fmt.Errorf("ListWorkflowRunJobs: runID is required")
	}

	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	apiPath := fmt.Sprintf("/repos/%s/%s/actions/runs/%s/jobs?per_page=100", owner, repo, runID)
	resp, err := p.DoRaw(ctx, http.MethodGet, apiPath, token, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// fall through to decode
	case http.StatusNotFound:
		return nil, nil
	case http.StatusForbidden:
		return nil, ErrPermissionDenied
	default:
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ListWorkflowRunJobs %s/%s/%s: status %d body %s", owner, repo, runID, resp.StatusCode, string(raw))
	}

	var body ghJobsResponseJSON
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("ListWorkflowRunJobs decode: %w", err)
	}

	jobs := make([]WorkflowJob, 0, len(body.Jobs))
	for _, j := range body.Jobs {
		steps := make([]WorkflowStep, 0, len(j.Steps))
		for _, s := range j.Steps {
			// ghStepJSON and WorkflowStep have identical fields (tags are
			// ignored for conversion), so a direct conversion is exact.
			steps = append(steps, WorkflowStep(s))
		}
		jobs = append(jobs, WorkflowJob{
			ID:          j.ID,
			Name:        j.Name,
			Status:      j.Status,
			Conclusion:  j.Conclusion,
			StartedAt:   j.StartedAt,
			CompletedAt: j.CompletedAt,
			HTMLURL:     j.HTMLURL,
			Steps:       steps,
		})
	}
	return jobs, nil
}

// FindLatestWorkflowRun returns the newest workflow_dispatch run for a
// given workflow file on a branch that was created at/after `since`
// (minus a small slack to tolerate clock skew). Used to attach the
// running build to a just-dispatched deploy so the step timeline can go
// live before the build's completion webhook arrives.
//
// Returns (nil, nil) when no matching run is visible yet — the caller
// retries on its next poll tick.
//
// GitHub API: GET /repos/{owner}/{repo}/actions/workflows/{workflow_file}/runs
// docs: https://docs.github.com/en/rest/actions/workflow-runs#list-workflow-runs-for-a-workflow
func (p *GitHubProvider) FindLatestWorkflowRun(
	ctx context.Context,
	installationID, owner, repo, workflowFile, branch string,
	since time.Time,
) (*WorkflowRunSummary, error) {
	if workflowFile == "" {
		return nil, fmt.Errorf("FindLatestWorkflowRun: workflowFile is required")
	}

	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("event", "workflow_dispatch")
	q.Set("per_page", "10")
	if branch != "" {
		q.Set("branch", branch)
	}
	apiPath := fmt.Sprintf(
		"/repos/%s/%s/actions/workflows/%s/runs?%s",
		owner, repo, url.PathEscape(workflowFile), q.Encode(),
	)
	resp, err := p.DoRaw(ctx, http.MethodGet, apiPath, token, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusNotFound:
		return nil, nil
	case http.StatusForbidden:
		return nil, ErrPermissionDenied
	default:
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("FindLatestWorkflowRun %s/%s/%s: status %d body %s", owner, repo, workflowFile, resp.StatusCode, string(raw))
	}

	var body ghRunsResponseJSON
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("FindLatestWorkflowRun decode: %w", err)
	}

	// Tolerate a little clock skew between Launch and GitHub when
	// filtering by creation time.
	cutoff := since.Add(-30 * time.Second)

	var best *WorkflowRunSummary
	for _, r := range body.WorkflowRuns {
		if r.CreatedAt.Before(cutoff) {
			continue
		}
		// Newest wins (the API returns newest-first, but don't rely on it).
		if best == nil || r.CreatedAt.After(best.CreatedAt) {
			best = &WorkflowRunSummary{
				ID:         r.ID,
				Status:     r.Status,
				Conclusion: r.Conclusion,
				HTMLURL:    r.HTMLURL,
				CreatedAt:  r.CreatedAt,
				Event:      r.Event,
				HeadBranch: r.HeadBranch,
			}
		}
	}
	return best, nil
}
