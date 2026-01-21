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

const linodeAPIURL = "https://api.linode.com/v4"

// LinodeProvider implements the Provider interface for Linode
type LinodeProvider struct {
	BaseCloudProvider
}

// NewLinodeProvider creates a new Linode provider
func NewLinodeProvider(keyGenerator sshkey.Generator) *LinodeProvider {
	configs := config.GetProviderConfigs()
	return &LinodeProvider{
		BaseCloudProvider: NewBaseCloudProvider(
			keyGenerator,
			configs["linode"],
			linodeAPIURL,
		),
	}
}

// Type returns the provider type
func (p *LinodeProvider) Type() enums.ServerProvider {
	return enums.ProviderLinode
}

// Connect tests the connection to Linode
func (p *LinodeProvider) Connect(ctx context.Context, credentials map[string]interface{}) error {
	token, err := ExtractToken(credentials)
	if err != nil {
		return err
	}

	client := p.NewClient(token)
	return ValidateConnection(ctx, client, "/account")
}

// Create creates a new server on Linode
func (p *LinodeProvider) Create(ctx context.Context, server *models.Server, credentials map[string]interface{}) (*CreateResult, error) {
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

	// Get provider data
	providerData := GetProviderData(server.ProviderData)
	region := GetStringField(providerData, "region", "")
	plan := GetStringField(providerData, "plan", "")
	os := p.getOperatingSystem(server)
	image := p.GetImage(os)

	// Create Linode instance (Linode doesn't require separate SSH key creation)
	serverName := strings.ReplaceAll(strings.ToLower(server.Name), " ", "-")
	linodeResp, err := p.createLinode(ctx, client, serverName, region, plan, image, keyPair.PublicKey)
	if err != nil {
		return nil, err
	}

	return &CreateResult{
		ProviderServerID: linodeResp.ID,
		PublicIPv4:       linodeResp.PublicIP,
		PublicKey:        keyPair.PublicKey,
		PrivateKey:       keyPair.PrivateKey,
		CPUCores:         linodeResp.VCPUs,
		MemoryMB:         linodeResp.Memory,
		DiskGB:           linodeResp.Disk / 1024, // Linode returns disk in MB
		ProviderData: map[string]interface{}{
			"linode_id": linodeResp.ID,
		},
	}, nil
}

// Delete deletes a server from Linode
func (p *LinodeProvider) Delete(ctx context.Context, server *models.Server, credentials map[string]interface{}) error {
	token, err := ExtractToken(credentials)
	if err != nil {
		return err
	}

	providerData := server.ProviderData
	if providerData == nil {
		return nil
	}

	linodeID := GetStringField(providerData, "linode_id", "")
	if linodeID == "" {
		return nil
	}

	client := p.NewClient(token)
	DoDeleteIgnoreErrors(ctx, client, "/linode/instances/"+linodeID)
	return nil
}

// GetPublicIPv4 fetches the public IPv4 address
func (p *LinodeProvider) GetPublicIPv4(ctx context.Context, server *models.Server, credentials map[string]interface{}) (string, error) {
	token, err := ExtractToken(credentials)
	if err != nil {
		return "", err
	}

	providerData := server.ProviderData
	if providerData == nil {
		return "", ErrServerNotFound
	}

	linodeID := GetStringField(providerData, "linode_id", "")
	if linodeID == "" {
		return "", ErrServerNotFound
	}

	client := p.NewClient(token)
	resp, err := DoGet(ctx, client, "/linode/instances/"+linodeID)
	if err != nil {
		return "", WrapHTTPError(err, "get linode")
	}

	return p.extractPublicIPv4(resp)
}

// GetImage returns the image ID for an operating system
func (p *LinodeProvider) GetImage(os enums.OperatingSystem) string {
	return p.GetImageFromConfig(os, "linode/ubuntu24.04")
}

// CredentialRules returns validation rules
func (p *LinodeProvider) CredentialRules() map[string]string {
	return CommonCredentialRules()
}

// CreateRules returns validation rules
func (p *LinodeProvider) CreateRules() map[string]string {
	return CommonCreateRules()
}

// CredentialData extracts credential data
func (p *LinodeProvider) CredentialData(input map[string]interface{}) map[string]interface{} {
	return CommonCredentialData(input)
}

// ProviderData extracts provider-specific data
func (p *LinodeProvider) ProviderData(input map[string]interface{}) map[string]interface{} {
	return CommonProviderData(input)
}

// Internal helper methods

type linodeResponse struct {
	ID       string
	PublicIP string
	VCPUs    int
	Memory   int
	Disk     int
}

func (p *LinodeProvider) createLinode(ctx context.Context, client *httpclient.Client, name, region, plan, image, publicKey string) (*linodeResponse, error) {
	body := map[string]interface{}{
		"label":           name,
		"region":          region,
		"type":            plan,
		"image":           image,
		"authorized_keys": []string{publicKey},
		"booted":          true,
	}

	resp, err := DoPost(ctx, client, "/linode/instances", body)
	if err != nil {
		return nil, WrapHTTPError(err, "create linode")
	}

	linodeID := ExtractServerID(resp, "id")

	var publicIP string
	if ipv4, ok := GetNestedArray(resp, "ipv4"); ok && len(ipv4) > 0 {
		publicIP, _ = ipv4[0].(string)
	}

	var specs struct {
		vcpus  int
		memory int
		disk   int
	}
	if specsData, ok := GetNestedMap(resp, "specs"); ok {
		specs.vcpus = GetIntField(specsData, "vcpus", 0)
		specs.memory = GetIntField(specsData, "memory", 0)
		specs.disk = GetIntField(specsData, "disk", 0)
	}

	return &linodeResponse{
		ID:       linodeID,
		PublicIP: publicIP,
		VCPUs:    specs.vcpus,
		Memory:   specs.memory,
		Disk:     specs.disk,
	}, nil
}

func (p *LinodeProvider) extractPublicIPv4(resp map[string]interface{}) (string, error) {
	if ipv4, ok := GetNestedArray(resp, "ipv4"); ok && len(ipv4) > 0 {
		if ip, ok := ipv4[0].(string); ok {
			return ip, nil
		}
	}
	return "", nil
}

func (p *LinodeProvider) getOperatingSystem(server *models.Server) enums.OperatingSystem {
	if server.OperatingSystem != nil {
		return enums.OperatingSystem(*server.OperatingSystem)
	}
	return enums.OSUbuntu24
}
