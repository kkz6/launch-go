#!/bin/bash
{{ shellDefaults }}

echo "Install Bun"

curl -fsSL https://bun.sh/install | bash

# Add bun to PATH for all users
echo 'export BUN_INSTALL="$HOME/.bun"' | sudo tee -a /etc/profile.d/bun.sh > /dev/null
echo 'export PATH="$BUN_INSTALL/bin:$PATH"' | sudo tee -a /etc/profile.d/bun.sh > /dev/null

echo "Bun installation complete."
