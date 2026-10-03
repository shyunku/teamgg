import http.server
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import threading
import types
import unittest
from datetime import datetime
from unittest.mock import patch
from zoneinfo import ZoneInfo


if sys.platform == "win32":
    sys.modules["fcntl"] = types.SimpleNamespace(LOCK_EX=1, LOCK_NB=2, flock=lambda *_: None)

spec = importlib.util.spec_from_file_location(
    "retention_scheduler", Path(__file__).with_name("retention_scheduler.py")
)
scheduler = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scheduler)


class RetentionSchedulerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.state_patch = patch.object(scheduler, "STATE_DIR", Path(self.temp.name))
        self.state_patch.start()
        self.addCleanup(self.state_patch.stop)
        self.now = datetime(2026, 9, 24, 2, 0, tzinfo=ZoneInfo("Asia/Seoul"))
        self.config = {
            "RETENTION_ENABLED": "true",
            "RETENTION_DELETE_ENABLED": "true",
            "RETENTION_WINDOW_START": "02:00",
            "RETENTION_WINDOW_END": "04:00",
        }

    def test_disabled_does_not_touch_docker(self):
        with patch.object(scheduler, "run") as run:
            scheduler.execute({}, self.now)
            run.assert_not_called()

    def test_preview_only_never_stops_or_records_attempt(self):
        config = dict(self.config, RETENTION_DELETE_ENABLED="false", RETENTION_PREVIEW_ONLY="true")
        with patch.object(scheduler, "run", return_value=types.SimpleNamespace(
                 returncode=0, stdout="container-id")) as run, \
             patch.object(scheduler, "check_disk"), \
             patch.object(scheduler, "backend_health"), \
             patch.object(scheduler, "retention_run", return_value={
                 "eligibleMatches": 4, "retainedPatches": ["16.18"], "expiredVersions": ["16.10"]
             }) as retention:
            scheduler.execute(config, self.now)
            retention.assert_called_once_with(config, dry_run=True)
            self.assertNotIn(scheduler.compose("stop", "backend"),
                             [call.args[0] for call in run.call_args_list])
            self.assertFalse((scheduler.STATE_DIR / "state.json").exists())

    def test_restore_marker_recovers_backend(self):
        scheduler.STATE_DIR.mkdir(exist_ok=True)
        marker = scheduler.STATE_DIR / "restore-needed"
        marker.touch()
        with patch.object(scheduler, "run", return_value=types.SimpleNamespace(
                 returncode=0, stdout="")) as run, \
             patch.object(scheduler, "backend_health") as health:
            scheduler.restore_backend_if_needed()
            run.assert_called_once_with(scheduler.compose("up", "-d", "--no-deps", "backend"), timeout=120)
            health.assert_called_once()
            self.assertFalse(marker.exists())

    def test_failed_restore_keeps_marker_for_retry(self):
        scheduler.STATE_DIR.mkdir(exist_ok=True)
        marker = scheduler.STATE_DIR / "restore-needed"
        marker.touch()
        with patch.object(scheduler, "run", return_value=types.SimpleNamespace(
                 returncode=1, stdout="failed")):
            with self.assertRaisesRegex(RuntimeError, "restoration failed"):
                scheduler.restore_backend_if_needed()
            self.assertTrue(marker.exists())

    def test_window_and_interval(self):
        self.assertTrue(scheduler.in_window(self.now, "02:00", "04:00"))
        self.assertFalse(scheduler.in_window(self.now, "03:00", "04:00"))
        scheduler.STATE_DIR.mkdir(exist_ok=True)
        (scheduler.STATE_DIR / "state.json").write_text(
            json.dumps({"completedAt": self.now.isoformat()}), encoding="utf-8"
        )
        with patch.object(scheduler, "check_disk"), patch.object(scheduler, "run") as run:
            scheduler.execute(self.config, self.now)
            run.assert_not_called()

    def test_deletion_failure_restores_backend_without_marking_complete(self):
        config = dict(self.config, RETENTION_MODE="offline")

        def fake_run(command, timeout=60):
            return types.SimpleNamespace(returncode=0, stdout="container-id")

        with patch.object(scheduler, "run", side_effect=fake_run) as run, \
             patch.object(scheduler, "check_disk"), \
             patch.object(scheduler, "backend_health"), \
             patch.object(scheduler, "retention_run", side_effect=[
                 {"eligibleMatches": 4, "retainedPatches": ["16.18"], "expiredVersions": ["16.10"]},
                 RuntimeError("delete failed"),
             ]):
            with self.assertRaisesRegex(RuntimeError, "delete failed"):
                scheduler.execute(config, self.now)
            commands = [call.args[0] for call in run.call_args_list]
            self.assertIn(scheduler.compose("stop", "backend"), commands)
            self.assertIn(scheduler.compose("up", "-d", "--no-deps", "backend"), commands)
            state = json.loads((scheduler.STATE_DIR / "state.json").read_text())
            self.assertNotIn("completedAt", state)

    def test_empty_preview_never_stops_backend(self):
        with patch.object(scheduler, "run", return_value=types.SimpleNamespace(
                 returncode=0, stdout="container-id")) as run, \
             patch.object(scheduler, "check_disk"), \
             patch.object(scheduler, "backend_health"), \
             patch.object(scheduler, "retention_run", return_value={
                 "eligibleMatches": 0, "retainedPatches": ["16.18"], "expiredVersions": []
             }):
            scheduler.execute(self.config, self.now)
            self.assertNotIn(scheduler.compose("stop", "backend"),
                             [call.args[0] for call in run.call_args_list])
            state = json.loads((scheduler.STATE_DIR / "state.json").read_text())
            self.assertEqual(state["completedAt"], self.now.isoformat())

    def test_offline_success_marks_complete_after_health_check(self):
        config = dict(self.config, RETENTION_MODE="offline")

        def fake_run(command, timeout=60):
            return types.SimpleNamespace(returncode=0, stdout="container-id")

        with patch.object(scheduler, "run", side_effect=fake_run) as run, \
             patch.object(scheduler, "check_disk"), \
             patch.object(scheduler, "backend_health") as health, \
             patch.object(scheduler, "retention_run", side_effect=[
                 {"eligibleMatches": 4, "retainedPatches": ["16.18"], "expiredVersions": ["16.10"]},
                 {"eligibleMatches": 4, "deletedMatches": 4, "deletedRows": {"matches": 4},
                  "durationMs": 100, "completed": True},
             ]):
            scheduler.execute(config, self.now)
            self.assertEqual(health.call_count, 2)
            self.assertIn(scheduler.compose("stop", "backend"), [call.args[0] for call in run.call_args_list])
            state = json.loads((scheduler.STATE_DIR / "state.json").read_text())
            self.assertEqual(state["completedAt"], self.now.isoformat())

    def test_online_success_never_stops_backend(self):
        def fake_run(command, timeout=60):
            return types.SimpleNamespace(returncode=0, stdout="container-id")

        with patch.object(scheduler, "run", side_effect=fake_run) as run, \
             patch.object(scheduler, "check_disk"), \
             patch.object(scheduler, "backend_health") as health, \
             patch.object(scheduler, "retention_run", side_effect=[
                 {"eligibleMatches": 4, "retainedPatches": ["16.18"], "expiredVersions": ["16.10"]},
                 {"eligibleMatches": 4, "deletedMatches": 4, "deletedRows": {"matches": 4},
                  "durationMs": 100, "completed": True, "online": True},
             ]) as retention:
            scheduler.execute(self.config, self.now)
            retention.assert_called_with(self.config, dry_run=False, disk_monitor=True)
            commands = [call.args[0] for call in run.call_args_list]
            self.assertNotIn(scheduler.compose("stop", "backend"), commands)
            self.assertNotIn(scheduler.compose("up", "-d", "--no-deps", "backend"), commands)
            self.assertEqual(health.call_count, 2)
            self.assertFalse((scheduler.STATE_DIR / "restore-needed").exists())
            state = json.loads((scheduler.STATE_DIR / "state.json").read_text())
            self.assertEqual(state["completedAt"], self.now.isoformat())

    def test_online_command_flags(self):
        captured = {}

        class FakeProcess:
            returncode = 0

            def __init__(self, command, stdout, **_):
                captured["command"] = command
                stdout.write('Data retention cleanup finished: {"eligibleMatches": 0}\n')

            def poll(self):
                return 0

        with patch.object(scheduler.subprocess, "Popen", FakeProcess):
            scheduler.retention_run(self.config, dry_run=False)
        command = captured["command"]
        for flag in ("DATA_RETENTION_ONLINE=true", "DATA_RETENTION_OFFLINE_ACK=false",
                     "DATA_RETENTION_DELETE_ACK=true", "DATA_RETENTION_BATCH_SIZE=20",
                     "DATA_RETENTION_BATCH_PAUSE=500ms", "DATA_RETENTION_WORK_LIMIT=2h"):
            self.assertIn(flag, command)
        with patch.object(scheduler.subprocess, "Popen", FakeProcess):
            scheduler.retention_run(dict(self.config, RETENTION_MODE="offline"), dry_run=False)
        for flag in ("DATA_RETENTION_ONLINE=false", "DATA_RETENTION_OFFLINE_ACK=true",
                     "DATA_RETENTION_BATCH_SIZE=100", "DATA_RETENTION_WORK_LIMIT=10m"):
            self.assertIn(flag, captured["command"])

    def test_invalid_mode_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "RETENTION_MODE"):
            scheduler.online_mode({"RETENTION_MODE": "fast"})
        self.assertEqual(scheduler.setting_duration({"X": "500ms"}, "X", "1s"), "500ms")
        with self.assertRaises(ValueError):
            scheduler.setting_duration({"X": "1.5h"}, "X", "1s")

    def test_due_run_tolerates_timer_drift(self):
        scheduler.STATE_DIR.mkdir(exist_ok=True)
        attempted = self.now.replace(second=3, microsecond=490000)
        (scheduler.STATE_DIR / "state.json").write_text(
            json.dumps({"attemptedAt": (attempted - scheduler.timedelta(hours=24)).isoformat(),
                        "completedAt": (attempted - scheduler.timedelta(days=7)).isoformat()}),
            encoding="utf-8",
        )
        with patch.object(scheduler, "check_disk"), \
             patch.object(scheduler, "run", return_value=types.SimpleNamespace(returncode=0, stdout="id")), \
             patch.object(scheduler, "backend_health"), \
             patch.object(scheduler, "retention_run", return_value={
                 "eligibleMatches": 0, "retainedPatches": ["16.18"], "expiredVersions": []
             }) as retention:
            scheduler.execute(self.config, self.now.replace(second=3, microsecond=400000))
            retention.assert_called_once()

    def test_work_limit_alerts_and_keeps_retry_pending(self):
        def fake_run(command, timeout=60):
            return types.SimpleNamespace(returncode=0, stdout="container-id")

        with patch.object(scheduler, "run", side_effect=fake_run), \
             patch.object(scheduler, "check_disk"), \
             patch.object(scheduler, "backend_health"), \
             patch.object(scheduler, "alert") as alert, \
             patch.object(scheduler, "retention_run", side_effect=[
                 {"eligibleMatches": 250, "retainedPatches": ["16.18"], "expiredVersions": ["16.10"]},
                 {"eligibleMatches": 250, "deletedMatches": 200, "deletedRows": {"matches": 200},
                  "durationMs": 600000, "completed": False},
             ]):
            scheduler.execute(self.config, self.now)
            alert.assert_called_once()
            self.assertIn("remainingMatches=50", alert.call_args.args[1])
            state = json.loads((scheduler.STATE_DIR / "state.json").read_text())
            self.assertNotIn("completedAt", state)
            self.assertEqual(state["attemptedAt"], self.now.isoformat())

    def test_main_failure_sends_alert(self):
        with patch.object(scheduler, "restore_backend_if_needed"), \
             patch.object(scheduler, "load_config", return_value={"RETENTION_ALERT_WEBHOOK_URL": "x"}), \
             patch.object(scheduler, "execute", side_effect=RuntimeError("disk floor reached")), \
             patch.object(scheduler, "alert") as alert, \
             patch.object(sys, "argv", ["retention_scheduler.py"]):
            self.assertEqual(scheduler.main(), 1)
            alert.assert_called_once_with({"RETENTION_ALERT_WEBHOOK_URL": "x"},
                                          "teamgg retention scheduler failed: disk floor reached")

    def test_alert_posts_slack_compatible_json(self):
        received = []

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_POST(self):
                body = self.rfile.read(int(self.headers["Content-Length"]))
                received.append((self.headers["Content-Type"], self.headers["User-Agent"], json.loads(body)))
                self.send_response(200)
                self.end_headers()

            def log_message(self, *_):
                pass

        server = http.server.HTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.handle_request)
        thread.start()
        try:
            url = f"http://127.0.0.1:{server.server_port}/hook"
            scheduler.alert({"RETENTION_ALERT_WEBHOOK_URL": url}, "retention failed")
            thread.join(timeout=5)
        finally:
            server.server_close()
        self.assertEqual(received, [("application/json", "teamgg-retention-scheduler/1.0",
                                     {"text": "retention failed"})])

    def test_alert_failure_is_logged_without_raising(self):
        with patch.object(scheduler, "log") as log:
            scheduler.alert({"RETENTION_ALERT_WEBHOOK_URL": "http://127.0.0.1:9/unreachable"}, "x")
            self.assertEqual(log.call_args.args[0], "alert_failed")


if __name__ == "__main__":
    unittest.main()
