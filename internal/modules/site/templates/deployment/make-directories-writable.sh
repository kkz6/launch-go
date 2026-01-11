{{/* Go Template: deployment/make-directories-writable.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/deployment/make-directories-writable.blade.php */}}
{{ range .WritableDirectories }}
DIRECTORY_IS_WRITEABLE=$(getfacl -p {{ $.ReleaseDirectory }}/{{ . }} | grep "^user:{{ $.Site.User }}:.*w" | wc -l)

if [ $DIRECTORY_IS_WRITEABLE -eq 0 ]; then
    # Make the directory writable (without sudo)
    setfacl -L -m u:{{ $.Site.User }}:rwX {{ $.ReleaseDirectory }}/{{ . }}
    setfacl -dL -m u:{{ $.Site.User }}:rwX {{ $.ReleaseDirectory }}/{{ . }}
fi

{{ end }}
