import base64
import contextlib
import datetime as dt
import importlib.util
import io
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import unittest
from unittest.mock import patch


SOURCE = Path(__file__).with_name("system_updates.py")


class SystemUpdatesTest(unittest.TestCase):
    def setUp(self):
        spec = importlib.util.spec_from_file_location("system_updates", SOURCE)
        self.updates = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.updates)
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name)
        self.updates.Path = self.host_path
        for name in ("STATE", "SCRIPT", "UNITS", "APT_CONF", "SETTINGS"):
            setattr(self.updates, name, self.host_path(getattr(self.updates, name)))
        self.source = base64.b64encode(SOURCE.read_bytes())
        self.updates.write(self.host_path("/usr/bin/unattended-upgrade"), "")
        self.units = set()
        self.running = False
        self.fail_timer_once = False
        self.fail_upgrade = False
        self.fail_refresh = False
        self.native_policy = ""
        self.commands = []
        self.journal = ""
        mocked = patch.object(self.updates.subprocess, "run", side_effect=self.run_command)
        mocked.start()
        self.addCleanup(mocked.stop)

    def host_path(self, value):
        return self.root / str(value).lstrip("/")

    def run_command(self, args, **kwargs):
        self.commands.append(args)
        output = ""
        if args[0] == "systemctl":
            verb = args[1]
            if verb == "is-active":
                return subprocess.CompletedProcess(args, 0 if args[-1] in self.units else 3)
            if verb == "show":
                output = "activating" if self.running else "inactive"
            elif verb == "restart":
                if self.fail_timer_once:
                    self.fail_timer_once = False
                    raise subprocess.CalledProcessError(1, args)
                self.units.add(args[-1])
            elif verb == "disable":
                self.units.discard(args[-1])
            elif verb == "start":
                self.running = True
        elif args[0] == "journalctl":
            output = self.journal
        elif args[0] == "apt-config":
            output = self.native_policy
        elif args[0] == "apt-get" and self.fail_refresh:
            raise subprocess.CalledProcessError(100, args)
        elif args[0] == "unattended-upgrade":
            marker = json.loads((self.updates.STATE / "active-run.json").read_text())
            self.assertEqual(self.updates.settings()["notifications"], marker["notifications"])
            self.assertEqual(str(self.updates.STATE / "apt.conf"), kwargs["env"]["APT_CONFIG"])
            policy = (self.updates.STATE / "security.conf").read_text()
            self.assertIn('"${distro_id}:${distro_codename}";', policy)
            self.assertIn('"${distro_id}:${distro_codename}-security";', policy)
            self.assertNotIn('${distro_codename}-updates', policy)
            self.assertNotIn('${distro_codename}-backports', policy)
            self.assertIn('#clear Unattended-Upgrade::Update-Days;', policy)
            self.assertIn('Unattended-Upgrade::Automatic-Reboot "false";', policy)
            history = Path(policy.split('Dir::Log::History "')[1].split('"')[0])
            history.write_text("Start-Date: 2026-11-15  03:00:00\n"
                               "Upgrade: openssl:arm64 (1, 2), apt:arm64 (1, 5)\n"
                               "End-Date: 2026-11-15  03:01:00\n\n")
            if self.fail_upgrade:
                raise subprocess.CalledProcessError(1, args)
        elif args[0] == "dpkg-query":
            output = "installed 2" if args[-1] == "openssl:arm64" else "unpacked 5"
        return subprocess.CompletedProcess(args, 0, stdout=output)

    def configure(self, enabled=True, schedule="04:30", notifications=True):
        return self.updates.configure(enabled, schedule, notifications, self.source)

    def invoke(self, action, *args):
        self.updates.write(self.host_path("/etc/os-release"), 'ID="ubuntu"\nVERSION_ID="22.04"\n')
        self.host_path("/run/systemd/system").mkdir(parents=True, exist_ok=True)
        output = io.StringIO()
        with patch.object(self.updates.sys, "argv", ["system-updates.py", action, *args]), contextlib.redirect_stdout(output):
            self.updates.main()
        return json.loads(output.getvalue()) if output.getvalue() else None

    def test_retry_starts_manual_service_without_changing_disabled_schedule(self):
        self.configure(enabled=False, schedule="09:15", notifications=False)
        saved = self.updates.SETTINGS.read_text()
        result = self.invoke("retry", self.source)
        self.assertTrue(result["running"])
        self.assertEqual(saved, self.updates.SETTINGS.read_text())
        self.assertIn(("systemctl", "start", "--no-block", self.updates.MANUAL_SERVICE), self.commands)
        self.assertFalse(any(command[0] in ("apt-get", "unattended-upgrade") for command in self.commands))
        self.assertIn(".py run-manual", (self.updates.UNITS / self.updates.MANUAL_SERVICE).read_text())
        self.invoke("run-manual")
        self.running = False
        self.assertEqual("succeeded", self.updates.status()["last_run"]["status"])
        self.assertFalse(self.updates.settings()["enabled"])

    def test_only_supported_ubuntu_releases_enable_host_actions(self):
        self.host_path("/run/systemd/system").mkdir(parents=True)
        for release, supported in (("22.04", True), ("24.04", True), ("20.04", False), ("26.04", False), ("", False)):
            with self.subTest(release=release):
                self.updates.write(self.host_path("/etc/os-release"), f'ID=ubuntu\nVERSION_ID="{release}"\n')
                output = io.StringIO()
                with patch.object(self.updates.sys, "argv", ["system-updates.py", "status"]), contextlib.redirect_stdout(output):
                    self.updates.main()
                self.assertEqual(supported, json.loads(output.getvalue())["supported"])
        self.updates.write(self.host_path("/etc/os-release"), 'ID=debian\nVERSION_ID="24.04"\n')
        output = io.StringIO()
        with patch.object(self.updates.sys, "argv", ["system-updates.py", "configure"]), contextlib.redirect_stdout(output):
            self.updates.main()
        self.assertEqual({"supported": False}, json.loads(output.getvalue()))
        self.assertFalse(self.updates.SETTINGS.exists())

    def test_retry_does_not_start_a_second_run_or_ignore_missing_updater(self):
        self.configure()
        self.running = True
        self.assertEqual({"error": "busy"}, self.invoke("retry", self.source))
        self.assertFalse(any(command[:2] == ("systemctl", "start") for command in self.commands))
        self.running = False
        self.host_path("/usr/bin/unattended-upgrade").unlink()
        self.assertFalse(self.updates.status()["updater_installed"])
        self.assertEqual({"error": "unattended_upgrades_missing"}, self.invoke("retry", self.source))

    def test_schedule_repair_restores_timer_using_saved_settings(self):
        self.configure(schedule="07:45", notifications=False)
        self.units.discard(self.updates.TIMER)
        result = self.invoke("repair", self.source)
        self.assertTrue(result["timer_active"])
        self.assertEqual("07:45", result["schedule"])
        self.assertFalse(result["notifications"])
        self.configure(enabled=False)
        self.assertEqual({"error": "schedule_disabled"}, self.invoke("repair", self.source))

    def test_update_logs_are_bounded_and_include_both_units(self):
        self.journal = "older output" + "x" * 65536
        result = self.invoke("logs")
        self.assertEqual("x" * 65536, result["logs"])
        command = self.commands[-1]
        self.assertIn("200", command)
        self.assertIn(self.updates.SERVICE, command)
        self.assertIn(self.updates.MANUAL_SERVICE, command)

    def test_notification_only_configuration_preserves_native_updates(self):
        self.native_policy = 'APT::Periodic::Unattended-Upgrade "1";'
        self.units.add("apt-daily-upgrade.timer")
        self.assertTrue(self.configure(enabled=False))
        self.assertFalse(self.updates.APT_CONF.exists())
        self.assertTrue(self.updates.status()["native_enabled"])

    def test_schedule_enable_and_disable(self):
        self.assertTrue(self.configure())
        timer = (self.updates.UNITS / self.updates.TIMER).read_text()
        self.assertIn("OnCalendar=*-*-* 04:30:00 UTC", timer)
        self.assertIn("Persistent=false", timer)
        self.assertTrue(self.updates.status()["timer_active"])
        self.assertIn('Unattended-Upgrade "0";', self.updates.APT_CONF.read_text())
        self.assertTrue(self.configure(enabled=False, notifications=False))
        status = self.updates.status()
        self.assertFalse(status["enabled"])
        self.assertFalse(status["timer_active"])
        self.assertFalse(status["notifications"])
        self.assertTrue(self.updates.APT_CONF.exists())

    def test_configuration_rolls_back_after_timer_failure(self):
        for was_enabled in (False, True):
            with self.subTest(was_enabled=was_enabled):
                self.configure(enabled=was_enabled)
                previous = self.updates.SETTINGS.read_text()
                previous_timer = (self.updates.UNITS / self.updates.TIMER).read_text()
                self.fail_timer_once = True
                with self.assertRaises(subprocess.CalledProcessError):
                    self.configure(schedule="09:00")
                self.assertEqual(previous, self.updates.SETTINGS.read_text())
                self.assertEqual(previous_timer, (self.updates.UNITS / self.updates.TIMER).read_text())
                self.assertEqual(was_enabled, self.updates.status()["timer_active"])
                if not was_enabled:
                    self.assertFalse(self.updates.APT_CONF.exists())

    def test_invalid_time_and_missing_updater_do_not_install_configuration(self):
        for schedule in ("3:00", "24:00", "12:60", "03:00; touch /tmp/invalid"):
            with self.subTest(schedule=schedule), self.assertRaises(ValueError):
                self.configure(schedule=schedule)
        self.host_path("/usr/bin/unattended-upgrade").unlink()
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            self.assertFalse(self.configure())
        self.assertEqual({"error": "unattended_upgrades_missing"}, json.loads(output.getvalue()))
        self.assertFalse(self.updates.SETTINGS.exists())

    def test_monitor_includes_current_state_without_scanning_apt_history(self):
        self.configure()
        self.running = True
        self.updates.write(self.host_path("/run/reboot-required"), "")
        self.updates.write(self.host_path("/proc/sys/kernel/random/boot_id"), "boot-a\n")
        with patch.object(self.updates, "recent_updates", side_effect=AssertionError("history scan")):
            status = self.updates.status(monitor=True)
            self.assertTrue(status["running"])
            self.assertTrue(status["timer_active"])
            self.assertEqual("04:30", status["schedule"])
            self.assertEqual("boot-a", status["reboot_id"])
            self.assertEqual([], status["reports"])
            self.assertNotIn("recently_updated", status)
            self.running = False
            self.units.remove(self.updates.TIMER)
            changed = self.updates.status(monitor=True)
            self.assertFalse(changed["running"])
            self.assertFalse(changed["timer_active"])

    def test_success_report_and_recent_manual_upgrades(self):
        self.configure()
        self.updates.run_updates()
        status = self.updates.status()
        self.assertEqual("succeeded", status["last_run"]["status"])
        self.assertEqual(2, status["security_patches_this_month"])
        self.assertEqual(2, len(status["last_run"]["packages"]))
        self.assertTrue(status["last_run"]["notifications"])
        self.assertFalse((self.updates.STATE / "active-run.json").exists())
        self.assertEqual(status["last_run"], self.updates.status(monitor=True)["reports"][-1])
        future = (dt.datetime.now() + dt.timedelta(days=1)).strftime("%Y-%m-%d  %H:%M:%S")
        self.updates.write(self.host_path("/var/log/apt/history.log"),
                           f"Start-Date: {future}\nUpgrade: openssl:arm64 (2, 3)\nEnd-Date: {future}\n")
        self.assertEqual({"name": "openssl:arm64", "version": "3"},
                         self.updates.status()["recently_updated"][0])

    def test_failed_upgrade_reports_only_fully_installed_packages(self):
        self.configure()
        self.fail_upgrade = True
        with self.assertRaises(subprocess.CalledProcessError):
            self.updates.run_updates()
        report = self.updates.status()["last_run"]
        self.assertEqual("failed", report["status"])
        self.assertEqual([{"name": "openssl:arm64", "version": "2"}], report["packages"])

    def test_report_remembers_disabled_notifications(self):
        self.configure(notifications=False)
        self.updates.run_updates()
        self.configure(notifications=True)
        self.assertTrue(self.updates.status()["notifications"])
        self.assertFalse(self.updates.status()["last_run"]["notifications"])

    def test_failed_package_refresh_records_failure_without_installing(self):
        self.configure()
        self.fail_refresh = True
        with self.assertRaises(subprocess.CalledProcessError):
            self.updates.run_updates()
        report = self.updates.status()["last_run"]
        self.assertEqual("failed", report["status"])
        self.assertEqual([], report["packages"])
        self.assertFalse(any(command[0] == "unattended-upgrade" for command in self.commands))

    def test_concurrent_configuration_returns_busy(self):
        self.configure()
        self.updates.write(self.host_path("/etc/os-release"), 'ID="ubuntu"\nVERSION_ID="24.04"\n')
        self.host_path("/run/systemd/system").mkdir(parents=True)
        output = io.StringIO()
        with (self.updates.STATE / "lock").open("w") as lock:
            self.updates.fcntl.flock(lock, self.updates.fcntl.LOCK_EX | self.updates.fcntl.LOCK_NB)
            with patch.object(self.updates.sys, "argv", ["system-updates.py", "configure"]), contextlib.redirect_stdout(output):
                self.updates.main()
        self.assertEqual({"error": "busy"}, json.loads(output.getvalue()))
        self.assertTrue(self.updates.settings()["enabled"])

    def interrupted_run(self):
        self.configure()
        started = dt.datetime.now(dt.timezone.utc) - dt.timedelta(minutes=1)
        run = {"id": started.strftime("%Y%m%dT%H%M%S.%fZ"), "notifications": False}
        self.updates.write(self.updates.STATE / "active-run.json", json.dumps(run))
        self.updates.write(self.updates.STATE / f'apt-{run["id"]}.log',
                           "Start-Date: 2026-11-15  03:00:00\n"
                           "Upgrade: openssl:arm64 (1, 2), apt:arm64 (1, 5)\n")
        return run

    def test_interrupted_run_is_recovered_once_without_installing(self):
        run = self.interrupted_run()
        report = self.updates.status()["last_run"]
        self.assertEqual(run["id"], report["id"])
        self.assertEqual("failed", report["status"])
        self.assertFalse(report["notifications"])
        self.assertEqual([{"name": "openssl:arm64", "version": "2"}], report["packages"])
        self.assertEqual([report], self.updates.status(monitor=True)["reports"])
        self.assertFalse((self.updates.STATE / "active-run.json").exists())
        self.assertFalse(any(command[0] in ("apt-get", "unattended-upgrade") for command in self.commands))

    def test_live_run_is_not_recovered_while_its_lock_is_held(self):
        self.interrupted_run()
        with (self.updates.STATE / "lock").open("a") as lock:
            self.updates.fcntl.flock(lock, self.updates.fcntl.LOCK_EX | self.updates.fcntl.LOCK_NB)
            self.assertEqual([], self.updates.status(monitor=True)["reports"])
            self.assertTrue((self.updates.STATE / "active-run.json").exists())
        self.assertEqual(1, len(self.updates.status(monitor=True)["reports"]))

    def test_existing_report_is_preserved_when_marker_cleanup_was_interrupted(self):
        run = self.interrupted_run()
        report = dict(run, status="succeeded", finished_at="2026-11-15T03:01:00+00:00",
                      packages=[], reboot_required=False)
        self.updates.write(self.updates.STATE / f'run-{run["id"]}.json', json.dumps(report))
        self.assertEqual([report], self.updates.status(monitor=True)["reports"])
        self.assertFalse((self.updates.STATE / "active-run.json").exists())
        self.assertFalse((self.updates.STATE / f'apt-{run["id"]}.log').exists())

    def test_new_run_recovers_previous_interruption_before_starting(self):
        run = self.interrupted_run()
        self.updates.run_updates()
        reports = self.updates.status(monitor=True)["reports"]
        self.assertEqual(2, len(reports))
        self.assertEqual(run["id"], reports[0]["id"])
        self.assertEqual("failed", reports[0]["status"])
        self.assertEqual("succeeded", reports[1]["status"])
        self.assertEqual(1, sum(command[0] == "unattended-upgrade" for command in self.commands))

    def test_killed_process_leaves_a_recoverable_run(self):
        self.configure()
        pid = os.fork()
        if pid == 0:
            try:
                def interrupt(args, **kwargs):
                    if args[0] == "apt-get":
                        os.kill(os.getpid(), signal.SIGKILL)
                    return self.run_command(args, **kwargs)

                self.updates.subprocess.run.side_effect = interrupt
                with (self.updates.STATE / "lock").open("a") as lock:
                    self.updates.fcntl.flock(lock, self.updates.fcntl.LOCK_EX)
                    self.updates.run_updates()
            finally:
                os._exit(1)
        _, exit_status = os.waitpid(pid, 0)
        self.assertTrue(os.WIFSIGNALED(exit_status))
        self.assertEqual(signal.SIGKILL, os.WTERMSIG(exit_status))
        self.assertTrue((self.updates.STATE / "active-run.json").exists())
        report = self.updates.status(monitor=True)["last_run"]
        self.assertEqual("failed", report["status"])
        self.assertEqual([], report["packages"])
        self.assertFalse((self.updates.STATE / "active-run.json").exists())


if __name__ == "__main__":
    unittest.main()
