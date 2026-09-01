# Running team.gg with Docker

This Compose stack runs the frontend, backend, ROFL replay analyzer, and administrator operations API. MySQL and Redis are expected to run on the Docker host or external infrastructure.

## Initial setup

Create the Compose and service environment files from the repository root.

```powershell
Copy-Item .env.example .env
Copy-Item apps/backend/.env.docker.example apps/backend/.env.docker
Copy-Item apps/lol-replay-analyzer/.env.docker.example apps/lol-replay-analyzer/.env.docker
Copy-Item apps/admin/.env.docker.example apps/admin/.env.docker
```

Each file has a separate responsibility:

- `.env`: published ports, bind addresses, and service environment file paths; do not put application secrets here
- `apps/backend/.env.docker`: database, Redis, Riot API, RSO, JWT, and backend runtime settings
- `apps/lol-replay-analyzer/.env.docker`: OpenAI, upload, decoder, and analyzer runtime settings
- `apps/admin/.env.docker`: administrator API origins, service endpoints, and its backend shared secret
- `apps/frontend/.env.dev` and `.env.production`: existing frontend build settings

To reuse an existing non-Docker service `.env`, change only the paths in the root `.env`:

```dotenv
BACKEND_ENV_FILE=./apps/backend/.env
ANALYZER_ENV_FILE=./apps/lol-replay-analyzer/.env
ADMIN_ENV_FILE=./apps/admin/.env
```

Pay particular attention to these settings:

- `DB_HOST`: use `host.docker.internal` for a database on a Windows or macOS host, or the actual reachable hostname for a production database
- `REDIS_ADDR`: use `host.docker.internal:6379` for Redis on the host, or the actual Redis endpoint in production
- `REPLAY_ANALYZER_SHARED_SECRET`: use the same sufficiently long random value in the backend and analyzer environments
- `REPLAY_ANALYZER_BASE_URL`: the analyzer URL returned to browsers; use `http://localhost:7720` in development and a public HTTPS URL in production
- `CORS_ORIGINS`: use `http://localhost:8080` in development and `https://teamgg.kr,https://www.teamgg.kr` in production
- `TEAMGG_API_BASE_URL`: Compose overrides this with the internal URL `http://backend:7713`, so it normally does not need to be changed
- `ADMIN_INTERNAL_SECRET`: use the same separate random value in the backend and admin environments; do not reuse JWT or replay secrets
- `ADMIN_BOOTSTRAP_USER_IDS`: optional comma-separated backend uid or login ID list for granting the first administrator access
- `APP_ADMIN_SERVER_HOST`: add the public admin API URL to frontend `.env.dev` and `.env.production`; if omitted, ordinary pages still work and only the admin page reports a configuration error

Generate a shared secret with the analyzer utility:

```powershell
Set-Location apps/lol-replay-analyzer
npm run generate:secret
```

## Development: run all four services

```powershell
docker compose --profile development up --build
```

- Frontend: http://localhost:8080
- Backend: http://localhost:7713
- Replay analyzer: http://localhost:7720
- Admin operations API: http://localhost:7730 (bound to loopback by default)

The frontend source is bind-mounted and uses Rollup watch with live reload. Rebuild the backend and analyzer images after changing their source code:

```powershell
docker compose up -d --build backend replay-analyzer admin
```

## Production: run only the three server services

Set the following values in `apps/backend/.env.docker`:

```dotenv
IS_PROD=true
DEBUG=false
REPLAY_ANALYZER_BASE_URL=https://replay-api.teamgg.kr
RSO_CLIENT_CALLBACK_URI=https://api.teamgg.kr/v1/auth/rsoLogin
```

Set the following values in `apps/lol-replay-analyzer/.env.docker`:

```dotenv
NODE_ENV=production
CORS_ORIGINS=https://teamgg.kr,https://www.teamgg.kr
```

Set the same long random ADMIN_INTERNAL_SECRET in both apps/backend/.env.docker and apps/admin/.env.docker. Set ADMIN_ALLOWED_ORIGINS to the production frontend origins, and add the first administrator team.gg uid or login ID to backend ADMIN_BOOTSTRAP_USER_IDS. After assigning a row in user_roles, the bootstrap list can be removed.

Restrict the published server ports to the reverse proxy in the root `.env`:

```dotenv
BACKEND_BIND_ADDRESS=127.0.0.1
ANALYZER_BIND_ADDRESS=127.0.0.1
ADMIN_BIND_ADDRESS=127.0.0.1
```

The frontend belongs to the `development` profile, so this command starts only the three server services:

```bash
docker compose run --rm backend migrate
docker compose up -d --build backend replay-analyzer admin
```

Task #64 adds the numeric-key foundation without automatically processing old
rows. During a low-load window, run the bounded parent-key backfill explicitly:

```bash
docker compose run --rm backend backfill-numeric-keys
```

`NUMERIC_KEY_BACKFILL_BATCH_SIZE` defaults to `1000` (`10`-`10000`) and
`NUMERIC_KEY_BACKFILL_WORK_LIMIT` defaults to `10m` (`1s`-`1h`). Re-run the
command until parents and generic children report complete. `ready` remains
false until the compact mastery shadow is separately validated. Do not run it
while the initial Champion Detail source backfill is active. This command does
not switch constraints or remove legacy string identifiers.

The 35-million-row `masteries` table is intentionally excluded from in-place
numeric-key updates. After stopping backend writes, prepare its compact numeric
shadow with the explicit maintenance command:

```bash
docker compose stop backend
docker compose run --rm \
  -e MASTERY_NUMERIC_SHADOW_OFFLINE_ACK=true \
  backend prepare-mastery-numeric-shadow
```

`MASTERY_NUMERIC_SHADOW_BATCH_SIZE` defaults to `10000` (`100`-`100000`) and
`MASTERY_NUMERIC_SHADOW_WORK_LIMIT` defaults to `10m` (`1s`-`1h`). The command
also accepts `MASTERY_NUMERIC_SHADOW_MAX_BATCHES`; `0` disables the batch-count
cap while retaining the work limit. The command
creates no normal-startup migration and never renames or drops the legacy
table. Re-run it until `copied=true validated=true`; keep the backend stopped
until the read/write cutover procedure has been verified. Do not persist the
offline acknowledgement as `true` in a production environment file.

The command installs atomic legacy-to-shadow synchronization triggers only
after the full copy checksum passes. The backend continues to write the legacy
table, so rollback remains available. To switch reads after validation, set:

```env
MASTERY_READ_SOURCE=numeric_v2
```

Then start the backend and confirm `Mastery read source: numeric_v2` before
checking summoner mastery and statistics APIs. Set it back to `legacy` and
restart the backend for an immediate read rollback; synchronized writes keep
the legacy table current.

`MASTERY_NUMERIC_SHADOW_DISABLE_BINLOG` defaults to `false`. Set it to `true`
only after confirming there are no replicas and explicitly accepting that the
rebuildable shadow copy will not be present in point-in-time binlog recovery.
The command fails before copying if the database account cannot change the
session binlog setting. Never store either maintenance acknowledgement as
`true` after the operation.

Configure the production reverse proxy with at least a `250m` request body limit for replay uploads, a read timeout longer than `ANALYSIS_TIMEOUT`, and disabled buffering for SSE responses. Use HTTPS for both public APIs and keep the container ports bound to loopback whenever possible.

The production frontend continues to be deployed to Amazon S3:

```powershell
Set-Location apps/frontend
npm ci
npm run build
```

Upload `public/`; the production bundle is generated under `public/build/production/`.

## Management commands

```powershell
# Status
docker compose ps

# Logs
docker compose logs -f backend replay-analyzer admin

# Restart
docker compose restart backend replay-analyzer admin

# Stop the development stack
docker compose --profile development down
```

The `backend_datafiles` volume stores Data Dragon files and statistics caches. `replay_decoder_cache` stores League executables and decoder artifacts. `replay_work` stores large uploads and decoded files while an analysis is running; the application removes them after successful or failed completion. Use `docker compose down -v` only when you intentionally want to delete these persistent data and cache volumes.

These are Docker named volumes, not host bind mounts. For example, `/app/datafiles` exists inside the backend container. To find its actual storage directory on a Linux Docker host, run:

```bash
docker volume inspect teamgg_backend_datafiles
```

Read the `Mountpoint` field; rootful Docker commonly stores it under `/var/lib/docker/volumes/teamgg_backend_datafiles/_data`.
