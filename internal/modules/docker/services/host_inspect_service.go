package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// HostInspectService is a thin wrapper over `docker ps`, `docker
// volume ls`, `docker network ls`, and a Traefik-config dump. It runs
// the commands via SSH and parses the JSON line-by-line.
//
// Read-only; no mutations. Used by the docker-server detail tabs to
// give operators visibility into what's actually on the host without
// having to ssh in themselves.
type HostInspectService struct {
	*BaseService
}

func NewHostInspectService(deps *ServiceDeps) *HostInspectService {
	return &HostInspectService{BaseService: NewBaseService(deps)}
}

// ContainerInfo mirrors `docker ps --format '{{json .}}'`'s fields,
// minus the ones we don't render. Extra fields are tolerated by
// json.Unmarshal so docker's CLI evolution doesn't break us.
type ContainerInfo struct {
	ID      string `json:"ID"`
	Names   string `json:"Names"`
	Image   string `json:"Image"`
	Command string `json:"Command"`
	Status  string `json:"Status"`
	State   string `json:"State"`
	Ports   string `json:"Ports"`
	Created string `json:"CreatedAt"`
}

// VolumeInfo matches `docker volume ls --format '{{json .}}'`.
type VolumeInfo struct {
	Name       string `json:"Name"`
	Driver     string `json:"Driver"`
	Scope      string `json:"Scope"`
	Mountpoint string `json:"Mountpoint"`
}

// NetworkInfo matches `docker network ls --format '{{json .}}'`.
type NetworkInfo struct {
	ID     string `json:"ID"`
	Name   string `json:"Name"`
	Driver string `json:"Driver"`
	Scope  string `json:"Scope"`
}

// ListContainers returns every container on the host (running + stopped).
func (s *HostInspectService) ListContainers(
	ctx context.Context, serverID, teamID string,
) ([]ContainerInfo, error) {
	out, err := s.runDockerJSON(ctx, serverID, teamID,
		"docker ps -a --format '{{json .}}'")
	if err != nil {
		return nil, err
	}
	rows := make([]ContainerInfo, 0, len(out))
	for _, line := range out {
		var c ContainerInfo
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}
		rows = append(rows, c)
	}
	return rows, nil
}

func (s *HostInspectService) ListVolumes(
	ctx context.Context, serverID, teamID string,
) ([]VolumeInfo, error) {
	out, err := s.runDockerJSON(ctx, serverID, teamID,
		"docker volume ls --format '{{json .}}'")
	if err != nil {
		return nil, err
	}
	rows := make([]VolumeInfo, 0, len(out))
	for _, line := range out {
		var v VolumeInfo
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			continue
		}
		rows = append(rows, v)
	}
	return rows, nil
}

func (s *HostInspectService) ListNetworks(
	ctx context.Context, serverID, teamID string,
) ([]NetworkInfo, error) {
	out, err := s.runDockerJSON(ctx, serverID, teamID,
		"docker network ls --format '{{json .}}'")
	if err != nil {
		return nil, err
	}
	rows := make([]NetworkInfo, 0, len(out))
	for _, line := range out {
		var n NetworkInfo
		if err := json.Unmarshal([]byte(line), &n); err != nil {
			continue
		}
		rows = append(rows, n)
	}
	return rows, nil
}

// TraefikSnapshot bundles the current static + dynamic config files
// Traefik is reading from. Useful for "why isn't my domain working?"
// debugging.
type TraefikSnapshot struct {
	StaticConfig string            `json:"static_config"`
	DynamicFiles map[string]string `json:"dynamic_files"`
}

// GetTraefikSnapshot reads /etc/launch/traefik/traefik.yml plus every
// file in /etc/launch/traefik/dynamic/ via SSH. Files are returned by
// path → content; non-readable entries are dropped silently.
func (s *HostInspectService) GetTraefikSnapshot(
	ctx context.Context, serverID, teamID string,
) (TraefikSnapshot, error) {
	client, cleanup, err := s.dialServer(ctx, serverID, teamID)
	if err != nil {
		return TraefikSnapshot{}, err
	}
	defer cleanup()

	snap := TraefikSnapshot{DynamicFiles: map[string]string{}}

	if static, err := client.Run(ctx, "sudo cat /etc/launch/traefik/traefik.yml 2>/dev/null"); err == nil {
		snap.StaticConfig = static.Stdout
	}

	// List + cat each dynamic file. One round-trip per file; the set is
	// small (one per app) so this is fine.
	listing, err := client.Run(ctx, "sudo ls /etc/launch/traefik/dynamic 2>/dev/null")
	if err != nil {
		return snap, nil
	}
	for _, name := range splitNonEmpty(listing.Stdout) {
		// Avoid path traversal — we control these names so it's just
		// belt-and-braces.
		if strings.ContainsAny(name, "/\\") {
			continue
		}
		path := fmt.Sprintf("/etc/launch/traefik/dynamic/%s", name)
		cmd := fmt.Sprintf("sudo cat %s 2>/dev/null", path)
		if content, err := client.Run(ctx, cmd); err == nil {
			snap.DynamicFiles[name] = content.Stdout
		}
	}
	return snap, nil
}

// runDockerJSON dials the server, runs a `--format '{{json .}}'`
// command, and returns the non-empty output lines. Each line is its
// own JSON object — docker prints one per row.
func (s *HostInspectService) runDockerJSON(
	ctx context.Context, serverID, teamID, command string,
) ([]string, error) {
	client, cleanup, err := s.dialServer(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	result, err := client.Run(ctx, command)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		// Combine stdout+stderr for the error message — `docker ps`
		// failures usually go to stderr.
		return nil, fmt.Errorf("docker command failed (exit %d): %s %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
	return splitNonEmpty(result.Stdout), nil
}

// dialServer opens an SSH connection to a docker server, after
// validating the server belongs to the caller's team and is a docker
// server. Returns a cleanup func — defer it.
func (s *HostInspectService) dialServer(
	ctx context.Context, serverID, teamID string,
) (*taskrunner.SSHClient, func(), error) {
	_ = ctx
	server, err := s.ServerRepos().Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, nil, err
	}
	if server.Type == nil || *server.Type != string(servertypes.ServerTypeDocker) {
		return nil, nil, fiberutil.BadRequest("Server is not a docker server")
	}
	client, err := taskrunner.NewSSHClientFromServerAsRoot(server)
	if err != nil {
		return nil, nil, err
	}
	if err := client.Connect(); err != nil {
		return nil, nil, err
	}
	return client, func() { _ = client.Close() }, nil
}

func splitNonEmpty(s string) []string {
	out := []string{}
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
