cd {{ .SitePath }}

ENV_PATH="{{ if .ZeroDowntimeDeployment }}{{ .SharedDirectory }}{{ else }}{{ .RepositoryDirectory }}{{ end }}/.env"
EXAMPLE_ENV_PATH="{{ if .ZeroDowntimeDeployment }}{{ .ReleaseDirectory }}{{ else }}{{ .RepositoryDirectory }}{{ end }}/.env.example"

if [ ! -f $ENV_PATH ] && [ -f $EXAMPLE_ENV_PATH ]; then
    cp $EXAMPLE_ENV_PATH $ENV_PATH
    cd {{ if .ZeroDowntimeDeployment }}{{ .SharedDirectory }}{{ else }}{{ .RepositoryDirectory }}{{ end }}

    {{ range $key, $value := .EnvVariables }}
        sed -i --follow-symlinks "s|^#* *{{ $key }}=.*|{{ $key }}={{ $value }}|g" .env
    {{ end }}

fi
