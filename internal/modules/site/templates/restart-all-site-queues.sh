{{/* Go Template: restart-all-site-queues.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/restart-all-site-queues.blade.php */}}
{{ if .HasQueues }}
# Restarting all queues for site {{ .Site.Address }}
{{ range .Commands }}
{{ . }}
{{ end }}
{{ end }}
