package services

import "testing"

func TestIsLaunchSystemName(t *testing.T) {
	cases := []struct {
		in   string
		want bool
		why  string
	}{
		{"launch-traefik", true, "control-plane service"},
		{"launch-traefik.1.ktub95ron6paof3zbwfxz2gug", true, "swarm-suffixed name still classifies as system (defence-in-depth in case caller forgets to strip)"},
		{"launch-app-acme-api", false, "user application container"},
		{"launch-db-acme-orders", false, "user database container"},
		{"launch-compose-acme-stack", false, "user compose container"},
		{"launch-build-acme-api-deploy42", false, "ephemeral build container"},
		{"acme-api", false, "user-named container outside launch- namespace"},
		{"my-launch-service", false, "substring match is NOT enough — must start with launch-"},
		{"launch-", true, "edge: prefix alone — defensive, treat as system"},
		{"launch-monitoring", true, "future control-plane service — system by default"},
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
