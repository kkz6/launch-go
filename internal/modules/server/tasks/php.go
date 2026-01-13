package tasks

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// AddPhpVersion creates a task to install a PHP version
func AddPhpVersion(version string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`sudo add-apt-repository ppa:ondrej/php -y
sudo apt-get update
sudo apt-get install -y php%s php%s-fpm php%s-cli php%s-common php%s-mysql php%s-pgsql php%s-sqlite3 php%s-curl php%s-gd php%s-mbstring php%s-xml php%s-zip php%s-bcmath php%s-intl php%s-readline php%s-soap php%s-ldap php%s-imap php%s-opcache`,
		version, version, version, version, version, version, version, version,
		version, version, version, version, version, version, version, version,
		version, version, version)

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
	phpScript := `<?php
$status = @opcache_get_status(true);
$config = @opcache_get_configuration();

if ($status === false) {
    echo json_encode(['enabled' => false, 'error' => 'OPcache is not enabled or not available']);
    exit;
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
	// Escape single quotes for bash
	escapedScript := strings.ReplaceAll(phpScript, "'", "'\\''")
	script := fmt.Sprintf(`php%s -d opcache.enable_cli=1 -r '%s'`, version, escapedScript)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Get OPcache Status"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// ResetOpcache creates a task to reset OPcache
func ResetOpcache(version string, sitePath string) *taskrunner.BaseTask {
	var script string
	if sitePath != "" {
		// Reset via the web application
		script = fmt.Sprintf(`php%s %s/artisan opcache:clear 2>/dev/null || php%s -r "opcache_reset();"`, version, sitePath, version)
	} else {
		script = fmt.Sprintf(`php%s -r "opcache_reset();"`, version)
	}

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Reset OPcache"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// ConfigureOpcache creates a task to configure OPcache settings
func ConfigureOpcache(version string, settings map[string]string) *taskrunner.BaseTask {
	var lines []string
	for key, value := range settings {
		lines = append(lines, fmt.Sprintf("opcache.%s=%s", key, value))
	}

	iniPath := fmt.Sprintf("/etc/php/%s/mods-available/opcache.ini", version)
	script := fmt.Sprintf(`sudo cat > %s << 'EOF'
zend_extension=opcache.so
%s
EOF
sudo service php%s-fpm restart`, iniPath, strings.Join(lines, "\n"), version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Configure OPcache"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}
