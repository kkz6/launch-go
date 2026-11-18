package providers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// EnsureContents provisions an immutable, binding-specific workflow. A retry
// accepts the exact file previously written, but never overwrites user edits.
// Creating without a SHA also protects the gap between the GET and PUT.
func (p *GitHubProvider) EnsureContents(ctx context.Context, installationID, owner, repo, path, content, message, branch string) (string, error) {
	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return "", err
	}
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)
	readPath := apiPath
	if branch != "" {
		readPath += "?ref=" + url.QueryEscape(branch)
	}
	response, err := p.DoRaw(ctx, http.MethodGet, readPath, token, nil)
	if err != nil {
		return "", err
	}
	if response.StatusCode == http.StatusOK {
		defer response.Body.Close()
		var current struct {
			SHA      string `json:"sha"`
			Encoding string `json:"encoding"`
			Content  string `json:"content"`
		}
		if err := DecodeJSON(response, &current); err != nil {
			return "", err
		}
		decoded, err := base64.StdEncoding.DecodeString(current.Content)
		if err != nil || current.Encoding != "base64" || current.SHA == "" {
			return "", errors.New("workflow contents could not be verified")
		}
		if string(decoded) != content {
			return "", errors.New("workflow was modified; preserve the file and resolve it before retrying")
		}
		return current.SHA, nil
	}
	status := response.StatusCode
	_ = response.Body.Close()
	if status == http.StatusForbidden {
		return "", ErrPermissionDenied
	}
	if status != http.StatusNotFound {
		return "", fmt.Errorf("read workflow contents: status %d", status)
	}

	body := map[string]any{"message": message, "content": base64.StdEncoding.EncodeToString([]byte(content))}
	if branch != "" {
		body["branch"] = branch
	}
	response, err = p.DoRaw(ctx, http.MethodPut, apiPath, token, body)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusForbidden {
		return "", ErrPermissionDenied
	}
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create workflow contents: status %d", response.StatusCode)
	}
	var result putContentsResponse
	if err := DecodeJSON(response, &result); err != nil {
		return "", err
	}
	if result.Content.SHA == "" {
		return "", errors.New("create workflow contents: missing content.sha")
	}
	return result.Content.SHA, nil
}
