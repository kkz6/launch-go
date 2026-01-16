export PHP_BINARY={{ .PHPBinary }}

# Ensure PATH includes common binary locations
export PATH="/usr/local/bin:/usr/bin:/bin:$PATH"

# Find the PHP binary path
PHP_BIN_PATH=$(which {{ .PHPBinary }} 2>/dev/null)

# If not found, try common locations
if [ -z "$PHP_BIN_PATH" ]; then
    if [ -x "/usr/bin/{{ .PHPBinary }}" ]; then
        PHP_BIN_PATH="/usr/bin/{{ .PHPBinary }}"
    elif [ -x "/usr/local/bin/{{ .PHPBinary }}" ]; then
        PHP_BIN_PATH="/usr/local/bin/{{ .PHPBinary }}"
    fi
fi

# Create a temporary directory with a 'php' symlink pointing to the correct PHP version
# This ensures composer's @php directive uses the correct PHP binary
if [ -n "$PHP_BIN_PATH" ] && [ -x "$PHP_BIN_PATH" ]; then
    LAUNCH_PHP_BIN_DIR=$(mktemp -d)
    ln -sf "$PHP_BIN_PATH" "$LAUNCH_PHP_BIN_DIR/php"
    export PATH="$LAUNCH_PHP_BIN_DIR:$PATH"

    # Cleanup function to remove temp directory
    cleanup_php_symlink() {
        rm -rf "$LAUNCH_PHP_BIN_DIR"
    }
    trap cleanup_php_symlink EXIT
fi
