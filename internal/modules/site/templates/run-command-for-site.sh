{{/* Go Template: run-command-for-site.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/run-command-for-site.blade.php */}}
cd {{ .Site.ApplicationDirectory }}

{{ .Command }}

echo "Done!"
