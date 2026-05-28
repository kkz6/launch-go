package tasks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Task type constants for PHP operations
const (
	AddPhpVersionTaskType         = "server:add_php_version"
	RemovePhpVersionTaskType      = "server:remove_php_version"
	InstallPhpExtensionTaskType   = "server:install_php_extension"
	UninstallPhpExtensionTaskType = "server:uninstall_php_extension"
	UpdateAlternativesTaskType    = "server:update_alternatives"
	GetOpcacheStatusTaskType      = "server:get_opcache_status"
	ResetOpcacheTaskType          = "server:reset_opcache"
	ClearOpcacheTaskType          = "server:clear_opcache"
	ConfigureOpcacheTaskType      = "server:configure_opcache"
)

// AddPhpVersion creates a task to install a PHP version on a server
// that's already been provisioned. Reuses the same Ubuntu (ondrej PPA)
// vs Debian (sury.org repo) branching as install_php.sh — see
// PhpPpaFunctions in internal/pkg/taskrunner/templates/functions.go
// for the longer rationale.
func AddPhpVersion(version string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`#!/bin/bash
set -e

. /etc/os-release
case "${ID}" in
    ubuntu)
        sudo add-apt-repository ppa:ondrej/php -y
        ;;
    debian)
        if [ ! -f /etc/apt/sources.list.d/sury-php.list ]; then
            sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
                apt-transport-https lsb-release ca-certificates curl gnupg
            sudo install -m 0755 -d /etc/apt/keyrings
            curl -fsSL https://packages.sury.org/php/apt.gpg \
                | sudo gpg --dearmor -o /etc/apt/keyrings/sury-php.gpg
            sudo chmod a+r /etc/apt/keyrings/sury-php.gpg
            echo "deb [signed-by=/etc/apt/keyrings/sury-php.gpg] https://packages.sury.org/php/ ${VERSION_CODENAME} main" \
                | sudo tee /etc/apt/sources.list.d/sury-php.list >/dev/null
        fi
        ;;
    *)
        echo "ERROR: PHP install supports only Ubuntu or Debian. ID=${ID:-<unset>}" >&2
        exit 1
        ;;
esac

sudo apt-get update
sudo apt-get install -y php%[1]s php%[1]s-fpm php%[1]s-cli php%[1]s-common php%[1]s-mysql php%[1]s-pgsql php%[1]s-sqlite3 php%[1]s-curl php%[1]s-gd php%[1]s-mbstring php%[1]s-xml php%[1]s-zip php%[1]s-bcmath php%[1]s-intl php%[1]s-readline php%[1]s-soap php%[1]s-ldap php%[1]s-imap php%[1]s-opcache`,
		version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Install PHP %s", version)),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(600), // 10 minutes
	)
}

// RemovePhpVersion creates a task to remove a PHP version
func RemovePhpVersion(version string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`sudo apt-get purge -y 'php%s-*'
sudo apt-get autoremove -y`, version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Remove PHP %s", version)),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// InstallPhpExtension creates a task to install a PHP extension
func InstallPhpExtension(version string, extension string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`sudo apt-get install -y php%s-%s
sudo service php%s-fpm restart`, version, extension, version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Install PHP Extension %s", extension)),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// UninstallPhpExtension creates a task to uninstall a PHP extension
func UninstallPhpExtension(version string, extension string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`sudo apt-get purge -y php%s-%s
sudo service php%s-fpm restart`, version, extension, version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Uninstall PHP Extension %s", extension)),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(120),
	)
}

// UpdateAlternatives creates a task to update PHP alternatives
func UpdateAlternatives(version string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`sudo update-alternatives --set php /usr/bin/php%s
sudo update-alternatives --set php-config /usr/bin/php-config%s
sudo update-alternatives --set phpize /usr/bin/phpize%s`, version, version, version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Update PHP Alternatives"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// GetOpcacheStatus creates a task to get OPcache status as JSON
func GetOpcacheStatus(version string) *taskrunner.BaseTask {
	// PHP script that outputs OPcache status as JSON
	// First checks if opcache extension is loaded before attempting to use it
	phpScript := `<?php
error_reporting(0);
ini_set('display_errors', 0);

// Check if OPcache extension is loaded
if (!extension_loaded('Zend OPcache')) {
    echo json_encode(['enabled' => false, 'error' => 'OPcache extension is not loaded']);
    exit(0);
}

// Check if opcache_get_status function exists
if (!function_exists('opcache_get_status')) {
    echo json_encode(['enabled' => false, 'error' => 'OPcache functions not available']);
    exit(0);
}

$status = @opcache_get_status(true);
$config = @opcache_get_configuration();

if ($status === false) {
    echo json_encode(['enabled' => false, 'error' => 'OPcache is not enabled for CLI']);
    exit(0);
}

$result = [
    'enabled' => $status['opcache_enabled'] ?? false,
    'cache_full' => $status['cache_full'] ?? false,
    'restart_pending' => $status['restart_pending'] ?? false,
    'restart_in_progress' => $status['restart_in_progress'] ?? false,
];

if (isset($status['memory_usage'])) {
    $result['memory'] = [
        'used_memory' => $status['memory_usage']['used_memory'] ?? 0,
        'free_memory' => $status['memory_usage']['free_memory'] ?? 0,
        'wasted_memory' => $status['memory_usage']['wasted_memory'] ?? 0,
        'current_wasted_percentage' => $status['memory_usage']['current_wasted_percentage'] ?? 0,
    ];
}

if (isset($status['opcache_statistics'])) {
    $stats = $status['opcache_statistics'];
    $hits = $stats['hits'] ?? 0;
    $misses = $stats['misses'] ?? 0;
    $total = $hits + $misses;
    $hitRate = $total > 0 ? ($hits / $total) * 100 : 0;

    $result['statistics'] = [
        'num_cached_scripts' => $stats['num_cached_scripts'] ?? 0,
        'num_cached_keys' => $stats['num_cached_keys'] ?? 0,
        'max_cached_keys' => $stats['max_cached_keys'] ?? 0,
        'hits' => $hits,
        'misses' => $misses,
        'blacklist_misses' => $stats['blacklist_misses'] ?? 0,
        'blacklist_miss_ratio' => $stats['blacklist_miss_ratio'] ?? 0,
        'oom_restarts' => $stats['oom_restarts'] ?? 0,
        'hash_restarts' => $stats['hash_restarts'] ?? 0,
        'manual_restarts' => $stats['manual_restarts'] ?? 0,
        'hit_rate' => round($hitRate, 2),
    ];
}

if (isset($status['interned_strings_usage'])) {
    $result['interned_strings'] = [
        'buffer_size' => $status['interned_strings_usage']['buffer_size'] ?? 0,
        'used_memory' => $status['interned_strings_usage']['used_memory'] ?? 0,
        'free_memory' => $status['interned_strings_usage']['free_memory'] ?? 0,
        'number_of_strings' => $status['interned_strings_usage']['number_of_strings'] ?? 0,
    ];
}

if (isset($status['jit'])) {
    $result['jit'] = [
        'enabled' => $status['jit']['enabled'] ?? false,
        'on' => $status['jit']['on'] ?? false,
        'kind' => $status['jit']['kind'] ?? 0,
        'opt_level' => $status['jit']['opt_level'] ?? 0,
        'opt_flags' => $status['jit']['opt_flags'] ?? 0,
        'buffer_size' => $status['jit']['buffer_size'] ?? 0,
        'buffer_free' => $status['jit']['buffer_free'] ?? 0,
    ];
}

if ($config && isset($config['directives'])) {
    $result['directives'] = $config['directives'];
}

echo json_encode($result);
`
	// Write PHP script to a temp file and execute it to avoid shell quoting issues
	script := fmt.Sprintf(`OPCACHE_SCRIPT=$(mktemp /tmp/opcache_status_XXXXXX.php)
cat > "$OPCACHE_SCRIPT" << 'OPCACHE_PHP_EOF'
%s
OPCACHE_PHP_EOF
php%s -d opcache.enable_cli=1 -d display_errors=0 -d error_reporting=0 "$OPCACHE_SCRIPT" 2>&1
rm -f "$OPCACHE_SCRIPT"`, phpScript, version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Get OPcache Status"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// ResetOpcache creates a task to reset OPcache by reloading PHP-FPM
// Note: CLI and FPM have separate OPcache instances, so we must reload FPM
func ResetOpcache(version string, sitePath string) *taskrunner.BaseTask {
	// Reset OPcache by reloading PHP-FPM service
	// This is the only reliable way to reset FPM's OPcache
	script := fmt.Sprintf(`echo "Resetting OPcache for PHP %s FPM..."

sudo service php%s-fpm reload

if [ $? -eq 0 ]; then
    echo "OPcache reset complete - PHP-FPM reloaded successfully"
else
    echo "FPM reload failed, attempting full restart..."
    sudo service php%s-fpm restart
    echo "OPcache reset complete - PHP-FPM restarted"
fi`, version, version, version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Reset OPcache"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// ClearOpcache creates a task to clear OPcache by reloading all PHP-FPM services
func ClearOpcache() *taskrunner.BaseTask {
	script := `echo "Clearing OPcache for all PHP versions..."

for service in $(systemctl list-units --type=service --state=running | grep 'php.*-fpm' | awk '{print $1}'); do
    echo "Reloading $service..."
    sudo systemctl reload "$service" 2>/dev/null || true
done

echo "OPcache cleared for all running PHP-FPM services"`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Clear OPcache"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}

// ConfigureOpcache creates a task to configure OPcache settings
func ConfigureOpcache(version string, settings map[string]string) *taskrunner.BaseTask {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var lines []string
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("opcache.%s=%s", key, settings[key]))
	}

	iniPath := fmt.Sprintf("/etc/php/%s/mods-available/opcache-custom.ini", version)
	script := fmt.Sprintf(`echo "Configuring OPcache for PHP %s"

sudo tee %s > /dev/null << 'OPCACHE_EOF'
[opcache]
%s
OPCACHE_EOF

echo "Enabling OPcache configuration..."
sudo phpenmod -v %s opcache-custom || true

echo "Restarting PHP-FPM..."
sudo service php%s-fpm restart || true

echo "OPcache configuration complete"`, version, iniPath, strings.Join(lines, "\n"), version, version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Configure OPcache"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}
