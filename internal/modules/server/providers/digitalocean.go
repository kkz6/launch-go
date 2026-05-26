package providers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/httpclient"
	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
)

const digitalOceanAPIURL = "https://api.digitalocean.com/v2"

// DigitalOceanProvider implements the Provider interface for DigitalOcean
type DigitalOceanProvider struct {
	BaseCloudProvider
}

// NewDigitalOceanProvider creates a new DigitalOcean provider
func NewDigitalOceanProvider(keyGenerator sshkey.Generator) *DigitalOceanProvider {
	configs := config.GetProviderConfigs()
	return &DigitalOceanProvider{
		BaseCloudProvider: NewBaseCloudProvider(
			keyGenerator,
			configs["digitalocean"],
			digitalOceanAPIURL,
		),
	}
}

// Type returns the provider type
func (p *DigitalOceanProvider) Type() types.ServerProvider {
	return types.ProviderDigitalOcean
}

// Connect tests the connection to DigitalOcean
func (p *DigitalOceanProvider) Connect(ctx context.Context, credentials map[string]interface{}) error {
	token, err := ExtractToken(credentials)
	if err != nil {
		return err
	}

	client := p.NewClient(token)
	return ValidateConnection(ctx, client, "/account")
}

// Create creates a new server on DigitalOcean
func (p *DigitalOceanProvider) Create(ctx context.Context, server *models.Server, credentials map[string]interface{}) (*CreateResult, error) {
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

	// Create SSH key on DigitalOcean
	sshKeyName := strings.ReplaceAll(strings.ToLower(server.Name), " ", "-") + "-" + server.ID
	sshKeyResp, err := p.createSSHKey(ctx, client, sshKeyName, keyPair.PublicKey)
	if err != nil {
		return nil, err
	}

	// Get provider data
	providerData := GetProviderData(server.ProviderData)
	region := GetStringField(providerData, "region", "")
	plan := GetStringField(providerData, "plan", "")
	os := p.getOperatingSystem(server)
	image := p.GetImage(os)

	// Create droplet — pass numeric SSH key ID so DO treats it as an ID, not a fingerprint
	droplet, err := p.createDroplet(ctx, client, sshKeyName, region, plan, image, sshKeyResp.NumericID)
	if err != nil {
		return nil, err
	}

	return &CreateResult{
		ProviderServerID: droplet.ID,
		PublicKey:        keyPair.PublicKey,
		PrivateKey:       keyPair.PrivateKey,
		CPUCores:         droplet.VCPUs,
		MemoryMB:         droplet.Memory,
		DiskGB:           droplet.Disk,
		SSHKeyID:         sshKeyResp.ID,
		ProviderData: map[string]interface{}{
			"droplet_id": droplet.ID,
			"ssh_key_id": sshKeyResp.ID,
		},
	}, nil
}

// Delete deletes a server from DigitalOcean
func (p *DigitalOceanProvider) Delete(ctx context.Context, server *models.Server, credentials map[string]interface{}) error {
	token, err := ExtractToken(credentials)
	if err != nil {
		return err
	}

	providerData := server.ProviderData
	if providerData == nil {
		return nil // Nothing to delete
	}

	dropletID := GetStringField(providerData, "droplet_id", "")
	if dropletID == "" {
		return nil // Nothing to delete
	}

	client := p.NewClient(token)
	DoDeleteIgnoreErrors(ctx, client, "/droplets/"+dropletID)
	return nil
}

// GetPublicIPv4 fetches the public IPv4 address of a server
func (p *DigitalOceanProvider) GetPublicIPv4(ctx context.Context, server *models.Server, credentials map[string]interface{}) (string, error) {
	token, err := ExtractToken(credentials)
	if err != nil {
		return "", err
	}

	providerData := server.ProviderData
	if providerData == nil {
		return "", fiberutil.NotFound()
	}

	dropletID := GetStringField(providerData, "droplet_id", "")
	if dropletID == "" {
		return "", fiberutil.NotFound()
	}

	client := p.NewClient(token)
	resp, err := DoGet(ctx, client, "/droplets/"+dropletID)
	if err != nil {
		return "", WrapHTTPError(err, "get droplet")
	}

	return p.extractPublicIPv4(resp)
}

// GetImage returns the image ID for an operating system
func (p *DigitalOceanProvider) GetImage(os types.OperatingSystem) string {
	return p.GetImageFromConfig(os, "ubuntu-24-04-x64")
}

// LookupImage asks DO whether the given image (slug or numeric ID) exists.
// Returns nil if DO returns the image, an error otherwise. Used by
// cmd/validate-images to catch retired image identifiers before they
// surface as 422s during provisioning.
func (p *DigitalOceanProvider) LookupImage(ctx context.Context, credentials map[string]interface{}, slug string) error {
	token, err := ExtractToken(credentials)
	if err != nil {
		return err
	}
	client := p.NewClient(token)
	if _, err := DoGet(ctx, client, "/images/"+slug); err != nil {
		return WrapHTTPError(err, "lookup image "+slug)
	}
	return nil
}

// CredentialRules returns validation rules for credentials
func (p *DigitalOceanProvider) CredentialRules() map[string]string {
	return CommonCredentialRules()
}

// CreateRules returns validation rules for server creation
func (p *DigitalOceanProvider) CreateRules() map[string]string {
	return CommonCreateRules()
}

// CredentialData extracts credential data from input
func (p *DigitalOceanProvider) CredentialData(input map[string]interface{}) map[string]interface{} {
	return CommonCredentialData(input)
}

// ProviderData extracts provider-specific data from input
func (p *DigitalOceanProvider) ProviderData(input map[string]interface{}) map[string]interface{} {
	return CommonProviderData(input)
}

// Internal helper methods

type doSSHKeyResponse struct {
	ID          string
	NumericID   int64
	Fingerprint string
}

func (p *DigitalOceanProvider) createSSHKey(ctx context.Context, client *httpclient.Client, name, publicKey string) (*doSSHKeyResponse, error) {
	body := map[string]interface{}{
		"name":       name,
		"public_key": publicKey,
	}

	resp, err := DoPost(ctx, client, "/account/keys", body)
	if err != nil {
		return nil, WrapHTTPError(err, "create SSH key")
	}

	sshKeyData, ok := GetNestedMap(resp, "ssh_key")
	if !ok {
		return nil, fmt.Errorf("invalid SSH key response")
	}

	idStr := ExtractServerID(sshKeyData, "id")
	numericID, _ := strconv.ParseInt(idStr, 10, 64)
	fingerprint, _ := sshKeyData["fingerprint"].(string)

	return &doSSHKeyResponse{
		ID:          idStr,
		NumericID:   numericID,
		Fingerprint: fingerprint,
	}, nil
}

type doDropletResponse struct {
	ID     string
	VCPUs  int
	Memory int
	Disk   int
}

func (p *DigitalOceanProvider) createDroplet(ctx context.Context, client *httpclient.Client, name, region, plan, image string, sshKeyID int64) (*doDropletResponse, error) {
	body := map[string]interface{}{
		"name":       name,
		"region":     region,
		"size":       plan,
		"image":      image,
		"backups":    false,
		"ipv6":       false,
		"monitoring": false,
		"ssh_keys":   []interface{}{sshKeyID},
	}

	log.Info().
		Str("name", name).
		Str("region", region).
		Str("size", plan).
		Str("image", image).
		Int64("ssh_key_id", sshKeyID).
		Msg("DO createDroplet: sending request")

	resp, err := DoPost(ctx, client, "/droplets", body)
	if err != nil {
		return nil, WrapHTTPError(err, "create droplet")
	}

	droplet, ok := GetNestedMap(resp, "droplet")
	if !ok {
		return nil, fmt.Errorf("invalid droplet response")
	}

	return &doDropletResponse{
		ID:     ExtractServerID(droplet, "id"),
		VCPUs:  GetIntField(droplet, "vcpus", 0),
		Memory: GetIntField(droplet, "memory", 0),
		Disk:   GetIntField(droplet, "disk", 0),
	}, nil
}

func (p *DigitalOceanProvider) extractPublicIPv4(resp map[string]interface{}) (string, error) {
	droplet, ok := GetNestedMap(resp, "droplet")
	if !ok {
		return "", fmt.Errorf("invalid droplet response")
	}

	networks, ok := GetNestedMap(droplet, "networks")
	if !ok {
		return "", nil
	}

	v4Networks, ok := GetNestedArray(networks, "v4")
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

func (p *DigitalOceanProvider) getOperatingSystem(server *models.Server) types.OperatingSystem {
	if server.OperatingSystem != nil {
		return types.OperatingSystem(*server.OperatingSystem)
	}
	return types.OSUbuntu24
}
