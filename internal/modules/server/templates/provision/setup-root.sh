echo "Add public key for this server"

sudo mkdir -p /root/.ssh
sudo touch /root/.ssh/authorized_keys

sudo tee -a /root/.ssh/authorized_keys > /dev/null <<EOF
{{ .Server.PublicKey }}
EOF

echo "Fix root permissions"

sudo chown root:root /root
sudo chown -R root:root /root/.ssh
sudo chmod 700 /root/.ssh
sudo chmod 600 /root/.ssh/authorized_keys

echo "SSH Keyscans for Source Providers"

sudo ssh-keygen -R github.com 2>/dev/null || true
sudo ssh-keygen -R bitbucket.org 2>/dev/null || true
sudo ssh-keygen -R gitlab.com 2>/dev/null || true
sudo ssh-keyscan -H github.com | sudo tee -a /root/.ssh/known_hosts > /dev/null
sudo ssh-keyscan -H bitbucket.org | sudo tee -a /root/.ssh/known_hosts > /dev/null
sudo ssh-keyscan -H gitlab.com | sudo tee -a /root/.ssh/known_hosts > /dev/null


{{ if eq .Server.Provider "aws" }}
# Enable root login in SSH config
echo "Enabling root login in SSH configuration..."
sudo sed -i 's/^#\?PermitRootLogin .*/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config

# Define the path to the authorized_keys file
AUTH_KEYS_FILE="/root/.ssh/authorized_keys"

# Backup the original file
sudo cp "$AUTH_KEYS_FILE" "$AUTH_KEYS_FILE.bak"

# Remove the specified lines from the authorized_keys file
sudo sed -i -e 's/.*exit 142" \(.*$\)/\1/' /root/.ssh/authorized_keys

# Confirm the changes
echo "Specified text removed from $AUTH_KEYS_FILE."

# Restart the SSH service
echo "Restarting SSH service..."
sudo service ssh start

# Reload Daemon
echo "Reloading Daemon service..."
sudo systemctl daemon-reload

# Update cloud config to enable root
echo "Updating cloud config to enable root..."
sudo sed -i 's/^disable_root: .*/disable_root: false/' /etc/cloud/cloud.cfg

echo "Script completed successfully."
{{ end }}
