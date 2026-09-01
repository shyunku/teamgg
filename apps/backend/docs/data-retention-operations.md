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
DATA_RETENTION_MATCH_PATCHES=8
DATA_RETENTION_BATCH_SIZE=100
DATA_RETENTION_BATCH_TIMEOUT=2m
DATA_RETENTION_WORK_LIMIT=10m
```

Actual deletion is rejected unless dry-run is disabled and both acknowledgements are true. Stop backend writes before acknowledging offline mode.

## Commands

Preview the retained patches, expired versions, and eligible match count:

```bash
docker compose run --rm backend cleanup-retention
```

Delete during a maintenance window:

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
2. Participant numeric mappings and participants
3. Team bans and teams
4. Summoner-match and DataExplorer relationships
5. Incremental-statistics processed markers
6. Match numeric mappings
7. Matches

Each batch is one transaction and uses `binlog_row_image=MINIMAL`. Any statement failure rolls back the current batch.

## Verification

- Confirm the dry-run target before deletion.
- Watch free disk, lock waits, backend health, and the command's row counters.
- After restart, verify summoner, champion, and meta-summary APIs.
- Normal `DELETE` creates reusable InnoDB space but may not shrink every `.ibd` file. The `matches` table and fully removed partitions/tables return space according to their storage layout.
