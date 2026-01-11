{{/*
  Install Composer 2 on Ubuntu server.
  This is a Go template file - use with text/template.

  Required template variables:
  - .Username: Server username for Composer configuration
*/}}
set -e
export DEBIAN_FRONTEND=noninteractive

# ==============================================================================
# Install Composer 2
# ==============================================================================

echo "Download and install Composer dependency manager"

curl -sS https://getcomposer.org/installer | php -- --2
sudo mv composer.phar /usr/local/bin/composer
sudo chmod +x /usr/local/bin/composer

echo "{{ .Username }} ALL=(root) NOPASSWD: /usr/local/bin/composer self-update*" | sudo tee /etc/sudoers.d/composer > /dev/null

# Create default auth.json

sudo mkdir -p /home/{{ .Username }}/.config/composer
sudo touch /home/{{ .Username }}/.config/composer/auth.json

sudo tee /home/{{ .Username }}/.config/composer/auth.json > /dev/null <<'EOF'
{
    "bearer": {},
    "bitbucket-oauth": {},
    "github-oauth": {},
    "gitlab-oauth": {},
    "gitlab-token": {},
    "http-basic": {}
}
EOF

sudo chown -R {{ .Username }}:{{ .Username }} /home/{{ .Username }}/.config/composer
sudo chmod 600 /home/{{ .Username }}/.config/composer/auth.json

echo "Composer 2 installation complete."
