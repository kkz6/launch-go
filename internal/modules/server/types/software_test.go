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
