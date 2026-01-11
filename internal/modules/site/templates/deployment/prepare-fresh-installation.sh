{{/* Go Template: deployment/prepare-fresh-installation.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/deployment/prepare-fresh-installation.blade.php */}}
cd {{ .Site.Path }}

{{ if .FreshInstallationScript }}
{{ .FreshInstallationScript }}
{{ end }}

cd {{ .Site.Path }}
