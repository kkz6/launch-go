{{ shellDefaults }}
{{ caddyReloadFunc }}

# Update Caddy site imports
{
    echo "# import /home/user/example.com/Caddyfile"

    # Loop through each site and generate the import statements
    {{ range .Sites }}
        echo "import {{ .Path }}/Caddyfile"
    {{ end }}
} | sudo tee /etc/caddy/Sites.caddy > /dev/null

# Reload Caddy
reloadCaddy
