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

// SoftwareGroup represents a group of software with the same service type
type SoftwareGroup struct {
	Group      string
	Label      string
	Type       ServiceType
	ImagePath  string
	Software   []Software
	HasStart   bool
	HasStop    bool
	HasRestart bool
	HasRemove  bool
	HasStatus  bool
}

// GetAllSoftwareGroups returns all software grouped by service type
func GetAllSoftwareGroups() []SoftwareGroup {
	return []SoftwareGroup{
		{
			Group:      "php",
			Label:      "PHP",
			Type:       ServiceTypePhp,
			ImagePath:  "/images/software/php.svg",
			Software:   AllPhpVersions(),
			HasStart:   true,
			HasStop:    true,
			HasRestart: true,
			HasRemove:  true,
			HasStatus:  true,
		},
		{
			Group:      "mysql",
			Label:      "MySQL",
			Type:       ServiceTypeMySql,
			ImagePath:  "/images/software/mysql.svg",
			Software:   []Software{SoftwareMySql80},
			HasStart:   true,
			HasStop:    true,
			HasRestart: true,
			HasRemove:  true,
			HasStatus:  true,
		},
		{
			Group:      "postgresql",
			Label:      "PostgreSQL",
			Type:       ServiceTypePostgreSql,
			ImagePath:  "/images/software/postgresql.svg",
			Software:   []Software{SoftwarePostgreSql16},
			HasStart:   true,
			HasStop:    true,
			HasRestart: true,
			HasRemove:  true,
			HasStatus:  true,
		},
		{
			Group:      "redis",
			Label:      "Redis",
			Type:       ServiceTypeRedis,
			ImagePath:  "/images/software/redis.svg",
			Software:   []Software{SoftwareRedis},
			HasStart:   true,
			HasStop:    true,
			HasRestart: true,
			HasRemove:  true,
			HasStatus:  true,
		},
		{
			Group:      "supervisor",
			Label:      "Supervisor",
			Type:       ServiceTypeSupervisor,
			ImagePath:  "/images/software/supervisor.svg",
			Software:   []Software{SoftwareSupervisor},
			HasStart:   true,
			HasStop:    true,
			HasRestart: true,
			HasRemove:  true,
			HasStatus:  true,
		},
		{
			Group:      "node",
			Label:      "Node.js",
			Type:       ServiceTypeNode,
			ImagePath:  "/images/software/nodejs.svg",
			Software:   []Software{SoftwareNode21},
			HasStart:   false,
			HasStop:    false,
			HasRestart: false,
			HasRemove:  true,
			HasStatus:  false,
		},
		{
			Group:      "bun",
			Label:      "Bun",
			Type:       ServiceTypeBun,
			ImagePath:  "/images/software/bun.svg",
			Software:   []Software{SoftwareBun},
			HasStart:   false,
			HasStop:    false,
			HasRestart: false,
			HasRemove:  true,
			HasStatus:  false,
		},
	}
}

// LogPath returns the log file path for the software
func (s Software) LogPath() string {
	paths := map[Software]string{
		SoftwareMySql80:      "/var/log/mysql/error.log",
		SoftwarePostgreSql16: "/var/log/postgresql/postgresql-16-main.log",
		SoftwareRedis:        "/var/log/redis/redis-server.log",
		SoftwarePhp56:        "/var/log/php5.6-fpm.log",
		SoftwarePhp70:        "/var/log/php7.0-fpm.log",
		SoftwarePhp71:        "/var/log/php7.1-fpm.log",
		SoftwarePhp72:        "/var/log/php7.2-fpm.log",
		SoftwarePhp73:        "/var/log/php7.3-fpm.log",
		SoftwarePhp74:        "/var/log/php7.4-fpm.log",
		SoftwarePhp80:        "/var/log/php8.0-fpm.log",
		SoftwarePhp81:        "/var/log/php8.1-fpm.log",
		SoftwarePhp82:        "/var/log/php8.2-fpm.log",
		SoftwarePhp83:        "/var/log/php8.3-fpm.log",
		SoftwarePhp84:        "/var/log/php8.4-fpm.log",
		SoftwareCaddy2:       "/var/log/caddy/access.log",
		SoftwareSupervisor:   "/var/log/supervisor/supervisord.log",
	}

	if path, ok := paths[s]; ok {
		return path
	}

	return ""
}

// HasLogPath returns true if the software has a log file path
func (s Software) HasLogPath() bool {
	return s.LogPath() != ""
}

// InstallTemplateName returns the template path for installing this software
func (s Software) InstallTemplateName() string {
	// PHP versions use the generic install_php.sh template
	if s.IsPhp() {
		return "software/install_php.sh"
	}

	templateNames := map[Software]string{
		SoftwareCaddy2:       "software/install_caddy2.sh",
		SoftwareComposer2:    "software/install_composer2.sh",
		SoftwareMySql80:      "software/install_mysql80.sh",
		SoftwarePostgreSql16: "software/install_postgresql16.sh",
		SoftwareNode21:       "software/install_node21.sh",
		SoftwareBun:          "software/install_bun.sh",
		SoftwareRedis:        "software/install_redis.sh",
		SoftwareSupervisor:   "software/install_supervisor.sh",
		SoftwareLaunchAgent:  "software/install_launch_agent.sh",
	}

	if name, ok := templateNames[s]; ok {
		return name
	}

	return "software/install_" + string(s) + ".sh"
}

// RemoveTemplateName returns the template path for removing this software
func (s Software) RemoveTemplateName() string {
	if s.IsPhp() {
		return "software/remove_php.sh"
	}

	return "software/remove_" + string(s) + ".sh"
}

// MaxConnections returns the recommended max connections based on server memory.
// Only applicable to database software (MySQL, PostgreSQL).
func (s Software) MaxConnections(memoryInMB int) int {
	if !s.IsDatabase() {
		return 0
	}
	switch {
	case memoryInMB <= 1024:
		return 100
	case memoryInMB <= 2048:
		return 200
	case memoryInMB <= 4096:
		return 400
	default:
		return 500
	}
}

// MaxChildren returns the recommended PHP-FPM max children based on server memory.
// Only applicable to PHP software.
func (s Software) MaxChildren(memoryInMB int) int {
	if !s.IsPhp() {
		return 0
	}
	gigabytes := memoryInMB/1024 - 1
	if gigabytes < 1 {
		gigabytes = 1
	}
	return int(float64(gigabytes) * 5 * 0.9)
}
