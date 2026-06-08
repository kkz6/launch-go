package providers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// putContentsResponseShaCarrier holds just the `sha` field from a
// content / commit object in PutContents responses. Named (rather
// than nested anonymous) so revive's no-nested-structs rule stays
// happy — same JSON wire shape either way.
type putContentsResponseShaCarrier struct {
	SHA string `json:"sha"`
}

// putContentsResponse is the slim view of GitHub's PUT contents
// response. Carries the blob sha (which the next update needs as the
// If-Match-like "sha" field) and the commit sha (kept for log/UX).
type putContentsResponse struct {
	Content putContentsResponseShaCarrier `json:"content"`
	Commit  putContentsResponseShaCarrier `json:"commit"`
}

// PutContents creates or updates a file in the repo at the given
// path. existingSHA == "" → create; non-empty → update (acts as
// If-Match — GitHub rejects with 409 if the file's current SHA
// doesn't match). Returns the new **file blob SHA** (content.sha
// in the response) so the caller can persist it and pass it back as
// existingSHA on the next sync. NOT the commit SHA — those are
// different values in the same response and PUT contents needs the
// blob SHA for its If-Match check. Storing the commit SHA causes
// every subsequent update to 409 with "file was modified since last
// sync" even when nothing actually changed.
//
// GitHub API: PUT /repos/{owner}/{repo}/contents/{path}
// docs: https://docs.github.com/en/rest/repos/contents#create-or-update-file-contents
//
// content is the raw file body; this helper base64-encodes it (the
// API doesn't accept plain text). branch defaults to the repo's
// default branch when empty — we always pass it explicitly to avoid
// surprises.
func (p *GitHubProvider) PutContents(
	ctx context.Context,
	installationID, owner, repo, path, content, message, existingSHA, branch string,
) (string, error) {
	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return "", err
	}

	// When the caller has no stored SHA the file may still exist on the repo —
	// e.g. several applications share one repo + workflow path, or the SHA was
	// never persisted. GitHub's create-or-update API rejects a create (no sha)
	// on an existing file with 422 "sha wasn't supplied", so resolve the current
	// blob SHA first and only fall back to a create when the file is absent.
	// This makes PutContents an idempotent upsert.
	if existingSHA == "" {
		currentSHA, exists, shaErr := p.getContentSHA(ctx, token, owner, repo, path, branch)
		if shaErr != nil {
			return "", shaErr
		}
		if exists {
			existingSHA = currentSHA
		}
	}

	body := map[string]interface{}{
		"message": message,
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
	}
	if existingSHA != "" {
		body["sha"] = existingSHA
	}
	if branch != "" {
		body["branch"] = branch
	}

	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)
	resp, err := p.DoRaw(ctx, http.MethodPut, apiPath, token, body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fiberutil.NotFound()
	}
	if resp.StatusCode == http.StatusForbidden {
		return "", ErrPermissionDenied
	}
	if resp.StatusCode == http.StatusConflict {
		// 409 means existingSHA was stale — the file was edited out
		// from under us. Re-fetch, re-render, retry is the caller's
		// responsibility; here we surface a clean error.
		return "", errors.New("file SHA conflict — repo file was modified since last sync")
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("PutContents %s/%s/%s: status %d body %s", owner, repo, path, resp.StatusCode, string(raw))
	}

	// Response shape (abbreviated):
	//   { "content": { "sha": "<blob>", "path": "...", ... },
	//     "commit":  { "sha": "<commit>", ... } }
	//
	// We return content.sha (the file blob). It's what gets passed
	// back as the "sha" field in the next PUT — i.e. the value
	// GitHub uses for its If-Match conflict check. The commit SHA is
	// useful for "show me the commit that did this" UX, but you can
	// not use it as the existingSHA on a subsequent update.
	var out putContentsResponse
	if err := DecodeJSON(resp, &out); err != nil {
		return "", err
	}
	if out.Content.SHA == "" {
		// Defensive: don't silently fall back to the commit SHA
		// here. The bug we just fixed was caused by that exact
		// behavior — if GitHub ever changes the response shape so
		// content is missing, we want a loud failure on the next
		// sync rather than a silent 409 cascade.
		return "", errors.New("PutContents: response missing content.sha")
	}
	return out.Content.SHA, nil
}

// getContentSHA returns the current blob SHA of the file at path on the given
// branch. ok is false when the file does not exist (404). Used by PutContents
// to upsert when no caller-supplied SHA is available.
//
// GitHub API: GET /repos/{owner}/{repo}/contents/{path}?ref={branch}
func (p *GitHubProvider) getContentSHA(
	ctx context.Context,
	token, owner, repo, path, branch string,
) (sha string, ok bool, err error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)
	if branch != "" {
		apiPath += "?ref=" + url.QueryEscape(branch)
	}

	resp, err := p.DoRaw(ctx, http.MethodGet, apiPath, token, nil)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound:
		return "", false, nil
	case http.StatusOK:
		var out putContentsResponseShaCarrier
		if decErr := DecodeJSON(resp, &out); decErr != nil {
			return "", false, decErr
		}
		return out.SHA, out.SHA != "", nil
	default:
		raw, _ := io.ReadAll(resp.Body)
		return "", false, fmt.Errorf("getContentSHA %s/%s/%s: status %d body %s", owner, repo, path, resp.StatusCode, string(raw))
	}
}

// GetActionsPublicKey fetches the repo's libsodium public key used to
// encrypt Actions secrets. Returns the GitHub-provided key id +
// raw 32-byte public key material (base64-decoded).
//
// GitHub API: GET /repos/{owner}/{repo}/actions/secrets/public-key
// docs: https://docs.github.com/en/rest/actions/secrets#get-a-repository-public-key
func (p *GitHubProvider) GetActionsPublicKey(
	ctx context.Context,
	installationID, owner, repo string,
) (keyID string, publicKey []byte, err error) {
	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return "", nil, err
	}
	apiPath := fmt.Sprintf("/repos/%s/%s/actions/secrets/public-key", owner, repo)
	resp, err := p.DoRaw(ctx, http.MethodGet, apiPath, token, nil)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil, fiberutil.NotFound()
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("GetActionsPublicKey: status %d body %s", resp.StatusCode, string(raw))
	}

	var out struct {
		KeyID string `json:"key_id"`
		Key   string `json:"key"`
	}
	if err := DecodeJSON(resp, &out); err != nil {
		return "", nil, err
	}
	if out.KeyID == "" || out.Key == "" {
		return "", nil, errors.New("GetActionsPublicKey: empty response")
	}
	raw, err := decodeRepoPublicKey(out.Key)
	if err != nil {
		return "", nil, err
	}
	return out.KeyID, raw, nil
}

// PutActionsSecret encrypts a plaintext secret with the repo's public
// key and writes it to GitHub Actions. This is the canonical
// libsodium sealed-box flow GitHub documents.
//
// GitHub API: PUT /repos/{owner}/{repo}/actions/secrets/{secret_name}
// docs: https://docs.github.com/en/rest/actions/secrets#create-or-update-a-repository-secret
func (p *GitHubProvider) PutActionsSecret(
	ctx context.Context,
	installationID, owner, repo, secretName, value string,
) error {
	keyID, repoPubKey, err := p.GetActionsPublicKey(ctx, installationID, owner, repo)
	if err != nil {
		return err
	}
	encrypted, err := sealActionsSecret(repoPubKey, value)
	if err != nil {
		return err
	}

	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return err
	}
	apiPath := fmt.Sprintf("/repos/%s/%s/actions/secrets/%s", owner, repo, secretName)
	body := map[string]interface{}{
		"encrypted_value": encrypted,
		"key_id":          keyID,
	}
	resp, err := p.DoRaw(ctx, http.MethodPut, apiPath, token, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 201 on create, 204 on update — both are success here.
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PutActionsSecret %s: status %d body %s", secretName, resp.StatusCode, string(raw))
	}
	return nil
}

// PutActionsVariable creates or updates a plaintext Actions variable.
// Variables are NOT sealed-box-encrypted; they're just JSON values.
// We POST for create then PATCH if the variable already exists.
//
// GitHub API:
//
//	POST  /repos/{owner}/{repo}/actions/variables   (create)
//	PATCH /repos/{owner}/{repo}/actions/variables/{name}  (update)
//
// docs: https://docs.github.com/en/rest/actions/variables
func (p *GitHubProvider) PutActionsVariable(
	ctx context.Context,
	installationID, owner, repo, name, value string,
) error {
	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return err
	}

	// Try create first.
	createPath := fmt.Sprintf("/repos/%s/%s/actions/variables", owner, repo)
	createBody := map[string]interface{}{"name": name, "value": value}
	resp, err := p.DoRaw(ctx, http.MethodPost, createPath, token, createBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusConflict, http.StatusUnprocessableEntity:
		// 409/422 means the variable already exists; fall through to PATCH.
		// (GitHub returns 422 with "already_exists" code for this case.)
	default:
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PutActionsVariable create %s: status %d body %s", name, resp.StatusCode, string(raw))
	}

	updatePath := fmt.Sprintf("/repos/%s/%s/actions/variables/%s", owner, repo, name)
	updateBody := map[string]interface{}{"value": value}
	resp2, err := p.DoRaw(ctx, http.MethodPatch, updatePath, token, updateBody)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("PutActionsVariable update %s: status %d body %s", name, resp2.StatusCode, string(raw))
	}
	return nil
}

// DeleteActionsSecret removes a secret. Used in slice I's "Disable
// GHA builds" cleanup. Idempotent: a 404 is treated as success.
func (p *GitHubProvider) DeleteActionsSecret(
	ctx context.Context,
	installationID, owner, repo, secretName string,
) error {
	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return err
	}
	apiPath := fmt.Sprintf("/repos/%s/%s/actions/secrets/%s", owner, repo, secretName)
	resp, err := p.DoRaw(ctx, http.MethodDelete, apiPath, token, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	raw, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("DeleteActionsSecret %s: status %d body %s", secretName, resp.StatusCode, string(raw))
}

// DeleteActionsVariable mirrors DeleteActionsSecret. Idempotent.
func (p *GitHubProvider) DeleteActionsVariable(
	ctx context.Context,
	installationID, owner, repo, name string,
) error {
	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return err
	}
	apiPath := fmt.Sprintf("/repos/%s/%s/actions/variables/%s", owner, repo, name)
	resp, err := p.DoRaw(ctx, http.MethodDelete, apiPath, token, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	raw, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("DeleteActionsVariable %s: status %d body %s", name, resp.StatusCode, string(raw))
}

// DeleteWorkflowRun removes a workflow run (and its logs) from GitHub Actions.
// Used by the "delete deployment" flow when the user opts to also remove the
// run from GitHub. Idempotent: a 404 (run already gone) is treated as success.
//
// GitHub API: DELETE /repos/{owner}/{repo}/actions/runs/{run_id}
// docs: https://docs.github.com/en/rest/actions/workflow-runs#delete-a-workflow-run
// Requires the installation to hold the `actions: write` permission (the same
// permission the deploy-dispatch flow already relies on).
func (p *GitHubProvider) DeleteWorkflowRun(
	ctx context.Context,
	installationID, owner, repo, runID string,
) error {
	if runID == "" {
		return errors.New("DeleteWorkflowRun: runID is required")
	}

	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return err
	}

	apiPath := fmt.Sprintf("/repos/%s/%s/actions/runs/%s", owner, repo, runID)
	resp, err := p.DoRaw(ctx, http.MethodDelete, apiPath, token, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusNotFound:
		return nil
	case http.StatusForbidden:
		return ErrPermissionDenied
	default:
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeleteWorkflowRun %s/%s/%s: status %d body %s", owner, repo, runID, resp.StatusCode, string(raw))
	}
}

// ErrWorkflowNotFound is returned by TriggerWorkflowDispatch when the
// workflow file is not yet present on the repo (or has been deleted).
// Callers that drive the GHA-deploy button use this to surface a clean
// "GitHub Actions setup hasn't finished yet — retry once the workflow
// has been committed" message rather than a generic 500.
var ErrWorkflowNotFound = errors.New("workflow file not present on repo")

// TriggerWorkflowDispatch fires a manual run of the named workflow on
// the given branch. Used by the docker module's "Deploy" button when an
// application or compose is configured for build_location=github_actions:
// instead of running an on-server build, we ask GitHub Actions to run
// the workflow whose `on: workflow_dispatch:` trigger we committed at
// bootstrap time. The build then notifies Launch via the existing
// webhook path on completion.
//
// GitHub API: POST /repos/{owner}/{repo}/actions/workflows/{workflow_file}/dispatches
// docs: https://docs.github.com/en/rest/actions/workflows#create-a-workflow-dispatch-event
//
// workflowFile is the basename of the workflow file under
// .github/workflows/ — e.g. "launch-deploy.yml". GitHub also accepts the
// numeric workflow id; we use the filename so we don't have to keep an
// id around. Returns 204 on success, ErrWorkflowNotFound on 404 (the
// workflow file isn't on the repo yet — bootstrap hasn't run or the
// user deleted it), permission errors otherwise.
func (p *GitHubProvider) TriggerWorkflowDispatch(
	ctx context.Context,
	installationID, owner, repo, workflowFile, branch string,
) error {
	if workflowFile == "" {
		return errors.New("TriggerWorkflowDispatch: workflowFile is required")
	}
	if branch == "" {
		return errors.New("TriggerWorkflowDispatch: branch is required")
	}

	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return err
	}

	apiPath := fmt.Sprintf(
		"/repos/%s/%s/actions/workflows/%s/dispatches",
		owner, repo, workflowFile,
	)
	body := map[string]interface{}{"ref": branch}

	resp, err := p.DoRaw(ctx, http.MethodPost, apiPath, token, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		// GitHub returns 404 when the workflow file doesn't exist on
		// the default branch (or whichever ref it indexes from). Caller
		// surfaces this as "GHA setup not finished yet" — actionable.
		return ErrWorkflowNotFound
	case http.StatusForbidden:
		return ErrPermissionDenied
	default:
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"TriggerWorkflowDispatch %s/%s/%s on %s: status %d body %s",
			owner, repo, workflowFile, branch, resp.StatusCode, string(raw),
		)
	}
}
