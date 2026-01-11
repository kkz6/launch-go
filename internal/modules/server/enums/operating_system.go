package enums

import (
	"database/sql/driver"
	"fmt"
)

// OperatingSystem represents supported operating systems
type OperatingSystem string

const (
	OSUbuntu20 OperatingSystem = "ubuntu_20"
	OSUbuntu22 OperatingSystem = "ubuntu_22"
	OSUbuntu24 OperatingSystem = "ubuntu_24"
)

func (o OperatingSystem) String() string {
	return string(o)
}

func (o OperatingSystem) Label() string {
	labels := map[OperatingSystem]string{
		OSUbuntu20: "Ubuntu 20.04",
		OSUbuntu22: "Ubuntu 22.04",
		OSUbuntu24: "Ubuntu 24.04",
	}
	if label, ok := labels[o]; ok {
		return label
	}

	return "Unknown"
}

func (o OperatingSystem) IsValid() bool {
	switch o {
	case OSUbuntu20, OSUbuntu22, OSUbuntu24:
		return true
	}

	return false
}

func (o *OperatingSystem) Scan(value interface{}) error {
	if value == nil {
		*o = OSUbuntu24
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*o = OperatingSystem(v)
	case string:
		*o = OperatingSystem(v)
	default:
		return fmt.Errorf("cannot scan type %T into OperatingSystem", value)
	}

	return nil
}

func (o OperatingSystem) Value() (driver.Value, error) {
	return string(o), nil
}

func ParseOperatingSystem(s string) (OperatingSystem, error) {
	os := OperatingSystem(s)
	if !os.IsValid() {
		return OSUbuntu24, fmt.Errorf("invalid operating system: %s", s)
	}

	return os, nil
}

func AllOperatingSystems() []OperatingSystem {
	return []OperatingSystem{OSUbuntu20, OSUbuntu22, OSUbuntu24}
}
