package models

import (
	"testing"
	"time"
)

func TestArchivableModel_IsArchived(t *testing.T) {
	t.Run("returns false when not archived", func(t *testing.T) {
		m := &ArchivableModel{}
		if m.IsArchived() {
			t.Error("expected IsArchived to return false for new model")
		}
	})

	t.Run("returns true when archived", func(t *testing.T) {
		now := time.Now()
		m := &ArchivableModel{ArchivedAt: &now}
		if !m.IsArchived() {
			t.Error("expected IsArchived to return true when ArchivedAt is set")
		}
	})
}

func TestArchivableModel_Archive(t *testing.T) {
	m := &ArchivableModel{}

	m.Archive()

	if m.ArchivedAt == nil {
		t.Fatal("expected ArchivedAt to be set after Archive()")
	}

	if !m.IsArchived() {
		t.Error("expected IsArchived to return true after Archive()")
	}

	// Verify timestamp is recent
	if time.Since(*m.ArchivedAt) > time.Second {
		t.Error("expected ArchivedAt to be set to current time")
	}
}

func TestArchivableModel_Unarchive(t *testing.T) {
	now := time.Now()
	m := &ArchivableModel{ArchivedAt: &now}

	m.Unarchive()

	if m.ArchivedAt != nil {
		t.Error("expected ArchivedAt to be nil after Unarchive()")
	}

	if m.IsArchived() {
		t.Error("expected IsArchived to return false after Unarchive()")
	}
}

func TestArchivableModel_GetArchivedAt(t *testing.T) {
	t.Run("returns nil when not archived", func(t *testing.T) {
		m := &ArchivableModel{}
		if m.GetArchivedAt() != nil {
			t.Error("expected GetArchivedAt to return nil for new model")
		}
	})

	t.Run("returns timestamp when archived", func(t *testing.T) {
		now := time.Now()
		m := &ArchivableModel{ArchivedAt: &now}

		got := m.GetArchivedAt()
		if got == nil {
			t.Fatal("expected GetArchivedAt to return non-nil")
		}
		if !got.Equal(now) {
			t.Errorf("expected GetArchivedAt to return %v, got %v", now, *got)
		}
	})
}

func TestArchivableModel_ImplementsInterface(t *testing.T) {
	// Compile-time check is in archivable.go, but this is a runtime verification
	var m ArchivableEntity = &ArchivableModel{}

	// Should not panic
	_ = m.IsArchived()
	m.Archive()
	_ = m.GetArchivedAt()
	m.Unarchive()
}

// Test the deprecated Archivable type for backwards compatibility
func TestArchivable_BackwardsCompatibility(t *testing.T) {
	t.Run("implements ArchivableEntity interface", func(t *testing.T) {
		var m ArchivableEntity = &Archivable{}
		_ = m.IsArchived()
		m.Archive()
		_ = m.GetArchivedAt()
		m.Unarchive()
	})

	t.Run("Restore calls Unarchive", func(t *testing.T) {
		now := time.Now()
		m := &Archivable{ArchivedAt: &now}

		m.Restore()

		if m.ArchivedAt != nil {
			t.Error("expected Restore() to clear ArchivedAt")
		}
	})
}

func TestArchiveUnarchiveCycle(t *testing.T) {
	m := &ArchivableModel{}

	// Initially not archived
	if m.IsArchived() {
		t.Error("new model should not be archived")
	}

	// Archive
	m.Archive()
	if !m.IsArchived() {
		t.Error("model should be archived after Archive()")
	}
	archivedAt := m.GetArchivedAt()
	if archivedAt == nil {
		t.Error("GetArchivedAt should return timestamp after Archive()")
	}

	// Unarchive
	m.Unarchive()
	if m.IsArchived() {
		t.Error("model should not be archived after Unarchive()")
	}
	if m.GetArchivedAt() != nil {
		t.Error("GetArchivedAt should return nil after Unarchive()")
	}

	// Archive again
	m.Archive()
	if !m.IsArchived() {
		t.Error("model should be archived after second Archive()")
	}
}
