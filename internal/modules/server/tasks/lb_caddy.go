package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

// Task type constants for load balancer Caddy operations
const (
	UpdateUpstreamCaddyfileTaskType = "server:update_upstream_caddyfile"
	UpdateUpstreamsImportsTaskType  = "server:update_upstreams_imports"
	RemoveUpstreamCaddyfileTaskType = "server:remove_upstream_caddyfile"
)

// UpdateUpstreamCaddyfileConfig holds configuration for updating an upstream's Caddyfile
type UpdateUpstreamCaddyfileConfig struct {
	UpstreamID       string
	CaddyfilePath    string
	CaddyfileContent string
}

// UpdateUpstreamCaddyfile creates a task to write an upstream's Caddyfile and reload Caddy
func UpdateUpstreamCaddyfile(config UpdateUpstreamCaddyfileConfig) *taskrunner.BaseTask {
	script := templates.MustRender("server", "lb/update_upstream_caddyfile.sh", struct {
		CaddyfilePath    string
		CaddyfileContent string
	}{
		CaddyfilePath:    config.CaddyfilePath,
		CaddyfileContent: config.CaddyfileContent,
	})

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Update Upstream Caddyfile"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}

// UpstreamImport represents an upstream for Caddy imports
type UpstreamImport struct {
	ID string
}

// UpdateUpstreamsImportsConfig holds configuration for updating the Upstreams.caddy import file
type UpdateUpstreamsImportsConfig struct {
	Upstreams []UpstreamImport
}

// UpdateUpstreamsImports creates a task to update /etc/caddy/Upstreams.caddy
func UpdateUpstreamsImports(config UpdateUpstreamsImportsConfig) *taskrunner.BaseTask {
	script := templates.MustRender("server", "lb/update_upstreams_imports.sh", struct {
		Upstreams []UpstreamImport
	}{
		Upstreams: config.Upstreams,
	})

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Update Upstreams Imports"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// RemoveUpstreamCaddyfileConfig holds configuration for removing an upstream's Caddyfile
type RemoveUpstreamCaddyfileConfig struct {
	UpstreamID    string
	CaddyfilePath string
}

// RemoveUpstreamCaddyfile creates a task to remove an upstream's Caddyfile and reload Caddy
func RemoveUpstreamCaddyfile(config RemoveUpstreamCaddyfileConfig) *taskrunner.BaseTask {
	script := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

# Remove the upstream Caddyfile
if [ -f '%s' ]; then
    rm -f '%s'
    echo "Upstream Caddyfile removed"
else
    echo "Upstream Caddyfile does not exist, skipping"
fi

# Reload Caddy
sudo /usr/sbin/service caddy reload

echo "Caddy reloaded successfully"
`, config.CaddyfilePath, config.CaddyfilePath)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove Upstream Caddyfile"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}
