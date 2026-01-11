{{/* Go Template: deploy-site-without-downtime.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/deploy-site-without-downtime.blade.php */}}
#!/bin/bash
set -euo pipefail

{{ template "deployment/shell-variables.sh.tmpl" . }}

# Create the necessary directories
mkdir -p {{ .RepositoryDirectory }}
mkdir -p {{ .SharedDirectory }}
mkdir -p {{ .ReleaseDirectory }}
mkdir -p {{ .LogsDirectory }}

# Cleanup old releases
{{ template "deployment/cleanup-old-releases.sh.tmpl" . }}

{{ if .Site.HookBeforeUpdatingRepository }}
echo "Running hook before updating repository"
cd {{ .ReleaseDirectory }}
{{ .Site.HookBeforeUpdatingRepository }}
{{ end }}

{{ if .Site.RepositoryURL }}
{{ template "deployment/update-repository.sh.tmpl" . }}

{{ if .Site.HookAfterUpdatingRepository }}
echo "Running hook after updating repository"
cd {{ .ReleaseDirectory }}
{{ .Site.HookAfterUpdatingRepository }}
{{ end }}

{{ end }}

{{ if not .Site.InstalledAt }}
{{ template "deployment/prepare-fresh-installation.sh.tmpl" . }}
{{ end }}

{{ template "deployment/link-shared-directories.sh.tmpl" . }}

{{ template "deployment/link-shared-files.sh.tmpl" . }}

{{ template "deployment/make-directories-writable.sh.tmpl" . }}

{{ if .Site.HookBeforeMakingCurrent }}
echo "Running hook before putting the site live"
cd {{ .ReleaseDirectory }}
{{ .Site.HookBeforeMakingCurrent }}
{{ end }}

{{ template "deployment/make-deployment-current.sh.tmpl" . }}

{{ if .Site.HookAfterMakingCurrent }}
echo "Running hook after putting the site live"
cd {{ .ReleaseDirectory }}
{{ .Site.HookAfterMakingCurrent }}
{{ end }}

echo "Done!"
