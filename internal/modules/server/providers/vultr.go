package providers

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/httpclient"
	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
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
func (p *VultrProvider) Type() types.ServerProvider {
	return types.ProviderVultr
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
	osID, err := strconv.Atoi(p.GetImage(p.getOperatingSystem(server)))
	if err != nil || osID <= 0 {
		return nil, fmt.Errorf("invalid Vultr OS image ID")
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
		return fmt.Errorf("cloud instance ID is missing")
	}

	client := p.NewClient(token)

	vultrID := GetStringField(providerData, "vultr_id", GetStringField(providerData, "provider_server_id", ""))
	if vultrID == "" {
		return fmt.Errorf("cloud instance ID is missing")
	}
	if err := DoDelete(ctx, client, "/instances/"+vultrID); err != nil {
		return err
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
		return "", fiberutil.NotFound()
	}

	vultrID := GetStringField(providerData, "vultr_id", "")
	if vultrID == "" {
		return "", fiberutil.NotFound()
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
func (p *VultrProvider) GetImage(os types.OperatingSystem) string {
	return p.GetImageFromConfig(os, "2284") // Ubuntu 24.04
}

// LookupImage checks the paginated Vultr OS catalog. Vultr does not
// provide an OS detail endpoint.
func (p *VultrProvider) LookupImage(ctx context.Context, credentials map[string]interface{}, image string) error {
	token, err := ExtractToken(credentials)
	if err != nil {
		return err
	}
	client := p.NewClient(token)
	var cursor string
	for {
		path := "/os?per_page=500"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		var page struct {
			OS []struct {
				ID int `json:"id"`
			} `json:"os"`
			Meta struct {
				Links struct {
					Next string `json:"next"`
				} `json:"links"`
			} `json:"meta"`
		}
		if err := client.Get(ctx, path, &page); err != nil {
			return WrapHTTPError(err, "list Vultr operating systems")
		}
		for _, os := range page.OS {
			if fmt.Sprint(os.ID) == image {
				return nil
			}
		}
		next := page.Meta.Links.Next
		if next == "" {
			return fmt.Errorf("OS %q not found in Vultr catalog", image)
		}
		if next == cursor {
			return fmt.Errorf("Vultr OS catalog repeated its pagination cursor")
		}
		cursor = next
	}
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

func (p *VultrProvider) createInstance(ctx context.Context, client *httpclient.Client, name, region, plan string, osID int, sshKeyID string) (*vultrInstanceResponse, error) {
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

func (p *VultrProvider) getOperatingSystem(server *models.Server) types.OperatingSystem {
	if server.OperatingSystem != nil {
		return types.OperatingSystem(*server.OperatingSystem)
	}
	return types.OSUbuntu24
}
