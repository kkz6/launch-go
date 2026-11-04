# Rename a prepared link on the same filesystem, so current is never absent.
(
    set -e
    RELEASE_DIRECTORY='{{ escape .ReleaseDirectory }}'
    CURRENT_DIRECTORY='{{ escape .CurrentDirectory }}'
    if [ ! -d "$RELEASE_DIRECTORY" ] || [ -L "$RELEASE_DIRECTORY" ]; then
        echo "Error: Target release directory does not exist or is a symlink: $RELEASE_DIRECTORY" >&2
        exit 1
    fi
    if [ -e "$CURRENT_DIRECTORY" ] && [ ! -L "$CURRENT_DIRECTORY" ]; then
        echo "Error: Current path is not a symlink: $CURRENT_DIRECTORY" >&2
        exit 1
    fi
    SWITCH_DIRECTORY=$(mktemp -d -- "${CURRENT_DIRECTORY}.switch.XXXXXX")
    trap 'rm -f -- "$SWITCH_DIRECTORY/current"; rmdir -- "$SWITCH_DIRECTORY"' EXIT
    ln -s -- "$RELEASE_DIRECTORY" "$SWITCH_DIRECTORY/current"
    mv -Tf -- "$SWITCH_DIRECTORY/current" "$CURRENT_DIRECTORY"
)
