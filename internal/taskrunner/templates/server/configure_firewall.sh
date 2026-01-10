{{/* Firewall configuration script */}}
{{define "server_configure_firewall"}}
#!/bin/bash
set -e

{{template "helpers"}}

# ============================================================================
# Firewall Configuration Script
# Server: {{.ServerName}}
# ============================================================================

print_info "Configuring firewall..."

# Reset UFW to default
print_info "Resetting UFW to defaults..."
ufw --force reset

# Set default policies
ufw default deny incoming
ufw default allow outgoing

# ----------------------------------------------------------------------------
# Allow SSH
# ----------------------------------------------------------------------------
print_info "Allowing SSH on port {{.SSHPort}}..."
ufw allow {{.SSHPort}}/tcp comment 'SSH'

# ----------------------------------------------------------------------------
# Standard Web Ports
# ----------------------------------------------------------------------------
{{if .AllowHTTP}}
print_info "Allowing HTTP (port 80)..."
ufw allow 80/tcp comment 'HTTP'
{{end}}

{{if .AllowHTTPS}}
print_info "Allowing HTTPS (port 443)..."
ufw allow 443/tcp comment 'HTTPS'
{{end}}

# ----------------------------------------------------------------------------
# Custom Rules
# ----------------------------------------------------------------------------
{{range .Rules}}
print_info "Adding rule: {{.Name}} ({{.Port}}/{{.Protocol}}{{if .FromIP}} from {{.FromIP}}{{end}})"
{{if .FromIP}}
ufw allow from {{.FromIP}} to any port {{.Port}} proto {{.Protocol}} comment '{{.Name}}'
{{else}}
ufw allow {{.Port}}/{{.Protocol}} comment '{{.Name}}'
{{end}}
{{end}}

# ----------------------------------------------------------------------------
# Enable Firewall
# ----------------------------------------------------------------------------
print_info "Enabling firewall..."
ufw --force enable

# Show status
print_info "Firewall status:"
ufw status verbose

print_success "Firewall configuration completed"
{{end}}
