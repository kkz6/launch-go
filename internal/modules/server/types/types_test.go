package types

import "testing"

// ----- ServiceType: Docker + Traefik -----

func TestServiceTypeDocker_IsValid(t *testing.T) {
	if !ServiceTypeDocker.IsValid() {
		t.Error("ServiceTypeDocker must be valid")
	}
}

func TestServiceTypeDocker_Label(t *testing.T) {
	if ServiceTypeDocker.Label() != "Docker" {
		t.Errorf("got %q, want %q", ServiceTypeDocker.Label(), "Docker")
	}
}

func TestServiceTypeTraefik_IsValid(t *testing.T) {
	if !ServiceTypeTraefik.IsValid() {
		t.Error("ServiceTypeTraefik must be valid")
	}
}

func TestServiceTypeTraefik_Label(t *testing.T) {
	if ServiceTypeTraefik.Label() != "Traefik" {
		t.Errorf("got %q, want %q", ServiceTypeTraefik.Label(), "Traefik")
	}
}

// ----- ServerType: Docker -----

func TestServerTypeDocker_IsValid(t *testing.T) {
	if !ServerTypeDocker.IsValid() {
		t.Error("ServerTypeDocker must be valid")
	}
}

func TestServerTypeDocker_Label(t *testing.T) {
	if ServerTypeDocker.Label() != "Docker Server" {
		t.Errorf("got %q, want %q", ServerTypeDocker.Label(), "Docker Server")
	}
}

func TestServerTypeDocker_Features(t *testing.T) {
	got := ServerTypeDocker.GetFeatures()

	want := map[ServerFeature]bool{
		ServerFeatureServices:        true,
		ServerFeatureBackups:         true,
		ServerFeatureSSLCertificates: true,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d features, got %d (%v)", len(want), len(got), got)
	}
	for _, f := range got {
		if !want[f] {
			t.Errorf("unexpected feature %q on docker server type", f)
		}
	}

	// Sanity: features that PHP servers have but Docker should not.
	if ServerTypeDocker.HasFeature(ServerFeatureSites) {
		t.Error("Docker server should not expose the Sites feature")
	}
	if ServerTypeDocker.HasFeature(ServerFeaturePhpManagement) {
		t.Error("Docker server should not expose the PHP Management feature")
	}
}

func TestServerTypeDocker_ProcessManager(t *testing.T) {
	if ServerTypeDocker.GetProcessManager() != ProcessManagerNone {
		t.Errorf("got %q, want %q", ServerTypeDocker.GetProcessManager(), ProcessManagerNone)
	}
}

func TestParseServerType_Docker(t *testing.T) {
	st, err := ParseServerType("docker")
	if err != nil {
		t.Fatalf("ParseServerType(docker) failed: %v", err)
	}
	if st != ServerTypeDocker {
		t.Errorf("got %q, want %q", st, ServerTypeDocker)
	}
}

func TestAllServerTypes_IncludesDocker(t *testing.T) {
	for _, st := range AllServerTypes() {
		if st == ServerTypeDocker {
			return
		}
	}
	t.Error("AllServerTypes() must include ServerTypeDocker")
}
