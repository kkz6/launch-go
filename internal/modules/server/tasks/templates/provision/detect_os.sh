#!/bin/bash
{{ shellDefaults }}

# Detect what's actually running on this box so downstream provision
# steps can branch on the truth instead of whatever the user picked in
# the dashboard dropdown. Emits a single ::LAUNCH::detected_os marker
# with five pipe-separated fields:
#
#   os_id | os_version | os_version_codename | arch | kernel
#
# Pipe is safe — none of these fields can contain it. Any field that
# can't be determined is left empty; the Go-side handler treats blanks
# as "still unknown" rather than failing.

OS_ID=""
OS_VERSION=""
OS_VERSION_CODENAME=""

if [ -r /etc/os-release ]; then
    # Subshell so the variables in /etc/os-release don't pollute the
    # outer script (it sets things like NAME, HOME_URL etc. that could
    # clobber our state).
    eval "$(
        . /etc/os-release
        printf 'OS_ID=%q\nOS_VERSION=%q\nOS_VERSION_CODENAME=%q\n' \
            "${ID:-}" "${VERSION_ID:-}" "${VERSION_CODENAME:-}"
    )"
fi

# uname -m → architecture as the kernel reports it (x86_64 / aarch64 /
# armv7l / …). We translate the kernel form to the Debian/Ubuntu apt
# form (amd64 / arm64 / armhf) so downstream consumers don't have to
# duplicate this mapping.
KERNEL_ARCH="$(uname -m 2>/dev/null || echo "")"
case "${KERNEL_ARCH}" in
    x86_64)  DETECTED_ARCH="amd64" ;;
    aarch64) DETECTED_ARCH="arm64" ;;
    armv7l)  DETECTED_ARCH="armhf" ;;
    *)       DETECTED_ARCH="${KERNEL_ARCH}" ;;
esac

DETECTED_KERNEL="$(uname -r 2>/dev/null || echo "")"

echo "Detected OS: id=${OS_ID} version=${OS_VERSION} codename=${OS_VERSION_CODENAME} arch=${DETECTED_ARCH} kernel=${DETECTED_KERNEL}"

# Marker the Go-side handler picks up. The handler parses this single
# line and writes the values onto the server row + broadcasts the
# update, so the UI sees the detected facts within a second of this
# step completing.
echo "::LAUNCH::detected_os::${OS_ID}|${OS_VERSION}|${OS_VERSION_CODENAME}|${DETECTED_ARCH}|${DETECTED_KERNEL}"
