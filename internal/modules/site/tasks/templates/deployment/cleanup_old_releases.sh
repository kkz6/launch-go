# Keep the newest configured releases, plus the active and previous serving
# releases. A retention of one must still leave a usable rollback target.
RELEASES_DIRECTORY=$(cd -- '{{ escape .ReleasesDirectory }}' && pwd -P)
CURRENT_RELEASE=$(readlink -f -- '{{ escape .CurrentDirectory }}')
CANDIDATE_RELEASE="$RELEASES_DIRECTORY/"'{{ escape .LatestDeploymentTimestamp }}'
RETENTION_COUNT={{ .RetentionCount }}
if [ "$RETENTION_COUNT" -lt 1 ]; then
    RETENTION_COUNT=10
fi

RELEASE_INDEX=0
while IFS= read -r RELEASE_NAME; do
    RELEASE_PATH="$RELEASES_DIRECTORY/$RELEASE_NAME"
    # Only prune release directories we created; never follow symlinks.
    [ -d "$RELEASE_PATH" ] && [ ! -L "$RELEASE_PATH" ] || continue
    [[ "$RELEASE_NAME" =~ ^[0-9]{14}(-[0-9A-HJKMNP-TV-Za-hjkmnp-tv-z]{26})?$ ]] || continue
    RELEASE_INDEX=$((RELEASE_INDEX + 1))
    if [ "$RELEASE_INDEX" -le "$RETENTION_COUNT" ] ||
       [ "$RELEASE_PATH" = "$CURRENT_RELEASE" ] ||
       [ "$RELEASE_PATH" = "${PREVIOUS_RELEASE:-}" ] ||
       [ "$RELEASE_PATH" = "$CANDIDATE_RELEASE" ]; then
        continue
    fi
    # A hook or external operation may have switched current. Stop rather
    # than prune using a serving-release snapshot that is no longer valid.
    if [ "$(readlink -f -- '{{ escape .CurrentDirectory }}')" != "$CURRENT_RELEASE" ]; then
        break
    fi
    rm -rf -- "$RELEASE_PATH"
done < <(find "$RELEASES_DIRECTORY" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | LC_ALL=C sort -r)
