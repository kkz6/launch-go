#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Install Bun"

waitForAptUnlock

# Install dependencies
aptGet install -y curl unzip

# Install Bun system-wide to /usr/local so it's available in non-login shells (e.g. deployments)
curl -fsSL https://bun.sh/install | sudo BUN_INSTALL="/usr/local" bash

# Create symlink if bun is not in /usr/local/bin
if [ ! -f /usr/local/bin/bun ] && [ -f /usr/local/.bun/bin/bun ]; then
    sudo ln -sf /usr/local/.bun/bin/bun /usr/local/bin/bun
fi

# Add bun to PATH for all users if not already present
if ! grep -q "BUN_INSTALL" /etc/profile.d/bun.sh 2>/dev/null; then
    sudo bash -c 'cat > /etc/profile.d/bun.sh << "BUNEOF"
export BUN_INSTALL="/usr/local"
export PATH="/usr/local/bin:$PATH"
BUNEOF'
    sudo chmod +x /etc/profile.d/bun.sh
fi

# Verify installation
/usr/local/bin/bun --version || echo "Bun installed but path verification pending"

echo "Bun installation complete."
