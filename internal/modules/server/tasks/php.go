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
		taskrunner.WithTimeout(600), // 10 minutes
	)
}

// RemovePhpVersion creates a task to remove a PHP version
func RemovePhpVersion(version string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`sudo apt-get purge -y 'php%s-*'
sudo apt-get autoremove -y`, version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Remove PHP %s", version)),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}

// InstallPhpExtension creates a task to install a PHP extension
func InstallPhpExtension(version string, extension string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`sudo apt-get install -y php%s-%s
sudo service php%s-fpm restart`, version, extension, version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Install PHP Extension %s", extension)),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}

// UninstallPhpExtension creates a task to uninstall a PHP extension
func UninstallPhpExtension(version string, extension string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`sudo apt-get purge -y php%s-%s
sudo service php%s-fpm restart`, version, extension, version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Uninstall PHP Extension %s", extension)),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(120),
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
		taskrunner.WithTimeout(30),
	)
}

// GetOpcacheStatus creates a task to get OPcache status
func GetOpcacheStatus(version string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`php%s -r "print_r(opcache_get_status());"`, version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Get OPcache Status"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(15),
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
		taskrunner.WithTimeout(30),
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
		taskrunner.WithTimeout(30),
	)
}
