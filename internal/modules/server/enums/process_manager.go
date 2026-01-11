package enums

// ProcessManager represents available process managers
type ProcessManager string

const (
	ProcessManagerSupervisor ProcessManager = "supervisor"
	ProcessManagerNone       ProcessManager = "none"
)

func (p ProcessManager) String() string {
	return string(p)
}

func (p ProcessManager) Label() string {
	labels := map[ProcessManager]string{
		ProcessManagerSupervisor: "Supervisor",
		ProcessManagerNone:       "None",
	}
	if label, ok := labels[p]; ok {
		return label
	}

	return "Unknown"
}

func (p ProcessManager) IsValid() bool {
	switch p {
	case ProcessManagerSupervisor, ProcessManagerNone:
		return true
	}

	return false
}

func (p ProcessManager) ServiceName() string {
	switch p {
	case ProcessManagerSupervisor:
		return "supervisor"
	case ProcessManagerNone:
		return ""
	}

	return ""
}

func AllProcessManagers() []ProcessManager {
	return []ProcessManager{ProcessManagerSupervisor, ProcessManagerNone}
}
