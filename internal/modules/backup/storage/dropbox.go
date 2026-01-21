package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kkz6/launch-go/internal/pkg/httpclient"
)

const dropboxAPIURL = "https://api.dropboxapi.com/2"

// DropboxProvider implements the Provider interface for Dropbox storage
type DropboxProvider struct {
	token      string
	httpClient *http.Client
}

// NewDropboxProvider creates a new Dropbox storage provider
func NewDropboxProvider(credentials map[string]interface{}) *DropboxProvider {
	p := &DropboxProvider{
		httpClient: httpclient.Default(),
	}

	if token, ok := credentials["token"].(string); ok {
		p.token = token
	}

	return p
}

// Connect tests the connection to Dropbox
func (p *DropboxProvider) Connect(ctx context.Context) error {
	if p.token == "" {
		return fmt.Errorf("dropbox token is required")
	}

	reqBody := map[string]string{"query": ""}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", dropboxAPIURL+"/check/user", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Dropbox: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to connect to Dropbox: status %d", resp.StatusCode)
	}

	return nil
}

// Delete removes files from Dropbox
func (p *DropboxProvider) Delete(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	entries := make([]map[string]string, len(paths))
	for i, path := range paths {
		entries[i] = map[string]string{"path": path}
	}

	reqBody := map[string]interface{}{"entries": entries}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", dropboxAPIURL+"/files/delete_batch", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete files from Dropbox: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete files from Dropbox: status %d", resp.StatusCode)
	}

	return nil
}

// GetConfigForAgent returns the configuration for the agent
func (p *DropboxProvider) GetConfigForAgent() map[string]interface{} {
	return map[string]interface{}{
		"token": p.token,
	}
}

// CredentialData extracts credential data from input
func (p *DropboxProvider) CredentialData(input map[string]interface{}) map[string]interface{} {
	data := make(map[string]interface{})

	if token, ok := input["token"].(string); ok {
		data["token"] = token
	}

	return data
}

// Type returns the storage driver type
func (p *DropboxProvider) Type() string {
	return "dropbox"
}

// GetToken returns the configured token
func (p *DropboxProvider) GetToken() string {
	return p.token
}

// SetHTTPClient sets a custom HTTP client (useful for testing)
func (p *DropboxProvider) SetHTTPClient(client *http.Client) {
	p.httpClient = client
}
