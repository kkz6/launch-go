// Package serverpath provides a unified path builder for server paths.
// It offers a fluent API for constructing common server paths like home directories,
// SSH paths, site directories, release directories, and system configuration paths.
package serverpath

import (
	"fmt"
	"path/filepath"
	"time"
)

// User represents a server user for path construction.
type User struct {
	Name string
}

// ForUser creates a User path builder for the given username.
func ForUser(username string) User {
	return User{Name: username}
}

// Home returns the home directory for the user.
// Returns "/root" for root user, "/home/{user}" for others.
func (u User) Home() string {
	if u.Name == "root" {
		return "/root"
	}

	return fmt.Sprintf("/home/%s", u.Name)
}

// SSH returns the .ssh directory path.
func (u User) SSH() string {
	return filepath.Join(u.Home(), ".ssh")
}

// AuthorizedKeys returns the authorized_keys file path.
func (u User) AuthorizedKeys() string {
	return filepath.Join(u.SSH(), "authorized_keys")
}

// KnownHosts returns the known_hosts file path.
func (u User) KnownHosts() string {
	return filepath.Join(u.SSH(), "known_hosts")
}

// PrivateKey returns the path to a private key file.
func (u User) PrivateKey(name string) string {
	return filepath.Join(u.SSH(), name)
}

// PublicKey returns the path to a public key file.
func (u User) PublicKey(name string) string {
	return filepath.Join(u.SSH(), name+".pub")
}

// Config returns the SSH config file path.
func (u User) Config() string {
	return filepath.Join(u.SSH(), "config")
}

// Join joins path segments under the user's home directory.
func (u User) Join(parts ...string) string {
	args := make([]string, 0, len(parts)+1)
	args = append(args, u.Home())
	args = append(args, parts...)

	return filepath.Join(args...)
}

// TaskDir returns the .launch-tasks directory for the user.
func (u User) TaskDir() string {
	return u.Join(".launch-tasks")
}

// Site represents a site for path construction.
type Site struct {
	User User
	Name string
}

// ForSite creates a Site path builder for the given username and site name.
func ForSite(username, siteName string) Site {
	return Site{User: ForUser(username), Name: siteName}
}

// Root returns the site root directory.
func (s Site) Root() string {
	return filepath.Join(s.User.Home(), s.Name)
}

// Current returns the current symlink path (used in zero-downtime deployments).
func (s Site) Current() string {
	return filepath.Join(s.Root(), "current")
}

// Repository returns the repository directory (used in standard deployments).
func (s Site) Repository() string {
	return filepath.Join(s.Root(), "repository")
}

// Releases returns the releases directory.
func (s Site) Releases() string {
	return filepath.Join(s.Root(), "releases")
}

// Release returns a specific release directory for the given timestamp.
func (s Site) Release(timestamp time.Time) string {
	return filepath.Join(s.Releases(), timestamp.Format("20060102150405"))
}

// ReleaseNamed returns a specific release directory by name.
func (s Site) ReleaseNamed(name string) string {
	return filepath.Join(s.Releases(), name)
}

// Shared returns the shared directory.
func (s Site) Shared() string {
	return filepath.Join(s.Root(), "shared")
}

// Storage returns the shared storage directory.
func (s Site) Storage() string {
	return filepath.Join(s.Shared(), "storage")
}

// Env returns the shared .env file path.
func (s Site) Env() string {
	return filepath.Join(s.Shared(), ".env")
}

// Logs returns the logs directory.
func (s Site) Logs() string {
	return filepath.Join(s.Root(), "logs")
}

// Join joins path segments under the site root directory.
func (s Site) Join(parts ...string) string {
	args := make([]string, 0, len(parts)+1)
	args = append(args, s.Root())
	args = append(args, parts...)

	return filepath.Join(args...)
}

// Application returns the current application directory based on deployment type.
// For zero-downtime deployments, returns the current symlink path.
// For standard deployments, returns the repository path.
func (s Site) Application(zeroDowntime bool) string {
	if zeroDowntime {
		return s.Current()
	}

	return s.Repository()
}

// WebDirectory returns the web directory path for the given web folder.
func (s Site) WebDirectory(zeroDowntime bool, webFolder string) string {
	return filepath.Join(s.Application(zeroDowntime), webFolder)
}

// System paths for common server configuration locations.

// CaddyConfig returns the main Caddyfile path.
func CaddyConfig() string {
	return "/etc/caddy/Caddyfile"
}

// CaddySites returns the Caddy sites include directory.
func CaddySites() string {
	return "/etc/caddy/sites"
}

// CaddySiteConfig returns the Caddy config file path for a specific site.
func CaddySiteConfig(siteName string) string {
	return filepath.Join(CaddySites(), siteName)
}

// PHPFPMPool returns the PHP-FPM pool.d directory for a PHP version.
func PHPFPMPool(version string) string {
	return fmt.Sprintf("/etc/php/%s/fpm/pool.d", version)
}

// PHPFPMConfig returns the PHP-FPM configuration file for a PHP version.
func PHPFPMConfig(version string) string {
	return fmt.Sprintf("/etc/php/%s/fpm/php-fpm.conf", version)
}

// PHPIni returns the PHP ini file path for the FPM SAPI.
func PHPIni(version string) string {
	return fmt.Sprintf("/etc/php/%s/fpm/php.ini", version)
}

// PHPCLIIni returns the PHP ini file path for the CLI SAPI.
func PHPCLIIni(version string) string {
	return fmt.Sprintf("/etc/php/%s/cli/php.ini", version)
}

// PHPBinary returns the PHP binary path for a specific version.
func PHPBinary(version string) string {
	return fmt.Sprintf("/usr/bin/php%s", version)
}

// SupervisorConfig returns the Supervisor conf.d directory.
func SupervisorConfig() string {
	return "/etc/supervisor/conf.d"
}

// SupervisorSiteConfig returns the Supervisor config file path for a specific site program.
func SupervisorSiteConfig(programName string) string {
	return filepath.Join(SupervisorConfig(), programName+".conf")
}

// NginxSites returns the Nginx sites-available directory.
func NginxSites() string {
	return "/etc/nginx/sites-available"
}

// NginxSitesEnabled returns the Nginx sites-enabled directory.
func NginxSitesEnabled() string {
	return "/etc/nginx/sites-enabled"
}

// NginxSiteConfig returns the Nginx config file path for a specific site.
func NginxSiteConfig(siteName string) string {
	return filepath.Join(NginxSites(), siteName)
}

// MySQLConfig returns the MySQL configuration file path.
func MySQLConfig() string {
	return "/etc/mysql/mysql.conf.d/mysqld.cnf"
}

// PostgreSQLConfig returns the PostgreSQL configuration file path.
func PostgreSQLConfig(version string) string {
	return fmt.Sprintf("/etc/postgresql/%s/main/postgresql.conf", version)
}

// PostgreSQLHBA returns the PostgreSQL HBA configuration file path.
func PostgreSQLHBA(version string) string {
	return fmt.Sprintf("/etc/postgresql/%s/main/pg_hba.conf", version)
}

// CronD returns the cron.d directory.
func CronD() string {
	return "/etc/cron.d"
}

// CronJob returns the path to a cron job file.
func CronJob(name string) string {
	return filepath.Join(CronD(), name)
}

// UFWRules returns the UFW user rules file path.
func UFWRules() string {
	return "/etc/ufw/user.rules"
}

// UFWRulesV6 returns the UFW IPv6 user rules file path.
func UFWRulesV6() string {
	return "/etc/ufw/user6.rules"
}

// SSLCertificate returns the Let's Encrypt certificate path for a domain.
func SSLCertificate(domain string) string {
	return fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain)
}

// SSLPrivateKey returns the Let's Encrypt private key path for a domain.
func SSLPrivateKey(domain string) string {
	return fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", domain)
}
