{{/* SSL installation script (Caddy handles this automatically, but this is for custom setups) */}}
{{define "site_install_ssl"}}
#!/bin/bash
set -e

{{template "helpers"}}

# ============================================================================
# SSL Certificate Installation
# Domain: {{.Domain}}
# ============================================================================

DOMAIN="{{.Domain}}"
{{range .Aliases}}
DOMAIN="${DOMAIN},{{.}}"
{{end}}

print_info "Installing SSL certificate for ${DOMAIN}"

{{if .UseCaddy}}
# Caddy handles SSL automatically via ACME
# Just need to update Caddyfile

CADDYFILE="/etc/caddy/Caddyfile"
print_info "Caddy will automatically obtain SSL certificate"

# Reload Caddy to apply changes
systemctl reload caddy

print_success "Caddy reloaded - SSL certificate will be obtained automatically"

{{else}}
# Using certbot for standalone SSL
print_info "Installing certbot..."
apt-get update -y
apt-get install -y certbot {{if .PHPVersion}}python3-certbot-nginx{{end}}

print_info "Obtaining SSL certificate..."
certbot certonly \
    --standalone \
    --non-interactive \
    --agree-tos \
    --email "{{.Email}}" \
    -d "{{.Domain}}" \
    {{range .Aliases}}-d "{{.}}" {{end}}

print_success "SSL certificate obtained"

# Setup auto-renewal
print_info "Setting up auto-renewal..."
systemctl enable certbot.timer
systemctl start certbot.timer

print_success "Auto-renewal configured"
{{end}}

# Verify SSL
print_info "Verifying SSL certificate..."
sleep 5

{{if .HealthCheckURL}}
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "https://{{.Domain}}" || echo "000")
if [ "$HTTP_STATUS" = "200" ] || [ "$HTTP_STATUS" = "301" ] || [ "$HTTP_STATUS" = "302" ]; then
    print_success "HTTPS is working"
else
    print_warning "HTTPS check returned: $HTTP_STATUS"
fi
{{end}}

print_success "SSL installation completed for {{.Domain}}"
{{end}}
