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

const linodeAPIURL = "https://api.linode.com/v4"

// LinodeProvider implements the Provider interface for Linode
type LinodeProvider struct {
	BaseProvider
	client *http.Client
}

// NewLinodeProvider creates a new Linode provider
func NewLinodeProvider(keyGenerator sshkey.Generator) *LinodeProvider {
	configs := config.GetProviderConfigs()
	return &LinodeProvider{
		BaseProvider: BaseProvider{
			keyGenerator: keyGenerator,
			config:       configs["linode"],
		},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Type returns the provider type
func (p *LinodeProvider) Type() enums.ServerProvider {
	return enums.ProviderLinode
}

// Connect tests the connection to Linode
func (p *LinodeProvider) Connect(ctx context.Context, credentials map[string]interface{}) error {
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

// Create creates a new server on Linode
func (p *LinodeProvider) Create(ctx context.Context, server *models.Server, credentials map[string]interface{}) (*CreateResult, error) {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return nil, ErrInvalidCredentials
	}

	keyPair, err := p.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

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
	linodeResp, err := p.doRequest(ctx, token, "POST", "/linode/instances", map[string]interface{}{
		"label":           serverName,
		"region":          region,
		"type":            plan,
		"image":           image,
		"authorized_keys": []string{keyPair.PublicKey},
		"booted":          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create linode: %w", err)
	}

	linodeID := fmt.Sprintf("%v", linodeResp["id"])

	var publicIP string
	if ipv4, ok := linodeResp["ipv4"].([]interface{}); ok && len(ipv4) > 0 {
		publicIP, _ = ipv4[0].(string)
	}

	var specs struct {
		vcpus  int
		memory int
		disk   int
	}
	if specsData, ok := linodeResp["specs"].(map[string]interface{}); ok {
		specs.vcpus = int(specsData["vcpus"].(float64))
		specs.memory = int(specsData["memory"].(float64))
		specs.disk = int(specsData["disk"].(float64))
	}

	return &CreateResult{
		ProviderServerID: linodeID,
		PublicIPv4:       publicIP,
		PublicKey:        keyPair.PublicKey,
		PrivateKey:       keyPair.PrivateKey,
		CPUCores:         specs.vcpus,
		MemoryMB:         specs.memory,
		DiskGB:           specs.disk / 1024,
		ProviderData: map[string]interface{}{
			"linode_id": linodeID,
		},
	}, nil
}

// Delete deletes a server from Linode
func (p *LinodeProvider) Delete(ctx context.Context, server *models.Server, credentials map[string]interface{}) error {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return ErrInvalidCredentials
	}

	providerData := server.ProviderData
	if providerData == nil {
		return nil
	}

	if linodeID, ok := providerData["linode_id"].(string); ok && linodeID != "" {
		_, _ = p.doRequest(ctx, token, "DELETE", "/linode/instances/"+linodeID, nil)
	}

	return nil
}

// GetPublicIPv4 fetches the public IPv4 address
func (p *LinodeProvider) GetPublicIPv4(ctx context.Context, server *models.Server, credentials map[string]interface{}) (string, error) {
	token, ok := credentials["token"].(string)
	if !ok || token == "" {
		return "", ErrInvalidCredentials
	}

	providerData := server.ProviderData
	if providerData == nil {
		return "", ErrServerNotFound
	}

	linodeID, ok := providerData["linode_id"].(string)
	if !ok || linodeID == "" {
		return "", ErrServerNotFound
	}

	resp, err := p.doRequest(ctx, token, "GET", "/linode/instances/"+linodeID, nil)
	if err != nil {
		return "", err
	}

	if ipv4, ok := resp["ipv4"].([]interface{}); ok && len(ipv4) > 0 {
		if ip, ok := ipv4[0].(string); ok {
			return ip, nil
		}
	}

	return "", nil
}

// GetImage returns the image ID for an operating system
func (p *LinodeProvider) GetImage(os enums.OperatingSystem) string {
	if img, ok := p.config.Images[os.String()]; ok {
		if str, ok := img.(string); ok {
			return str
		}
	}
	return "linode/ubuntu24.04"
}

// CredentialRules returns validation rules
func (p *LinodeProvider) CredentialRules() map[string]string {
	return map[string]string{"token": "required"}
}

// CreateRules returns validation rules
func (p *LinodeProvider) CreateRules() map[string]string {
	return map[string]string{"plan": "required", "region": "required"}
}

// CredentialData extracts credential data
func (p *LinodeProvider) CredentialData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"token": input["token"]}
}

// ProviderData extracts provider-specific data
func (p *LinodeProvider) ProviderData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"plan": input["plan"], "region": input["region"]}
}

func (p *LinodeProvider) doRequest(ctx context.Context, token, method, path string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, linodeAPIURL+path, reqBody)
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
