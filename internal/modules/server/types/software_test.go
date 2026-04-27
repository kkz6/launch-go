package types

import (
	"testing"
)

func TestSortSoftwareStack(t *testing.T) {
	t.Run("PHP and Composer - Composer comes after PHP", func(t *testing.T) {
		stack := []Software{
			SoftwareComposer2,
			SoftwarePhp83,
			SoftwareCaddy2,
			SoftwareSupervisor,
		}
		sorted := SortSoftwareStack(stack)

		// Find positions
		phpPos, composerPos := -1, -1
		for i, s := range sorted {
			if s.IsPhp() {
				phpPos = i
			}
			if s == SoftwareComposer2 {
				composerPos = i
			}
		}

		if composerPos <= phpPos {
			t.Errorf("Composer (pos %d) should come after PHP (pos %d)", composerPos, phpPos)
		}

		// Verify order
		expected := []Software{SoftwareSupervisor, SoftwareCaddy2, SoftwarePhp83, SoftwareComposer2}
		if len(sorted) != len(expected) {
			t.Errorf("Expected %d items, got %d", len(expected), len(sorted))
		}
		for i, s := range expected {
			if sorted[i] != s {
				t.Errorf("Position %d: expected %s, got %s", i, s, sorted[i])
			}
		}
	})

	t.Run("No PHP - Composer is removed", func(t *testing.T) {
		stack := []Software{
			SoftwareComposer2,
			SoftwareCaddy2,
			SoftwareNode21,
		}
		sorted := SortSoftwareStack(stack)

		// Composer should be removed
		for _, s := range sorted {
			if s == SoftwareComposer2 {
				t.Error("Composer should be removed when no PHP is present")
			}
		}

		if len(sorted) != 2 {
			t.Errorf("Expected 2 items (Caddy, Node), got %d", len(sorted))
		}
	})

	t.Run("Full stack sorted correctly", func(t *testing.T) {
		stack := []Software{
			SoftwareMySQL80,
			SoftwareComposer2,
			SoftwarePhp83,
			SoftwareCaddy2,
			SoftwareSupervisor,
		}
		sorted := SortSoftwareStack(stack)

		// Verify order: Supervisor < Caddy < PHP < Composer < MySQL
		expected := []Software{
			SoftwareSupervisor, // 10
			SoftwareCaddy2,     // 20
			SoftwarePhp83,      // 30
			SoftwareComposer2,  // 40
			SoftwareMySQL80,    // 50
		}
		if len(sorted) != len(expected) {
			t.Errorf("Expected %d items, got %d", len(expected), len(sorted))
		}
		for i, s := range expected {
			if sorted[i] != s {
				t.Errorf("Position %d: expected %s, got %s", i, s, sorted[i])
			}
		}
	})
}

func TestSoftware_RequiresPhp(t *testing.T) {
	if !SoftwareComposer2.RequiresPhp() {
		t.Error("Composer should require PHP")
	}
	if SoftwareCaddy2.RequiresPhp() {
		t.Error("Caddy should not require PHP")
	}
	if SoftwarePhp83.RequiresPhp() {
		t.Error("PHP should not require PHP")
	}
}

func TestSoftware_InstallOrder(t *testing.T) {
	// Verify key ordering relationships
	if SoftwareSupervisor.InstallOrder() >= SoftwareCaddy2.InstallOrder() {
		t.Error("Supervisor should install before Caddy")
	}
	if SoftwareCaddy2.InstallOrder() >= SoftwarePhp83.InstallOrder() {
		t.Error("Caddy should install before PHP")
	}
	if SoftwarePhp83.InstallOrder() >= SoftwareComposer2.InstallOrder() {
		t.Error("PHP should install before Composer")
	}
	if SoftwareComposer2.InstallOrder() >= SoftwareMySQL80.InstallOrder() {
		t.Error("Composer should install before MySQL")
	}
}

// ----- Docker / Traefik software -----

func TestSoftwareDocker_IsValid(t *testing.T) {
	if !SoftwareDocker.IsValid() {
		t.Error("SoftwareDocker must be valid")
	}
}

func TestSoftwareTraefik_IsValid(t *testing.T) {
	if !SoftwareTraefik.IsValid() {
		t.Error("SoftwareTraefik must be valid")
	}
}

func TestSoftwareDocker_Label(t *testing.T) {
	if SoftwareDocker.Label() != "Docker" {
		t.Errorf("got %q, want %q", SoftwareDocker.Label(), "Docker")
	}
}

func TestSoftwareTraefik_Label(t *testing.T) {
	if SoftwareTraefik.Label() != "Traefik" {
		t.Errorf("got %q, want %q", SoftwareTraefik.Label(), "Traefik")
	}
}

func TestSoftwareDocker_InstallTemplate(t *testing.T) {
	want := "software/install_docker.sh"
	if got := SoftwareDocker.InstallTemplateName(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSoftwareTraefik_InstallTemplate(t *testing.T) {
	want := "software/install_traefik.sh"
	if got := SoftwareTraefik.InstallTemplateName(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSoftwareDocker_GetServiceType(t *testing.T) {
	if got := SoftwareDocker.GetServiceType(); got != ServiceTypeDocker {
		t.Errorf("got %q, want %q", got, ServiceTypeDocker)
	}
}

func TestSoftwareTraefik_GetServiceType(t *testing.T) {
	if got := SoftwareTraefik.GetServiceType(); got != ServiceTypeTraefik {
		t.Errorf("got %q, want %q", got, ServiceTypeTraefik)
	}
}

func TestSoftwareDocker_GetVersion(t *testing.T) {
	if got := SoftwareDocker.GetVersion(); got == "" {
		t.Error("SoftwareDocker.GetVersion() must not be empty")
	}
}

func TestSoftwareTraefik_GetVersion(t *testing.T) {
	if got := SoftwareTraefik.GetVersion(); got == "" {
		t.Error("SoftwareTraefik.GetVersion() must not be empty")
	}
}

func TestSoftwareDocker_NotPhpNotDatabase(t *testing.T) {
	if SoftwareDocker.IsPhp() {
		t.Error("Docker is not PHP")
	}
	if SoftwareDocker.IsDatabase() {
		t.Error("Docker is not a database")
	}
	if SoftwareDocker.RequiresPhp() {
		t.Error("Docker does not require PHP")
	}
}

func TestSoftwareDocker_BeforeTraefikInstallOrder(t *testing.T) {
	if SoftwareDocker.InstallOrder() >= SoftwareTraefik.InstallOrder() {
		t.Errorf("Docker (order %d) must install before Traefik (order %d)",
			SoftwareDocker.InstallOrder(), SoftwareTraefik.InstallOrder())
	}
}

func TestSortSoftwareStack_DockerStack(t *testing.T) {
	stack := []Software{
		SoftwareTraefik,
		SoftwareLaunchAgent,
		SoftwareDocker,
	}
	got := SortSoftwareStack(stack)
	want := []Software{SoftwareDocker, SoftwareTraefik, SoftwareLaunchAgent}
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pos %d: got %s, want %s", i, got[i], want[i])
		}
	}
}

func TestAllSoftware_IncludesDockerAndTraefik(t *testing.T) {
	hasDocker, hasTraefik := false, false
	for _, s := range AllSoftware() {
		switch s {
		case SoftwareDocker:
			hasDocker = true
		case SoftwareTraefik:
			hasTraefik = true
		}
	}
	if !hasDocker {
		t.Error("AllSoftware() must include SoftwareDocker")
	}
	if !hasTraefik {
		t.Error("AllSoftware() must include SoftwareTraefik")
	}
}
