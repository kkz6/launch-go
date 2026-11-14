{{ shellDefaults }}
{{ caddyReloadFunc }}

# Keep imports for sites that are no longer managed by launchctl.
imports=/etc/caddy/Sites.caddy
exec 9>/etc/caddy/.sites.lock
flock 9
temp=$(mktemp /etc/caddy/.sites.XXXXXX)
trap 'rm -f -- "$temp"' EXIT
if [[ -f "$imports" ]]; then
    {{ if .RemovePath }}
    grep -Fxv -- 'import {{ escape .RemovePath }}/Caddyfile' "$imports" > "$temp" || [[ $? -eq 1 ]]
    {{ else }}
    cp -- "$imports" "$temp"
    {{ end }}
fi
{{ if .AddPath }}
line='import {{ escape .AddPath }}/Caddyfile'
if ! grep -Fxq -- "$line" "$temp"; then
    printf '%s\n' "$line" >> "$temp"
fi
{{ end }}
chmod 0644 "$temp"
mv -- "$temp" "$imports"

# Reload Caddy
reloadCaddy
