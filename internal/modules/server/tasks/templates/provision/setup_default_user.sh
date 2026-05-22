#!/bin/bash
{{ shellDefaults }}

echo "Rename existing user 1000 if it exists, otherwise create a new user"

TARGET_USER="{{ .Username }}"
PROTECTED_USERS=("ubuntu" "root" "ec2-user")

if id "$TARGET_USER" > /dev/null 2>&1; then
    echo "User $TARGET_USER already exists. Skipping user creation."
elif getent passwd 1000 > /dev/null 2>&1; then
    OLD_USERNAME=$(getent passwd 1000 | cut -d: -f1)

    # Check if the current user is protected (ubuntu, root, ec2-user, etc.)
    if [[ " ${PROTECTED_USERS[@]} " =~ " $OLD_USERNAME " ]]; then
        echo "User $OLD_USERNAME is a protected user. Skipping renaming and adding user."
        echo "Setup default user"
        sudo useradd -m -d "/home/$TARGET_USER" -s /bin/bash "$TARGET_USER"
    else
        echo "Renaming existing user 1000 ($OLD_USERNAME)"
        sudo pkill -9 -u "$OLD_USERNAME" || true
        sudo pkill -KILL -u "$OLD_USERNAME" || true
        sudo usermod --login "$TARGET_USER" --move-home --home "/home/$TARGET_USER" "$OLD_USERNAME"
        sudo groupmod --new-name "$TARGET_USER" "$OLD_USERNAME"
    fi
else
    echo "Setup default user"
    sudo useradd -m -d "/home/$TARGET_USER" -s /bin/bash "$TARGET_USER"
fi

echo "Create the user's home directory"

sudo mkdir -p /home/{{ .Username }}/{{ .WorkingDirectory }}
sudo mkdir -p /home/{{ .Username }}/.ssh

echo "Add user to groups"

sudo adduser {{ .Username }} sudo
sudo id {{ .Username }}
sudo groups {{ .Username }}

echo "Set shell"

sudo chsh -s /bin/bash {{ .Username }}

echo "Init default profile/bashrc"

sudo cp /root/.bashrc /home/{{ .Username }}/.bashrc
sudo cp /root/.profile /home/{{ .Username }}/.profile

echo "Copy SSH settings from root and create new key"

sudo cp /root/.ssh/authorized_keys /home/{{ .Username }}/.ssh/authorized_keys
sudo cp /root/.ssh/known_hosts /home/{{ .Username }}/.ssh/known_hosts
sudo rm -f /home/{{ .Username }}/.ssh/id_rsa /home/{{ .Username }}/.ssh/id_rsa.pub
sudo ssh-keygen -f /home/{{ .Username }}/.ssh/id_rsa -t rsa -N ''

{{ if .SSHKeys }}
echo "Add SSH keys to authorized_keys"
{{ range .SSHKeys }}
sudo tee -a /home/{{ $.Username }}/.ssh/authorized_keys > /dev/null <<EOF
{{ . }}
EOF
{{ end }}
{{ end }}

echo "Set password"
PASSWORD=$(mkpasswd -m sha-512 "{{ .Password }}")

echo "{{ .Username }}:$PASSWORD" | sudo chpasswd

# Add default landing page. Originally for the PHP/Caddy stack to serve
# from /home/{user}/default, but the file is harmless on docker servers
# where Traefik does the routing. Kept as a single shared step so we
# only have one default-user script to maintain.
echo "Add default landing page"
sudo mkdir -p /home/"{{ .Username }}/default"
sudo tee /home/"{{ .Username }}/default/index.html" > /dev/null <<EOF
This server is managed by <a href="{{ .AppURL }}">{{ .AppName }}</a>.
EOF

echo "Fix user permissions"

sudo chown -R {{ .Username }}:{{ .Username }} /home/{{ .Username }}
sudo chmod -R 755 /home/{{ .Username }}
sudo chmod 700 /home/{{ .Username }}/.ssh
sudo chmod 700 /home/{{ .Username }}/.ssh/id_rsa
sudo chmod 600 /home/{{ .Username }}/.ssh/authorized_keys
