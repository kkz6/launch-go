package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

// Task type constants for Caddy operations
const (
	UpdateCaddyfileTaskType        = "site:update_caddyfile"
	UpdateCaddySiteImportsTaskType = "site:update_caddy_site_imports"
	PrettifyCaddyfileTaskType      = "site:prettify_caddyfile"
	RemoveCaddyfileTaskType        = "site:remove_caddyfile"
)

// UpdateCaddyfileConfig holds configuration for updating a site's Caddyfile
type UpdateCaddyfileConfig struct {
	CaddyfilePath    string
	CaddyfileContent string
	SiteUser         string
}

// UpdateCaddyfile creates a task to update a site's Caddyfile
func UpdateCaddyfile(config UpdateCaddyfileConfig) *taskrunner.BaseTask {
	script := templates.MustRender("site", "update_caddyfile.sh", struct {
		CaddyfilePath    string
		CaddyfileContent string
		SiteUser         string
	}{
		CaddyfilePath:    config.CaddyfilePath,
		CaddyfileContent: config.CaddyfileContent,
		SiteUser:         config.SiteUser,
	})

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Update Caddyfile"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}

// SiteImport represents a site for Caddy imports
type SiteImport struct {
	Path string
}

// UpdateCaddySiteImportsConfig holds configuration for updating Caddy site imports
type UpdateCaddySiteImportsConfig struct {
	Sites []SiteImport
}

// UpdateCaddySiteImports creates a task to update the global Caddy site imports
func UpdateCaddySiteImports(config UpdateCaddySiteImportsConfig) *taskrunner.BaseTask {
	script := templates.MustRender("site", "update_caddy_site_imports.sh", struct {
		Sites []SiteImport
	}{
		Sites: config.Sites,
	})

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Update Caddy Site Imports"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// PrettifyCaddyfileConfig holds configuration for prettifying a Caddyfile
type PrettifyCaddyfileConfig struct {
	CaddyfilePath string
}

// PrettifyCaddyfile creates a task to format a Caddyfile
func PrettifyCaddyfile(config PrettifyCaddyfileConfig) *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail

caddy fmt ` + config.CaddyfilePath + ` --overwrite

echo "Caddyfile formatted successfully"
`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Prettify Caddyfile"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// RemoveCaddyfileConfig holds configuration for removing a Caddyfile
type RemoveCaddyfileConfig struct {
	CaddyfilePath string
}

// RemoveCaddyfile creates a task to remove a site's Caddyfile
func RemoveCaddyfile(config RemoveCaddyfileConfig) *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail

# Remove the Caddyfile
rm -f ` + config.CaddyfilePath + `

echo "Caddyfile removed"
`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove Caddyfile"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}
