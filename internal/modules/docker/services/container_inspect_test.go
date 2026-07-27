package services

import "testing"

func TestContainerIDPattern(t *testing.T) {
	// Whitelist hex-only IDs — anything else gets rejected before we
	// interpolate the value into a `docker inspect <id>` shell call.
	good := []string{
		"757ed9547979", // 12-char short ID
		"757ed9547979aa5566bb1234567890ababcdcdef12345678901234567890aaaa", // 64-char long ID
		"000000000000",
	}
	bad := []string{
		"",
		"abc",                               // too short
		"757ed9547979g0",                    // non-hex char
		"foo; rm -rf /",                     // injection attempt
		"757ed9547979 --format='something'", // arg injection
		"757ed9547979\n",                    // newline
		"757ed9547979/",                     // slash
		"$(whoami)",                         // command sub
	}
	for _, id := range good {
		if !containerIDPattern.MatchString(id) {
			t.Errorf("good id %q should match", id)
		}
	}
	for _, id := range bad {
		if containerIDPattern.MatchString(id) {
			t.Errorf("bad id %q should NOT match", id)
		}
	}
}

func TestProjectContainerInspect_StripsLeadingSlashFromName(t *testing.T) {
	// Docker prefixes container names with "/" in the inspect output.
	// The Containers tab uses the bare name (no slash) — strip it
	// here so the dialog and the table agree.
	raw := rawContainerInspect{Name: "/launch-traefik.1.kt95"}
	got := projectContainerInspect(raw)
	if got.Name != "launch-traefik.1.kt95" {
		t.Errorf("Name = %q, want %q (leading slash should be stripped)",
			got.Name, "launch-traefik.1.kt95")
	}
}

func TestProjectContainerInspect_CommandFallbackToPathArgs(t *testing.T) {
	// Some images set ENTRYPOINT but no CMD; inspect returns
	// Config.Cmd as null and surfaces the entrypoint at Path/Args.
	// We fall back so the dialog always has *something* in the
	// Command field.
	raw := rawContainerInspect{Path: "/usr/bin/traefik", Args: []string{"--api"}}
	got := projectContainerInspect(raw)
	if got.Command != "/usr/bin/traefik --api" {
		t.Errorf("Command = %q, want %q", got.Command, "/usr/bin/traefik --api")
	}
}

func TestProjectContainerInspect_HealthLogCapped(t *testing.T) {
	// Health check log can grow to hundreds of entries; we only keep
	// the last 5 so the dialog stays a sensible size.
	raw := rawContainerInspect{}
	raw.State.Health = &rawContainerHealth{Status: "healthy"}
	for i := 0; i < 12; i++ {
		raw.State.Health.Log = append(raw.State.Health.Log, rawContainerHealthLog{})
	}
	got := projectContainerInspect(raw)
	if got.Health == nil {
		t.Fatal("Health should be populated")
	}
	if len(got.Health.Log) != 5 {
		t.Errorf("Health.Log length = %d, want 5", len(got.Health.Log))
	}
}

func TestProjectContainerInspect_KeepsEntrypointCmdPathArgsSeparately(t *testing.T) {
	// For docker, "command" splits into Entrypoint + Cmd (structural)
	// AND Path + Args (runtime). The dialog renders each branch so a
	// debugging session has the full picture without resorting to
	// `docker inspect` over SSH. Pin that the projector preserves
	// each list verbatim rather than over-summarising.
	raw := rawContainerInspect{
		Path: "traefik",
		Args: []string{"--api", "--providers.docker.network=launch-network"},
	}
	raw.Config.Entrypoint = []string{"/entrypoint.sh"}
	raw.Config.Cmd = []string{"--api"}

	got := projectContainerInspect(raw)
	if len(got.Entrypoint) != 1 || got.Entrypoint[0] != "/entrypoint.sh" {
		t.Errorf("Entrypoint = %v, want [/entrypoint.sh]", got.Entrypoint)
	}
	if len(got.Cmd) != 1 || got.Cmd[0] != "--api" {
		t.Errorf("Cmd = %v, want [--api]", got.Cmd)
	}
	if got.Path != "traefik" {
		t.Errorf("Path = %q, want traefik", got.Path)
	}
	if len(got.Args) != 2 {
		t.Errorf("Args length = %d, want 2", len(got.Args))
	}
	// Legacy summary still set — the joined Cmd, since it's present.
	if got.Command != "--api" {
		t.Errorf("Command summary = %q, want %q", got.Command, "--api")
	}
}

func TestProjectContainerInspect_VolumeMountFavorsName(t *testing.T) {
	// For type=volume mounts, the Source on the host is something
	// like /var/lib/docker/volumes/<sha>/_data which is useless to
	// the user. Show the volume Name instead.
	raw := rawContainerInspect{
		Mounts: []rawContainerMount{
			{Type: "volume", Name: "acme-data", Source: "/var/lib/docker/volumes/acme-data/_data", Destination: "/data", RW: true},
			{Type: "bind", Source: "/etc/launch/traefik", Destination: "/etc/traefik", RW: false},
		},
	}
	got := projectContainerInspect(raw)
	if got.Mounts[0].Source != "acme-data" {
		t.Errorf("named volume Source = %q, want %q", got.Mounts[0].Source, "acme-data")
	}
	if got.Mounts[0].ReadOnly {
		t.Errorf("RW=true mount should report read_only=false")
	}
	if got.Mounts[1].Source != "/etc/launch/traefik" {
		t.Errorf("bind mount Source = %q, want host path", got.Mounts[1].Source)
	}
	if !got.Mounts[1].ReadOnly {
		t.Errorf("RW=false mount should report read_only=true")
	}
}
