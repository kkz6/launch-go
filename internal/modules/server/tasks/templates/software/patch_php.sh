#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

PHP_SERIES="{{ .Version }}"
FPM_SERVICE="php${PHP_SERIES}-fpm"
PHP_BINARY="php${PHP_SERIES}"
FPM_BINARY="php-fpm${PHP_SERIES}"

if [ -z "${PHP_SERIES}" ]; then
    echo "ERROR: Invalid PHP series" >&2
    exit 1
fi

if [ ! -r /etc/os-release ]; then
    echo "ERROR: Unable to detect the operating system" >&2
    exit 1
fi

. /etc/os-release
case "${ID:-}" in
    ubuntu|debian)
        ;;
    *)
        echo "ERROR: PHP patching supports only Ubuntu or Debian. ID=${ID:-<unset>}" >&2
        exit 1
        ;;
esac

if ! command -v "${PHP_BINARY}" >/dev/null 2>&1; then
    echo "ERROR: PHP ${PHP_SERIES} is not installed" >&2
    exit 1
fi

if ! systemctl list-unit-files "${FPM_SERVICE}.service" --no-legend 2>/dev/null | grep -q "^${FPM_SERVICE}.service"; then
    echo "ERROR: ${FPM_SERVICE} is not installed" >&2
    exit 1
fi

mapfile -t phpPackages < <(
    dpkg-query -W -f='${db:Status-Abbrev} ${binary:Package}\n' "php${PHP_SERIES}*" 2>/dev/null \
        | awk '$1 == "ii" { print $2 }' \
        | sort -u
)

if [ "${#phpPackages[@]}" -eq 0 ]; then
    echo "ERROR: No installed packages found for PHP ${PHP_SERIES}" >&2
    exit 1
fi

echo "Patching installed PHP ${PHP_SERIES} packages:"
printf ' - %s\n' "${phpPackages[@]}"

waitForAptUnlock
aptGet update

waitForAptUnlock
aptGet install \
    -o Dpkg::Options::="--force-confdef" \
    -o Dpkg::Options::="--force-confold" \
    --only-upgrade \
    --no-install-recommends \
    -y \
    -- "${phpPackages[@]}"

if ! command -v "${FPM_BINARY}" >/dev/null 2>&1; then
    echo "ERROR: ${FPM_BINARY} is unavailable after patching" >&2
    exit 1
fi

sudo "${FPM_BINARY}" -t
sudo systemctl restart "${FPM_SERVICE}"
sudo systemctl is-active --quiet "${FPM_SERVICE}"

installedVersion="$("${PHP_BINARY}" -r 'echo PHP_VERSION;')"
case "${installedVersion}" in
    "${PHP_SERIES}"|"${PHP_SERIES}".*)
        ;;
    *)
        echo "ERROR: Expected PHP ${PHP_SERIES}, detected ${installedVersion:-<empty>}" >&2
        exit 1
        ;;
esac

echo "LAUNCH_PHP_PATCH_VERSION=${installedVersion}"
echo "PHP ${installedVersion} patched successfully"
