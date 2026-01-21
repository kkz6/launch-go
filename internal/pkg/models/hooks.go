package models

import (
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// StatusInitializer is implemented by models that have a default status value.
// Models implementing this interface can use InitializeStatus in their BeforeCreate hook.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.StatusTracking[enums.MyStatus]
//	}
//
//	func (m *MyModel) DefaultStatus() enums.MyStatus {
//	    return enums.MyStatusPending
//	}
//
//	func (m *MyModel) BeforeCreate(tx *gorm.DB) error {
//	    if err := m.BaseModel.BeforeCreate(tx); err != nil {
//	        return err
//	    }
//	    models.InitializeStatus(m)
//	    return nil
//	}
type StatusInitializer[S ~string] interface {
	GetStatus() S
	SetStatus(S)
	DefaultStatus() S
}

// InitializeStatus sets the default status on a model if the current status is empty.
// Call this in your model's BeforeCreate hook.
func InitializeStatus[S ~string, T StatusInitializer[S]](model T) {
	if model.GetStatus() == S("") {
		model.SetStatus(model.DefaultStatus())
	}
}

// TokenInitializer is implemented by models that have a token field that should
// be auto-generated if empty. Models implementing this interface can use
// InitializeToken in their BeforeCreate hook.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.Tokenized
//	}
//
//	func (m *MyModel) BeforeCreate(tx *gorm.DB) error {
//	    if err := m.BaseModel.BeforeCreate(tx); err != nil {
//	        return err
//	    }
//	    models.InitializeToken(m, 32)
//	    return nil
//	}
type TokenInitializer interface {
	TokenField() *string
}

// InitializeToken generates a secure hex token if the model's token field is empty.
// The length parameter specifies the length of the hex string (must be even).
// Note: length is the desired hex string length, so we generate length/2 bytes.
func InitializeToken(model TokenInitializer, length int) {
	field := model.TokenField()
	if field != nil && *field == "" {
		*field = security.MustHexToken(length / 2)
	}
}

// DefaultValueSetter is implemented by models that have multiple fields with defaults.
// This interface provides a hook for setting all default values at once.
//
// Usage:
//
//	func (m *MyModel) SetDefaults() {
//	    if m.User == "" {
//	        m.User = "root"
//	    }
//	    if m.Processes == 0 {
//	        m.Processes = 1
//	    }
//	}
//
//	func (m *MyModel) BeforeCreate(tx *gorm.DB) error {
//	    if err := m.BaseModel.BeforeCreate(tx); err != nil {
//	        return err
//	    }
//	    m.SetDefaults()
//	    return nil
//	}
type DefaultValueSetter interface {
	SetDefaults()
}

// InitializeDefaults calls SetDefaults on models that implement DefaultValueSetter.
// This is a convenience function for use in BeforeCreate hooks.
func InitializeDefaults(model DefaultValueSetter) {
	model.SetDefaults()
}

// TimestampInitializer is a convenience type for models that need to track
// when certain events occurred (installed, verified, synced, etc.)
// This is typically used with pointer time.Time fields.
//
// Example field patterns this helps with:
//   - InstalledAt, ProvisionedAt, VerifiedAt
//   - SyncedAt, LastCheckedAt
//   - FailedAt, ExpiredAt
