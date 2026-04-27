package services

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/types"
)

// boolPtr is a small helper for the *bool fields on CreateServerRequest.
func boolPtr(b bool) *bool { return &b }

func TestDockerServerSoftwareStack_Default(t *testing.T) {
	got := dockerServerSoftwareStack(&dto.CreateServerRequest{Type: "docker"})
	want := []types.Software{
		types.SoftwareDocker,
		types.SoftwareTraefik,
		types.SoftwareLaunchAgent,
	}
	assertSoftwareSlicesEqual(t, got, want)
}

func TestDockerServerSoftwareStack_InstallAgentExplicitTrue(t *testing.T) {
	got := dockerServerSoftwareStack(&dto.CreateServerRequest{
		Type:         "docker",
		InstallAgent: boolPtr(true),
	})
	want := []types.Software{
		types.SoftwareDocker,
		types.SoftwareTraefik,
		types.SoftwareLaunchAgent,
	}
	assertSoftwareSlicesEqual(t, got, want)
}

func TestDockerServerSoftwareStack_NoAgent(t *testing.T) {
	got := dockerServerSoftwareStack(&dto.CreateServerRequest{
		Type:         "docker",
		InstallAgent: boolPtr(false),
	})
	want := []types.Software{
		types.SoftwareDocker,
		types.SoftwareTraefik,
	}
	assertSoftwareSlicesEqual(t, got, want)
}

func TestDockerServerSoftwareStack_RespectsInstallOrder(t *testing.T) {
	got := dockerServerSoftwareStack(&dto.CreateServerRequest{Type: "docker"})

	// Docker must come before Traefik (Traefik runs as a Docker container).
	if got[0].InstallOrder() >= got[1].InstallOrder() {
		t.Errorf("expected Docker (order %d) before Traefik (order %d)",
			got[0].InstallOrder(), got[1].InstallOrder())
	}
	// Launch Agent installs last.
	if len(got) >= 3 && got[2].InstallOrder() < got[1].InstallOrder() {
		t.Errorf("expected Launch Agent (order %d) after Traefik (order %d)",
			got[2].InstallOrder(), got[1].InstallOrder())
	}
}

func assertSoftwareSlicesEqual(t *testing.T, got, want []types.Software) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pos %d: got %s, want %s", i, got[i], want[i])
		}
	}
}
