# team.gg Admin Server

The admin server is a narrow gateway for the team.gg operations dashboard. It does not receive the team.gg JWT signing key, database credentials, or Docker socket.

## Responsibilities

- validate a browser access token through the backend internal authorization endpoint;
- expose bounded service-health, operational-metric, event, and audit views;
- enforce an explicit Origin allowlist and request timeouts;
- redact credential-like fields before returning or persisting metadata;
- write administrator reads to the backend audit log.

## Local run

Set the variables documented in `.env.docker.example` in your shell, then run:

```bash
go run .
```

The default port is `7730`. `ADMIN_INTERNAL_SECRET` must match the backend value. Add an administrator through `user_roles`, or set `ADMIN_BOOTSTRAP_USER_IDS` on the backend to a comma-separated team.gg UID or login ID during initial setup.
