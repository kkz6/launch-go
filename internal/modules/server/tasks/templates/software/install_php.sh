#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

{{ phpPpaFunctions }}

echo "Install PHP {{ .Version }}"

ensurePhpPpaInstalled

waitForAptUnlock

aptGet install -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" -y --allow-downgrades --allow-remove-essential --allow-change-held-packages \
    php{{ .Version }}-bcmath \
    php{{ .Version }}-cli \
    php{{ .Version }}-curl \
    php{{ .Version }}-dev \
    php{{ .Version }}-fpm \
    php{{ .Version }}-gd \
    php{{ .Version }}-imap \
    php{{ .Version }}-intl \
    php{{ .Version }}-mbstring \
    php{{ .Version }}-mysql \
    php{{ .Version }}-sqlite3 \
    php{{ .Version }}-common \
    php{{ .Version }}-xml \
    php{{ .Version }}-zip

echo "Install Redis for PHP {{ .Version }}"

waitForAptUnlock
yes '' | aptGet -y install php{{ .Version }}-redis || echo "php{{ .Version }}-redis not available, skipping..."

echo "Install Memcached for PHP {{ .Version }}"

waitForAptUnlock
yes '' | aptGet -y install php{{ .Version }}-memcached || echo "php{{ .Version }}-memcached not available, skipping..."

echo "Install Swoole for PHP {{ .Version }}"

waitForAptUnlock
yes '' | aptGet -y install php{{ .Version }}-swoole || echo "php{{ .Version }}-swoole not available, skipping..."

echo "Update PHP CLI config"

# Update PHP CLI settings in php.ini
sudo sed -i "s/error_reporting = .*/error_reporting = E_ALL/" /etc/php/{{ .Version }}/cli/php.ini
sudo sed -i "s/display_errors = .*/display_errors = On/" /etc/php/{{ .Version }}/cli/php.ini
sudo sed -i "s/memory_limit = .*/memory_limit = 512M/" /etc/php/{{ .Version }}/cli/php.ini
sudo sed -i "s/;date.timezone.*/date.timezone = UTC/" /etc/php/{{ .Version }}/cli/php.ini
sudo sed -i "s/;cgi.fix_pathinfo=1/cgi.fix_pathinfo=0/" /etc/php/{{ .Version }}/cli/php.ini

echo "Update PHP FPM config"

# Update PHP FPM settings in php.ini
sudo sed -i "s/error_reporting = .*/error_reporting = E_ALL/" /etc/php/{{ .Version }}/fpm/php.ini
sudo sed -i "s/display_errors = .*/display_errors = Off/" /etc/php/{{ .Version }}/fpm/php.ini
sudo sed -i "s/memory_limit = .*/memory_limit = 512M/" /etc/php/{{ .Version }}/fpm/php.ini
sudo sed -i "s/;date.timezone.*/date.timezone = UTC/" /etc/php/{{ .Version }}/fpm/php.ini
sudo sed -i "s/;cgi.fix_pathinfo=1/cgi.fix_pathinfo=0/" /etc/php/{{ .Version }}/fpm/php.ini

echo "Update PHP FPM pool config"

# Update FPM pool configuration
sudo sed -i "s/;request_terminate_timeout.*/request_terminate_timeout = 60/" /etc/php/{{ .Version }}/fpm/pool.d/www.conf
sudo sed -i "s/^user = www-data/user = {{ .Username }}/" /etc/php/{{ .Version }}/fpm/pool.d/www.conf
sudo sed -i "s/^group = www-data/group = {{ .Username }}/" /etc/php/{{ .Version }}/fpm/pool.d/www.conf
sudo sed -i "s/;listen\.owner.*/listen.owner = {{ .Username }}/" /etc/php/{{ .Version }}/fpm/pool.d/www.conf
sudo sed -i "s/;listen\.group.*/listen.group = {{ .Username }}/" /etc/php/{{ .Version }}/fpm/pool.d/www.conf
sudo sed -i "s/;listen\.mode.*/listen.mode = 0666/" /etc/php/{{ .Version }}/fpm/pool.d/www.conf
sudo sed -i "s/^pm.max_children.*=.*/pm.max_children = {{ .MaxChildren }}/" /etc/php/{{ .Version }}/fpm/pool.d/www.conf

echo "Update PHP session config"

# Update session directory permissions
sudo chmod 733 /var/lib/php/sessions
# Set the sticky bit on the session directory to prevent other users from deleting session files
sudo chmod +t /var/lib/php/sessions

echo "PHP configuration updates complete."

sudo service php{{ .Version }}-fpm restart > /dev/null 2>&1

echo "{{ .Username }} ALL=NOPASSWD: /usr/sbin/service php{{ .Version }}-fpm reload" | sudo tee /etc/sudoers.d/php-fpm > /dev/null
