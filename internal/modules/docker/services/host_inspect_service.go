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
//
// System (set by ListContainers) marks containers Launch owns —
// Traefik, future control-plane services. The UI hides these by
// default behind a "Show system containers" toggle. Dokploy uses the
// same approach (excludes `dokploy*` from the user view).
type ContainerInfo struct {
	ID      string `json:"ID"`
	Names   string `json:"Names"`
	Image   string `json:"Image"`
	Command string `json:"Command"`
	Status  string `json:"Status"`
	State   string `json:"State"`
	Ports   string `json:"Ports"`
	Created string `json:"CreatedAt"`
	System  bool   `json:"system"`
}

// VolumeInfo matches `docker volume ls --format '{{json .}}'`.
// System has the same semantics as ContainerInfo.System.
type VolumeInfo struct {
	Name       string `json:"Name"`
	Driver     string `json:"Driver"`
	Scope      string `json:"Scope"`
	Mountpoint string `json:"Mountpoint"`
	System     bool   `json:"system"`
}

// NetworkInfo matches `docker network ls --format '{{json .}}'`.
//
// System has the same semantics as ContainerInfo.System, plus we mark
// docker's three built-in networks (bridge / host / none) as system
// — they exist on every docker host and aren't user-managed.
type NetworkInfo struct {
	ID     string `json:"ID"`
	Name   string `json:"Name"`
	Driver string `json:"Driver"`
	Scope  string `json:"Scope"`
	System bool   `json:"system"`
}

// isLaunchSystemName matches the names we set on containers/volumes/
// networks that Launch installs as part of provisioning. Mirrors
// dokploy's `name.includes("dokploy")` rule.
//
// Naming convention to keep this honest:
//   - System containers: `launch-<service>` (e.g. launch-traefik)
//   - User app containers: `launch-app-<project>-<name>` (NOT system)
//   - User database containers: `launch-db-<project>-<name>` (NOT system)
//
// We classify only what starts with `launch-` AND is NOT one of those
// user prefixes. Anything outside the `launch-` namespace is user-
// supplied even if it happens to contain the substring.
func isLaunchSystemName(name string) bool {
	if !strings.HasPrefix(name, "launch-") {
		return false
	}
	rest := strings.TrimPrefix(name, "launch-")
	switch {
	case strings.HasPrefix(rest, "app-"),
		strings.HasPrefix(rest, "db-"),
		strings.HasPrefix(rest, "compose-"),
		strings.HasPrefix(rest, "build-"):
		return false
	}
	return true
}

// dockerBuiltinNetworks: docker's three default networks. Marked
// system so they don't clutter the user's Networks tab.
var dockerBuiltinNetworks = map[string]struct{}{
	"bridge": {},
	"host":   {},
	"none":   {},
}

// ListContainers returns every container on the host (running + stopped).
// Each row is tagged with `System=true` if Launch installed it. The UI
// hides system rows behind a toggle.
//
// Containers spawned by swarm services carry a name like
// `launch-traefik.1.<task-id>` — we strip the swarm task suffix
// before classifying so the service-name-based match works.
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
		c.System = isLaunchSystemName(stripSwarmTaskSuffix(c.Names))
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
		v.System = isLaunchSystemName(v.Name)
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
		// Mark Launch-installed networks AND docker's built-ins as system.
		// The Networks tab is user-noisy without this — bridge/host/none
		// are noise to a user who wants to see their own networks.
		if _, builtin := dockerBuiltinNetworks[n.Name]; builtin {
			n.System = true
		} else if isLaunchSystemName(n.Name) || n.Name == "launch-network" {
			n.System = true
		}
		rows = append(rows, n)
	}
	return rows, nil
}

// stripSwarmTaskSuffix removes the `.<replica>.<task-id>` part docker
// appends to swarm-spawned containers. So `launch-traefik.1.ktub95...`
// becomes `launch-traefik` for the purposes of classification.
//
// Containers can have comma-separated names ("foo,bar") if they're
// linked — we classify on the first which is the canonical one.
func stripSwarmTaskSuffix(names string) string {
	first := names
	if i := strings.Index(first, ","); i >= 0 {
		first = first[:i]
	}
	// `name.<replica>.<task-id>` — split on first dot, the prefix is
	// the service name.
	if i := strings.Index(first, "."); i >= 0 {
		return first[:i]
	}
	return first
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
