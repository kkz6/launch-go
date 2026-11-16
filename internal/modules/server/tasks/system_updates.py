import base64
import datetime as dt
import fcntl
import gzip
import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

STATE = Path("/var/lib/launchctl/system-updates")
SCRIPT = Path("/usr/local/lib/launchctl/system-updates.py")
UNITS = Path("/etc/systemd/system")
APT_CONF = Path("/etc/apt/apt.conf.d/99zz-launchctl-system-updates")
TIMER = "launchctl-system-updates.timer"
SERVICE = "launchctl-system-updates.service"
MANUAL_SERVICE = "launchctl-system-updates-manual.service"
SETTINGS = STATE / "settings.json"
UTC = dt.timezone.utc

UNIT = """[Unit]
Description=Launchctl Ubuntu security updates
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/bin/python3 /usr/local/lib/launchctl/system-updates.py run
Environment=DEBIAN_FRONTEND=noninteractive
"""

# Read after the host APT snippets, for this invocation only.
SECURITY = """#clear Unattended-Upgrade::Allowed-Origins;
#clear Unattended-Upgrade::Origins-Pattern;
#clear Unattended-Upgrade::Update-Days;
Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}";
    "${distro_id}:${distro_codename}-security";
    "${distro_id}ESMApps:${distro_codename}-apps-security";
    "${distro_id}ESM:${distro_codename}-infra-security";
};
Unattended-Upgrade::Automatic-Reboot "false";
Unattended-Upgrade::InstallOnShutdown "false";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "false";
Unattended-Upgrade::Remove-Unused-Dependencies "false";
Unattended-Upgrade::Remove-New-Unused-Dependencies "false";
Unattended-Upgrade::Mail "";
"""


def command(*args, **kwargs):
    return subprocess.run(args, check=True, text=True, **kwargs)


def active(unit):
    return subprocess.run(
        ("systemctl", "is-active", "--quiet", unit), check=False
    ).returncode == 0


def write(path, contents, mode=0o600, durable=False):
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, delete=False) as file:
        file.write(contents)
        os.fchmod(file.fileno(), mode)
        if durable:
            file.flush()
            os.fsync(file.fileno())
    os.replace(file.name, path)
    if durable:
        directory = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)


def settings():
    if SETTINGS.is_file():
        return json.loads(SETTINGS.read_text())
    return {"enabled": False, "schedule": "03:00", "notifications": False}


def configure(enabled, schedule, notifications, source):
    if not re.fullmatch(r"(?:[01][0-9]|2[0-3]):[0-5][0-9]", schedule):
        raise ValueError("Invalid UTC maintenance time")
    if enabled and not Path("/usr/bin/unattended-upgrade").is_file():
        print(json.dumps({"error": "unattended_upgrades_missing"}))
        return False

    timer = UNITS / TIMER
    service = UNITS / SERVICE
    paths = (APT_CONF, SCRIPT, timer, service, SETTINGS)
    previous = {path: path.read_text() if path.is_file() else None for path in paths}
    was_enabled = settings()["enabled"]
    owns_policy = APT_CONF.is_file()
    modes = {APT_CONF: 0o644, service: 0o644, timer: 0o644}
    try:
        write(SCRIPT, base64.b64decode(source).decode())
        write(service, UNIT, modes[service])
        write(timer, f"""[Unit]
Description=Schedule Launchctl Ubuntu security updates

[Timer]
OnCalendar=*-*-* {schedule}:00 UTC
AccuracySec=1s
Persistent=false

[Install]
WantedBy=timers.target
""", modes[timer])
        # Turn off the distro's automatic installations while Launchctl owns
        # the policy. Its regular package-list refresh can continue.
        if enabled or owns_policy:
            write(APT_CONF, 'APT::Periodic::Unattended-Upgrade "0";\n'
                  'Unattended-Upgrade::InstallOnShutdown "false";\n', modes[APT_CONF])
        write(SETTINGS, json.dumps({
            "enabled": enabled, "schedule": schedule, "notifications": notifications,
        }))
        command("systemctl", "daemon-reload", stdout=sys.stderr)
        if enabled:
            command("systemctl", "enable", TIMER, stdout=sys.stderr)
            command("systemctl", "restart", TIMER, stdout=sys.stderr)
        elif active(TIMER) or was_enabled:
            command("systemctl", "disable", "--now", TIMER, stdout=sys.stderr)
    except (OSError, subprocess.CalledProcessError):
        if not was_enabled:
            subprocess.run(("systemctl", "disable", "--now", TIMER), stdout=sys.stderr)
        for path, contents in previous.items():
            if contents is None:
                path.unlink(missing_ok=True)
            else:
                write(path, contents, modes.get(path, 0o600))
        subprocess.run(("systemctl", "daemon-reload"), stdout=sys.stderr)
        if was_enabled:
            subprocess.run(("systemctl", "enable", TIMER), stdout=sys.stderr)
            subprocess.run(("systemctl", "restart", TIMER), stdout=sys.stderr)
        raise
    return True


def service_running():
    return any(subprocess.run(
        ("systemctl", "show", unit, "-p", "ActiveState", "--value"),
        capture_output=True, text=True, check=False,
    ).stdout.strip() in ("active", "activating", "deactivating")
               for unit in (SERVICE, MANUAL_SERVICE))


def prepare_retry(source):
    if not Path("/usr/bin/unattended-upgrade").is_file():
        print(json.dumps({"error": "unattended_upgrades_missing"}))
        return False
    if service_running():
        print(json.dumps({"error": "busy"}))
        return False
    write(SCRIPT, base64.b64decode(source).decode())
    write(UNITS / MANUAL_SERVICE, UNIT.replace(".py run\n", ".py run-manual\n"), 0o644)
    # A manual run can be requested with the recurring schedule disabled.
    # Keep the user's saved schedule and notification preference unchanged.
    if not SETTINGS.is_file():
        write(SETTINGS, json.dumps(settings()))
    command("systemctl", "daemon-reload", stdout=sys.stderr)
    return True


def logs():
    result = command("journalctl", "--no-pager", "--output=short-iso", "-n", "200",
                     "-u", SERVICE, "-u", MANUAL_SERVICE, capture_output=True)
    return {"supported": True, "logs": result.stdout[-65536:]}


def installed(package):
    result = subprocess.run(
        ("dpkg-query", "-W", "-f=${db:Status-Status} ${Version}", package["name"]),
        capture_output=True, text=True,
    )
    return result.stdout == f'installed {package["version"]}'


def history(path):
    if not path.is_file():
        return []
    opener = gzip.open if path.suffix == ".gz" else open
    try:
        with opener(path, "rt", errors="replace") as file:
            return file.read().split("\n\n")
    except (EOFError, FileNotFoundError):
        return []


def upgrades(transaction):
    for line in transaction.splitlines():
        if line.startswith("Upgrade: "):
            for name, version in re.findall(
                r"([a-z0-9][a-z0-9+.:~-]*) \([^,()]+, ([^\s()]+)\)", line
            ):
                yield {"name": name, "version": version}


def finish_run(run, outcome):
    run_id = run["id"]
    report_path = STATE / f"run-{run_id}.json"
    history_path = STATE / f"apt-{run_id}.log"
    # A process may stop after persisting the report but before removing its
    # marker. Preserve the completed result when recovering that marker.
    if not report_path.is_file():
        packages = []
        for transaction in history(history_path):
            if outcome == "failed" or "\nEnd-Date:" in transaction:
                packages.extend(upgrades(transaction))
        if outcome == "failed":
            packages = [package for package in packages if installed(package)]
        write(report_path, json.dumps({
            "id": run_id,
            "finished_at": dt.datetime.now(UTC).isoformat(timespec="seconds"),
            "status": outcome,
            "packages": packages,
            "reboot_required": Path("/run/reboot-required").exists(),
            "notifications": run["notifications"],
        }), durable=True)
    (STATE / "active-run.json").unlink(missing_ok=True)
    history_path.unlink(missing_ok=True)


def recover_run():
    # The caller holds STATE/lock, so no live run can own this marker.
    try:
        run = json.loads((STATE / "active-run.json").read_text())
    except FileNotFoundError:
        return
    finish_run(run, "failed")


def recover_interrupted_run():
    if not (STATE / "active-run.json").is_file():
        return
    with (STATE / "lock").open("a") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return
        recover_run()


def run_updates():
    recover_run()
    started = dt.datetime.now(UTC)
    run = {
        "id": started.strftime("%Y%m%dT%H%M%S.%fZ"),
        "notifications": settings()["notifications"],
    }
    history_path = STATE / f'apt-{run["id"]}.log'
    bootstrap = STATE / "apt.conf"
    policy = STATE / "security.conf"
    outcome = "failed"
    write(STATE / "active-run.json", json.dumps(run), durable=True)
    try:
        write(bootstrap, f'Dir::Etc::main "{policy}";\n')
        host_main = '#include "/etc/apt/apt.conf";\n' if Path("/etc/apt/apt.conf").is_file() else ""
        write(policy, host_main + SECURITY + f'Dir::Log::History "{history_path}";\n')
        command("apt-get", "-o", "APT::Update::Error-Mode=any", "update")
        # APT loads its main file after apt.conf.d. Keep the security policy
        # local to this process, including during concurrent manual APT use.
        command("unattended-upgrade", env=dict(os.environ, APT_CONFIG=str(bootstrap)))
        outcome = "succeeded"
    finally:
        finish_run(run, outcome)
        cutoff = started.timestamp() - 35 * 86400
        for path in STATE.glob("run-*.json"):
            if path.stat().st_mtime < cutoff:
                path.unlink()


def recent_updates(reports):
    batches = [
        (dt.datetime.fromisoformat(report["finished_at"]).timestamp(), report["packages"])
        for report in reports
    ]
    # APT writes timestamp fields in the server's local time.
    for path in Path("/var/log/apt").glob("history.log*"):
        for transaction in history(path):
            ended = re.search(r"^End-Date: +(.+)$", transaction, re.MULTILINE)
            if ended:
                try:
                    when = dt.datetime.strptime(ended[1], "%Y-%m-%d %H:%M:%S").timestamp()
                except ValueError:
                    continue
                packages = list(upgrades(transaction))
                if packages:
                    batches.append((when, packages))
    recent = {}
    for _, packages in sorted(batches, key=lambda batch: batch[0], reverse=True):
        for package in packages:
            recent.setdefault(package["name"], package)
            if len(recent) == 10:
                return list(recent.values())
    return list(recent.values())


def status(monitor=False):
    running = service_running()
    if not running:
        recover_interrupted_run()
    current = settings()
    reports = []
    for path in sorted(STATE.glob("run-*.json")):
        try:
            reports.append(json.loads(path.read_text()))
        except FileNotFoundError:
            continue
    reboot_id = ""
    if Path("/run/reboot-required").exists():
        reboot_id = Path("/proc/sys/kernel/random/boot_id").read_text().strip()
    month = dt.datetime.now(UTC).strftime("%Y-%m")
    result = {
        **current,
        "supported": True,
        "managed": SETTINGS.is_file(),
        "updater_installed": Path("/usr/bin/unattended-upgrade").is_file(),
        "reboot_required": bool(reboot_id),
        "reboot_id": reboot_id,
        "timer_active": current["enabled"] and active(TIMER),
        "running": running,
        "native_enabled": not current["enabled"] and active("apt-daily-upgrade.timer")
        and bool(re.search(r'APT::Periodic::Unattended-Upgrade "[1-9][0-9]*";',
                           command("apt-config", "dump", capture_output=True).stdout)),
        "security_patches_this_month": sum(len(r["packages"]) for r in reports
                                          if r["finished_at"].startswith(month)),
        "last_run": reports[-1] if reports else None,
    }
    if monitor:
        result["reports"] = reports
    else:
        result["recently_updated"] = recent_updates(reports)
    return result


def main():
    os_release = Path("/etc/os-release")
    if (not os_release.is_file()
            or not re.search(r'^ID=(?:"ubuntu"|ubuntu)$', os_release.read_text(), re.MULTILINE)
            or not re.search(r'^VERSION_ID=(?:"(?:22\.04|24\.04)"|(?:22\.04|24\.04))$',
                             os_release.read_text(), re.MULTILINE)
            or not Path("/run/systemd/system").is_dir()):
        print(json.dumps({"supported": False}))
        return
    action = sys.argv[1]
    if action in ("status", "monitor"):
        print(json.dumps(status(monitor=action == "monitor")))
        return
    if action == "logs":
        print(json.dumps(logs()))
        return
    STATE.mkdir(mode=0o700, parents=True, exist_ok=True)
    with (STATE / "lock").open("w") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            print(json.dumps({"error": "busy"}))
            if action in ("run", "run-manual"):
                sys.exit(1)
            return
        if action == "configure" and configure(sys.argv[2] == "true", sys.argv[3], sys.argv[4] == "true", sys.argv[5]):
            print(json.dumps(status()))
        elif action == "repair":
            current = settings()
            if not SETTINGS.is_file() or not current["enabled"]:
                print(json.dumps({"error": "schedule_disabled"}))
            elif service_running():
                print(json.dumps({"error": "busy"}))
            elif configure(current["enabled"], current["schedule"], current["notifications"], sys.argv[2]):
                print(json.dumps(status()))
        elif action == "retry":
            if not prepare_retry(sys.argv[2]):
                return
        elif action == "run-manual" or (action == "run" and settings()["enabled"]):
            run_updates()
    if action == "retry":
        # Release the shared lock before the service starts and acquires it.
        # systemd coalesces duplicate starts of this single manual-run unit.
        command("systemctl", "start", "--no-block", MANUAL_SERVICE, stdout=sys.stderr)
        print(json.dumps(status()))


if __name__ == "__main__":
    main()
