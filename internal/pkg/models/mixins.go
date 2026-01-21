package models

import "time"

// Named provides a Name field for models that need a display name.
// Embed this to add consistent naming support.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.Named
//	}
type Named struct {
	Name string `gorm:"type:varchar(255);not null" json:"name"`
}

// GetName returns the model's name
func (n *Named) GetName() string {
	return n.Name
}

// SetName sets the model's name
func (n *Named) SetName(name string) {
	n.Name = name
}

// Described provides an optional Description field.
// Embed this for models that support descriptions.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.Described
//	}
type Described struct {
	Description *string `gorm:"type:text" json:"description,omitempty"`
}

// GetDescription returns the model's description or empty string if nil
func (d *Described) GetDescription() string {
	if d.Description == nil {
		return ""
	}
	return *d.Description
}

// SetDescription sets the model's description
func (d *Described) SetDescription(desc string) {
	d.Description = &desc
}

// HasDescription returns true if a description is set
func (d *Described) HasDescription() bool {
	return d.Description != nil && *d.Description != ""
}

// StatusTracking provides a generic Status field for models that track state.
// The type parameter S must be a string-based type (like an enum).
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.StatusTracking[enums.MyStatus]
//	}
type StatusTracking[S ~string] struct {
	Status S `gorm:"type:varchar(50);not null" json:"status"`
}

// GetStatus returns the current status
func (s *StatusTracking[S]) GetStatus() S {
	return s.Status
}

// SetStatus sets the status
func (s *StatusTracking[S]) SetStatus(status S) {
	s.Status = status
}

// IsStatus checks if the current status matches the given status
func (s *StatusTracking[S]) IsStatus(status S) bool {
	return s.Status == status
}

// ProgressTracking provides fields for tracking progress percentage and messages.
// Embed this for models that have long-running operations with progress reporting.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.ProgressTracking
//	}
type ProgressTracking struct {
	Progress        int     `gorm:"type:int;not null;default:0" json:"progress"`
	ProgressMessage *string `gorm:"type:varchar(255)" json:"progress_message,omitempty"`
}

// GetProgress returns the current progress percentage (0-100)
func (p *ProgressTracking) GetProgress() int {
	return p.Progress
}

// SetProgress sets the progress percentage (clamped to 0-100)
func (p *ProgressTracking) SetProgress(progress int) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	p.Progress = progress
}

// GetProgressMessage returns the current progress message or empty string if nil
func (p *ProgressTracking) GetProgressMessage() string {
	if p.ProgressMessage == nil {
		return ""
	}
	return *p.ProgressMessage
}

// SetProgressMessage sets the progress message
func (p *ProgressTracking) SetProgressMessage(message string) {
	p.ProgressMessage = &message
}

// UpdateProgress sets both progress percentage and message atomically
func (p *ProgressTracking) UpdateProgress(progress int, message string) {
	p.SetProgress(progress)
	p.SetProgressMessage(message)
}

// ResetProgress resets progress to 0 and clears the message
func (p *ProgressTracking) ResetProgress() {
	p.Progress = 0
	p.ProgressMessage = nil
}

// IsComplete returns true if progress is 100%
func (p *ProgressTracking) IsComplete() bool {
	return p.Progress >= 100
}

// Archivable provides soft-archive functionality for models that should not be deleted
// but should be hidden from normal queries. Unlike soft delete, archived records
// are not automatically filtered by GORM.
//
// Deprecated: Use ArchivableModel instead for new code. This type is kept for
// backwards compatibility.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.Archivable
//	}
type Archivable struct {
	ArchivedAt *time.Time `gorm:"type:timestamp null;index" json:"archived_at,omitempty"`
}

// Compile-time check that Archivable implements ArchivableEntity
var _ ArchivableEntity = (*Archivable)(nil)

// IsArchived returns true if the model has been archived
func (a *Archivable) IsArchived() bool {
	return a.ArchivedAt != nil
}

// Archive marks the model as archived by setting ArchivedAt to now
func (a *Archivable) Archive() {
	now := time.Now()
	a.ArchivedAt = &now
}

// Unarchive removes the archived status by clearing ArchivedAt
func (a *Archivable) Unarchive() {
	a.ArchivedAt = nil
}

// Restore removes the archived status by clearing ArchivedAt
// Deprecated: Use Unarchive() instead for interface compliance
func (a *Archivable) Restore() {
	a.Unarchive()
}

// GetArchivedAt returns the archived timestamp
func (a *Archivable) GetArchivedAt() *time.Time {
	return a.ArchivedAt
}

// Tokenized provides a secure token field for models that need authentication tokens.
// The token is stored as a 32-character hex string.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.Tokenized
//	}
type Tokenized struct {
	Token string `gorm:"type:varchar(64);not null" json:"-"`
}

// GetToken returns the token
func (t *Tokenized) GetToken() string {
	return t.Token
}

// SetToken sets the token
func (t *Tokenized) SetToken(token string) {
	t.Token = token
}

// TokenField returns a pointer to the token field for initialization
func (t *Tokenized) TokenField() *string {
	return &t.Token
}
