package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Task type constants for user operations
const (
	RenameLocalUserTaskType = "server:rename_local_user"
)

// RenameLocalUser creates a task to rename the local user on a server
// This renames the Linux user account, updates home directory, and updates group membership
func RenameLocalUser(oldUsername, newUsername string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`#!/bin/bash
set -e

OLD_USER="%s"
NEW_USER="%s"

# Check if old user exists
if ! id "$OLD_USER" &>/dev/null; then
    echo "User $OLD_USER does not exist"
    exit 0
fi

# Check if new user already exists
if id "$NEW_USER" &>/dev/null; then
    echo "User $NEW_USER already exists"
    exit 1
fi

# Kill any processes running as the old user
pkill -u "$OLD_USER" 2>/dev/null || true
sleep 2

# Rename the user
usermod -l "$NEW_USER" "$OLD_USER"

# Rename the group if it matches the old username
if getent group "$OLD_USER" &>/dev/null; then
    groupmod -n "$NEW_USER" "$OLD_USER"
fi

# Rename the home directory
if [ -d "/home/$OLD_USER" ]; then
    usermod -d "/home/$NEW_USER" -m "$NEW_USER"
fi

# Update sudoers if the old user had sudo access
if [ -f "/etc/sudoers.d/$OLD_USER" ]; then
    mv "/etc/sudoers.d/$OLD_USER" "/etc/sudoers.d/$NEW_USER"
    sed -i "s/$OLD_USER/$NEW_USER/g" "/etc/sudoers.d/$NEW_USER"
fi

# Update any cron jobs
if [ -f "/var/spool/cron/crontabs/$OLD_USER" ]; then
    mv "/var/spool/cron/crontabs/$OLD_USER" "/var/spool/cron/crontabs/$NEW_USER"
    chown "$NEW_USER:crontab" "/var/spool/cron/crontabs/$NEW_USER"
fi

# Update PHP-FPM pool configurations if they exist
for pool_file in /etc/php/*/fpm/pool.d/*.conf; do
    if [ -f "$pool_file" ]; then
        sed -i "s/user = $OLD_USER/user = $NEW_USER/g" "$pool_file"
        sed -i "s/group = $OLD_USER/group = $NEW_USER/g" "$pool_file"
        sed -i "s|/home/$OLD_USER|/home/$NEW_USER|g" "$pool_file"
    fi
done

# Update supervisor configurations if they exist
for conf_file in /etc/supervisor/conf.d/*.conf; do
    if [ -f "$conf_file" ]; then
        sed -i "s/user=$OLD_USER/user=$NEW_USER/g" "$conf_file"
        sed -i "s|/home/$OLD_USER|/home/$NEW_USER|g" "$conf_file"
    fi
done

# Update Caddy site configurations if they exist
for caddy_file in /etc/caddy/sites/*.caddy; do
    if [ -f "$caddy_file" ]; then
        sed -i "s|/home/$OLD_USER|/home/$NEW_USER|g" "$caddy_file"
    fi
done

# Restart services to pick up changes
systemctl restart php*-fpm 2>/dev/null || true
systemctl restart supervisor 2>/dev/null || true
systemctl reload caddy 2>/dev/null || true

echo "Successfully renamed user from $OLD_USER to $NEW_USER"
`, oldUsername, newUsername)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Rename Local User"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(120),
	)
}
