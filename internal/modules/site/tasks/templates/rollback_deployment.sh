{{ shellDefaults }}

echo "Rolling back to previous release..."

# Inventory can become stale before this task acquires the deployment lock.
# Revalidate the physical tree and current link while holding that lock.
if [ -L '{{ escape .SitePath }}' ]; then
    echo 'Error: Site path is a symlink.' >&2
    exit 1
fi
SITE_ROOT=$(readlink -f -- '{{ escape .SitePath }}')
RELEASES_DIRECTORY="$SITE_ROOT/releases"
TARGET_DIRECTORY='{{ escape .ReleaseDirectory }}'
if [ ! -d "$RELEASES_DIRECTORY" ] || [ -L "$RELEASES_DIRECTORY" ] ||
   [ ! -d "$TARGET_DIRECTORY" ] || [ -L "$TARGET_DIRECTORY" ]; then
    echo 'Error: Rollback release is no longer a retained directory.' >&2
    exit 1
fi
TARGET_REAL=$(readlink -f -- "$TARGET_DIRECTORY")
if [ "$TARGET_REAL" != "$RELEASES_DIRECTORY/${TARGET_DIRECTORY##*/}" ]; then
    echo 'Error: Rollback release is outside the release directory.' >&2
    exit 1
fi
CURRENT_REAL=$(readlink -f -- '{{ escape .CurrentDirectory }}' 2>/dev/null || true)
if [ "$CURRENT_REAL" = "$TARGET_REAL" ]; then
    echo 'Error: This release is already live. Use runtime refresh to retry activation.' >&2
    exit 1
fi

{{ template "deployment/make_deployment_current.sh" . }}

echo 'LAUNCHCTL_RELEASE_SWITCH_COMPLETED'
{{ .RuntimeScript }}

echo "Rollback completed successfully!"
printf 'Current release is now: %s\n' '{{ escape .ReleaseDirectory }}'
