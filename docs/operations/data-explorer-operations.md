# DataExplorer operations

## Collection behavior

Set `DATA_EXPLORER_METRICS_ENABLED=true` to emit an operational snapshot immediately after DataExplorer starts and every `DATA_EXPLORER_METRICS_INTERVAL` afterward. The default interval is five minutes and values below one minute are clamped to one minute.

The snapshot is a parseable `key=value` log with `event=data_explorer_metrics`. It includes:

- daily Riot API budget usage for summoner and match work;
- pending, processing, and failed queue counts;
- estimated `summoners`, `matches`, and `masteries` rows;
- exact combined DataExplorer job rows;
- daily net row growth relative to the first sample recorded by the database that day;
- total schema data and index bytes;
- reclaimable InnoDB bytes reported as `data_free`;
- interval deltas for MySQL temporary tables and on-disk temporary tables;
- temporary tablespace allocation when the MySQL engine exposes `information_schema.files`.

Large InnoDB tables are never scanned with `COUNT(*)`. Their row counts and daily growth use `information_schema.tables.table_rows`, so fields ending in `_estimated` are capacity indicators rather than billing-grade insertion counters. Queue status counts remain exact.

`temp_status_available=false` is expected on the first sample because MySQL exposes cumulative counters and an interval needs two samples. `temp_space_available=false` means the current MySQL engine or account does not expose temporary tablespace size; the rest of the snapshot remains valid.

## Schema migration

Task 67 adds `data_explorer_metrics_daily`, which preserves the first and latest row estimates for each database date. Apply the migration before starting a server whose `DB_MIGRATION_MODE` is `validate`:

```bash
go run . migrate
```

For a container, run the backend image with the same environment and append the `migrate` command before starting the normal service. The migration is versioned as `20260821_001`.

## Alert thresholds

Threshold crossings emit one `event=data_explorer_alert state=firing` warning. The warning is not repeated every collection interval. A later healthy sample emits one `state=recovered` record.

| Environment variable | Default | Meaning |
|---|---:|---|
| `DATA_EXPLORER_ALERT_BUDGET_PERCENT` | `80` | Daily summoner or match budget usage percentage |
| `DATA_EXPLORER_ALERT_SUMMONER_QUEUE` | `10000` | Pending summoner jobs |
| `DATA_EXPLORER_ALERT_MATCH_QUEUE` | `10000` | Pending match jobs |
| `DATA_EXPLORER_ALERT_FAILED_JOBS` | `100` | Combined failed summoner and match jobs |
| `DATA_EXPLORER_ALERT_DATABASE_BYTES` | `0` | Schema data+index size; accepts bytes or `K/KB/M/MB/G/GB/T/TB` (1024-based), and `0` disables the alert |
| `DATA_EXPLORER_ALERT_DAILY_ROW_GROWTH` | `1000000` | Per-table estimated daily net growth and exact queue growth |
| `DATA_EXPLORER_ALERT_TEMP_DISK_PERCENT` | `25` | On-disk temporary tables as a percentage of interval temporary tables |

Any alert threshold can be set to `0` to disable that alert. Percentage values above 100 are clamped to 100. The temporary-table ratio is evaluated only when at least ten temporary tables were created during the interval.

Set `DATA_EXPLORER_ALERT_DATABASE_BYTES` to the schema size at which the database volume reaches roughly 80% of its usable capacity. Human-readable values such as `200M`, `5G`, and `1.5TB` are supported; legacy integer values continue to mean bytes. This metric is schema allocation, not host filesystem free space, so the infrastructure layer should retain a separate filesystem or managed-database storage alarm.

## Production budget sizing

Keep both daily budgets finite. Start with the safe defaults:

```dotenv
DATA_EXPLORER_DAILY_SUMMONER_BUDGET=500
DATA_EXPLORER_DAILY_MATCH_BUDGET=1500
```

Then use at least seven complete daily snapshots:

1. Record the 50th and 95th percentile daily schema-byte and row-growth values.
2. Reserve at least 20% of the database volume and Riot API capacity for interactive traffic, retries, statistics jobs, and migrations.
3. Calculate the allowed daily storage growth as `(alert_database_bytes - current_database_bytes) / desired_retention_days`.
4. Increase budgets only while the observed 95th percentile growth remains below that allowance and queues do not accumulate.
5. If budget usage reaches 80% but queues stay flat, the budget is adequate. If pending queues grow for several days, raise the smaller constrained budget gradually rather than increasing worker concurrency first.
6. If temporary-disk percentage or database latency rises, reduce ingestion and fix aggregation queries before raising budgets.

Summoner work fans out to several Riot endpoints and may add many mastery rows, while match work stores a match and all participant-related rows. Therefore a `1:3` summoner-to-match budget is only a starting point; use the observed production row and byte growth to set the final ratio.

Useful log filters:

```bash
grep -E "event=data_explorer_(metrics|alert)" output.log | tail -n 200
```
