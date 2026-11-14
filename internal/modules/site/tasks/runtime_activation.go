package tasks

import (
	"fmt"
	"path/filepath"

	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

// RuntimeActivationConfig identifies the runtime selected for this site. PHP-FPM
// is shared by every site using the same PHP version on the server.
type RuntimeActivationConfig struct {
	SitePath   string
	SiteType   sitetypes.SiteType
	PhpVersion *sitetypes.PhpVersion
}

// RefreshRuntime retries activation of the current code without switching a
// release or rerunning installation, build hooks, or database migrations.
func RefreshRuntime(config RuntimeActivationConfig) (*taskrunner.BaseTask, error) {
	if !filepath.IsAbs(config.SitePath) {
		return nil, fmt.Errorf("runtime refresh requires an absolute site path")
	}
	script, err := renderRuntimeActivation(config)
	if err != nil {
		return nil, err
	}
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Refresh PHP Runtime"),
		taskrunner.WithScript(withDeploymentLock(config.SitePath, script)),
		taskrunner.WithTimeoutSeconds(90),
	), nil
}

func renderRuntimeActivation(config RuntimeActivationConfig) (string, error) {
	if !config.SiteType.IsValid() {
		return "", fmt.Errorf("runtime refresh requires a valid site type")
	}
	if config.SiteType == sitetypes.SiteTypeStatic {
		return "echo 'Static site: no PHP runtime refresh required.'\n", nil
	}
	// Generic sites are PHP sites too; SiteType.IsPHP currently excludes them.
	if config.PhpVersion == nil || !config.PhpVersion.IsValid() {
		return "", fmt.Errorf("select a valid PHP version for the site before refreshing its runtime or rolling back")
	}
	return templates.MustRender("site", "deployment/refresh_runtime.sh", struct {
		ServiceName string
		BinaryPath  string
	}{
		ServiceName: config.PhpVersion.FPMServiceName(),
		BinaryPath:  "/usr/sbin/php-fpm" + config.PhpVersion.GetVersion(),
	}), nil
}

func renderDeploymentRuntime(config RuntimeActivationConfig) (string, error) {
	// Older sites can deploy using the default CLI binary without a recorded
	// HTTP runtime. Preserve those deployments without guessing an FPM service.
	if config.SiteType.IsValid() && config.SiteType != sitetypes.SiteTypeStatic && config.PhpVersion == nil {
		return "echo 'LAUNCHCTL_RUNTIME_REFRESH_SKIPPED: no PHP version is selected. Select the site PHP version to enable automatic runtime refresh.' >&2\n", nil
	}
	return renderRuntimeActivation(config)
}

func runtimeConfigurationFailure(err error) string {
	return "#!/bin/bash\nprintf '%s\\n' '" + escapeShellPath(err.Error()) + "' >&2\nexit 1\n"
}
