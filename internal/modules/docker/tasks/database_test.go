package tasks

import (
	"strings"
	"testing"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
)

func TestDatabaseContainerName(t *testing.T) {
	got := DatabaseContainerName("acme", "orders-db")
	want := "launch-db-acme-orders-db"
	if got != want {
		t.Errorf("DatabaseContainerName = %q, want %q", got, want)
	}
}

func TestShellEscapeArg(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"hello", "hello"},                // safe pass-through
		{"hello world", "'hello world'"},  // space → quote
		{"foo$BAR", `'foo$BAR'`},          // $ would expand → quote
		{"it's", `'it'\''s'`},             // single-quote escape
		{"", "''"},                        // empty → empty quoted
	}
	for _, c := range cases {
		if got := shellEscapeArg(c.in); got != c.want {
			t.Errorf("shellEscapeArg(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDatabaseLifecycleScript_KnownActions(t *testing.T) {
	for _, action := range []string{"start", "stop", "restart", "rm"} {
		script := DatabaseLifecycleScript("launch-db-x", action, "")
		if !strings.Contains(script, "docker "+action) {
			t.Errorf("script for %q missing 'docker %s'", action, action)
		}
		if action == "rm" && !strings.Contains(script, "docker stop") {
			t.Errorf("rm action must stop first")
		}
		// volume-rm step should be absent when volumeToRemove is "".
		if strings.Contains(script, "docker volume rm") {
			t.Errorf("script for %q must not include volume rm when no volume specified", action)
		}
	}
}

func TestDatabaseLifecycleScript_RmWithVolume(t *testing.T) {
	// When the caller passes a volume name, the rm action must
	// append a `docker volume rm` step. The volume removal is
	// best-effort (`|| true`) — pin both the call and the swallow.
	script := DatabaseLifecycleScript("launch-db-x", "rm", "launch-db-01-data")
	if !strings.Contains(script, `docker volume rm "launch-db-01-data"`) {
		t.Errorf("rm with volumeToRemove must include docker volume rm; got:\n%s", script)
	}
	if !strings.Contains(script, "|| true") {
		t.Errorf("volume rm must be best-effort (|| true); got:\n%s", script)
	}
}

func TestDatabaseLifecycleScript_VolumeNotRemovedForNonRm(t *testing.T) {
	// A non-rm action with a non-empty volumeToRemove must not run
	// `docker volume rm` — only the `rm` branch wires the cleanup.
	// Defence against future callers passing the wrong combination.
	for _, action := range []string{"start", "stop", "restart"} {
		script := DatabaseLifecycleScript("launch-db-x", action, "launch-db-01-data")
		if strings.Contains(script, "docker volume rm") {
			t.Errorf("non-rm action %q must not include volume rm; got:\n%s", action, script)
		}
	}
}

func TestDatabaseLifecycleScript_UpdateRestart(t *testing.T) {
	// update-restart:<policy> is the phase-5 addition. Make sure each
	// allowed policy is honoured and unknown ones produce an explicit
	// error script (exit 1) rather than building an arbitrary docker
	// command.
	for _, policy := range []string{"no", "on-failure", "always", "unless-stopped"} {
		script := DatabaseLifecycleScript("launch-db-x", "update-restart:"+policy, "")
		if !strings.Contains(script, "docker update --restart="+policy) {
			t.Errorf("expected docker update with policy %q, got:\n%s", policy, script)
		}
	}
	bad := DatabaseLifecycleScript("launch-db-x", "update-restart:rogue", "")
	if !strings.Contains(bad, "unsupported restart policy") {
		t.Errorf("bad policy should produce error script; got:\n%s", bad)
	}
	if !strings.Contains(bad, "exit 1") {
		t.Errorf("bad policy script must exit 1")
	}
}

func TestDatabaseLifecycleScript_UnknownActionRejected(t *testing.T) {
	// Defence-in-depth: even though the service validates, a bad
	// action must not produce a shell that runs an arbitrary docker
	// subcommand.
	script := DatabaseLifecycleScript("launch-db-x", "exec -it bash", "")
	if !strings.Contains(script, "unknown lifecycle action") {
		t.Errorf("expected unknown-action error, got:\n%s", script)
	}
	if strings.Contains(script, "docker exec") {
		t.Errorf("script must not include the rogue docker subcommand")
	}
}

func TestRunDatabaseScript_PullsImageAndStartsContainer(t *testing.T) {
	// Smoke test for the script renderer — covers the env var loop
	// and the network attachment that we'd otherwise only catch in
	// a real deploy.
	script := RunDatabaseScript(DatabaseRunConfig{
		ContainerName: "launch-db-x",
		Image:         "postgres:16",
		EnvVars: []string{
			"POSTGRES_USER=u",
			"POSTGRES_PASSWORD=p",
			"POSTGRES_DB=d",
		},
		InternalPort: 5432,
	})

	mustHaves := []string{
		`IMAGE="postgres:16"`,
		`docker pull "${IMAGE}"`,
		`--network launch-network`,
		"POSTGRES_USER=u",
		"POSTGRES_PASSWORD=p",
		"::LAUNCH::container_id::",
	}
	for _, want := range mustHaves {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q\n----\n%s", want, script)
		}
	}
}

// Make sure the Engine type round-trips through the script renderer so
// the type-safety on the renderer signature isn't accidentally
// stringly-typed.
var _ dockertypes.DatabaseEngine = dockertypes.DatabaseEnginePostgres
