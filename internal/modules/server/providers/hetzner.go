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

const hetznerAPIURL = "https://api.hetzner.cloud/v1"

// HetznerProvider implements the Provider interface for Hetzner Cloud
type HetznerProvider struct {
	BaseProvider
	client *http.Client
}

// NewHetznerProvider creates a new Hetzner provider
func NewHetznerProvider(keyGenerator sshkey.Generator) *HetznerProvider {
	configs := config.GetProviderConfigs()
	return &HetznerProvider{
		BaseProvider: BaseProvider{
			keyGenerator: keyGenerator,
			config:       configs["hetzner"],
		},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Type returns the provider type
func (p *HetznerProvider) Type() enums.ServerProvider {
	return enums.ProviderHetzner
}

// Connect tests the connection to Hetzner
func (p *HetznerProvider) Connect(ctx context.Context, credentials map[string]interface{}) error {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return ErrInvalidCredentials
	}

	_, err := p.doRequest(ctx, token, "GET", "/servers", nil)
	if err != nil {
		return ErrConnectionFailed
	}

	return nil
}

// Create creates a new server on Hetzner
func (p *HetznerProvider) Create(ctx context.Context, server *models.Server, credentials map[string]interface{}) (*CreateResult, error) {
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
	sshKeyResp, err := p.doRequest(ctx, token, "POST", "/ssh_keys", map[string]interface{}{
		"name":       sshKeyName,
		"public_key": keyPair.PublicKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH key: %w", err)
	}

	sshKeyData := sshKeyResp["ssh_key"].(map[string]interface{})
	sshKeyID := fmt.Sprintf("%v", sshKeyData["id"])

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

	serverName := strings.ReplaceAll(strings.ToLower(server.Name), " ", "-")
	serverResp, err := p.doRequest(ctx, token, "POST", "/servers", map[string]interface{}{
		"automount":   false,
		"image":       image,
		"ssh_keys":    []interface{}{sshKeyID},
		"name":        serverName,
		"location":    region,
		"server_type": plan,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	serverData := serverResp["server"].(map[string]interface{})
	serverID := fmt.Sprintf("%v", serverData["id"])

	var publicIP string
	if publicNet, ok := serverData["public_net"].(map[string]interface{}); ok {
		if ipv4, ok := publicNet["ipv4"].(map[string]interface{}); ok {
			publicIP, _ = ipv4["ip"].(string)
		}
	}

	return &CreateResult{
		ProviderServerID: serverID,
		PublicIPv4:       publicIP,
		PublicKey:        keyPair.PublicKey,
		PrivateKey:       keyPair.PrivateKey,
		SSHKeyID:         sshKeyID,
		ProviderData: map[string]interface{}{
			"hetzner_id": serverID,
			"ssh_key_id": sshKeyID,
		},
	}, nil
}

// Delete deletes a server from Hetzner
func (p *HetznerProvider) Delete(ctx context.Context, server *models.Server, credentials map[string]interface{}) error {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return ErrInvalidCredentials
	}

	providerData := server.ProviderData
	if providerData == nil {
		return nil
	}

	if hetznerID, ok := providerData["hetzner_id"].(string); ok && hetznerID != "" {
		_, _ = p.doRequest(ctx, token, "DELETE", "/servers/"+hetznerID, nil)
	}

	if sshKeyID, ok := providerData["ssh_key_id"].(string); ok && sshKeyID != "" {
		_, _ = p.doRequest(ctx, token, "DELETE", "/ssh_keys/"+sshKeyID, nil)
	}

	return nil
}

// GetPublicIPv4 fetches the public IPv4 address
func (p *HetznerProvider) GetPublicIPv4(ctx context.Context, server *models.Server, credentials map[string]interface{}) (string, error) {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return "", ErrInvalidCredentials
	}

	providerData := server.ProviderData
	if providerData == nil {
		return "", ErrServerNotFound
	}

	hetznerID, ok := providerData["hetzner_id"].(string)
	if !ok || hetznerID == "" {
		return "", ErrServerNotFound
	}

	resp, err := p.doRequest(ctx, token, "GET", "/servers/"+hetznerID, nil)
	if err != nil {
		return "", err
	}

	serverData := resp["server"].(map[string]interface{})
	if publicNet, ok := serverData["public_net"].(map[string]interface{}); ok {
		if ipv4, ok := publicNet["ipv4"].(map[string]interface{}); ok {
			if ip, ok := ipv4["ip"].(string); ok {
				return ip, nil
			}
		}
	}

	return "", nil
}

// GetImage returns the image ID for an operating system
func (p *HetznerProvider) GetImage(os enums.OperatingSystem) string {
	if img, ok := p.config.Images[os.String()]; ok {
		if str, ok := img.(string); ok {
			return str
		}
	}
	return "ubuntu-24.04"
}

// CredentialRules returns validation rules
func (p *HetznerProvider) CredentialRules() map[string]string {
	return map[string]string{"token": "required"}
}

// CreateRules returns validation rules
func (p *HetznerProvider) CreateRules() map[string]string {
	return map[string]string{"plan": "required", "region": "required"}
}

// CredentialData extracts credential data
func (p *HetznerProvider) CredentialData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"token": input["token"]}
}

// ProviderData extracts provider-specific data
func (p *HetznerProvider) ProviderData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"plan": input["plan"], "region": input["region"]}
}

func (p *HetznerProvider) doRequest(ctx context.Context, token, method, path string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, hetznerAPIURL+path, reqBody)
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
