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
)

const vultrAPIURL = "https://api.vultr.com/v2"

// VultrProvider implements the Provider interface for Vultr
type VultrProvider struct {
	BaseProvider
	client *http.Client
}

// NewVultrProvider creates a new Vultr provider
func NewVultrProvider(keyGenerator KeyPairGenerator) *VultrProvider {
	configs := config.GetProviderConfigs()
	return &VultrProvider{
		BaseProvider: BaseProvider{
			keyGenerator: keyGenerator,
			config:       configs["vultr"],
		},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Type returns the provider type
func (p *VultrProvider) Type() enums.ServerProvider {
	return enums.ProviderVultr
}

// Connect tests the connection to Vultr
func (p *VultrProvider) Connect(ctx context.Context, credentials map[string]interface{}) error {
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

// Create creates a new server on Vultr
func (p *VultrProvider) Create(ctx context.Context, server *models.Server, credentials map[string]interface{}) (*CreateResult, error) {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return nil, ErrInvalidCredentials
	}

	keyPair, err := p.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create SSH key
	sshKeyName := "server-" + server.ID + "-key"
	sshKeyResp, err := p.doRequest(ctx, token, "POST", "/ssh-keys", map[string]interface{}{
		"name":    sshKeyName,
		"ssh_key": keyPair.PublicKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH key: %w", err)
	}

	sshKeyData := sshKeyResp["ssh_key"].(map[string]interface{})
	sshKeyID := sshKeyData["id"].(string)

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
	osID := p.GetImage(os)

	serverName := strings.ReplaceAll(strings.ToLower(server.Name), " ", "-")
	instanceResp, err := p.doRequest(ctx, token, "POST", "/instances", map[string]interface{}{
		"label":    serverName,
		"region":   region,
		"plan":     plan,
		"os_id":    osID,
		"sshkey_id": []string{sshKeyID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	instance := instanceResp["instance"].(map[string]interface{})
	instanceID := instance["id"].(string)
	mainIP, _ := instance["main_ip"].(string)

	vcpuCount := int(instance["vcpu_count"].(float64))
	ram := int(instance["ram"].(float64))
	disk := int(instance["disk"].(float64))

	return &CreateResult{
		ProviderServerID: instanceID,
		PublicIPv4:       mainIP,
		PublicKey:        keyPair.PublicKey,
		PrivateKey:       keyPair.PrivateKey,
		CPUCores:         vcpuCount,
		MemoryMB:         ram,
		DiskGB:           disk,
		SSHKeyID:         sshKeyID,
		ProviderData: map[string]interface{}{
			"vultr_id":   instanceID,
			"ssh_key_id": sshKeyID,
		},
	}, nil
}

// Delete deletes a server from Vultr
func (p *VultrProvider) Delete(ctx context.Context, server *models.Server, credentials map[string]interface{}) error {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return ErrInvalidCredentials
	}

	providerData := server.ProviderData
	if providerData == nil {
		return nil
	}

	if vultrID, ok := providerData["vultr_id"].(string); ok && vultrID != "" {
		_, _ = p.doRequest(ctx, token, "DELETE", "/instances/"+vultrID, nil)
	}

	if sshKeyID, ok := providerData["ssh_key_id"].(string); ok && sshKeyID != "" {
		_, _ = p.doRequest(ctx, token, "DELETE", "/ssh-keys/"+sshKeyID, nil)
	}

	return nil
}

// GetPublicIPv4 fetches the public IPv4 address
func (p *VultrProvider) GetPublicIPv4(ctx context.Context, server *models.Server, credentials map[string]interface{}) (string, error) {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return "", ErrInvalidCredentials
	}

	providerData := server.ProviderData
	if providerData == nil {
		return "", ErrServerNotFound
	}

	vultrID, ok := providerData["vultr_id"].(string)
	if !ok || vultrID == "" {
		return "", ErrServerNotFound
	}

	resp, err := p.doRequest(ctx, token, "GET", "/instances/"+vultrID, nil)
	if err != nil {
		return "", err
	}

	instance := resp["instance"].(map[string]interface{})
	if mainIP, ok := instance["main_ip"].(string); ok {
		return mainIP, nil
	}

	return "", nil
}

// GetImage returns the image ID for an operating system
func (p *VultrProvider) GetImage(os enums.OperatingSystem) string {
	if img, ok := p.config.Images[os.String()]; ok {
		if str, ok := img.(string); ok {
			return str
		}
	}
	return "2284" // Ubuntu 24.04
}

// CredentialRules returns validation rules
func (p *VultrProvider) CredentialRules() map[string]string {
	return map[string]string{"token": "required"}
}

// CreateRules returns validation rules
func (p *VultrProvider) CreateRules() map[string]string {
	return map[string]string{"plan": "required", "region": "required"}
}

// CredentialData extracts credential data
func (p *VultrProvider) CredentialData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"token": input["token"]}
}

// ProviderData extracts provider-specific data
func (p *VultrProvider) ProviderData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"plan": input["plan"], "region": input["region"]}
}

func (p *VultrProvider) doRequest(ctx context.Context, token, method, path string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, vultrAPIURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed: %s", string(respBody))
	}

	if len(respBody) == 0 {
		return nil, nil
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	return result, nil
}
