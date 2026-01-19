package enums

// RunAsUser represents the user type to run a script as
type RunAsUser string

const (
	// RunAsUserRoot runs the script as the root user
	RunAsUserRoot RunAsUser = "root"

	// RunAsUserLocal runs the script as the local system user (e.g., "launch")
	RunAsUserLocal RunAsUser = "local"
)

// IsValid checks if the RunAsUser value is valid
func (r RunAsUser) IsValid() bool {
	switch r {
	case RunAsUserRoot, RunAsUserLocal:
		return true
	}
	return false
}

// String returns the string representation
func (r RunAsUser) String() string {
	return string(r)
}

// AllRunAsUsers returns all valid RunAsUser values
func AllRunAsUsers() []RunAsUser {
	return []RunAsUser{RunAsUserRoot, RunAsUserLocal}
}
