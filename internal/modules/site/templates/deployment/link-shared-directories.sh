{{/* Go Template: deployment/link-shared-directories.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/deployment/link-shared-directories.blade.php */}}
{{ range .SharedDirectories }}
if [ ! -d "{{ $.SharedDirectory }}/{{ . }}" ]; then
    # Create shared directory if it does not exist.
    mkdir -p {{ $.SharedDirectory }}/{{ . }}

    if [ -d "{{ $.ReleaseDirectory }}/{{ . }}" ]; then
        # Copy contents of release directory to shared directory if it exists.
        cp -r {{ $.ReleaseDirectory }}/{{ . }} {{ $.SharedDirectory }}/{{ dirName . }}
    fi
fi

#  Remove shared directory from release directory if it exists.
rm -rf {{ $.ReleaseDirectory }}/{{ . }}

# Create parent directory of shared directory in release directory if it does not exist,
# otherwise symlink will fail.
mkdir -p $(dirname {{ $.ReleaseDirectory }}/{{ . }})

# Symlink shared directory to release directory.
ln -nfs --relative {{ $.SharedDirectory }}/{{ . }} {{ $.ReleaseDirectory }}/{{ . }}

{{ end }}
