package services

import "testing"

// TestIsLaunchSystemName pins the allow-list behaviour. Regression
// guard: an earlier deny-list implementation mis-classified every
// application container (named `launch-<project>-<app>` by
// tasks.ContainerNameFor) as system because it lacked the `app-`
// segment the rule expected. The allow-list approach makes it
// impossible for that misfire to recur — anything that isn't on the
// list is user-owned, full stop.
//
// Adding a new control-plane container? Update the allow-list in
// host_inspect_service.go AND extend this test case-by-case.
func TestIsLaunchSystemName(t *testing.T) {
	cases := []struct {
		in   string
		want bool
		why  string
	}{
		// System — on the allow-list.
		{"launch-traefik", true, "control-plane reverse proxy"},
		{"launch-traefik.1.ktub95ron6paof3zbwfxz2gug", false, "swarm-suffixed name doesn't match exactly — callers must stripSwarmTaskSuffix first"},

		// User workloads — must NOT be classified as system.
		// These are the actual names tasks.ContainerNameFor and
		// tasks.DatabaseContainerName produce. The user-reported bug
		// (`launch-test-ssss` showing up as system) is the first case
		// below — explicit regression guard.
		{"launch-test-ssss", false, "application container named `launch-<project>-<app>` — was the bug repro"},
		{"launch-acme-api", false, "application container — short app name"},
		{"launch-db-acme-orders", false, "database container — `launch-db-<project>-<db>`"},
		{"launch-app-acme-api", false, "no longer a real pattern, but if it ever shows up it's still a user app"},

		// Non-launch-prefixed.
		{"acme-api", false, "user-named container outside launch- namespace"},
		{"my-launch-service", false, "substring match is NOT enough — never treated as system"},
		{"", false, "empty name — defensive, never classify as system"},

		// Speculative future names — explicitly NOT system until added
		// to the allow-list. Demonstrates the allow-list invariant.
		{"launch-monitoring", false, "speculative future container — not on the allow-list yet"},
		{"launch-", false, "prefix alone — not on the allow-list"},
	}
	for _, c := range cases {
		if got := isLaunchSystemName(c.in); got != c.want {
			t.Errorf("isLaunchSystemName(%q) = %v, want %v (%s)", c.in, got, c.want, c.why)
		}
	}
}

func TestStripSwarmTaskSuffix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"launch-traefik.1.ktub95ron6paof3zbwfxz2gug", "launch-traefik"},
		{"my-app", "my-app"}, // no dot → unchanged
		{"foo,bar", "foo"},   // linked-container alias list — first name wins
		{"launch-traefik.1.xxx,another", "launch-traefik"},
		{"", ""},
	}
	for _, c := range cases {
		if got := stripSwarmTaskSuffix(c.in); got != c.want {
			t.Errorf("stripSwarmTaskSuffix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDockerBuiltinNetworks_AllThreePresent(t *testing.T) {
	// Spec-level guard: docker ships exactly these three default
	// networks on every host. If this list drifts, the Networks UI
	// will start leaking noise back into the user view.
	for _, n := range []string{"bridge", "host", "none"} {
		if _, ok := dockerBuiltinNetworks[n]; !ok {
			t.Errorf("dockerBuiltinNetworks should contain %q", n)
		}
	}
}
