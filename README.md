# team.gg

This monorepo contains the team.gg League of Legends match history, custom-game team builder, statistics, and ROFL replay analysis services.

## Repository structure

```text
apps/frontend/             Svelte web client
apps/backend/              Go API server
apps/lol-replay-analyzer/  Node.js/TypeScript ROFL analysis server
apps/admin/                Go administrator operations API
```

## Development

Create the environment files for each service, then start all three services with Docker Compose.

```powershell
Copy-Item .env.example .env
Copy-Item apps/backend/.env.docker.example apps/backend/.env.docker
Copy-Item apps/lol-replay-analyzer/.env.docker.example apps/lol-replay-analyzer/.env.docker
Copy-Item apps/admin/.env.docker.example apps/admin/.env.docker
docker compose --profile development up --build
```

See [DOCKER.md](./DOCKER.md) for detailed Docker and deployment instructions.

## Production

When the frontend is hosted on Amazon S3, start the three server services.

```powershell
docker compose up -d --build backend replay-analyzer admin
```

## License

team.gg is licensed under the [GNU General Public License v3.0](./LICENSE) (`GPL-3.0-only`).
