{{ range .WritableDirectories }}
DIRECTORY_IS_WRITEABLE=$(getfacl -p {{ $.ReleaseDirectory }}/{{ . }} | grep "^user:{{ $.Username }}:.*w" | wc -l)

if [ $DIRECTORY_IS_WRITEABLE -eq 0 ]; then
    # Make the directory writable (without sudo)
    setfacl -L -m u:{{ $.Username }}:rwX {{ $.ReleaseDirectory }}/{{ . }}
    setfacl -dL -m u:{{ $.Username }}:rwX {{ $.ReleaseDirectory }}/{{ . }}
fi

{{ end }}
