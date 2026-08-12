package types

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// Software represents installable software
type Software string

const (
	SoftwareCaddy2       Software = "caddy2"
	SoftwareCaddy2LB     Software = "caddy2_lb"
	SoftwareComposer2    Software = "composer2"
	SoftwareMySQL80      Software = "mysql80"
	SoftwarePostgreSQL16 Software = "postgresql16"
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
	SoftwareDocker       Software = "docker"
	SoftwareTraefik      Software = "traefik"
)

var allSoftware = []Software{
	SoftwareCaddy2, SoftwareCaddy2LB, SoftwareComposer2, SoftwareMySQL80, SoftwarePostgreSQL16,
	SoftwareNode21, SoftwareBun, SoftwarePhp56, SoftwarePhp70, SoftwarePhp71,
	SoftwarePhp72, SoftwarePhp73, SoftwarePhp74, SoftwarePhp80, SoftwarePhp81,
	SoftwarePhp82, SoftwarePhp83, SoftwarePhp84, SoftwareRedis, SoftwareSupervisor,
	SoftwareLaunchAgent, SoftwareDocker, SoftwareTraefik,
}

var softwareLabels = map[Software]string{
	SoftwareCaddy2:       "Caddy 2",
	SoftwareCaddy2LB:     "Caddy 2 (Load Balancer)",
	SoftwareComposer2:    "Composer 2",
	SoftwareMySQL80:      "MySQL 8.0",
	SoftwarePostgreSQL16: "PostgreSQL 16",
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
	SoftwareDocker:       "Docker",
	SoftwareTraefik:      "Traefik",
}

func (s Software) String() string {
	return string(s)
}

func (s Software) Label() string {
	return enumtypes.Label(s, softwareLabels, "Unknown")
}

func (s Software) IsValid() bool {
	return enumtypes.IsValid(s, allSoftware...)
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
		SoftwareMySQL80:      "8.0",
		SoftwarePostgreSQL16: "16",
		SoftwareSupervisor:   "latest",
		SoftwareComposer2:    "2.0",
		SoftwareCaddy2:       "2.0",
		SoftwareCaddy2LB:     "2.0",
		SoftwareNode21:       "21",
		SoftwareBun:          "latest",
		SoftwareRedis:        "latest",
		SoftwareLaunchAgent:  "latest",
		SoftwareDocker:       "27.3",
		SoftwareTraefik:      "3.1",
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
	case SoftwareMySQL80:
		return ServiceTypeMySQL
	case SoftwarePostgreSQL16:
		return ServiceTypePostgreSQL
	case SoftwareSupervisor:
		return ServiceTypeSupervisor
	case SoftwareRedis:
		return ServiceTypeRedis
	case SoftwareCaddy2, SoftwareCaddy2LB:
		return ServiceTypeCaddy
	case SoftwareComposer2:
		return ServiceTypeComposer
	case SoftwareNode21:
		return ServiceTypeNode
	case SoftwareBun:
		return ServiceTypeBun
	case SoftwareLaunchAgent:
		return ServiceTypeLaunchAgent
	case SoftwareDocker:
		return ServiceTypeDocker
	case SoftwareTraefik:
		return ServiceTypeTraefik
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
	return s == SoftwareMySQL80 || s == SoftwarePostgreSQL16
}

// CanRemove reports whether Launch has a supported uninstall task for the
// software and the service may be removed independently from its server.
func (s Software) CanRemove() bool {
	if s.IsPhp() {
		return true
	}

	switch s {
	case SoftwareMySQL80, SoftwarePostgreSQL16, SoftwareRedis, SoftwareSupervisor:
		return true
	default:
		return false
	}
}

func (s Software) Group() string {
	switch s {
	case SoftwarePhp56, SoftwarePhp70, SoftwarePhp71, SoftwarePhp72,
		SoftwarePhp73, SoftwarePhp74, SoftwarePhp80, SoftwarePhp81,
		SoftwarePhp82, SoftwarePhp83, SoftwarePhp84:
		return "php"
	case SoftwareMySQL80:
		return "mysql"
	case SoftwarePostgreSQL16:
		return "postgresql"
	case SoftwareSupervisor:
		return "supervisor"
	case SoftwareRedis:
		return "redis"
	case SoftwareCaddy2, SoftwareCaddy2LB:
		return "caddy"
	case SoftwareComposer2:
		return "composer"
	case SoftwareNode21:
		return "node"
	case SoftwareBun:
		return "bun"
	case SoftwareLaunchAgent:
		return "launch-agent"
	case SoftwareDocker:
		return "docker"
	case SoftwareTraefik:
		return "traefik"
	}

	return ""
}

func (s *Software) Scan(value interface{}) error {
	return enumtypes.ScanString(s, value)
}

func (s Software) Value() (driver.Value, error) {
	return enumtypes.ValueString(s)
}

func ParseSoftware(str string) (Software, error) {
	s := Software(str)
	if !s.IsValid() {
		return "", fmt.Errorf("invalid software: %s", str)
	}

	return s, nil
}

func AllSoftware() []Software {
	return allSoftware
}

func AllPhpVersions() []Software {
	return []Software{
		SoftwarePhp84, SoftwarePhp83, SoftwarePhp82, SoftwarePhp81, SoftwarePhp80,
		SoftwarePhp74, SoftwarePhp73, SoftwarePhp72, SoftwarePhp71, SoftwarePhp70, SoftwarePhp56,
	}
}

func AllDatabaseTypes() []Software {
	return []Software{SoftwareMySQL80, SoftwarePostgreSQL16}
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
			Type:       ServiceTypeMySQL,
			ImagePath:  "/images/software/mysql.svg",
			Software:   []Software{SoftwareMySQL80},
			HasStart:   true,
			HasStop:    true,
			HasRestart: true,
			HasRemove:  true,
			HasStatus:  true,
		},
		{
			Group:      "postgresql",
			Label:      "PostgreSQL",
			Type:       ServiceTypePostgreSQL,
			ImagePath:  "/images/software/postgresql.svg",
			Software:   []Software{SoftwarePostgreSQL16},
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
		SoftwareMySQL80:      "/var/log/mysql/error.log",
		SoftwarePostgreSQL16: "/var/log/postgresql/postgresql-16-main.log",
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
		SoftwareCaddy2:       "/var/log/caddy/caddy.log",
		SoftwareCaddy2LB:     "/var/log/caddy/caddy.log",
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
		SoftwareCaddy2LB:     "software/install_caddy2_loadbalancer.sh",
		SoftwareComposer2:    "software/install_composer2.sh",
		SoftwareMySQL80:      "software/install_mysql80.sh",
		SoftwarePostgreSQL16: "software/install_postgresql16.sh",
		SoftwareNode21:       "software/install_node21.sh",
		SoftwareBun:          "software/install_bun.sh",
		SoftwareRedis:        "software/install_redis.sh",
		SoftwareSupervisor:   "software/install_supervisor.sh",
		SoftwareLaunchAgent:  "software/install_launch_agent.sh",
		SoftwareTraefik:      "software/install_traefik.sh",
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

	// Install templates carry the version (install_mysql80.sh) but remove
	// templates don't (remove_mysql.sh), so deriving the remove name from
	// the enum alone pointed at files that were never written and panicked
	// MustRender inside RemoveServiceJob.
	switch s {
	case SoftwareMySQL80:
		return "software/remove_mysql.sh"
	case SoftwarePostgreSQL16:
		return "software/remove_postgresql.sh"
	}

	return "software/remove_" + string(s) + ".sh"
}

// SupportsRemove reports whether a removal script ships for this software.
//
// Deliberately a static switch rather than a lookup against the template
// registry: the registry is populated at boot, so a lookup would make the
// answer depend on initialisation order and silently report "not removable"
// anywhere templates had not been registered yet.
// TestSupportsRemoveMatchesShippedTemplates keeps this list honest against
// the files that actually exist.
func (s Software) SupportsRemove() bool {
	if s.IsPhp() {
		return true
	}

	switch s {
	case SoftwareRedis, SoftwareSupervisor, SoftwareMySQL80, SoftwarePostgreSQL16, SoftwareLaunchAgent:
		return true
	default:
		return false
	}
}

// ConnectionName returns the database driver name for use in application configuration.
// Returns "mysql" for MySQL and "pgsql" for PostgreSQL.
func (s Software) ConnectionName() string {
	switch s {
	case SoftwareMySQL80:
		return "mysql"
	case SoftwarePostgreSQL16:
		return "pgsql"
	default:
		return ""
	}
}

// Port returns the default port for the database type.
func (s Software) Port() string {
	switch s {
	case SoftwareMySQL80:
		return "3306"
	case SoftwarePostgreSQL16:
		return "5432"
	default:
		return ""
	}
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

// BinaryPath returns the executable binary path for the software.
// For PHP versions, returns "php8.3", "php8.2", etc.
func (s Software) BinaryPath() string {
	if s.IsPhp() {
		return "php" + s.GetVersion()
	}
	return ""
}

// FPMServiceName returns the PHP-FPM service name for the software.
// For PHP versions, returns "php8.3-fpm", "php8.2-fpm", etc.
func (s Software) FPMServiceName() string {
	if s.IsPhp() {
		return "php" + s.GetVersion() + "-fpm"
	}
	return ""
}

// PhpFPMServiceFromVersion returns the PHP-FPM service name from a version string or software name.
// Accepts both formats:
//   - Version string: "8.3" returns "php8.3-fpm"
//   - Software name: "php83" returns "php8.3-fpm"
func PhpFPMServiceFromVersion(version string) string {
	if version == "" {
		return ""
	}

	return "php" + PhpVersionSeries(version) + "-fpm"
}

// PhpBinaryFromVersion returns the PHP binary name from a version string or software name.
// Accepts both formats:
//   - Version string: "8.3" returns "php8.3"
//   - Software name: "php83" returns "php8.3"
//
// If version is empty, returns "php" as the default.
func PhpBinaryFromVersion(version string) string {
	if version == "" {
		return "php"
	}

	return "php" + PhpVersionSeries(version)
}

// PhpSocketFromVersion returns the PHP-FPM socket path from a version string or software name.
// Accepts both formats:
//   - Version string: "8.3" returns "/run/php/php8.3-fpm.sock"
//   - Software name: "php83" returns "/run/php/php8.3-fpm.sock"
func PhpSocketFromVersion(version string) string {
	if version == "" {
		return ""
	}

	return "/run/php/php" + PhpVersionSeries(version) + "-fpm.sock"
}

// PhpVersionSeries returns the major.minor package series for a PHP
// software key or detected runtime version. Package names, binaries, FPM
// units and sockets are tied to the series, not to the full patch version.
func PhpVersionSeries(version string) string {
	software := Software(version)
	if software.IsPhp() {
		return software.GetVersion()
	}

	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[:2], ".")
	}

	return version
}

// SoftwareFromPhpVersion returns the Software enum for a PHP version string.
// For example, "8.3" returns SoftwarePhp83.
func SoftwareFromPhpVersion(version string) Software {
	version = PhpVersionSeries(version)
	versionMap := map[string]Software{
		"5.6": SoftwarePhp56,
		"7.0": SoftwarePhp70,
		"7.1": SoftwarePhp71,
		"7.2": SoftwarePhp72,
		"7.3": SoftwarePhp73,
		"7.4": SoftwarePhp74,
		"8.0": SoftwarePhp80,
		"8.1": SoftwarePhp81,
		"8.2": SoftwarePhp82,
		"8.3": SoftwarePhp83,
		"8.4": SoftwarePhp84,
	}
	if s, ok := versionMap[version]; ok {
		return s
	}
	return ""
}

// RequiresPhp returns true if this software requires PHP to be installed first.
func (s Software) RequiresPhp() bool {
	return s == SoftwareComposer2
}

// InstallOrder returns the installation priority for sorting.
// Lower numbers are installed first.
func (s Software) InstallOrder() int {
	// Order matters for dependencies:
	// 1. System services (supervisor) - needed by PHP-FPM and other services
	// 2. Web server (caddy) - can install independently
	// 3. PHP versions - needed by composer
	// 4. Composer - requires PHP
	// 5. Databases (mysql, postgresql)
	// 6. Other tools (redis, node, bun)
	// 7. Launch agent - last
	orders := map[Software]int{
		SoftwareSupervisor:   10,
		SoftwareCaddy2:       20,
		SoftwareCaddy2LB:     20,
		SoftwarePhp56:        30,
		SoftwarePhp70:        30,
		SoftwarePhp71:        30,
		SoftwarePhp72:        30,
		SoftwarePhp73:        30,
		SoftwarePhp74:        30,
		SoftwarePhp80:        30,
		SoftwarePhp81:        30,
		SoftwarePhp82:        30,
		SoftwarePhp83:        30,
		SoftwarePhp84:        30,
		SoftwareComposer2:    40, // After PHP
		SoftwareMySQL80:      50,
		SoftwarePostgreSQL16: 50,
		SoftwareRedis:        60,
		SoftwareNode21:       70,
		SoftwareBun:          70,
		SoftwareDocker:       15,
		SoftwareTraefik:      80,
		SoftwareLaunchAgent:  100,
	}
	if order, ok := orders[s]; ok {
		return order
	}
	return 99
}

// SortSoftwareStack sorts a software stack by installation order and removes
// software with unmet dependencies (e.g., Composer without PHP).
func SortSoftwareStack(stack []Software) []Software {
	// Check if PHP is in the stack
	hasPhp := false
	for _, s := range stack {
		if s.IsPhp() {
			hasPhp = true
			break
		}
	}

	// Filter out software with unmet dependencies
	var filtered []Software
	for _, s := range stack {
		if s.RequiresPhp() && !hasPhp {
			// Skip Composer if no PHP is installed
			continue
		}
		filtered = append(filtered, s)
	}

	// Sort by install order
	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[j].InstallOrder() < filtered[i].InstallOrder() {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	return filtered
}
