{{ shellDefaults }}
{{ caddyReloadFunc }}

echo "Updating Upstreams.caddy import file"

# Write the Upstreams.caddy file with all upstream imports
{
    echo "# Load balancer upstream configurations"
    echo "# Managed by Launch - do not edit manually"
    echo ""

    {{ range .Upstreams }}
    echo "import /etc/caddy/upstreams/{{ .ID }}.caddy"
    {{ end }}
} | sudo tee /etc/caddy/Upstreams.caddy > /dev/null

# Reload Caddy
reloadCaddy

echo "Upstreams imports updated successfully"
