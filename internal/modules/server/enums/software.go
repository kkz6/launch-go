package enums

import (
	"database/sql/driver"
	"fmt"
)

// Software represents installable software
type Software string

const (
	SoftwareCaddy2       Software = "caddy2"
	SoftwareComposer2    Software = "composer2"
	SoftwareMySql80      Software = "mysql80"
	SoftwarePostgreSql16 Software = "postgresql16"
	SoftwareNode21       Software = "node21"
	SoftwareBun          Software = "bun"
	SoftwarePhp56        Software = "php56"
	SoftwarePhp70        Software = "php70"
	SoftwarePhp71        Software = "php71"
	SoftwarePhp72        Software = "php72"
	SoftwarePhp73        Software = "php73"
	SoftwarePhp74        Software = "php74"
	SoftwarePhp80        Software = "php80"
	SoftwarePhp81        Software = "php81"
	SoftwarePhp82        Software = "php82"
	SoftwarePhp83        Software = "php83"
	SoftwarePhp84        Software = "php84"
	SoftwareRedis        Software = "redis"
	SoftwareSupervisor   Software = "supervisor"
	SoftwareLaunchAgent  Software = "launch_agent"
)

func (s Software) String() string {
	return string(s)
}

func (s Software) Label() string {
	labels := map[Software]string{
		SoftwareCaddy2:       "Caddy 2",
		SoftwareComposer2:    "Composer 2",
		SoftwareMySql80:      "MySQL 8.0",
		SoftwarePostgreSql16: "PostgreSQL 16",
		SoftwareNode21:       "Node 21",
		SoftwareBun:          "Bun",
		SoftwarePhp56:        "PHP 5.6",
		SoftwarePhp70:        "PHP 7.0",
		SoftwarePhp71:        "PHP 7.1",
		SoftwarePhp72:        "PHP 7.2",
		SoftwarePhp73:        "PHP 7.3",
		SoftwarePhp74:        "PHP 7.4",
		SoftwarePhp80:        "PHP 8.0",
		SoftwarePhp81:        "PHP 8.1",
		SoftwarePhp82:        "PHP 8.2",
		SoftwarePhp83:        "PHP 8.3",
		SoftwarePhp84:        "PHP 8.4",
		SoftwareRedis:        "Redis",
		SoftwareSupervisor:   "Supervisor",
		SoftwareLaunchAgent:  "Launch Agent",
	}
	if label, ok := labels[s]; ok {
		return label
	}

	return "Unknown"
}

func (s Software) IsValid() bool {
	switch s {
	case SoftwareCaddy2, SoftwareComposer2, SoftwareMySql80, SoftwarePostgreSql16,
		SoftwareNode21, SoftwareBun, SoftwarePhp56, SoftwarePhp70, SoftwarePhp71,
		SoftwarePhp72, SoftwarePhp73, SoftwarePhp74, SoftwarePhp80, SoftwarePhp81,
		SoftwarePhp82, SoftwarePhp83, SoftwarePhp84, SoftwareRedis, SoftwareSupervisor,
		SoftwareLaunchAgent:
		return true
	}

	return false
}

func (s Software) GetVersion() string {
	versions := map[Software]string{
		SoftwarePhp56:        "5.6",
		SoftwarePhp70:        "7.0",
		SoftwarePhp71:        "7.1",
		SoftwarePhp72:        "7.2",
		SoftwarePhp73:        "7.3",
		SoftwarePhp74:        "7.4",
		SoftwarePhp80:        "8.0",
		SoftwarePhp81:        "8.1",
		SoftwarePhp82:        "8.2",
		SoftwarePhp83:        "8.3",
		SoftwarePhp84:        "8.4",
		SoftwareMySql80:      "8.0",
		SoftwarePostgreSql16: "16",
		SoftwareSupervisor:   "latest",
		SoftwareComposer2:    "2.0",
		SoftwareCaddy2:       "2.0",
		SoftwareNode21:       "21",
		SoftwareBun:          "latest",
		SoftwareRedis:        "latest",
		SoftwareLaunchAgent:  "latest",
	}
	if v, ok := versions[s]; ok {
		return v
	}

	return "1.0"
}

func (s Software) GetServiceType() ServiceType {
	switch s {
	case SoftwarePhp56, SoftwarePhp70, SoftwarePhp71, SoftwarePhp72,
		SoftwarePhp73, SoftwarePhp74, SoftwarePhp80, SoftwarePhp81,
		SoftwarePhp82, SoftwarePhp83, SoftwarePhp84:
		return ServiceTypePhp
	case SoftwareMySql80:
		return ServiceTypeMySql
	case SoftwarePostgreSql16:
		return ServiceTypePostgreSql
	case SoftwareSupervisor:
		return ServiceTypeSupervisor
	case SoftwareRedis:
		return ServiceTypeRedis
	case SoftwareCaddy2:
		return ServiceTypeCaddy
	case SoftwareComposer2:
		return ServiceTypeComposer
	case SoftwareNode21:
		return ServiceTypeNode
	case SoftwareBun:
		return ServiceTypeBun
	case SoftwareLaunchAgent:
		return ServiceTypeLaunchAgent
	}

	return ""
}

func (s Software) IsPhp() bool {
	switch s {
	case SoftwarePhp56, SoftwarePhp70, SoftwarePhp71, SoftwarePhp72,
		SoftwarePhp73, SoftwarePhp74, SoftwarePhp80, SoftwarePhp81,
		SoftwarePhp82, SoftwarePhp83, SoftwarePhp84:
		return true
	}

	return false
}

func (s Software) IsDatabase() bool {
	return s == SoftwareMySql80 || s == SoftwarePostgreSql16
}

func (s Software) Group() string {
	switch s {
	case SoftwarePhp56, SoftwarePhp70, SoftwarePhp71, SoftwarePhp72,
		SoftwarePhp73, SoftwarePhp74, SoftwarePhp80, SoftwarePhp81,
		SoftwarePhp82, SoftwarePhp83, SoftwarePhp84:
		return "php"
	case SoftwareMySql80:
		return "mysql"
	case SoftwarePostgreSql16:
		return "postgresql"
	case SoftwareSupervisor:
		return "supervisor"
	case SoftwareRedis:
		return "redis"
	case SoftwareCaddy2:
		return "caddy"
	case SoftwareComposer2:
		return "composer"
	case SoftwareNode21:
		return "node"
	case SoftwareBun:
		return "bun"
	case SoftwareLaunchAgent:
		return "launch-agent"
	}

	return ""
}

func (s *Software) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*s = Software(v)
	case string:
		*s = Software(v)
	default:
		return fmt.Errorf("cannot scan type %T into Software", value)
	}

	return nil
}

func (s Software) Value() (driver.Value, error) {
	return string(s), nil
}

func ParseSoftware(str string) (Software, error) {
	s := Software(str)
	if !s.IsValid() {
		return "", fmt.Errorf("invalid software: %s", str)
	}

	return s, nil
}

func AllSoftware() []Software {
	return []Software{
		SoftwareCaddy2, SoftwareComposer2, SoftwareMySql80, SoftwarePostgreSql16,
		SoftwareNode21, SoftwareBun, SoftwarePhp56, SoftwarePhp70, SoftwarePhp71,
		SoftwarePhp72, SoftwarePhp73, SoftwarePhp74, SoftwarePhp80, SoftwarePhp81,
		SoftwarePhp82, SoftwarePhp83, SoftwarePhp84, SoftwareRedis, SoftwareSupervisor,
		SoftwareLaunchAgent,
	}
}

func AllPhpVersions() []Software {
	return []Software{
		SoftwarePhp84, SoftwarePhp83, SoftwarePhp82, SoftwarePhp81, SoftwarePhp80,
		SoftwarePhp74, SoftwarePhp73, SoftwarePhp72, SoftwarePhp71, SoftwarePhp70, SoftwarePhp56,
	}
}

func AllDatabaseTypes() []Software {
	return []Software{SoftwareMySql80, SoftwarePostgreSql16}
}
