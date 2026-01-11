package enums

import "fmt"

// ServiceOption represents actions that can be performed on a service
type ServiceOption string

const (
	ServiceOptionStart   ServiceOption = "start"
	ServiceOptionStop    ServiceOption = "stop"
	ServiceOptionRestart ServiceOption = "restart"
	ServiceOptionRemove  ServiceOption = "remove"
	ServiceOptionStatus  ServiceOption = "status"
)

func (o ServiceOption) String() string {
	return string(o)
}

func (o ServiceOption) Label() string {
	labels := map[ServiceOption]string{
		ServiceOptionStart:   "Start",
		ServiceOptionStop:    "Stop",
		ServiceOptionRestart: "Restart",
		ServiceOptionRemove:  "Remove",
		ServiceOptionStatus:  "Status",
	}
	if label, ok := labels[o]; ok {
		return label
	}

	return "Unknown"
}

func (o ServiceOption) IsValid() bool {
	switch o {
	case ServiceOptionStart, ServiceOptionStop, ServiceOptionRestart,
		ServiceOptionRemove, ServiceOptionStatus:
		return true
	}

	return false
}

func ParseServiceOption(s string) (ServiceOption, error) {
	option := ServiceOption(s)
	if !option.IsValid() {
		return "", fmt.Errorf("invalid service option: %s", s)
	}

	return option, nil
}

func AllServiceOptions() []ServiceOption {
	return []ServiceOption{
		ServiceOptionStart, ServiceOptionStop, ServiceOptionRestart,
		ServiceOptionRemove, ServiceOptionStatus,
	}
}
