# Incremental Mastery Statistics Operations

Task #61 replaces the periodic full-table `GROUP BY` over `masteries` with a materialized per-champion aggregate.

## Data flow

1. Migration `20260830_001` creates `mastery_statistics_aggregates` and `mastery_statistics_dirty_champions`.
2. It creates the `(champion_id, champion_points DESC, champion_level)` index with `ALGORITHM=INPLACE, LOCK=NONE`.
3. `AFTER INSERT`, `AFTER UPDATE`, and `AFTER DELETE` triggers enqueue only the affected champion ID. Repeated writes collapse into one dirty row.
4. The migration seeds the dirty queue once from existing champion IDs.
5. Before a mastery snapshot is generated, the collector refreshes at most 1,000 dirty champions. Each refresh is a bounded index-range scan for one champion, not a global sort or grouping operation.
6. Snapshot reads use the small materialized table. Top-ranker reads use one indexed `LIMIT 30` lookup per champion.

A database timestamp is captured before reading the queue. A dirty row is acknowledged only when its `dirty_at` is older than that cutoff. A write occurring during refresh therefore remains queued for the next run. If the process stops after saving an aggregate but before acknowledgement, the idempotent refresh runs again.

## Deployment

Apply the migration once before deploying the backend version that reads the aggregate table:

```bash
docker compose run --rm backend migrate
docker compose up -d --build backend
```

The normal backend startup mode remains `validate`; it does not apply pending migrations. The database user needs `CREATE`, `ALTER`, `INDEX`, and `TRIGGER` privileges.

The covering index is large because it includes all mastery rows. Although MySQL is instructed to use online DDL, apply it during a low-traffic window and confirm that the database has enough temporary and permanent disk space. Do not remove the pre-existing `(champion_id, champion_points)` index as part of this migration; assess duplicate-index removal separately under Task #63 after production query usage is measured.

The first statistics run performs the one-time backfill as separate champion-range scans. Later runs touch only champions queued by writes. Existing aggregate and queue tables are safe to retain during an application rollback; the previous backend ignores them, while triggers continue recording changes.

## Production verification

Run `scripts/verify-mastery-statistics.sql` after migration and again after the first mastery collection. Replace `@champion_id` if necessary.

Completion evidence for Task #61 requires:

- all three triggers and the covering index are present;
- the dirty queue reaches zero after a collection when writes are paused briefly;
- sampled materialized values exactly match a direct per-champion aggregate;
- `EXPLAIN` selects the covering index for aggregate and top-ranker queries without a full-table scan;
- the collection log reports bounded dirty counts and a substantially shorter duration than the former full-table aggregation;
- no material increase in lock-wait or deadlock events is observed during collection.

Useful log filter:

```bash
grep -Ei "mastery_statistics incremental aggregates|mastery_statistics collection failed|deadlock|lock wait" output.log | tail -n 200
```
