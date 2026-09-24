import importlib.util
import json
from pathlib import Path
import sys
import tempfile
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
                scheduler.execute(self.config, self.now)
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

    def test_success_marks_complete_after_health_check(self):
        def fake_run(command, timeout=60):
            return types.SimpleNamespace(returncode=0, stdout="container-id")

        with patch.object(scheduler, "run", side_effect=fake_run), \
             patch.object(scheduler, "check_disk"), \
             patch.object(scheduler, "backend_health") as health, \
             patch.object(scheduler, "retention_run", side_effect=[
                 {"eligibleMatches": 4, "retainedPatches": ["16.18"], "expiredVersions": ["16.10"]},
                 {"eligibleMatches": 4, "deletedMatches": 4, "deletedRows": {"matches": 4},
                  "durationMs": 100, "completed": True},
             ]):
            scheduler.execute(self.config, self.now)
            self.assertEqual(health.call_count, 2)
            state = json.loads((scheduler.STATE_DIR / "state.json").read_text())
            self.assertEqual(state["completedAt"], self.now.isoformat())


if __name__ == "__main__":
    unittest.main()
