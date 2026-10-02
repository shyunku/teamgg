# Data retention operations

`cleanup-retention` limits raw match storage without external archives. It keeps recent patch data and permanently deletes older match graphs.

## Policy

| Data | Policy |
|---|---|
| Raw matches | Keep the latest 8 short patches |
| Invalid or empty versions | Keep |
| Summoner cache | Keep; refresh on normal Riot updates |
| DataExplorer done jobs | Delete after retention only when cleanup is enabled |
| Pending/processing/failed jobs | Keep |

## Safety defaults

```ini
DATA_RETENTION_DRY_RUN=true
DATA_RETENTION_DELETE_ACK=false
DATA_RETENTION_OFFLINE_ACK=false
DATA_RETENTION_ONLINE=false
DATA_RETENTION_MATCH_PATCHES=8
DATA_RETENTION_BATCH_SIZE=100        # 20 when online
DATA_RETENTION_BATCH_TIMEOUT=2m
DATA_RETENTION_BATCH_PAUSE=0s        # 500ms when online
DATA_RETENTION_WORK_LIMIT=10m        # at most 3h
DATA_RETENTION_ENFORCE_LOAD_GUARD=false
DATA_RETENTION_MAX_THREADS_RUNNING=4
DATA_RETENTION_MAX_LOCK_WAITS=0
```

Actual deletion is rejected unless dry-run is disabled, `DATA_RETENTION_DELETE_ACK=true`, and either online mode is enabled or offline mode is acknowledged. Stop backend writes before acknowledging offline mode.

## Online mode

Online mode deletes while the backend keeps serving. The cleanup session uses `READ COMMITTED` so it takes no gap locks on live index ranges, and a 5-second lock wait so it yields to API writers. Each batch of 20 matches stays one transaction. A deadlock or lock wait timeout rolls back that batch and retries it up to 5 times. When the load guard trips, the run waits 10 seconds instead of failing. A short pause between batches limits I/O pressure on the small buffer pool. Result JSON reports `online`, `throttledWaits`, and `retriedBatches`.

An API request that re-saves an expired match at the exact moment it is deleted can still fail; the backend's existing write retry covers lock conflicts, but not a deleted parent row. Expired patches are rarely re-fetched, so this is accepted.

Throughput is bound by random reads from EBS because the production InnoDB buffer pool is 128 MiB. The 2026-10-02 offline run deleted about 250 matches per minute.

## Commands

Preview the retained patches, expired versions, and eligible match count:

```bash
docker compose run --rm backend cleanup-retention
```

Delete online, without stopping the backend:

```bash
docker compose run --rm --no-deps \
  -e DATA_RETENTION_DRY_RUN=false \
  -e DATA_RETENTION_DELETE_ACK=true \
  -e DATA_RETENTION_ONLINE=true \
  -e DATA_RETENTION_ENFORCE_LOAD_GUARD=true \
  backend cleanup-retention
```

Delete offline during a maintenance window:

```bash
docker compose stop backend
docker compose run --rm --no-deps \
  -e DATA_RETENTION_DRY_RUN=false \
  -e DATA_RETENTION_DELETE_ACK=true \
  -e DATA_RETENTION_OFFLINE_ACK=true \
  backend cleanup-retention
docker compose up -d backend
```

The command can stop at its work limit and be rerun. Deleted rows are gone, so the next run selects only remaining expired matches.

## Delete order

1. Perk selections, styles, perks, participant details
2. Participants
3. Team bans and teams
4. Summoner-match and DataExplorer relationships
5. Incremental-statistics processed markers
6. Matches

Each batch is one transaction and uses `binlog_row_image=MINIMAL`. Any statement failure rolls back the current batch.

Numeric match and participant identity mappings are retained. They are migration infrastructure rather than raw match payload, and preserving them lets a later re-fetch reuse the same numeric IDs while participant backfill and read cutover remain incomplete.

## Optional host scheduler (#76)

The host-side scheduler is disabled by default. It runs the existing cleanup command, not a second deletion implementation. It needs Python 3.9+ on the Linux Docker host. Copy [the configuration template](../../../.env.retention.example) to the ignored root `.env.retention` and set the host path containing the MySQL data volume. Never put deletion acknowledgements into the backend's normal environment file.

On the EC2 host, install the systemd units after creating the ignored root `.env.retention` with both enable switches set to `true`:

```bash
sudo install -m 644 apps/backend/deploy/teamgg-retention.service /etc/systemd/system/
sudo install -m 644 apps/backend/deploy/teamgg-retention.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now teamgg-retention.timer
systemctl list-timers teamgg-retention.timer
journalctl -u teamgg-retention.service -n 30 --no-pager
```

The timer checks hourly; the script enforces its KST maintenance window, 7-day default interval, 24-hour retry interval, host disk floor, backend health, and a host file lock. It first runs a dry-run with the database load guard enabled. If no matches are eligible, it records completion without stopping the backend. `RETENTION_MODE=online` (default) then runs the cleanup in online mode for up to `RETENTION_WORK_LIMIT=2h` and checks backend health afterwards. `RETENTION_MODE=offline` stops only the backend, runs the bounded cleanup, and attempts to restart and health-check it even after deletion failure; a restore marker plus `ExecStopPost` retries backend restoration if the service is interrupted, and completion is recorded only after a successful restart. The due check allows 10 minutes of timer drift so a due run starts at the beginning of the window. The service unit allows 150 minutes for one run. A timed-out or low-disk job is stopped; the next eligible window resumes from remaining matches. JSON logs include preview, deleted matches and rows, duration, completion, restoration, and failure reason. An optional Slack-compatible webhook receives failure alerts and work-limit stops with the remaining match count; without a configured webhook, inspect the journal.

For a one-shot read-only host integration check, set `RETENTION_ENABLED=true RETENTION_PREVIEW_ONLY=true` for that process. It does not stop the backend or write a successful-run timestamp.

Before enabling it, verify production dry-run targets, backup/recovery posture, disk floor and the maintenance window. Installing the timer and setting both enable switches are separate production actions.

## Verification

- Confirm the dry-run target before deletion.
- Watch free disk, lock waits, backend health, and the command's row counters.
- After restart, verify summoner, champion, and meta-summary APIs.
- Normal `DELETE` creates reusable InnoDB space but may not shrink every `.ibd` file. The `matches` table and fully removed partitions/tables return space according to their storage layout.
