{{/* Go Template: deployment/link-shared-files.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/deployment/link-shared-files.blade.php */}}
{{ range .SharedFiles }}
# Create directories in shared and release directories if they don't exist
mkdir -p {{ $.ReleaseDirectory }}/{{ dirName . }}
mkdir -p {{ $.SharedDirectory }}/{{ dirName . }}

# If the shared file does not exist, but the release file does, copy the release file to shared
if [ ! -f "{{ $.SharedDirectory }}/{{ . }}" ] && [ -f "{{ $.ReleaseDirectory }}/{{ . }}" ]; then
    cp {{ $.ReleaseDirectory }}/{{ . }} {{ $.SharedDirectory }}/{{ . }}
fi

# If the shared file still does not exist, create it
if [ ! -f "{{ $.SharedDirectory }}/{{ . }}" ]; then
    touch {{ $.SharedDirectory }}/{{ . }}
fi

# If the release file exists, remove it
if [ -f "{{ $.ReleaseDirectory }}/{{ . }}" ]; then
    rm -rf {{ $.ReleaseDirectory }}/{{ . }}
fi

# Create symlink
ln -nfs --relative {{ $.SharedDirectory }}/{{ . }} {{ $.ReleaseDirectory }}/{{ . }}

{{ end }}
