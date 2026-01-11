package enums

import (
	"database/sql/driver"
	"fmt"
)

// ServiceType represents types of services
type ServiceType string

const (
	ServiceTypePhp         ServiceType = "php"
	ServiceTypeMySql       ServiceType = "mysql"
	ServiceTypePostgreSql  ServiceType = "postgresql"
	ServiceTypeSupervisor  ServiceType = "process_manager"
	ServiceTypeRedis       ServiceType = "memory_database"
	ServiceTypeCaddy       ServiceType = "webserver"
	ServiceTypeComposer    ServiceType = "package_manager"
	ServiceTypeNode        ServiceType = "node"
	ServiceTypeBun         ServiceType = "bun"
	ServiceTypeLaunchAgent ServiceType = "launch_agent"
)

func (s ServiceType) String() string {
	return string(s)
}

func (s ServiceType) Label() string {
	labels := map[ServiceType]string{
		ServiceTypePhp:         "PHP",
		ServiceTypeMySql:       "MySQL",
		ServiceTypePostgreSql:  "PostgreSQL",
		ServiceTypeSupervisor:  "Supervisor",
		ServiceTypeRedis:       "Redis",
		ServiceTypeCaddy:       "Caddy",
		ServiceTypeComposer:    "Composer",
		ServiceTypeNode:        "Node.js",
		ServiceTypeBun:         "Bun",
		ServiceTypeLaunchAgent: "Launch Agent",
	}
	if label, ok := labels[s]; ok {
		return label
	}

	return "Unknown"
}

func (s ServiceType) IsValid() bool {
	switch s {
	case ServiceTypePhp, ServiceTypeMySql, ServiceTypePostgreSql,
		ServiceTypeSupervisor, ServiceTypeRedis, ServiceTypeCaddy,
		ServiceTypeComposer, ServiceTypeNode, ServiceTypeBun, ServiceTypeLaunchAgent:
		return true
	}

	return false
}

func (s ServiceType) IsDatabase() bool {
	return s == ServiceTypeMySql || s == ServiceTypePostgreSql
}

func (s ServiceType) GetDatabasePort() int {
	switch s {
	case ServiceTypeMySql:
		return 3306
	case ServiceTypePostgreSql:
		return 5432
	}

	return 0
}

func (s ServiceType) GetDatabaseConnection() string {
	switch s {
	case ServiceTypeMySql:
		return "mysql"
	case ServiceTypePostgreSql:
		return "pgsql"
	}

	return ""
}

func (s *ServiceType) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*s = ServiceType(v)
	case string:
		*s = ServiceType(v)
	default:
		return fmt.Errorf("cannot scan type %T into ServiceType", value)
	}

	return nil
}

func (s ServiceType) Value() (driver.Value, error) {
	return string(s), nil
}

func ParseServiceType(str string) (ServiceType, error) {
	s := ServiceType(str)
	if !s.IsValid() {
		return "", fmt.Errorf("invalid service type: %s", str)
	}

	return s, nil
}

func AllServiceTypes() []ServiceType {
	return []ServiceType{
		ServiceTypePhp, ServiceTypeMySql, ServiceTypePostgreSql,
		ServiceTypeSupervisor, ServiceTypeRedis, ServiceTypeCaddy,
		ServiceTypeComposer, ServiceTypeNode, ServiceTypeBun, ServiceTypeLaunchAgent,
	}
}
