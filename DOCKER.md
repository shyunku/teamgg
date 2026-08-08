# Running team.gg with Docker

This Compose stack runs the frontend, backend, and ROFL replay analyzer. MySQL and Redis are expected to run on the Docker host or external infrastructure.

## Initial setup

Create the Compose and service environment files from the repository root.

```powershell
Copy-Item .env.example .env
Copy-Item apps/backend/.env.docker.example apps/backend/.env.docker
Copy-Item apps/lol-replay-analyzer/.env.docker.example apps/lol-replay-analyzer/.env.docker
```

Each file has a separate responsibility:

- `.env`: published ports, bind addresses, and service environment file paths; do not put application secrets here
- `apps/backend/.env.docker`: database, Redis, Riot API, RSO, JWT, and backend runtime settings
- `apps/lol-replay-analyzer/.env.docker`: OpenAI, upload, decoder, and analyzer runtime settings
- `apps/frontend/.env.dev` and `.env.production`: existing frontend build settings

To reuse an existing non-Docker service `.env`, change only the paths in the root `.env`:

```dotenv
BACKEND_ENV_FILE=./apps/backend/.env
ANALYZER_ENV_FILE=./apps/lol-replay-analyzer/.env
```

Pay particular attention to these settings:

- `DB_HOST`: use `host.docker.internal` for a database on a Windows or macOS host, or the actual reachable hostname for a production database
- `REDIS_ADDR`: use `host.docker.internal:6379` for Redis on the host, or the actual Redis endpoint in production
- `REPLAY_ANALYZER_SHARED_SECRET`: use the same sufficiently long random value in the backend and analyzer environments
- `REPLAY_ANALYZER_BASE_URL`: the analyzer URL returned to browsers; use `http://localhost:7720` in development and a public HTTPS URL in production
- `CORS_ORIGINS`: use `http://localhost:8080` in development and `https://teamgg.kr,https://www.teamgg.kr` in production
- `TEAMGG_API_BASE_URL`: Compose overrides this with the internal URL `http://backend:7713`, so it normally does not need to be changed

Generate a shared secret with the analyzer utility:

```powershell
Set-Location apps/lol-replay-analyzer
npm run generate:secret
```

## Development: run all three services

```powershell
docker compose --profile development up --build
```

- Frontend: http://localhost:8080
- Backend: http://localhost:7713
- Replay analyzer: http://localhost:7720

The frontend source is bind-mounted and uses Rollup watch with live reload. Rebuild the backend and analyzer images after changing their source code:

```powershell
docker compose up -d --build backend replay-analyzer
```

## Production: run only the two server services

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

Restrict the published server ports to the reverse proxy in the root `.env`:

```dotenv
BACKEND_BIND_ADDRESS=127.0.0.1
ANALYZER_BIND_ADDRESS=127.0.0.1
```

The frontend belongs to the `development` profile, so this command starts only the backend and replay analyzer:

```bash
docker compose up -d --build backend replay-analyzer
```

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
docker compose logs -f backend replay-analyzer

# Restart
docker compose restart backend replay-analyzer

# Stop the development stack
docker compose --profile development down
```

The `backend_datafiles` volume stores Data Dragon files and statistics caches. `replay_decoder_cache` stores League executables and decoder artifacts. `replay_work` stores large uploads and decoded files while an analysis is running; the application removes them after successful or failed completion. Use `docker compose down -v` only when you intentionally want to delete these persistent data and cache volumes.

These are Docker named volumes, not host bind mounts. For example, `/app/datafiles` exists inside the backend container. To find its actual storage directory on a Linux Docker host, run:

```bash
docker volume inspect teamgg_backend_datafiles
```

Read the `Mountpoint` field; rootful Docker commonly stores it under `/var/lib/docker/volumes/teamgg_backend_datafiles/_data`.
