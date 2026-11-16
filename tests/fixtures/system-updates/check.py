"""Run only inside the disposable systemd container created by run.sh."""
import base64
import datetime as dt
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import time


ROOT = Path("/workspace")
SOURCE = ROOT / "internal/modules/server/tasks/system_updates.py"
ENCODED = base64.b64encode(SOURCE.read_bytes()).decode()
STATE = Path("/var/lib/launchctl/system-updates")
REPO = Path("/var/tmp/launchctl-update-fixture")
CODENAME = dict(line.split("=", 1) for line in Path("/etc/os-release").read_text().splitlines() if "=" in line)["VERSION_CODENAME"].strip('"')


def run(*args, **kwargs):
    try:
        return subprocess.run(args, check=True, text=True, capture_output=True, **kwargs).stdout.strip()
    except subprocess.CalledProcessError as error:
        print(error.stdout, error.stderr, flush=True)
        raise


def helper(action, *args):
    output = run("python3", "-B", str(SOURCE), action, *args)
    return json.loads(output.splitlines()[-1]) if output else None


def configure(enabled, schedule, notifications=True):
    return helper("configure", str(enabled).lower(), schedule, str(notifications).lower(), ENCODED)


def package(name, version, pocket):
    build = REPO / "build" / f"{name}-{version}"
    (build / "DEBIAN").mkdir(parents=True, exist_ok=True)
    (build / "DEBIAN").chmod(0o755)
    (build / "DEBIAN/control").write_text(
        f"Package: {name}\nVersion: {version}\nArchitecture: all\n"
        "Maintainer: Launchctl CI <ci@example.test>\nDescription: Local update policy fixture\n"
    )
    directory = REPO / pocket
    directory.mkdir(parents=True, exist_ok=True)
    target = directory / f"{name}_{version}_all.deb"
    run("dpkg-deb", "--build", str(build), str(target))
    return target


def index(pocket):
    directory = REPO / pocket
    records = []
    for path in sorted(directory.glob("*.deb")):
        contents = path.read_bytes()
        records.append(run("dpkg-deb", "-f", str(path)) + "\n" +
                       f"Filename: ./{path.name}\nSize: {len(contents)}\n"
                       f"SHA256: {hashlib.sha256(contents).hexdigest()}\n")
    data = ("\n".join(records) + "\n").encode()
    (directory / "Packages").write_bytes(data)
    suite = CODENAME + ("-security" if pocket == "security" else "-updates")
    (directory / "Release").write_text(
        f"Origin: Ubuntu\nLabel: Ubuntu\nSuite: {suite}\nCodename: {suite}\n"
        "Architectures: amd64 arm64 all\nComponents: main\n"
        f"SHA256:\n {hashlib.sha256(data).hexdigest()} {len(data)} Packages\n"
    )


def wait_report(previous=None, timeout=100):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        status = helper("status")
        report = status["last_run"]
        if report and report["id"] != previous and not status["running"]:
            return status
        time.sleep(1)
    raise AssertionError("Timed out waiting for update report: " + run("journalctl", "--no-pager", "-n", "60"))


def installed(name):
    return run("dpkg-query", "-W", "-f=${Version}", name)


def main():
    os.umask(0o022)
    assert Path("/.dockerenv").is_file(), "This test mutates APT configuration and requires a disposable Docker container"
    assert Path("/run/systemd/system").is_dir(), "Real systemd is required"
    assert helper("status")["supported"]
    print(run("python3", "--version"), run("systemctl", "--version").splitlines()[0], run("dpkg", "--print-architecture"), flush=True)

    # Real APT/dpkg with deterministic local repositories. A security package
    # must upgrade; an ordinary -updates package must remain at its old version.
    for path in [Path("/etc/apt/sources.list"), *Path("/etc/apt/sources.list.d").glob("*")]:
        path.unlink(missing_ok=True)
    REPO.mkdir(mode=0o755, parents=True, exist_ok=True)
    for name, pocket in (("launchctl-ci-security", "security"), ("launchctl-ci-ordinary", "updates")):
        initial = package(name, "1.0", pocket)
        run("dpkg", "-i", str(initial))
        package(name, "2.0", pocket)
        index(pocket)
    sources = Path("/etc/apt/sources.list.d/launchctl-fixture.list")
    good_sources = "".join(f"deb [trusted=yes] file:{REPO}/{pocket} ./\n" for pocket in ("security", "updates"))
    sources.write_text(good_sources)
    # These inherited settings must not override the selected schedule or
    # enable ordinary/third-party upgrades and automatic restarts.
    Path("/etc/apt/apt.conf.d/99zzzz-launchctl-fixture").write_text(
        f'Unattended-Upgrade::Allowed-Origins {{ "Ubuntu:{CODENAME}-updates"; "ThirdParty:stable"; }};\n'
        'Unattended-Upgrade::Origins-Pattern { "origin=*"; };\n'
        'Unattended-Upgrade::Update-Days { "invalid-day"; };\n'
        'Unattended-Upgrade::Automatic-Reboot "true";\n'
        'APT::Update::Pre-Invoke { "sleep 3"; };\n'
    )
    schedule = (dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=1)).strftime("%H:%M")
    current = configure(True, schedule)
    assert current["timer_active"] and current["updater_installed"]
    run("systemd-analyze", "verify", "/etc/systemd/system/launchctl-system-updates.service", "/etc/systemd/system/launchctl-system-updates.timer")
    run("systemd-analyze", "calendar", f"*-*-* {schedule}:00 UTC")
    current = wait_report()
    assert current["last_run"]["status"] == "succeeded", current
    assert installed("launchctl-ci-security") == "2.0"
    assert installed("launchctl-ci-ordinary") == "1.0"
    assert any(p["name"].startswith("launchctl-ci-security") for p in current["last_run"]["packages"])
    policy = run("apt-config", "dump", env=dict(os.environ, APT_CONFIG=str(STATE / "apt.conf")))
    origins = "\n".join(line for line in policy.splitlines() if "Unattended-Upgrade::Allowed-Origins" in line)
    assert "-updates" not in origins and "ThirdParty" not in origins, origins
    assert 'Unattended-Upgrade::Automatic-Reboot "false"' in policy
    assert "Unattended-Upgrade::Update-Days::" not in policy
    assert "Unattended-Upgrade::Origins-Pattern::" not in policy
    print("Daily UTC timer and security-only APT upgrade passed", flush=True)

    run("systemctl", "stop", "launchctl-system-updates.timer")
    assert not helper("status")["timer_active"]
    repaired = helper("repair", ENCODED)
    assert repaired["timer_active"] and repaired["schedule"] == schedule
    disabled = configure(False, schedule, False)
    assert not disabled["timer_active"]
    package("launchctl-ci-security", "3.0", "security")
    index("security")
    previous = current["last_run"]["id"]
    requested = helper("retry", ENCODED)
    assert requested["running"] and not requested["enabled"]
    assert helper("retry", ENCODED) == {"error": "busy"}
    # Execute the actual reboot guard only: no reboot command is sent.
    reboot_source = (ROOT / "internal/modules/server/jobs/reboot_server.go").read_text()
    guard = re.search(r"const restartGuard = `(.*?)`", reboot_source, re.S).group(1)
    guarded = subprocess.run(("bash", "-c", guard + "printf allowed"), capture_output=True, text=True)
    assert guarded.returncode == 73 and "allowed" not in guarded.stdout
    current = wait_report(previous)
    assert current["last_run"]["status"] == "succeeded" and not current["last_run"]["notifications"]
    assert installed("launchctl-ci-security") == "3.0" and installed("launchctl-ci-ordinary") == "1.0"
    assert not current["enabled"] and current["schedule"] == schedule
    with (STATE / "lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        assert configure(True, schedule) == {"error": "busy"}
    assert subprocess.run(("bash", "-c", guard + "true"), capture_output=True).returncode == 0
    print("Repair, manual retry, settings preservation, and restart/update exclusion passed", flush=True)

    previous = current["last_run"]["id"]
    sources.write_text("deb [trusted=yes] file:/missing-launchctl-fixture ./\n")
    helper("retry", ENCODED)
    failed = wait_report(previous)
    assert failed["last_run"]["status"] == "failed"
    assert helper("logs")["logs"]
    sources.write_text(good_sources)
    helper("retry", ENCODED)
    recovered = wait_report(failed["last_run"]["id"])
    assert recovered["last_run"]["status"] == "succeeded"
    Path("/run/reboot-required").touch()
    pending = helper("monitor")
    assert pending["reboot_required"] and re.fullmatch(r"[a-f0-9-]{36}", pending["reboot_id"])
    Path("/run/reboot-required").unlink()
    assert not helper("monitor")["reboot_required"]
    print("Failed-run reporting, diagnostic logs, recovery, and reboot markers passed", flush=True)


if __name__ == "__main__":
    main()
