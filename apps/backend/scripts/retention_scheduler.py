#!/usr/bin/env python3
"""Run the existing retention command in a guarded, offline maintenance window."""

import fcntl
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timedelta
from zoneinfo import ZoneInfo


ROOT = Path(__file__).resolve().parents[3]
STATE_DIR = ROOT / ".tmp" / "retention-scheduler"
TZ = ZoneInfo("Asia/Seoul")
CONTAINER = "teamgg-retention-scheduler"


def log(event, **fields):
    print(json.dumps({"time": datetime.now(TZ).isoformat(timespec="seconds"), "event": event, **fields},
                     ensure_ascii=False), flush=True)


def load_config(path):
    config = {}
    if path.exists():
        for raw in path.read_text(encoding="utf-8").splitlines():
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if "=" not in line:
                raise ValueError("invalid retention configuration line")
            key, value = line.split("=", 1)
            config[key.strip()] = value.strip().strip('"').strip("'")
    config.update({key: value for key, value in os.environ.items() if key.startswith("RETENTION_")})
    return config


def setting_int(config, key, fallback, minimum, maximum):
    value = int(config.get(key, fallback))
    if not minimum <= value <= maximum:
        raise ValueError(f"{key} must be between {minimum} and {maximum}")
    return value


def setting_duration(config, key, fallback):
    value = config.get(key, fallback)
    if not value or value[-1] not in ("s", "m", "h") or not value[:-1].isdigit():
        raise ValueError(f"{key} must be a Go duration using s, m, or h")
    return value


def in_window(now, start, end):
    current = now.strftime("%H:%M")
    return start <= current < end if start < end else current >= start or current < end


def run(command, timeout=60):
    return subprocess.run(command, cwd=ROOT, text=True, stdout=subprocess.PIPE,
                          stderr=subprocess.STDOUT, timeout=timeout, check=False)


def compose(*args):
    return ["docker", "compose", *args]


def check_disk(config):
    path = Path(config.get("RETENTION_DISK_PATH", "/"))
    minimum = setting_int(config, "RETENTION_MIN_FREE_GIB", 12, 1, 10000)
    free = shutil.disk_usage(path).free
    if free < minimum * 1024 ** 3:
        raise RuntimeError(f"disk floor reached: freeGiB={free // 1024 ** 3} minimumGiB={minimum}")
    return free


def read_result(output):
    for line in reversed(output.splitlines()):
        marker = "Data retention cleanup finished: "
        if marker in line:
            return json.loads(line.split(marker, 1)[1])
    raise RuntimeError("retention result JSON missing from backend output")


def retention_run(config, dry_run, disk_monitor=False):
    flags = [
        "-e", f"DATA_RETENTION_DRY_RUN={'true' if dry_run else 'false'}",
        "-e", f"DATA_RETENTION_DELETE_ACK={'false' if dry_run else 'true'}",
        "-e", f"DATA_RETENTION_OFFLINE_ACK={'false' if dry_run else 'true'}",
        "-e", "DATA_RETENTION_ENFORCE_LOAD_GUARD=true",
        "-e", f"DATA_RETENTION_MAX_THREADS_RUNNING={setting_int(config, 'RETENTION_MAX_THREADS_RUNNING', 4, 1, 100)}",
        "-e", f"DATA_RETENTION_MAX_LOCK_WAITS={setting_int(config, 'RETENTION_MAX_LOCK_WAITS', 0, 0, 100)}",
        "-e", f"DATA_RETENTION_MATCH_PATCHES={setting_int(config, 'RETENTION_MATCH_PATCHES', 8, 3, 30)}",
        "-e", f"DATA_RETENTION_BATCH_SIZE={setting_int(config, 'RETENTION_BATCH_SIZE', 100, 10, 1000)}",
        "-e", f"DATA_RETENTION_BATCH_TIMEOUT={setting_duration(config, 'RETENTION_BATCH_TIMEOUT', '2m')}",
        "-e", f"DATA_RETENTION_WORK_LIMIT={setting_duration(config, 'RETENTION_WORK_LIMIT', '10m')}",
    ]
    command = compose("run", "--rm", "--no-deps", "--name", CONTAINER, *flags, "backend", "cleanup-retention")
    timeout = setting_int(config, "RETENTION_COMMAND_TIMEOUT_MINUTES", 15, 2, 70) * 60
    with tempfile.TemporaryFile(mode="w+t", encoding="utf-8") as output:
        process = subprocess.Popen(command, cwd=ROOT, stdout=output, stderr=subprocess.STDOUT, text=True)
        stop_reason = None
        started = time.monotonic()
        while process.poll() is None:
            if time.monotonic() - started > timeout:
                stop_reason = "command_timeout"
            elif disk_monitor:
                try:
                    check_disk(config)
                except RuntimeError:
                    stop_reason = "disk_floor"
            if stop_reason:
                run(["docker", "stop", "-t", "10", CONTAINER], timeout=30)
                try:
                    process.wait(timeout=30)
                except subprocess.TimeoutExpired:
                    process.terminate()
                    process.wait(timeout=30)
                break
            time.sleep(2)
        output.seek(0)
        content = output.read()
    if stop_reason:
        raise RuntimeError(stop_reason)
    if process.returncode:
        raise RuntimeError(f"retention command exit={process.returncode}: {content[-1200:]}")
    return read_result(content)


def backend_health(timeout=120):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        container = run(compose("ps", "-q", "backend"))
        if container.returncode == 0 and container.stdout.strip():
            health = run(["docker", "inspect", "-f", "{{.State.Health.Status}}", container.stdout.strip()])
            if health.returncode == 0 and health.stdout.strip() == "healthy":
                return
        time.sleep(3)
    raise RuntimeError("backend did not become healthy after restoration")


def alert(config, message):
    url = config.get("RETENTION_ALERT_WEBHOOK_URL", "")
    if not url:
        return
    import urllib.request
    request = urllib.request.Request(url, data=json.dumps({"text": message}).encode(),
                                     headers={"Content-Type": "application/json"}, method="POST")
    try:
        urllib.request.urlopen(request, timeout=5).close()
    except Exception as exc:
        log("alert_failed", error=type(exc).__name__)


def execute(config, now):
    if config.get("RETENTION_ENABLED", "false").lower() != "true":
        log("disabled")
        return
    if config.get("RETENTION_DELETE_ENABLED", "false").lower() != "true":
        log("deletion_disabled")
        return
    start = config.get("RETENTION_WINDOW_START", "02:00")
    end = config.get("RETENTION_WINDOW_END", "04:00")
    for value in (start, end):
        datetime.strptime(value, "%H:%M")
    if not in_window(now, start, end):
        log("outside_window", start=start, end=end)
        return

    STATE_DIR.mkdir(parents=True, exist_ok=True)
    with (STATE_DIR / "lock").open("w") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            log("already_running")
            return
        state_path = STATE_DIR / "state.json"
        state = json.loads(state_path.read_text(encoding="utf-8")) if state_path.exists() else {}
        interval = setting_int(config, "RETENTION_INTERVAL_DAYS", 30, 1, 365)
        retry = setting_int(config, "RETENTION_RETRY_HOURS", 24, 1, 168)
        for key, delay in (("completedAt", timedelta(days=interval)), ("attemptedAt", timedelta(hours=retry))):
            if state.get(key) and now < datetime.fromisoformat(state[key]) + delay:
                log("not_due", reason=key)
                return
        check_disk(config)
        container = run(compose("ps", "-q", "backend"))
        if container.returncode or not container.stdout.strip():
            raise RuntimeError("backend is not running; refusing automated deletion")
        backend_health(timeout=5)
        state["attemptedAt"] = now.isoformat()
        state_path.write_text(json.dumps(state), encoding="utf-8")
        preview = retention_run(config, dry_run=True)
        log("preview", eligibleMatches=preview["eligibleMatches"],
            retainedPatches=preview["retainedPatches"], expiredVersions=preview["expiredVersions"])
        if not preview["eligibleMatches"]:
            state["completedAt"] = now.isoformat()
            state_path.write_text(json.dumps(state), encoding="utf-8")
            return
        check_disk(config)

        # A stop command can partially succeed. Always attempt restoration once issued.
        result = None
        try:
            stopped = run(compose("stop", "backend"), timeout=120)
            if stopped.returncode:
                raise RuntimeError(f"backend stop failed: {stopped.stdout[-500:]}")
            result = retention_run(config, dry_run=False, disk_monitor=True)
            log("delete_result", eligibleMatches=result["eligibleMatches"],
                deletedMatches=result["deletedMatches"], deletedRows=result.get("deletedRows", {}),
                remainingMatches=max(0, result["eligibleMatches"] - result["deletedMatches"]),
                durationMs=result["durationMs"], completed=result["completed"],
                stopReason="complete" if result["completed"] else "work_limit")
        finally:
            restored = run(compose("up", "-d", "--no-deps", "backend"), timeout=120)
            if restored.returncode:
                raise RuntimeError(f"backend restoration failed: {restored.stdout[-500:]}")
            backend_health()
            log("backend_restored")
        if result and result["completed"]:
            state["completedAt"] = now.isoformat()
            state_path.write_text(json.dumps(state), encoding="utf-8")


def main():
    config = {}
    try:
        config = load_config(ROOT / ".env.retention")
        execute(config, datetime.now(TZ))
    except Exception as exc:
        reason = str(exc)
        log("failed", reason=reason)
        alert(config, f"teamgg retention scheduler failed: {reason}")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
