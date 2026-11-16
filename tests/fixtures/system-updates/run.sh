#!/usr/bin/env bash
set -euo pipefail

version="${1:?Ubuntu version is required (22.04 or 24.04)}"
platform="${2:-linux/$(docker version --format '{{.Server.Arch}}')}"
case "$version" in 22.04|24.04) ;; *) echo "Unsupported test version: $version" >&2; exit 1;; esac
root="$(cd "$(dirname "$0")/../../.." && pwd)"
name="launchctl-system-updates-${version//./-}-${platform##*/}-$$"
image="launchctl-system-updates-test:${version}-${platform##*/}"

cleanup() {
  code=$?
  if [ "$code" -ne 0 ]; then docker logs "$name" 2>/dev/null || true; fi
  docker rm -f "$name" >/dev/null 2>&1 || true
  exit "$code"
}
trap cleanup EXIT

docker build --platform "$platform" --build-arg "UBUNTU_VERSION=$version" -t "$image" "$root/tests/fixtures/system-updates"
docker run -d --name "$name" --platform "$platform" --privileged --cgroupns=private \
  --tmpfs /run --tmpfs /run/lock --tmpfs /tmp -v "$root:/workspace:ro" "$image" >/dev/null
for attempt in $(seq 1 60); do
  if docker exec "$name" test -d /run/systemd/system; then break; fi
  sleep 1
done
docker exec "$name" python3 -B /workspace/internal/modules/server/tasks/system_updates_test.py -v
docker exec "$name" python3 -B /workspace/tests/fixtures/system-updates/check.py
