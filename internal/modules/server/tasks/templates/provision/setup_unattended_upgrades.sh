#!/bin/bash
{{ shellDefaults }}

echo "Setup unattended upgrades"

# Configure the allowed origins for unattended upgrades
sudo tee /etc/apt/apt.conf.d/50unattended-upgrades > /dev/null <<EOF
Unattended-Upgrade::Allowed-Origins {
    "\${distro_id} \${distro_codename}-security";
};
Unattended-Upgrade::Package-Blacklist {
    //
};
EOF

# Configure the periodic settings for APT
sudo tee /etc/apt/apt.conf.d/10periodic > /dev/null <<EOF
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
APT::Periodic::Unattended-Upgrade "1";
EOF
