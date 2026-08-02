package tasks

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// PHPVersionConfigFile describes one managed runtime configuration file that
// must change together with the site's PHP version.
type PHPVersionConfigFile struct {
	Path     string
	Contents string
	Owner    string
}

// UpdatePHPVersionConfig contains everything needed to atomically switch the
// PHP runtime used by a site and its managed background processes.
type UpdatePHPVersionConfig struct {
	SiteAddress        string
	Version            string
	PHPBinary          string
	FPMService         string
	FPMSocket          string
	CaddyfilePath      string
	Files              []PHPVersionConfigFile
	SupervisorPrograms []string
}

// UpdatePHPVersion updates all managed configuration files as one operation.
// A shell ERR trap restores every touched file and reloads Caddy/Supervisor if
// validation or a process restart fails.
func UpdatePHPVersion(config UpdatePHPVersionConfig) *taskrunner.BaseTask {
	var script strings.Builder
	script.WriteString(`#!/bin/bash
set -Eeuo pipefail

TEMP_DIR="$(mktemp -d)"
ROLLING_BACK=0

reload_caddy() {
    timeout 30 /usr/sbin/service caddy reload
}

rollback() {
    local exit_code="$1"
    if [ "$ROLLING_BACK" -eq 1 ]; then
        exit "$exit_code"
    fi

    ROLLING_BACK=1
    trap - ERR HUP INT TERM
    echo "PHP runtime update failed; restoring previous configuration..." >&2
`)

	for i := len(config.Files) - 1; i >= 0; i-- {
		file := config.Files[i]
		path := strconv.Quote(file.Path)
		fmt.Fprintf(&script, `    if [ -f "$TEMP_DIR/missing-%d" ]; then
        rm -f %s
    elif [ -e "$TEMP_DIR/backup-%d" ]; then
        cp -a "$TEMP_DIR/backup-%d" %s
    fi
`, i, path, i, i, path)
	}

	if config.CaddyfilePath != "" {
		script.WriteString("    reload_caddy >/dev/null 2>&1 || true\n")
	}
	if len(config.SupervisorPrograms) > 0 {
		script.WriteString("    supervisorctl reread >/dev/null 2>&1 || true\n")
		script.WriteString("    supervisorctl update >/dev/null 2>&1 || true\n")
	}

	script.WriteString(`    rm -rf "$TEMP_DIR"
    exit "$exit_code"
}

trap 'rollback $?' ERR
trap 'rollback 129' HUP
trap 'rollback 130' INT
trap 'rollback 143' TERM

`)

	fmt.Fprintf(&script, "command -v %s >/dev/null 2>&1\n", strconv.Quote(config.PHPBinary))
	fmt.Fprintf(&script, "systemctl is-active --quiet %s\n", strconv.Quote(config.FPMService))
	fmt.Fprintf(&script, "test -S %s\n\n", strconv.Quote(config.FPMSocket))

	for i, file := range config.Files {
		path := strconv.Quote(file.Path)
		encoded := base64.StdEncoding.EncodeToString([]byte(file.Contents))
		fmt.Fprintf(&script, `mkdir -p "$(dirname %s)"
if [ -e %s ]; then
    cp -a %s "$TEMP_DIR/backup-%d"
else
    touch "$TEMP_DIR/missing-%d"
fi
printf '%%s' %s | base64 --decode > "$TEMP_DIR/new-%d"
chmod 0644 "$TEMP_DIR/new-%d"
mv "$TEMP_DIR/new-%d" %s
`, path, path, path, i, i, strconv.Quote(encoded), i, i, i, path)
		if file.Owner != "" {
			fmt.Fprintf(&script, "chown %s %s\n", strconv.Quote(file.Owner), path)
		}
		script.WriteString("\n")
	}

	if config.CaddyfilePath != "" {
		fmt.Fprintf(&script, "caddy fmt %s --overwrite\n", strconv.Quote(config.CaddyfilePath))
		script.WriteString("caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile\n")
		script.WriteString("reload_caddy\n\n")
	}

	if len(config.SupervisorPrograms) > 0 {
		script.WriteString("supervisorctl reread\n")
		script.WriteString("supervisorctl update\n")
		for _, program := range config.SupervisorPrograms {
			fmt.Fprintf(&script, "supervisorctl restart %s\n", strconv.Quote(program+":*"))
		}
		script.WriteString("\n")
	}

	script.WriteString(`trap - ERR HUP INT TERM
rm -rf "$TEMP_DIR"
echo "PHP runtime updated successfully"
`)

	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Switch %s to PHP %s", config.SiteAddress, config.Version)),
		taskrunner.WithScript(script.String()),
		taskrunner.WithTimeoutSeconds(180),
	)
}
