# Docker Environments Design

**Date:** 2026-04-08
**Status:** Approved

## Problem

The current setup has a single `docker-compose.yml` at the repo root with a single `docker-compose.override.yml` for dev. This conflates environment concerns: the base file already has `pangolin` wired in (unsuitable for local dev), and there is no clean path to a prod deployment using pre-built GitHub images. A clearer, more explicit multi-environment structure is needed.

## Goal

Introduce a `docker/` folder containing a shared base compose file and three environment-specific overlays. A `Makefile` provides named targets for each environment. Separate `.env.*` files hold per-environment secrets/config.

---

## File Structure

```
docker/
├── compose.base.yml      # Shared service definitions (no build/image, no pangolin)
├── compose.dev.yml       # Dev-local: ports, mailpit, local builds, no pangolin
├── compose.preprod.yml   # Pre-prod: expose, pangolin, local builds
└── compose.prod.yml      # Prod: expose, pangolin, ghcr.io images

.env.dev                  # Dev-local secrets/config (gitignored)
.env.preprod              # Pre-prod secrets/config (gitignored)
.env.prod                 # Prod secrets/config (gitignored)
.env.dev.example          # Committed example (secrets redacted)
.env.preprod.example      # Committed example (secrets redacted)
.env.prod.example         # Committed example (secrets redacted)

Makefile                  # Targets: dev, preprod, prod, dev-down, preprod-down, prod-down
```

The existing `docker-compose.yml` and `docker-compose.override.yml` at the repo root are deleted.

The `.gitignore` gains entries for `.env.dev`, `.env.preprod`, `.env.prod`.

---

## compose.base.yml

Defines the shared skeleton for all three services:

- **`postgres`**: `postgres:17-alpine`, env vars, volume mount, healthcheck, `expose: 5432`, on `project_network`.
- **`backend`**: all env vars, `depends_on: postgres (healthy)`, `expose: 8080`, on `project_network`. No `build` or `image` key — provided by overlay.
- **`frontend`**: env vars, `depends_on: backend`, `expose: 3000`, on `project_network`. No `build` or `image` key — provided by overlay.
- **`volumes`**: `postgres_data`.
- **`networks`**: `project_network` (bridge, created by Compose). No `pangolin` here.

---

## compose.dev.yml

Dev-local environment. No pangolin — services are accessed directly via `localhost` ports.

- **`mailpit`**: `axllent/mailpit:latest`, `ports: 8025:8025`, `expose: 1025`, on `project_network`.
- **`backend`**: `build: ./backend`, overrides SMTP env vars (`SMTP_HOST: mailpit`, `SMTP_PORT: 1025`, blank credentials, `SMTP_FROM: noreply@fabyoumeme.local`), `LOG_LEVEL: debug`.
- **`frontend`**: `build: ./frontend`, `ports: 3000:3000`.
- **`postgres`**: `ports: 5432:5432` (for local DB inspection tools).
- No `pangolin` network declared.

---

## compose.preprod.yml

Pre-production environment. Uses locally-built images, exposed via pangolin reverse proxy.

- **`backend`**: `build: ./backend`.
- **`frontend`**: `build: ./frontend`, adds `pangolin` network.
- **`networks`**: declares `pangolin` as `external: true`.

---

## compose.prod.yml

Production environment. Uses pre-built images from GitHub Container Registry.

- **`backend`**: `image: ghcr.io/morgankryze/fabyoumeme-backend:latest`.
- **`frontend`**: `image: ghcr.io/morgankryze/fabyoumeme-frontend:latest`, adds `pangolin` network.
- **`networks`**: declares `pangolin` as `external: true`.

---

## Makefile

```makefile
.PHONY: dev dev-down preprod preprod-down prod prod-down

dev:
	docker compose -f docker/compose.base.yml -f docker/compose.dev.yml --env-file .env.dev up --build -d

dev-down:
	docker compose -f docker/compose.base.yml -f docker/compose.dev.yml --env-file .env.dev down

preprod:
	docker compose -f docker/compose.base.yml -f docker/compose.preprod.yml --env-file .env.preprod up --build -d

preprod-down:
	docker compose -f docker/compose.base.yml -f docker/compose.preprod.yml --env-file .env.preprod down

prod:
	docker compose -f docker/compose.base.yml -f docker/compose.prod.yml --env-file .env.prod up -d

prod-down:
	docker compose -f docker/compose.base.yml -f docker/compose.prod.yml --env-file .env.prod down
```

---

## .gitignore additions

```
.env.dev
.env.preprod
.env.prod
```

---

## Verification

1. `make dev` — all services start; `localhost:3000` serves frontend, `localhost:8025` shows Mailpit, backend reachable at `localhost:8080`.
2. `make preprod` — services start with local builds; frontend accessible via pangolin-routed domain.
3. `make prod` — services start from `ghcr.io` images; no local build step.
4. `make dev-down` / `make preprod-down` / `make prod-down` — all containers stop cleanly.
5. `git status` confirms `.env.dev`, `.env.preprod`, `.env.prod` are not tracked.
