package enumtypes

// StringEnum is the interface that all string-based enums should implement.
type StringEnum interface {
	~string
	String() string
	IsValid() bool
}

// LabeledEnum extends StringEnum with a human-readable label.
type LabeledEnum interface {
	StringEnum
	Label() string
}

// InstallationStatus represents the installation state of a resource
type InstallationStatus string

const (
	StatusPending      InstallationStatus = "pending"
	StatusInstalling   InstallationStatus = "installing"
	StatusInstalled    InstallationStatus = "installed"
	StatusFailed       InstallationStatus = "failed"
	StatusUninstalling InstallationStatus = "uninstalling"
)

// String returns the string representation of the InstallationStatus
func (s InstallationStatus) String() string {
	return string(s)
}

// IsValid returns true if the status is a valid InstallationStatus
func (s InstallationStatus) IsValid() bool {
	switch s {
	case StatusPending, StatusInstalling, StatusInstalled, StatusFailed, StatusUninstalling:
		return true
	}

	return false
}
