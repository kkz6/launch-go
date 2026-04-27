package types

import (
	"database/sql/driver"
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// Type identifies the kind of docker registry this credential targets.
type Type string

const (
	// TypeDockerHub — hub.docker.com.
	TypeDockerHub Type = "docker_hub"
	// TypeGHCR — GitHub Container Registry.
	TypeGHCR Type = "ghcr"
	// TypeGeneric — a self-hosted or third-party registry; URL is required.
	TypeGeneric Type = "generic"
)

var allTypes = []Type{TypeDockerHub, TypeGHCR, TypeGeneric}

var typeLabels = map[Type]string{
	TypeDockerHub: "Docker Hub",
	TypeGHCR:      "GitHub Container Registry",
	TypeGeneric:   "Generic Registry",
}

// String returns the wire string value.
func (t Type) String() string { return string(t) }

// Label returns the user-facing label.
func (t Type) Label() string { return enumtypes.Label(t, typeLabels, "Unknown") }

// IsValid reports whether the type is one of the known values.
func (t Type) IsValid() bool { return enumtypes.IsValid(t, allTypes...) }

// DefaultURL returns the registry URL we pre-fill for known types.
// Generic returns an empty string so the caller must supply the URL.
func (t Type) DefaultURL() string {
	switch t {
	case TypeDockerHub:
		return "https://index.docker.io/v1/"
	case TypeGHCR:
		return "ghcr.io"
	}
	return ""
}

// Scan implements sql.Scanner for GORM.
func (t *Type) Scan(value any) error { return enumtypes.ScanString(t, value) }

// Value implements driver.Valuer for GORM.
func (t Type) Value() (driver.Value, error) { return enumtypes.ValueString(t) }

// ParseType validates an input string and returns the matching Type.
func ParseType(s string) (Type, error) {
	t := Type(s)
	if !t.IsValid() {
		return "", fmt.Errorf("invalid docker registry type: %s", s)
	}
	return t, nil
}

// AllTypes returns every valid Type.
func AllTypes() []Type { return allTypes }
