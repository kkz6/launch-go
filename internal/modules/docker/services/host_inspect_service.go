package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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

// ContainerInspect is the curated subset of `docker inspect` we
// surface to the UI. We deliberately skip env vars (could leak
// secrets) and the raw config blob (overwhelming for a tooltip-ish
// view). Each field below maps to a real `docker inspect` location;
// missing values are returned empty rather than omitted so the UI
// can render a uniform table.
type ContainerInspect struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	ImageID string `json:"image_id"`
	// Command keeps the legacy single-string summary (Cmd joined with
	// spaces, falling back to Path + Args). The UI prefers Entrypoint
	// + Cmd as separate arrays when available — see below.
	Command   string `json:"command"`
	Entrypoint []string `json:"entrypoint,omitempty"`
	Cmd        []string `json:"cmd,omitempty"`
	// Path + Args are the *actual* exec'd binary + flags at runtime.
	// For containers where ENTRYPOINT is empty and CMD is set, Path
	// is the first element of Cmd; for the inverse, Path is the
	// entrypoint binary and Args are everything after. Surfacing both
	// lets the dialog show the structural view AND the runtime view.
	Path string   `json:"path,omitempty"`
	Args []string `json:"args,omitempty"`

	CreatedAt     string                    `json:"created_at"`
	State         ContainerInspectState     `json:"state"`
	Health        *ContainerInspectHealth   `json:"health,omitempty"`
	RestartCount  int                       `json:"restart_count"`
	Platform      string                    `json:"platform"`
	Resources     ContainerInspectResources `json:"resources"`
	RestartPolicy string                    `json:"restart_policy"`
	Mounts        []ContainerInspectMount   `json:"mounts"`
	Networks      []ContainerInspectNetwork `json:"networks"`
	Labels        map[string]string         `json:"labels,omitempty"`

	// Raw is the full `docker inspect <id>` JSON, passed through
	// unchanged. Powers the "View raw config" dialog — mirrors
	// dokploy's ShowContainerConfig view. json.RawMessage so we
	// don't pay the cost of re-marshalling something we just parsed.
	Raw json.RawMessage `json:"raw,omitempty"`
}

type ContainerInspectState struct {
	Status     string `json:"status"`
	Running    bool   `json:"running"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	ExitCode   int    `json:"exit_code"`
	Error      string `json:"error,omitempty"`
	OOMKilled  bool   `json:"oom_killed"`
	Pid        int    `json:"pid"`
}

type ContainerInspectHealth struct {
	Status        string                       `json:"status"` // healthy / unhealthy / starting
	FailingStreak int                          `json:"failing_streak"`
	Log           []ContainerInspectHealthLog `json:"log,omitempty"`
}

type ContainerInspectHealthLog struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
}

type ContainerInspectResources struct {
	MemoryLimitBytes int64   `json:"memory_limit_bytes"`
	CPUShares        int64   `json:"cpu_shares"`
	NanoCPUs         int64   `json:"nano_cpus"`
}

type ContainerInspectMount struct {
	Type        string `json:"type"`        // "bind" or "volume"
	Source      string `json:"source"`      // host path or volume name
	Destination string `json:"destination"` // in-container path
	ReadOnly    bool   `json:"read_only"`
}

type ContainerInspectNetwork struct {
	Name       string `json:"name"`
	IPAddress  string `json:"ip_address"`
	MACAddress string `json:"mac_address,omitempty"`
}

// rawContainerInspect is the slim subset of `docker inspect` JSON we
// decode. We pull only what ContainerInspect exposes — the inspect
// output is enormous and most of it is irrelevant to the UI.
type rawContainerInspect struct {
	ID      string `json:"Id"`
	Name    string `json:"Name"`
	Image   string `json:"Image"`
	Created string `json:"Created"`
	Path    string `json:"Path"`
	Args    []string `json:"Args"`
	State   struct {
		Status     string `json:"Status"`
		Running    bool   `json:"Running"`
		Paused     bool   `json:"Paused"`
		Restarting bool   `json:"Restarting"`
		OOMKilled  bool   `json:"OOMKilled"`
		ExitCode   int    `json:"ExitCode"`
		Error      string `json:"Error"`
		StartedAt  string `json:"StartedAt"`
		FinishedAt string `json:"FinishedAt"`
		Pid        int    `json:"Pid"`
		Health     *struct {
			Status        string `json:"Status"`
			FailingStreak int    `json:"FailingStreak"`
			Log           []struct {
				Start    string `json:"Start"`
				End      string `json:"End"`
				ExitCode int    `json:"ExitCode"`
				Output   string `json:"Output"`
			} `json:"Log"`
		} `json:"Health"`
	} `json:"State"`
	RestartCount int    `json:"RestartCount"`
	Platform     string `json:"Platform"`
	Config struct {
		Image      string            `json:"Image"`
		Cmd        []string          `json:"Cmd"`
		Entrypoint []string          `json:"Entrypoint"`
		Labels     map[string]string `json:"Labels"`
	} `json:"Config"`
	HostConfig struct {
		Memory     int64 `json:"Memory"`
		CPUShares  int64 `json:"CpuShares"`
		NanoCPUs   int64 `json:"NanoCpus"`
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
	} `json:"HostConfig"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		Mode        string `json:"Mode"`
		RW          bool   `json:"RW"`
	} `json:"Mounts"`
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress  string `json:"IPAddress"`
			MacAddress string `json:"MacAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

// InspectContainer runs `docker inspect <id>` on the host and projects
// the result into ContainerInspect. The container ID can be the short
// or long form; we treat it as untrusted and validate it to a hex
// charset to avoid command injection through the URL parameter.
func (s *HostInspectService) InspectContainer(
	ctx context.Context, serverID, teamID, containerID string,
) (ContainerInspect, error) {
	if !containerIDPattern.MatchString(containerID) {
		return ContainerInspect{}, fiberutil.BadRequest("Invalid container ID")
	}

	out, err := s.runDockerJSON(ctx, serverID, teamID,
		fmt.Sprintf("docker inspect %s --format '{{json .}}'", containerID))
	if err != nil {
		return ContainerInspect{}, err
	}
	if len(out) == 0 {
		return ContainerInspect{}, fiberutil.NotFound()
	}

	var raw rawContainerInspect
	if err := json.Unmarshal([]byte(out[0]), &raw); err != nil {
		return ContainerInspect{}, fmt.Errorf("decode docker inspect: %w", err)
	}

	projected := projectContainerInspect(raw)
	// Stash the raw JSON for the "View raw config" dialog. Doing this
	// here (not in the projector) keeps projectContainerInspect a pure
	// function the tests can construct inputs for.
	projected.Raw = json.RawMessage(out[0])
	return projected, nil
}

// containerIDPattern: short (12 hex) or long (64 hex) docker IDs only.
// Defends the docker-inspect command construction from injection via
// the URL path.
var containerIDPattern = regexp.MustCompile(`^[a-f0-9]{12,64}$`)

func projectContainerInspect(raw rawContainerInspect) ContainerInspect {
	out := ContainerInspect{
		ID:           strings.TrimPrefix(raw.ID, ""),
		Name:         strings.TrimPrefix(raw.Name, "/"),
		Image:        raw.Config.Image,
		ImageID:      raw.Image,
		CreatedAt:    raw.Created,
		Platform:     raw.Platform,
		RestartCount: raw.RestartCount,
		Labels:       raw.Config.Labels,
	}

	// Structural view: Entrypoint and Cmd come straight from Config.
	// Runtime view: Path + Args is what docker actually exec'd.
	// Both are useful — they often diverge (Entrypoint=traefik,
	// Cmd=[--api], Path=traefik, Args=[--api,--providers.docker,...]
	// on new servers; --providers.swarm on legacy pre-v2 servers).
	out.Entrypoint = raw.Config.Entrypoint
	out.Cmd = raw.Config.Cmd
	out.Path = raw.Path
	out.Args = raw.Args

	// Legacy single-string Command, kept for any caller that wants
	// the one-line summary. Prefer Cmd, fall back to Path + Args.
	if len(raw.Config.Cmd) > 0 {
		out.Command = strings.Join(raw.Config.Cmd, " ")
	} else if raw.Path != "" {
		out.Command = strings.TrimSpace(raw.Path + " " + strings.Join(raw.Args, " "))
	}

	out.State = ContainerInspectState{
		Status:     raw.State.Status,
		Running:    raw.State.Running,
		StartedAt:  raw.State.StartedAt,
		FinishedAt: raw.State.FinishedAt,
		ExitCode:   raw.State.ExitCode,
		Error:      raw.State.Error,
		OOMKilled:  raw.State.OOMKilled,
		Pid:        raw.State.Pid,
	}

	if raw.State.Health != nil {
		h := ContainerInspectHealth{
			Status:        raw.State.Health.Status,
			FailingStreak: raw.State.Health.FailingStreak,
		}
		// Keep the last 5 health-check entries — full log can be
		// hundreds of rows long and overwhelms the dialog.
		const maxLog = 5
		logs := raw.State.Health.Log
		if len(logs) > maxLog {
			logs = logs[len(logs)-maxLog:]
		}
		for _, l := range logs {
			h.Log = append(h.Log, ContainerInspectHealthLog{
				Start:    l.Start,
				End:      l.End,
				ExitCode: l.ExitCode,
				Output:   l.Output,
			})
		}
		out.Health = &h
	}

	out.Resources = ContainerInspectResources{
		MemoryLimitBytes: raw.HostConfig.Memory,
		CPUShares:        raw.HostConfig.CPUShares,
		NanoCPUs:         raw.HostConfig.NanoCPUs,
	}
	out.RestartPolicy = raw.HostConfig.RestartPolicy.Name

	for _, m := range raw.Mounts {
		source := m.Source
		if m.Type == "volume" && m.Name != "" {
			source = m.Name // for named volumes, the name is friendlier than the host path
		}
		out.Mounts = append(out.Mounts, ContainerInspectMount{
			Type:        m.Type,
			Source:      source,
			Destination: m.Destination,
			ReadOnly:    !m.RW,
		})
	}

	for name, n := range raw.NetworkSettings.Networks {
		out.Networks = append(out.Networks, ContainerInspectNetwork{
			Name:       name,
			IPAddress:  n.IPAddress,
			MACAddress: n.MacAddress,
		})
	}

	return out
}

// ListContainers returns every container on the host (running + stopped).
// Each row is tagged with `System=true` if Launch installed it. The UI
// hides system rows behind a toggle.
//
// Legacy (pre-v2) servers ran Traefik as a swarm service, which makes
// the spawned container's name `launch-traefik.1.<task-id>`. We still
// strip that suffix before classifying so old servers continue to
// hide their Traefik container behind the "system" filter. New
// servers run Traefik as a plain container named `launch-traefik`,
// where stripSwarmTaskSuffix is effectively a no-op.
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
// New (v2+) docker servers don't use swarm, so this helper is mostly
// a no-op for fresh provisions. It stays around to support pre-v2
// servers where Traefik runs as `docker service create launch-traefik`
// — see ForDockerServer() in server/types/provision_step.go for the
// rationale on dropping swarm.
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

// TraefikSnapshot bundles the user-visible Traefik config files —
// strictly the dynamic configs Launch generates for the user's apps,
// plus any custom YAML the user adds. Launch's own plumbing
// (traefik.yml static config, acme.json cert storage, certificate
// archives, access logs) is deliberately NOT included: this is a SaaS
// product and exposing the internal infrastructure invites edits we
// can't safely accept.
//
// Dokploy is self-hosted so it surfaces the whole /etc/dokploy/traefik
// tree; the trade-off here is intentional.
type TraefikSnapshot struct {
	DynamicFiles map[string]string `json:"dynamic_files"`
}

// dynamicFileExcludes — files we skip even when they sit inside the
// dynamic directory. Keep this list narrow; anything Launch wouldn't
// want users editing should live somewhere else.
var traefikDynamicExcludes = map[string]struct{}{
	"acme.json":      {},
	"access.log":     {},
	"access.log.tmp": {},
}

// GetTraefikSnapshot reads every file in /etc/launch/traefik/dynamic/
// via SSH and returns them as a name → content map. Files in
// traefikDynamicExcludes and any non-YAML extensions are filtered out.
//
// We intentionally don't read the static traefik.yml at the
// /etc/launch/traefik/ root — that file is Launch infrastructure and
// editing it would brick Traefik on the next domain change.
func (s *HostInspectService) GetTraefikSnapshot(
	ctx context.Context, serverID, teamID string,
) (TraefikSnapshot, error) {
	client, cleanup, err := s.dialServer(ctx, serverID, teamID)
	if err != nil {
		return TraefikSnapshot{}, err
	}
	defer cleanup()

	snap := TraefikSnapshot{DynamicFiles: map[string]string{}}

	// List + cat each dynamic file. One round-trip per file; the set
	// is small (one per app domain) so this is fine.
	listing, err := client.Run(ctx, "sudo ls /etc/launch/traefik/dynamic 2>/dev/null")
	if err != nil {
		return snap, nil
	}
	for _, name := range splitNonEmpty(listing.Stdout) {
		// Belt-and-braces: even though we control these names, refuse
		// any path-traversal-shaped entry.
		if strings.ContainsAny(name, "/\\") {
			continue
		}
		if _, excluded := traefikDynamicExcludes[name]; excluded {
			continue
		}
		// Filter to YAML files. Everything Launch writes is .yml; if a
		// user drops a non-YAML file in there we don't want to dump it
		// to the SaaS UI uncritically.
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
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

// WriteTraefikDynamicFile overwrites a file in
// /etc/launch/traefik/dynamic/ with the given contents.
//
// Hard constraints (path traversal + arbitrary write are the two
// failure modes that matter; anything else is a YAML validity concern
// best left to Traefik itself, which will refuse to load a bad file):
//   - filename must match safeTraefikFilenamePattern (no slashes, no
//     dots leading the name, must end .yml or .yaml)
//   - contents are capped at 256 KiB to bound the write
//
// The file is written via `sudo tee` so it inherits root ownership
// just like everything else in the dynamic directory.
func (s *HostInspectService) WriteTraefikDynamicFile(
	ctx context.Context, serverID, teamID, filename, contents string,
) error {
	if err := validateTraefikFilename(filename); err != nil {
		return err
	}
	const maxBytes = 256 * 1024
	if len(contents) > maxBytes {
		return fmt.Errorf("contents exceed %d bytes", maxBytes)
	}

	client, cleanup, err := s.dialServer(ctx, serverID, teamID)
	if err != nil {
		return err
	}
	defer cleanup()

	path := fmt.Sprintf("/etc/launch/traefik/dynamic/%s", filename)

	// Use a heredoc so we don't have to escape every shell metachar
	// in the YAML. The sentinel `__LAUNCH_EOF_<random>__` is chosen so
	// it can't appear in real config — capital underscore prefix is
	// rare in YAML, and we'd notice immediately if it ever clashed.
	cmd := fmt.Sprintf(
		`sudo tee %s >/dev/null <<'__LAUNCH_EOF_%s__'
%s
__LAUNCH_EOF_%s__
`,
		path,
		"TRAEFIK", // fixed sentinel; we only run one write at a time per host
		contents,
		"TRAEFIK",
	)
	result, err := client.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("write failed (exit %d): %s", result.ExitCode, result.Stderr)
	}
	return nil
}

// safeTraefikFilenamePattern enforces:
//   - 1–80 chars
//   - first char alphanumeric (no leading dot → no hidden files)
//   - body: alphanumeric, dash, dot, underscore
//   - ends in .yml or .yaml
var safeTraefikFilenamePattern = regexp.MustCompile(
	`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,75}\.(yml|yaml)$`,
)

func validateTraefikFilename(name string) error {
	if name == "" {
		return fmt.Errorf("filename is required")
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return fmt.Errorf("filename must not contain path separators or ..")
	}
	if _, excluded := traefikDynamicExcludes[name]; excluded {
		return fmt.Errorf("filename %q is reserved", name)
	}
	if !safeTraefikFilenamePattern.MatchString(name) {
		return fmt.Errorf("filename must be 1-80 chars, start with alphanumeric, and end .yml/.yaml")
	}
	return nil
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
