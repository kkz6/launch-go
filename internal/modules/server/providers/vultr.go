package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/httpclient"
	"github.com/kkz6/launch-go/internal/pkg/sshkey"
)

const vultrAPIURL = "https://api.vultr.com/v2"

// VultrProvider implements the Provider interface for Vultr
type VultrProvider struct {
	BaseCloudProvider
}

// NewVultrProvider creates a new Vultr provider
func NewVultrProvider(keyGenerator sshkey.Generator) *VultrProvider {
	configs := config.GetProviderConfigs()
	return &VultrProvider{
		BaseCloudProvider: NewBaseCloudProvider(
			keyGenerator,
			configs["vultr"],
			vultrAPIURL,
		),
	}
}

// Type returns the provider type
func (p *VultrProvider) Type() enums.ServerProvider {
	return enums.ProviderVultr
}

// Connect tests the connection to Vultr
func (p *VultrProvider) Connect(ctx context.Context, credentials map[string]interface{}) error {
	token, err := ExtractToken(credentials)
	if err != nil {
		return err
	}

	client := p.NewClient(token)
	return ValidateConnection(ctx, client, "/account")
}

// Create creates a new server on Vultr
func (p *VultrProvider) Create(ctx context.Context, server *models.Server, credentials map[string]interface{}) (*CreateResult, error) {
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
	osID := p.GetImage(os)

	// Create instance
	serverName := strings.ReplaceAll(strings.ToLower(server.Name), " ", "-")
	instanceResp, err := p.createInstance(ctx, client, serverName, region, plan, osID, sshKeyID)
	if err != nil {
		return nil, err
	}

	return &CreateResult{
		ProviderServerID: instanceResp.ID,
		PublicIPv4:       instanceResp.MainIP,
		PublicKey:        keyPair.PublicKey,
		PrivateKey:       keyPair.PrivateKey,
		CPUCores:         instanceResp.VCPUCount,
		MemoryMB:         instanceResp.RAM,
		DiskGB:           instanceResp.Disk,
		SSHKeyID:         sshKeyID,
		ProviderData: map[string]interface{}{
			"vultr_id":   instanceResp.ID,
			"ssh_key_id": sshKeyID,
		},
	}, nil
}

// Delete deletes a server from Vultr
func (p *VultrProvider) Delete(ctx context.Context, server *models.Server, credentials map[string]interface{}) error {
	token, err := ExtractToken(credentials)
	if err != nil {
		return err
	}

	providerData := server.ProviderData
	if providerData == nil {
		return nil
	}

	client := p.NewClient(token)

	if vultrID := GetStringField(providerData, "vultr_id", ""); vultrID != "" {
		DoDeleteIgnoreErrors(ctx, client, "/instances/"+vultrID)
	}

	if sshKeyID := GetStringField(providerData, "ssh_key_id", ""); sshKeyID != "" {
		DoDeleteIgnoreErrors(ctx, client, "/ssh-keys/"+sshKeyID)
	}

	return nil
}

// GetPublicIPv4 fetches the public IPv4 address
func (p *VultrProvider) GetPublicIPv4(ctx context.Context, server *models.Server, credentials map[string]interface{}) (string, error) {
	token, err := ExtractToken(credentials)
	if err != nil {
		return "", err
	}

	providerData := server.ProviderData
	if providerData == nil {
		return "", ErrServerNotFound
	}

	vultrID := GetStringField(providerData, "vultr_id", "")
	if vultrID == "" {
		return "", ErrServerNotFound
	}

	client := p.NewClient(token)
	resp, err := DoGet(ctx, client, "/instances/"+vultrID)
	if err != nil {
		return "", WrapHTTPError(err, "get instance")
	}

	instance, ok := GetNestedMap(resp, "instance")
	if !ok {
		return "", fmt.Errorf("invalid instance response")
	}

	return GetStringField(instance, "main_ip", ""), nil
}

// GetImage returns the image ID for an operating system
func (p *VultrProvider) GetImage(os enums.OperatingSystem) string {
	return p.GetImageFromConfig(os, "2284") // Ubuntu 24.04
}

// CredentialRules returns validation rules
func (p *VultrProvider) CredentialRules() map[string]string {
	return CommonCredentialRules()
}

// CreateRules returns validation rules
func (p *VultrProvider) CreateRules() map[string]string {
	return CommonCreateRules()
}

// CredentialData extracts credential data
func (p *VultrProvider) CredentialData(input map[string]interface{}) map[string]interface{} {
	return CommonCredentialData(input)
}

// ProviderData extracts provider-specific data
func (p *VultrProvider) ProviderData(input map[string]interface{}) map[string]interface{} {
	return CommonProviderData(input)
}

// Internal helper methods

type vultrSSHKeyResponse struct {
	ID string
}

func (p *VultrProvider) createSSHKey(ctx context.Context, client *httpclient.Client, name, publicKey string) (*vultrSSHKeyResponse, error) {
	body := map[string]interface{}{
		"name":    name,
		"ssh_key": publicKey,
	}

	resp, err := DoPost(ctx, client, "/ssh-keys", body)
	if err != nil {
		return nil, WrapHTTPError(err, "create SSH key")
	}

	sshKeyData, ok := GetNestedMap(resp, "ssh_key")
	if !ok {
		return nil, fmt.Errorf("invalid SSH key response")
	}

	return &vultrSSHKeyResponse{
		ID: GetStringField(sshKeyData, "id", ""),
	}, nil
}

type vultrInstanceResponse struct {
	ID        string
	MainIP    string
	VCPUCount int
	RAM       int
	Disk      int
}

func (p *VultrProvider) createInstance(ctx context.Context, client *httpclient.Client, name, region, plan, osID, sshKeyID string) (*vultrInstanceResponse, error) {
	body := map[string]interface{}{
		"label":     name,
		"region":    region,
		"plan":      plan,
		"os_id":     osID,
		"sshkey_id": []string{sshKeyID},
	}

	resp, err := DoPost(ctx, client, "/instances", body)
	if err != nil {
		return nil, WrapHTTPError(err, "create instance")
	}

	instance, ok := GetNestedMap(resp, "instance")
	if !ok {
		return nil, fmt.Errorf("invalid instance response")
	}

	return &vultrInstanceResponse{
		ID:        GetStringField(instance, "id", ""),
		MainIP:    GetStringField(instance, "main_ip", ""),
		VCPUCount: GetIntField(instance, "vcpu_count", 0),
		RAM:       GetIntField(instance, "ram", 0),
		Disk:      GetIntField(instance, "disk", 0),
	}, nil
}

func (p *VultrProvider) getOperatingSystem(server *models.Server) enums.OperatingSystem {
	if server.OperatingSystem != nil {
		return enums.OperatingSystem(*server.OperatingSystem)
	}
	return enums.OSUbuntu24
}
