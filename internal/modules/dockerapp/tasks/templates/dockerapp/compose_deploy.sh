#!/bin/bash
{{ shellDefaults }}

echo "Deploy compose project {{ .Project }}"

PROJECT_DIR="/opt/launch/apps/{{ .AppName }}"
sudo mkdir -p "${PROJECT_DIR}"

# Write the compose file. Heredoc keeps any embedded $VAR untouched.
sudo tee "${PROJECT_DIR}/docker-compose.yml" >/dev/null <<'LAUNCH_EOF'
{{ .ComposeYAML }}
LAUNCH_EOF

# Write the .env file. Compose picks it up automatically when invoked
# from the project directory.
sudo tee "${PROJECT_DIR}/.env" >/dev/null <<'LAUNCH_EOF'
{{ .ComposeEnv }}
LAUNCH_EOF
sudo chmod 600 "${PROJECT_DIR}/.env"

{{ if .RegistryURL }}
# Login so private images in the compose file can be pulled.
echo "{{ .RegistryPassword }}" | sudo docker login "{{ .RegistryURL }}" --username "{{ .RegistryUsername }}" --password-stdin
{{ end }}

cd "${PROJECT_DIR}"

# Pull updated images then bring the stack up. --remove-orphans drops
# services that were renamed/removed in the new file.
sudo docker compose -p "{{ .Project }}" pull
sudo docker compose -p "{{ .Project }}" up -d --remove-orphans

echo "Deploy of compose project {{ .Project }} complete."
