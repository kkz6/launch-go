package paths

import (
	"fmt"
	"path/filepath"
)

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
