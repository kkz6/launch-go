package services

import "testing"

func TestContainerIDPattern(t *testing.T) {
	// Whitelist hex-only IDs — anything else gets rejected before we
	// interpolate the value into a `docker inspect <id>` shell call.
	good := []string{
		"757ed9547979",                                                     // 12-char short ID
		"757ed9547979aa5566bb1234567890ababcdcdef12345678901234567890aaaa", // 64-char long ID
		"000000000000",
	}
	bad := []string{
		"",
		"abc",                                  // too short
		"757ed9547979g0",                       // non-hex char
		"foo; rm -rf /",                        // injection attempt
		"757ed9547979 --format='something'",    // arg injection
		"757ed9547979\n",                       // newline
		"757ed9547979/",                        // slash
		"$(whoami)",                            // command sub
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
	raw.State.Health = &struct {
		Status        string `json:"Status"`
		FailingStreak int    `json:"FailingStreak"`
		Log           []struct {
			Start    string `json:"Start"`
			End      string `json:"End"`
			ExitCode int    `json:"ExitCode"`
			Output   string `json:"Output"`
		} `json:"Log"`
	}{Status: "healthy"}
	for i := 0; i < 12; i++ {
		raw.State.Health.Log = append(raw.State.Health.Log, struct {
			Start    string `json:"Start"`
			End      string `json:"End"`
			ExitCode int    `json:"ExitCode"`
			Output   string `json:"Output"`
		}{})
	}
	got := projectContainerInspect(raw)
	if got.Health == nil {
		t.Fatal("Health should be populated")
	}
	if len(got.Health.Log) != 5 {
		t.Errorf("Health.Log length = %d, want 5", len(got.Health.Log))
	}
}

func TestProjectContainerInspect_VolumeMountFavorsName(t *testing.T) {
	// For type=volume mounts, the Source on the host is something
	// like /var/lib/docker/volumes/<sha>/_data which is useless to
	// the user. Show the volume Name instead.
	raw := rawContainerInspect{
		Mounts: []struct {
			Type        string `json:"Type"`
			Name        string `json:"Name"`
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
			Mode        string `json:"Mode"`
			RW          bool   `json:"RW"`
		}{
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
