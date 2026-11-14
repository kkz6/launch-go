package services

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

type contaboAPI interface {
	ListInstances(context.Context, providers.ContaboCredentials, int, int) (providers.ContaboInstancePage, error)
	GetInstance(context.Context, providers.ContaboCredentials, int64) (providers.ContaboInstance, error)
}

func (s *Service) contaboCredentials(ctx context.Context, accountID, teamID string) (providers.ContaboCredentials, error) {
	var creds providers.ContaboCredentials
	account, err := s.repos.ServerProvider().FindByID(ctx, accountID)
	if err != nil {
		return creds, err
	}
	if !account.CanProvision(teamID, types.ProviderContabo) {
		return creds, fiberutil.NotFound("Contabo connection not found")
	}
	if err := json.Unmarshal([]byte(account.Credentials.String()), &creds); err != nil || !creds.Complete() {
		return creds, fiberutil.BadRequest("Reconnect this Contabo account to update its credentials")
	}
	return creds, nil
}

func (s *Service) ListContaboVPSs(ctx context.Context, accountID, teamID string, page int) (dto.ContaboVPSPage, error) {
	creds, err := s.contaboCredentials(ctx, accountID, teamID)
	if err != nil {
		return dto.ContaboVPSPage{}, err
	}
	result, err := s.contabo.ListInstances(ctx, creds, page, 25)
	if err != nil {
		return dto.ContaboVPSPage{}, err
	}
	keys := make([]string, 0, len(result.Data))
	for _, instance := range result.Data {
		keys = append(keys, instance.Key())
	}
	imported := make(map[string]string, len(keys))
	if len(keys) > 0 {
		servers, err := s.repos.Server().FindContaboImports(ctx, teamID, keys)
		if err != nil {
			return dto.ContaboVPSPage{}, err
		}
		for _, server := range servers {
			if server.ContaboInstanceKey != nil {
				imported[*server.ContaboInstanceKey] = server.ID
			}
		}
	}

	instances := make([]dto.ContaboVPS, 0, len(result.Data))
	for _, instance := range result.Data {
		if instance.ID <= 0 {
			continue
		}
		serverID := imported[instance.Key()]
		instances = append(instances, dto.ContaboVPS{
			ID: strconv.FormatInt(instance.ID, 10), Name: contaboInstanceName(instance),
			IP: instance.PublicIPv4(), Region: firstNonEmpty(instance.RegionName, instance.Region),
			Plan: firstNonEmpty(instance.ProductName, instance.ProductID), Status: instance.Status,
			OSType: instance.OSType, CanImport: instance.CanImport() && serverID == "", ImportedServerID: serverID,
		})
	}
	return dto.ContaboVPSPage{Instances: instances, Page: page, TotalPages: result.Pagination.TotalPages}, nil
}

func (s *Service) ImportContaboVPS(ctx context.Context, accountID, teamID, userID string, req *dto.ImportContaboVPSRequest) (dto.ServerResponse, error) {
	instanceID, err := strconv.ParseInt(req.InstanceID, 10, 64)
	if err != nil || instanceID <= 0 {
		return dto.ServerResponse{}, fiberutil.Validation("Invalid Contabo VPS ID")
	}
	creds, err := s.contaboCredentials(ctx, accountID, teamID)
	if err != nil {
		return dto.ServerResponse{}, err
	}
	instance, err := s.contabo.GetInstance(ctx, creds, instanceID)
	if err != nil {
		return dto.ServerResponse{}, err
	}
	if !instance.CanImport() {
		return dto.ServerResponse{}, fiberutil.Validation("Select a running Linux VPS with a public IPv4 address")
	}

	// Use the manual SSH onboarding flow. Keeping this a Custom Server
	// ensures that removing its Launchctl record cannot cancel the VPS.
	key := instance.Key()
	server, err := s.buildAndDispatchServer(ctx, teamID, userID, &dto.CreateServerRequest{
		Name: contaboInstanceName(instance), Provider: types.ProviderCustom.String(),
		Type: req.Type, OperatingSystem: req.OperatingSystem, IP: instance.PublicIPv4(), SSHPort: req.SSHPort,
	}, &key)
	if err != nil {
		return dto.ServerResponse{}, err
	}
	return dto.ToServerResponse(server), nil
}

func contaboInstanceName(instance providers.ContaboInstance) string {
	name := []rune(firstNonEmpty(instance.DisplayName, instance.Name))
	if len(name) < 2 {
		return "Contabo VPS " + strconv.FormatInt(instance.ID, 10)
	}
	return string(name[:min(len(name), 255)])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
