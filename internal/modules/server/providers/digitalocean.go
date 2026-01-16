package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/sshkey"
)

const digitalOceanAPIURL = "https://api.digitalocean.com/v2"

// DigitalOceanProvider implements the Provider interface for DigitalOcean
type DigitalOceanProvider struct {
	BaseProvider
	client *http.Client
}

// NewDigitalOceanProvider creates a new DigitalOcean provider
func NewDigitalOceanProvider(keyGenerator sshkey.Generator) *DigitalOceanProvider {
	configs := config.GetProviderConfigs()
	return &DigitalOceanProvider{
		BaseProvider: BaseProvider{
			keyGenerator: keyGenerator,
			config:       configs["digitalocean"],
		},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Type returns the provider type
func (p *DigitalOceanProvider) Type() enums.ServerProvider {
	return enums.ProviderDigitalOcean
}

// Connect tests the connection to DigitalOcean
func (p *DigitalOceanProvider) Connect(ctx context.Context, credentials map[string]interface{}) error {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return ErrInvalidCredentials
	}

	_, err := p.doRequest(ctx, token, "GET", "/account", nil)
	if err != nil {
		return ErrConnectionFailed
	}

	return nil
}

// Create creates a new server on DigitalOcean
func (p *DigitalOceanProvider) Create(ctx context.Context, server *models.Server, credentials map[string]interface{}) (*CreateResult, error) {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return nil, ErrInvalidCredentials
	}

	// Generate SSH key pair
	keyPair, err := p.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create SSH key on DigitalOcean
	sshKeyName := strings.ReplaceAll(strings.ToLower(server.Name), " ", "-") + "-" + server.ID
	sshKeyBody := map[string]interface{}{
		"name":       sshKeyName,
		"public_key": keyPair.PublicKey,
	}

	sshKeyResp, err := p.doRequest(ctx, token, "POST", "/account/keys", sshKeyBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH key: %w", err)
	}

	sshKeyData, ok := sshKeyResp["ssh_key"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid SSH key response")
	}
	sshKeyID := fmt.Sprintf("%v", sshKeyData["id"])

	// Get provider data
	providerData := server.ProviderData
	if providerData == nil {
		providerData = make(map[string]interface{})
	}

	region, _ := providerData["region"].(string)
	plan, _ := providerData["plan"].(string)
	var os enums.OperatingSystem
	if server.OperatingSystem != nil {
		os = enums.OperatingSystem(*server.OperatingSystem)
	} else {
		os = enums.OSUbuntu24
	}
	image := p.GetImage(os)

	// Create droplet
	dropletBody := map[string]interface{}{
		"name":       sshKeyName,
		"region":     region,
		"size":       plan,
		"image":      image,
		"backups":    false,
		"ipv6":       false,
		"monitoring": false,
		"ssh_keys":   []interface{}{sshKeyID},
	}

	dropletResp, err := p.doRequest(ctx, token, "POST", "/droplets", dropletBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create droplet: %w", err)
	}

	droplet, ok := dropletResp["droplet"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid droplet response")
	}

	dropletID := fmt.Sprintf("%v", droplet["id"])
	vcpus := int(droplet["vcpus"].(float64))
	memory := int(droplet["memory"].(float64))
	disk := int(droplet["disk"].(float64))

	return &CreateResult{
		ProviderServerID: dropletID,
		PublicKey:        keyPair.PublicKey,
		PrivateKey:       keyPair.PrivateKey,
		CPUCores:         vcpus,
		MemoryMB:         memory,
		DiskGB:           disk,
		SSHKeyID:         sshKeyID,
		ProviderData: map[string]interface{}{
			"droplet_id": dropletID,
			"ssh_key_id": sshKeyID,
		},
	}, nil
}

// Delete deletes a server from DigitalOcean
func (p *DigitalOceanProvider) Delete(ctx context.Context, server *models.Server, credentials map[string]interface{}) error {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return ErrInvalidCredentials
	}

	providerData := server.ProviderData
	if providerData == nil {
		return nil // Nothing to delete
	}

	dropletID, ok := providerData["droplet_id"].(string)
	if !ok || dropletID == "" {
		return nil // Nothing to delete
	}

	_, _ = p.doRequest(ctx, token, "DELETE", "/droplets/"+dropletID, nil)
	return nil
}

// GetPublicIPv4 fetches the public IPv4 address of a server
func (p *DigitalOceanProvider) GetPublicIPv4(ctx context.Context, server *models.Server, credentials map[string]interface{}) (string, error) {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return "", ErrInvalidCredentials
	}

	providerData := server.ProviderData
	if providerData == nil {
		return "", ErrServerNotFound
	}

	dropletID, ok := providerData["droplet_id"].(string)
	if !ok || dropletID == "" {
		return "", ErrServerNotFound
	}

	resp, err := p.doRequest(ctx, token, "GET", "/droplets/"+dropletID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get droplet: %w", err)
	}

	droplet, ok := resp["droplet"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid droplet response")
	}

	networks, ok := droplet["networks"].(map[string]interface{})
	if !ok {
		return "", nil
	}

	v4Networks, ok := networks["v4"].([]interface{})
	if !ok {
		return "", nil
	}

	for _, n := range v4Networks {
		network, ok := n.(map[string]interface{})
		if !ok {
			continue
		}
		if network["type"] == "public" {
			if ip, ok := network["ip_address"].(string); ok {
				return ip, nil
			}
		}
	}

	return "", nil
}

// GetImage returns the image ID for an operating system
func (p *DigitalOceanProvider) GetImage(os enums.OperatingSystem) string {
	osKey := os.String()
	if img, ok := p.config.Images[osKey]; ok {
		if str, ok := img.(string); ok {
			return str
		}
	}
	// Default to ubuntu 24.04
	return "ubuntu-24-04-x64"
}

// CredentialRules returns validation rules for credentials
func (p *DigitalOceanProvider) CredentialRules() map[string]string {
	return map[string]string{
		"token": "required",
	}
}

// CreateRules returns validation rules for server creation
func (p *DigitalOceanProvider) CreateRules() map[string]string {
	return map[string]string{
		"plan":   "required",
		"region": "required",
	}
}

// CredentialData extracts credential data from input
func (p *DigitalOceanProvider) CredentialData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"token": input["token"],
	}
}

// ProviderData extracts provider-specific data from input
func (p *DigitalOceanProvider) ProviderData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"plan":   input["plan"],
		"region": input["region"],
	}
}

func (p *DigitalOceanProvider) doRequest(ctx context.Context, token, method, path string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, digitalOceanAPIURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if len(respBody) == 0 {
		return nil, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}
