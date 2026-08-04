package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPhpVersionSeries(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "8.3", PhpVersionSeries("php83"))
	assert.Equal(t, "8.3", PhpVersionSeries("8.3"))
	assert.Equal(t, "8.3", PhpVersionSeries("8.3.6"))
	assert.Equal(t, "7.4", PhpVersionSeries("7.4.33-1+ubuntu"))
}

func TestPhpRuntimeIdentifiersUseSeries(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "php8.3-fpm", PhpFPMServiceFromVersion("8.3.6"))
	assert.Equal(t, "php8.3", PhpBinaryFromVersion("8.3.6"))
	assert.Equal(t, "/run/php/php8.3-fpm.sock", PhpSocketFromVersion("8.3.6"))
	assert.Equal(t, SoftwarePhp83, SoftwareFromPhpVersion("8.3.6"))
}

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

func TestSoftware_Docker_IsValid(t *testing.T) {
	for _, sw := range []Software{SoftwareDocker, SoftwareTraefik} {
		t.Run(string(sw), func(t *testing.T) {
			if !sw.IsValid() {
				t.Errorf("%s should be a valid software", sw)
			}
			if sw.Label() == "Unknown" {
				t.Errorf("%s should have a label", sw)
			}
			if sw.GetVersion() == "" {
				t.Errorf("%s should have a version", sw)
			}
		})
	}
}

func TestSoftware_Docker_GetServiceType(t *testing.T) {
	if SoftwareDocker.GetServiceType() != ServiceTypeDocker {
		t.Errorf("SoftwareDocker should map to ServiceTypeDocker, got %s", SoftwareDocker.GetServiceType())
	}
	if SoftwareTraefik.GetServiceType() != ServiceTypeTraefik {
		t.Errorf("SoftwareTraefik should map to ServiceTypeTraefik, got %s", SoftwareTraefik.GetServiceType())
	}
}

func TestSoftware_Docker_Group(t *testing.T) {
	if SoftwareDocker.Group() != "docker" {
		t.Errorf("SoftwareDocker group should be 'docker', got %q", SoftwareDocker.Group())
	}
	if SoftwareTraefik.Group() != "traefik" {
		t.Errorf("SoftwareTraefik group should be 'traefik', got %q", SoftwareTraefik.Group())
	}
}

func TestSoftware_Docker_InstallOrder(t *testing.T) {
	// Docker installs early; Traefik installs after the base stack so the
	// launch-network bridge it joins already exists. (Pre-v2 servers used
	// an overlay network from the swarm install — same ordering still
	// holds.)
	if SoftwareDocker.InstallOrder() >= SoftwareTraefik.InstallOrder() {
		t.Error("Docker should install before Traefik")
	}
}

func TestSoftware_Traefik_InstallTemplateName(t *testing.T) {
	if got, want := SoftwareTraefik.InstallTemplateName(), "software/install_traefik.sh"; got != want {
		t.Errorf("SoftwareTraefik install template = %q, want %q", got, want)
	}
}
