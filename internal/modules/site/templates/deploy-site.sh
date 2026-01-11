{{/* Go Template: deploy-site.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/deploy-site.blade.php */}}
#!/bin/bash
set -euo pipefail

{{ template "deployment/shell-variables.sh.tmpl" . }}

# Create the necessary directories
mkdir -p {{ .RepositoryDirectory }}
mkdir -p {{ .LogsDirectory }}

{{ if and .Site.InstalledAt .Site.HookBeforeUpdatingRepository }}
echo "Running hook before updating repository"
cd {{ .RepositoryDirectory }}
{{ .Site.HookBeforeUpdatingRepository }}
{{ end }}

{{ if .Site.RepositoryURL }}
{{ template "deployment/update-repository.sh.tmpl" . }}

{{ if .Site.HookAfterUpdatingRepository }}
echo "Running hook after updating repository"
cd {{ .RepositoryDirectory }}
{{ .Site.HookAfterUpdatingRepository }}
{{ end }}

{{ end }}

{{ if not .Site.InstalledAt }}
{{ template "deployment/prepare-fresh-installation.sh.tmpl" . }}
{{ end }}

{{ if and .Site.InstalledAt (eq .Site.Type "wordpress") }}
echo "Wordpress already installed!"
{{ end }}

echo "Done!"
