# A read-only descriptor on the installed binary supplies a shared per-version
# lock without creating writable lock files or granting new sudo permissions.
(
    set -e
    echo 'LAUNCHCTL_RUNTIME_REFRESH_STARTED {{ .ServiceName }}'
    if ! exec 8<'{{ .BinaryPath }}'; then
        echo 'LAUNCHCTL_RUNTIME_REFRESH_FAILED: selected PHP-FPM runtime is not installed or readable. The current code has not been reverted.' >&2
        exit 1
    fi
    if ! flock -x -w 30 8; then
        echo 'LAUNCHCTL_RUNTIME_REFRESH_FAILED: timed out waiting for the shared PHP-FPM runtime. Retry runtime refresh; the current code has not been reverted.' >&2
        exit 1
    fi
    # Close the lock descriptor in the service process so reloaded workers do
    # not inherit it. The parent holds the lock until the reload command exits.
    if ! timeout 30 sudo -n /usr/sbin/service {{ .ServiceName }} reload 8<&-; then
        echo 'LAUNCHCTL_RUNTIME_REFRESH_FAILED: PHP-FPM reload failed. Retry runtime refresh; the current code has not been reverted.' >&2
        exit 1
    fi
    echo 'LAUNCHCTL_RUNTIME_REFRESH_COMPLETED {{ .ServiceName }}'
)
