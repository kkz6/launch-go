#!/bin/bash
set -euo pipefail

OLD_USER="{{ .OldUsername }}"
NEW_USER="{{ .NewUsername }}"

# Idempotent: skip if already renamed
if id "$NEW_USER" &>/dev/null; then
    echo "User $NEW_USER already exists, skipping rename."
    exit 0
fi

# Verify old user exists
if ! id "$OLD_USER" &>/dev/null; then
    echo "ERROR: User $OLD_USER does not exist."
    exit 1
fi

# Kill processes owned by old user (gracefully)
pkill -u "$OLD_USER" || true
sleep 2

# Rename user and move home directory
usermod -l "$NEW_USER" -d "/home/$NEW_USER" -m "$OLD_USER"

# Rename primary group
groupmod -n "$NEW_USER" "$OLD_USER"

# Update sudoers if present
if [ -f "/etc/sudoers.d/$OLD_USER" ]; then
    mv "/etc/sudoers.d/$OLD_USER" "/etc/sudoers.d/$NEW_USER"
    sed -i "s/$OLD_USER/$NEW_USER/g" "/etc/sudoers.d/$NEW_USER"
fi

# Update Caddyfile references
if [ -f /etc/caddy/Caddyfile ]; then
    sed -i "s|/home/$OLD_USER/|/home/$NEW_USER/|g" /etc/caddy/Caddyfile
    systemctl reload caddy || true
fi

# Update systemd service files (queue workers, daemons)
for f in /etc/systemd/system/worker-*.service /etc/systemd/system/daemon-*.service; do
    [ -f "$f" ] && sed -i "s|/home/$OLD_USER|/home/$NEW_USER|g; s|User=$OLD_USER|User=$NEW_USER|g" "$f"
done
systemctl daemon-reload || true

# Update crontab
if crontab -u "$NEW_USER" -l &>/dev/null; then
    crontab -u "$NEW_USER" -l | sed "s|/home/$OLD_USER|/home/$NEW_USER|g" | crontab -u "$NEW_USER" -
fi

echo "Successfully renamed $OLD_USER to $NEW_USER"
