package providers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// PutContents creates or updates a file in the repo at the given
// path. existingSHA == "" → create; non-empty → update (acts as
// If-Match — GitHub rejects with 409 if the file's current SHA
// doesn't match). Returns the new file's commit SHA so the caller
// can persist it for next-sync drift detection.
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

	// Response shape: { "content": {...}, "commit": { "sha": "...", ... } }
	var out struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := DecodeJSON(resp, &out); err != nil {
		return "", err
	}
	return out.Commit.SHA, nil
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
