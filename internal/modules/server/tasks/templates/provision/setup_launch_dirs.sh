#!/bin/bash
{{ shellDefaults }}

echo "Creating launch directory tree under {{ .RootDir }}"

sudo mkdir -p "{{ .RootDir }}/traefik/dynamic"
sudo mkdir -p "{{ .RootDir }}/logs"
sudo mkdir -p "{{ .RootDir }}/applications"

if id "{{ .Username }}" >/dev/null 2>&1; then
    sudo chown -R "{{ .Username }}:{{ .Username }}" "{{ .RootDir }}"
fi
sudo chmod 755 "{{ .RootDir }}"
