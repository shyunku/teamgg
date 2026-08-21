# team.gg Backend

The backend API server for **team.gg**, a League of Legends service for custom-game team building, participant balancing, match history, and statistics analysis. It is built with Go and Gin and includes real-time communication and background data collection jobs.

---

## Tech Stack

- **Language**: Go 1.26.5
- **Web Framework**: [Gin Web Framework](https://github.com/gin-gonic/gin)
- **Realtime**: [go-socket.io](https://github.com/googollee/go-socket.io)
- **Database**: MySQL / PostgreSQL with [sqlx](https://github.com/jmoiron/sqlx)
- **Cache / In-Memory Database**: Redis with [go-redis/v9](https://github.com/redis/go-redis)
- **Third-Party APIs**: Riot Games API and Riot Sign On (RSO)

---

## Project Structure

```text
apps/backend/
├── controllers/          # API handlers and route definitions
│   ├── middlewares/      # Gin middleware, including authentication and logging
│   ├── socket/           # Socket.IO handlers
│   └── v1/               # Version 1 API routes for auth, custom games, statistics, and more
├── models/               # Database models and DTOs for summoners, matches, participants, and more
├── service/              # Core business logic and background schedulers
│   └── statistics/       # Periodic champion, tier, and mastery aggregation
├── libs/                 # Shared packages for database access, cryptography, and HTTP clients
├── third_party/          # Third-party clients, including the Riot API client
├── migrations/           # Database migration scripts
├── scripts/              # Build, daemon, stop, and restart scripts
├── go.mod / go.sum       # Go module and dependency definitions
└── main.go               # Application entry point
```

---

## Configuration

Create a `.env` file in the project root and configure the following environment variables.

```env
# Server port
APP_SERVER_PORT=8080

# Database configuration
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_HOST=127.0.0.1
DB_PORT=3306
DB_NAME=team_gg

# Schema migration policy: validate (default), up, or off
DB_MIGRATION_MODE=validate
DB_MIGRATION_LOCK_TIMEOUT=30s
DB_MIGRATION_SUMMONER_MATCH_BATCH=1000

# Redis (optional; defaults preserve the old localhost setup)
REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0

# JWT secrets and expiration times
JWT_ACCESS_SECRET=your_jwt_access_secret
JWT_ACCESS_EXPIRE=3600
JWT_REFRESH_SECRET=your_jwt_refresh_secret
JWT_REFRESH_EXPIRE=604800

# Riot Sign On (RSO) and Riot API client
RSO_CLIENT_ID=your_rso_client_id
RSO_CLIENT_SECRET=your_rso_client_secret
RSO_CLIENT_CALLBACK_URI=http://localhost:8080/v1/auth/callback

# Runtime environment and diagnostics
DEBUG=true
IS_PROD=false

# Replay analyzer integration
REPLAY_ANALYZER_BASE_URL=http://localhost:7720
REPLAY_ANALYZER_SHARED_SECRET=replace-with-a-long-random-shared-secret
REPLAY_ANALYSIS_STALE_AFTER=45m

# Background DataExplorer jobs (optional)
DATA_EXPLORER_ENABLED=false
DATA_EXPLORER_BOOTSTRAP_ENABLED=false
DATA_EXPLORER_PARTICIPANT_DISCOVERY=disabled
DATA_EXPLORER_MAX_DEPTH=0
DATA_EXPLORER_SUMMONER_WORKERS=1
DATA_EXPLORER_MATCH_WORKERS=2
DATA_EXPLORER_DAILY_SUMMONER_BUDGET=500
DATA_EXPLORER_DAILY_MATCH_BUDGET=1500
DATA_EXPLORER_MATCH_COUNT=3
DATA_EXPLORER_SUMMONER_REVISIT_INTERVAL=720h
DATA_EXPLORER_MATCH_REVISIT_INTERVAL=8760h
DATA_EXPLORER_CLEANUP_ENABLED=false
DATA_EXPLORER_CLEANUP_INTERVAL=30s
DATA_EXPLORER_CLEANUP_BATCH_SIZE=500
DATA_EXPLORER_COMPLETED_JOB_RETENTION=24h
DATA_EXPLORER_SOURCE_RETENTION=24h
DATA_EXPLORER_BOOTSTRAP_BATCH_SIZE=500
DATA_EXPLORER_BOOTSTRAP_INTERVAL=5s
DATA_EXPLORER_POLL_INTERVAL=1s
DATA_EXPLORER_LEASE=5m
DATA_EXPLORER_MAX_ATTEMPTS=8
DATA_EXPLORER_DEBUG=false
DATA_EXPLORER_STATUS_LOG_INTERVAL=30s

# Statistics snapshots (optional; each loop is disabled unless explicitly enabled)
STATISTICS_TIER_ENABLED=true
STATISTICS_TIER_PERIOD=12h
STATISTICS_TIER_INITIAL_DELAY=30s

STATISTICS_MASTERY_ENABLED=true
STATISTICS_MASTERY_PERIOD=12h
STATISTICS_MASTERY_INITIAL_DELAY=2m

STATISTICS_CHAMPION_DETAIL_ENABLED=true
STATISTICS_CHAMPION_DETAIL_PERIOD=24h
STATISTICS_CHAMPION_DETAIL_INITIAL_DELAY=10m

STATISTICS_RETRY_DELAY=5m
STATISTICS_LOCK_RETRY_DELAY=15s
STATISTICS_LOCK_TIMEOUT=1s
```

DataExplorer is disabled unless `DATA_EXPLORER_ENABLED=true` is set explicitly. Historical participant bootstrap is separately opt-in through `DATA_EXPLORER_BOOTSTRAP_ENABLED=true` because it can enqueue a large existing dataset.

`DATA_EXPLORER_PARTICIPANT_DISCOVERY=disabled` stores requested matches without adding every match participant to the exploration queue. Set it to `bounded` only when relationship expansion is intended, and set `DATA_EXPLORER_MAX_DEPTH` to the highest permitted hop. Depth `0` processes seed summoners and their recent matches without expanding to other participants; depth `1` permits one participant hop. Values above `10` are clamped to `10`, and invalid values fall back to `0`. A job keeps its persisted depth when an expired lease is recovered after restart.

Set a `DATA_EXPLORER_*_BUDGET` variable to `0` only when an uncapped daily budget is explicitly intended. Production should use finite summoner and match budgets configured according to the rate limits of the Riot API product key and available database capacity.

Successful jobs persist `last_processed_at` and `next_eligible_at` in separate processing-state tables. Queue insertion checks this state, so removing completed queue rows does not immediately enqueue the same relationship graph again. The default revisit windows are 30 days for summoners and 365 days for immutable matches. Existing `done` jobs are copied to these tables through restart-safe primary-key cursors in the same small batch size; this state backfill runs even while deletion is disabled.

Completed-job cleanup is disabled by default because it deletes database rows. After confirming the retention policy, set `DATA_EXPLORER_CLEANUP_ENABLED=true` to remove only `done` summoner/match jobs and completed match-source rows in batches. The default batch is 500 rows per table every 30 seconds, with 24-hour retention. Pending, processing, and failed jobs are never deleted by this cleanup. Safety bounds cap the batch at 5,000 rows, enforce a minimum five-second cleanup interval and one-hour retention, and prevent either revisit interval from being shorter than 24 hours.

Statistics timing values use Go duration syntax, such as `30s`, `5m`, and `12h`. Each statistics job uses a different initial delay so that database aggregation jobs do not all start at once during server startup. `STATISTICS_LOCK_RETRY_DELAY` controls the short retry interval used when another statistics job holds the lock, while `STATISTICS_RETRY_DELAY` controls retries after an actual collection error. When multiple server instances are running, MySQL advisory locks and the shared `statistics_snapshots` cache prevent duplicate aggregation work.

`REPLAY_ANALYZER_BASE_URL` is the public URL that browsers upload ROFL files to directly. `REPLAY_ANALYZER_SHARED_SECRET` must be the same long random value on this server and the replay analyzer; it signs short-lived upload tickets and authenticates analyzer status callbacks.
`REPLAY_ANALYSIS_STALE_AFTER` releases a job that remained queued, uploading, or analyzing without a callback for too long. Keep it longer than the analyzer's `ANALYSIS_TIMEOUT`; the default is `45m`.

When running in a container, the server can start entirely from process environment variables; a physical `.env` file is no longer required. `REDIS_ADDR` should point to a reachable Redis host such as `host.docker.internal:6379` or a managed Redis endpoint.

---

## Running and Building

### 1. Prerequisites

- [Go](https://go.dev/doc/install) 1.26.5
- A running MySQL or PostgreSQL database instance
- A running Redis server

### 2. Install Dependencies

```bash
go mod download
```

### 3. Apply and Validate the Database Schema

For a completely empty database, import `scheme.ddl` once. After that, the
versioned migration runner manages incremental schema changes and records them
in `schema_migrations` with a checksum and dirty state.

Run pending migrations explicitly before starting a new backend release:

```bash
go run . migrate
# Docker Compose production deployment:
docker compose run --rm backend migrate
```

Normal startup defaults to `DB_MIGRATION_MODE=validate`: it does not mutate the
application schema and refuses to start when a migration is missing, dirty, has
a changed checksum, or its expected schema has drifted. `up` applies migrations
during startup and `off` disables both migration and drift checks; prefer the
explicit `migrate` command and reserve `off` for emergency diagnosis.

The `summoner_matches` migration creates a deduplicated replacement with the
`(puuid, match_id)` primary key, mirrors concurrent inserts with a temporary
trigger, backfills in bounded keyset batches, and swaps both tables atomically.
The cursor is stored in `schema_migration_progress`, so a failed deployment can
resume without copying all previously completed batches again.
It retains `summoner_matches_legacy_20260820`; remove that backup manually only
after row-count and application verification. The database user needs `CREATE`,
`ALTER`, `INDEX`, `TRIGGER`, and `DROP` privileges. Run large index migrations
during a low-traffic maintenance window.

### 4. Run the Development Server

```bash
go run main.go
```

### 5. Build and Deployment Scripts

The `scripts/` directory contains operational utilities.

- **Build**: `bash scripts/build.sh`
- **Run in the background**: `bash scripts/daemon.sh`
- **Stop the server**: `bash scripts/stop.sh`
- **Rebuild and restart**: `bash scripts/build_and_restart.sh`
- **Build and run on Windows**: `scripts\\build_and_restart.bat`
