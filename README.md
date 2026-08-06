# team.gg

This monorepo contains the team.gg League of Legends match history, custom-game team builder, statistics, and ROFL replay analysis services.

## Repository structure

```text
apps/frontend/             Svelte web client
apps/backend/              Go API server
apps/lol-replay-analyzer/  Node.js/TypeScript ROFL analysis server
```

## Development

Create the environment files for each service, then start all three services with Docker Compose.

```powershell
Copy-Item .env.compose.example .env.compose
Copy-Item apps/backend/.env.docker.example apps/backend/.env
Copy-Item apps/lol-replay-analyzer/.env.docker.example apps/lol-replay-analyzer/.env
docker compose --env-file .env.compose --profile development up --build
```

See [DOCKER.md](./DOCKER.md) for detailed Docker and deployment instructions.

## Production

When the frontend is hosted on Amazon S3, start only the backend and replay analyzer services.

```powershell
docker compose --env-file .env.compose up -d --build backend replay-analyzer
```
