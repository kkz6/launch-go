package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/httpclient"
	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
)

const hetznerAPIURL = "https://api.hetzner.cloud/v1"

// HetznerProvider implements the Provider interface for Hetzner Cloud
type HetznerProvider struct {
	BaseCloudProvider
}

// NewHetznerProvider creates a new Hetzner provider
func NewHetznerProvider(keyGenerator sshkey.Generator) *HetznerProvider {
	configs := config.GetProviderConfigs()
	return &HetznerProvider{
		BaseCloudProvider: NewBaseCloudProvider(
			keyGenerator,
			configs["hetzner"],
			hetznerAPIURL,
		),
	}
}

// Type returns the provider type
func (p *HetznerProvider) Type() enums.ServerProvider {
	return enums.ProviderHetzner
}

// Connect tests the connection to Hetzner
func (p *HetznerProvider) Connect(ctx context.Context, credentials map[string]interface{}) error {
	token, err := ExtractToken(credentials)
	if err != nil {
		return err
	}

	client := p.NewClient(token)
	return ValidateConnection(ctx, client, "/servers")
}

// Create creates a new server on Hetzner
func (p *HetznerProvider) Create(ctx context.Context, server *models.Server, credentials map[string]interface{}) (*CreateResult, error) {
	token, err := ExtractToken(credentials)
	if err != nil {
		return nil, err
	}

	client := p.NewClient(token)

	// Generate SSH key pair
	keyPair, err := p.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create SSH key
	sshKeyName := "server-" + server.ID + "-key"
	sshKeyResp, err := p.createSSHKey(ctx, client, sshKeyName, keyPair.PublicKey)
	if err != nil {
		return nil, err
	}
	sshKeyID := sshKeyResp.ID

	// Get provider data
	providerData := GetProviderData(server.ProviderData)
	region := GetStringField(providerData, "region", "")
	plan := GetStringField(providerData, "plan", "")
	os := p.getOperatingSystem(server)
	image := p.GetImage(os)

	// Create server
	serverName := strings.ReplaceAll(strings.ToLower(server.Name), " ", "-")
	serverResp, err := p.createServer(ctx, client, serverName, region, plan, image, sshKeyID)
	if err != nil {
		return nil, err
	}

	return &CreateResult{
		ProviderServerID: serverResp.ID,
		PublicIPv4:       serverResp.PublicIP,
		PublicKey:        keyPair.PublicKey,
		PrivateKey:       keyPair.PrivateKey,
		SSHKeyID:         sshKeyID,
		ProviderData: map[string]interface{}{
			"hetzner_id": serverResp.ID,
			"ssh_key_id": sshKeyID,
		},
	}, nil
}

// Delete deletes a server from Hetzner
func (p *HetznerProvider) Delete(ctx context.Context, server *models.Server, credentials map[string]interface{}) error {
	token, err := ExtractToken(credentials)
	if err != nil {
		return err
	}

	providerData := server.ProviderData
	if providerData == nil {
		return nil
	}

	client := p.NewClient(token)

	if hetznerID := GetStringField(providerData, "hetzner_id", ""); hetznerID != "" {
		DoDeleteIgnoreErrors(ctx, client, "/servers/"+hetznerID)
	}

	if sshKeyID := GetStringField(providerData, "ssh_key_id", ""); sshKeyID != "" {
		DoDeleteIgnoreErrors(ctx, client, "/ssh_keys/"+sshKeyID)
	}

	return nil
}

// GetPublicIPv4 fetches the public IPv4 address
func (p *HetznerProvider) GetPublicIPv4(ctx context.Context, server *models.Server, credentials map[string]interface{}) (string, error) {
	token, err := ExtractToken(credentials)
	if err != nil {
		return "", err
	}

	providerData := server.ProviderData
	if providerData == nil {
		return "", ErrServerNotFound
	}

	hetznerID := GetStringField(providerData, "hetzner_id", "")
	if hetznerID == "" {
		return "", ErrServerNotFound
	}

	client := p.NewClient(token)
	resp, err := DoGet(ctx, client, "/servers/"+hetznerID)
	if err != nil {
		return "", WrapHTTPError(err, "get server")
	}

	return p.extractPublicIPv4(resp)
}

// GetImage returns the image ID for an operating system
func (p *HetznerProvider) GetImage(os enums.OperatingSystem) string {
	return p.GetImageFromConfig(os, "ubuntu-24.04")
}

// CredentialRules returns validation rules
func (p *HetznerProvider) CredentialRules() map[string]string {
	return CommonCredentialRules()
}

// CreateRules returns validation rules
func (p *HetznerProvider) CreateRules() map[string]string {
	return CommonCreateRules()
}

// CredentialData extracts credential data
func (p *HetznerProvider) CredentialData(input map[string]interface{}) map[string]interface{} {
	return CommonCredentialData(input)
}

// ProviderData extracts provider-specific data
func (p *HetznerProvider) ProviderData(input map[string]interface{}) map[string]interface{} {
	return CommonProviderData(input)
}

// Internal helper methods

type hetznerSSHKeyResponse struct {
	ID string
}

func (p *HetznerProvider) createSSHKey(ctx context.Context, client *httpclient.Client, name, publicKey string) (*hetznerSSHKeyResponse, error) {
	body := map[string]interface{}{
		"name":       name,
		"public_key": publicKey,
	}

	resp, err := DoPost(ctx, client, "/ssh_keys", body)
	if err != nil {
		return nil, WrapHTTPError(err, "create SSH key")
	}

	sshKeyData, ok := GetNestedMap(resp, "ssh_key")
	if !ok {
		return nil, fmt.Errorf("invalid SSH key response")
	}

	return &hetznerSSHKeyResponse{
		ID: ExtractServerID(sshKeyData, "id"),
	}, nil
}

type hetznerServerResponse struct {
	ID       string
	PublicIP string
}

func (p *HetznerProvider) createServer(ctx context.Context, client *httpclient.Client, name, region, plan, image, sshKeyID string) (*hetznerServerResponse, error) {
	body := map[string]interface{}{
		"automount":   false,
		"image":       image,
		"ssh_keys":    []interface{}{sshKeyID},
		"name":        name,
		"location":    region,
		"server_type": plan,
	}

	resp, err := DoPost(ctx, client, "/servers", body)
	if err != nil {
		return nil, WrapHTTPError(err, "create server")
	}

	serverData, ok := GetNestedMap(resp, "server")
	if !ok {
		return nil, fmt.Errorf("invalid server response")
	}

	var publicIP string
	if publicNet, ok := GetNestedMap(serverData, "public_net"); ok {
		if ipv4, ok := GetNestedMap(publicNet, "ipv4"); ok {
			publicIP = GetStringField(ipv4, "ip", "")
		}
	}

	return &hetznerServerResponse{
		ID:       ExtractServerID(serverData, "id"),
		PublicIP: publicIP,
	}, nil
}

func (p *HetznerProvider) extractPublicIPv4(resp map[string]interface{}) (string, error) {
	serverData, ok := GetNestedMap(resp, "server")
	if !ok {
		return "", fmt.Errorf("invalid server response")
	}

	if publicNet, ok := GetNestedMap(serverData, "public_net"); ok {
		if ipv4, ok := GetNestedMap(publicNet, "ipv4"); ok {
			return GetStringField(ipv4, "ip", ""), nil
		}
	}

	return "", nil
}

func (p *HetznerProvider) getOperatingSystem(server *models.Server) enums.OperatingSystem {
	if server.OperatingSystem != nil {
		return enums.OperatingSystem(*server.OperatingSystem)
	}
	return enums.OSUbuntu24
}
