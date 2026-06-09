package dto

import "time"

// DeploymentGHAStep is one step of a GitHub Actions job, surfaced in the
// deployment view's live step timeline (#87).
type DeploymentGHAStep struct {
	Name        string     `json:"name"`
	Status      string     `json:"status"`               // queued | in_progress | completed
	Conclusion  string     `json:"conclusion,omitempty"` // success | failure | skipped | cancelled | ""
	Number      int        `json:"number"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// DeploymentGHAJob is one job of a workflow run, with its ordered steps.
// Application runs have a single job; compose runs have one per built
// service (the matrix).
type DeploymentGHAJob struct {
	Name       string              `json:"name"`
	Status     string              `json:"status"`
	Conclusion string              `json:"conclusion,omitempty"`
	HTMLURL    string              `json:"html_url,omitempty"`
	Steps      []DeploymentGHAStep `json:"steps"`
}

// DeploymentGHASteps is the read-endpoint payload for a deployment's
// GitHub Actions step timeline. Mirrors the deployment.gha_steps
// WebSocket event shape so the frontend can consume both with one type.
type DeploymentGHASteps struct {
	DeploymentID string             `json:"deployment_id"`
	RunID        string             `json:"run_id,omitempty"`
	RunURL       string             `json:"run_url,omitempty"`
	RunStatus    string             `json:"run_status"` // queued | in_progress | completed | pending
	Jobs         []DeploymentGHAJob `json:"jobs"`
}
