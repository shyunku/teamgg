# Champion Detail Statistics Operations

Task #62 replaces the Champion Detail and meta collection pipeline that copied and sorted recent participant data through 24 explicit temporary tables.

## Architecture

- The collector first selects only the configured recent game versions through `matches_game_version_index`.
- It normalizes each qualifying participant, rune page, item build, and same-position opponent into one row in `champion_detail_statistics_source`.
- The global statistics advisory lock already serializes all collectors, so the staging table can be safely refreshed with `TRUNCATE` followed by one atomic `INSERT ... WITH ... SELECT` statement.
- Meta and counter outputs are calculated with statement-scoped CTEs over that single staging copy.
- The public statistics DTOs and snapshot payload format are unchanged.

A permanent staging table is intentional. MySQL 8 cannot reference the same `TEMPORARY` table more than once in one query, while both meta and counter calculations need several independent aggregates over the normalized source. The staging table contains only the current recent-patch working set and is replaced on the next collection.

Window functions and grouped queries can still create internal MySQL temporary tables. This change removes the 24 application-managed full and intermediate copies; it does not claim that internal temporary work becomes zero.

## Deployment

Apply migrations before deploying the new backend:

```bash
docker compose run --rm backend migrate
docker compose up -d --build backend
```

Migration `20260830_002` creates only an empty staging table and its indexes. The potentially expensive recent-patch population occurs later inside the scheduled Champion Detail statistics collection, not during migration.

The database user needs `CREATE`, `INDEX`, `DROP`, `INSERT`, `SELECT`, and `TRUNCATE` privileges. `TRUNCATE` takes a metadata lock only on the dedicated staging table and does not truncate source match tables.

## Verification

Before deployment, capture the current InnoDB temporary allocation and global temporary-table counters. Run the same queries after one complete Champion Detail collection. The historical production baseline was approximately 7.8 GB of InnoDB temporary allocation.

Run `scripts/verify-champion-detail-statistics.sql` and retain:

- staging row count and physical size;
- recent patch distribution;
- source join `EXPLAIN` showing `matches_game_version_index` and match-key participant lookups;
- global `Created_tmp_tables` and `Created_tmp_disk_tables` deltas;
- InnoDB temporary allocated and free bytes before and after collection;
- the three structured duration logs below.

```bash
grep -Ei "champion detail filtered source ready|champion detail meta query complete|champion detail counter query complete|champion_detail_statistics collection failed|deadlock|lock wait" output.log | tail -n 200
```

Task #62 can be completed after an operating-scale run confirms that:

1. the API snapshot is populated and sampled champion/meta/counter values match the previous output;
2. the source contains only the selected recent patches;
3. total collection time and InnoDB temporary growth are materially below the old pipeline;
4. no new deadlocks or material lock waits occur.

## Rollback

The old backend ignores `champion_detail_statistics_source`, so application rollback does not require a schema rollback. Leave the staging table in place until the previous backend is stable. It can be removed later with a separate migration after Task #62 production verification.
