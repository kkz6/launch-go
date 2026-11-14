FROM ubuntu:24.04

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends php8.3-fpm php8.3-cli libfcgi-bin sudo \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home launcher \
    && printf 'launcher ALL=(root) NOPASSWD: /usr/sbin/service php8.3-fpm reload\n' > /etc/sudoers.d/launchctl-fixture \
    && chmod 440 /etc/sudoers.d/launchctl-fixture
