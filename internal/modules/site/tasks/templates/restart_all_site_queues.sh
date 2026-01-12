{{ if .HasQueues }}
# Restarting all queues for site {{ .SiteAddress }}
{{ range .Commands }}
{{ . }}
{{ end }}
{{ end }}
