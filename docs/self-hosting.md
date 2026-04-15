# Self-Hosting Guide

## Prerequisites

- Docker and Docker Compose installed on the host
- A pre-existing reverse proxy (e.g. Nginx, Traefik, Pangolin) routing:
  - `/api/*` → backend (port 8080)
  - `/*` → frontend (port 3000)
- An SMTP server or relay for magic link delivery
- A running RustFS instance accessible from the Docker network (the project uses an external `pangolin` network to reach it)
- DNS pointing your domain to the host

---

## First boot

1. **Copy the stage template for your deployment** — e.g. `cp .env.dev.example .env.dev` for local dev, `cp .env.preprod.example .env.preprod` for staging, or `cp .env.prod.example .env.prod` for production — and fill in all required values (see [Environment variables](#environment-variables) below). Each stage gets its own isolated Postgres volume and Docker project; the Makefile wires them up.

2. **Set `SEED_ADMIN_EMAIL`** to a real inbox you control. On first startup, the backend detects that no admin exists and automatically creates the admin user + sends a magic link to that address. Subsequent restarts with the same env var are no-ops.

   > **Do not leave this at the template placeholder value.** The admin row is created once, keyed by email — if the magic link is sent to an address you do not own, you will not receive it and requesting another link from the login page will just route it to the same dead inbox. Recovery is a chore (edit the stage env file, set a different `SEED_ADMIN_EMAIL`, restart the backend — this creates a _second_ admin keyed by the new address; the first stays orphaned but harmless).

3. **Start the stack:**

   ```plain
   docker compose up --build
   ```

4. **Check your inbox.** Click the magic link to log in as admin. The link expires in 15 minutes. If it expires before you click, request a new one from the login page.

5. **Create invites** from the Admin panel so other players can register.

---

## Development vs production

The repository ships a `docker-compose.override.yml` that activates automatically in development. It:

- Starts **Mailpit** (a local SMTP catcher) on port `8025` — captured emails are viewable at `http://localhost:8025`
- Mounts source directories as volumes for hot-reload

In production, `docker-compose.override.yml` should not be present. Use `docker compose -f docker-compose.yml up` if you want to be explicit.

---

## Keeping env files in sync after an upgrade

When you `git pull` a new version of FabDoYouMeme, upstream may have added new environment variables — for example, the `PUBLIC_OPERATOR_*` variables that drive the `/privacy` page. Your existing `.env.dev` / `.env.preprod` / `.env.prod` won't have them, and Docker Compose will either warn about unset variables or silently use empty strings where defaults were expected.

A helper script, [`scripts/env-migrate.sh`](../scripts/env-migrate.sh), detects this drift and helps you migrate forward without touching any values you've customized. It auto-detects which deployments exist (dev / preprod / prod) by probing for `.env.*.example` files, so running it on a dev-only clone only processes `dev`.

```bash
make env-check      # quick summary: which deployments are out of sync? (exits 1 on drift — safe in CI)
make env-diff       # per-variable diff with reasons: "added upstream", "may be deprecated", etc.
make env-migrate    # interactive: shows preview, asks y/N, appends missing defaults with their comments
```

**What `env-migrate` does and does not do**:

- ✓ Appends variables that exist in `.env.*.example` but are missing from the live `.env.*` file, copying each variable's original comment block from the example so context travels with it.
- ✓ Variables are appended in the order they appear in the example, grouped under a dated header: `# --- Added by scripts/env-migrate.sh on YYYY-MM-DD ---`.
- ✓ Bootstraps a missing live file from the example if you confirm (useful for a fresh preprod / prod setup).
- ✗ **Never** overwrites a value you've already set — if `SMTP_HOST=smtp.ovh.net` is already in `.env.prod`, it stays that way even if the example has a different default.
- ✗ **Never** removes "extra" variables (present in live but not in example). They may be deprecated upstream or a custom override — the script flags them via `env-diff` and leaves the decision to you.

Typical upgrade flow:

```bash
git pull                      # pull new code, including new .env.*.example entries
make env-check                # "✗ prod — 3 missing" tells you there's work
make env-diff ENV=prod        # see exactly which variables and their defaults
make env-migrate ENV=prod     # append them; review the new block; edit values if needed
make prod                     # restart the stack with the updated env file
```

Scope to a single deployment with `ENV=dev|preprod|prod` on any target, or invoke the script directly for the same effect: `./scripts/env-migrate.sh check prod`.

---

## Applying database migrations

Migrations run automatically at backend startup via `golang-migrate`. To run them manually:

```bash
migrate -path ./backend/db/migrations -database "$DATABASE_URL" up
```

To roll back one migration:

```bash
migrate -path ./backend/db/migrations -database "$DATABASE_URL" down 1
```

---

## Environment variables

All variables come from `.env`. Never commit this file.

### Core infrastructure

| Variable            | Required | Default             | Description                                                                 |
| ------------------- | -------- | ------------------- | --------------------------------------------------------------------------- |
| `POSTGRES_PASSWORD` | yes      | —                   | PostgreSQL password                                                         |
| `DATABASE_URL`      | auto     | —                   | Constructed by Docker Compose from `POSTGRES_PASSWORD`; do not set manually |
| `RUSTFS_ENDPOINT`   | yes      | —                   | RustFS base URL, e.g. `https://rustfs.example.com`                          |
| `RUSTFS_ACCESS_KEY` | yes      | —                   | RustFS access key                                                           |
| `RUSTFS_SECRET_KEY` | yes      | —                   | RustFS secret key                                                           |
| `RUSTFS_BUCKET`     | no       | `fabyoumeme-assets` | S3 bucket name                                                              |

### Stage gate

These two variables must match each other. They gate the [danger zone](#danger-zone) — a set of destructive admin actions that are unmounted entirely in prod (the routes return 404, not 403). `APP_ENV` is read by the Go backend; `PUBLIC_APP_ENV` is the SvelteKit-visible mirror that the admin sidebar and danger zone page read through `$env/dynamic/public`. **Both default to `prod` when unset — fail safe.** A partially configured deploy cannot accidentally expose the danger zone.

| Variable         | Required | Default | Description                                                                                                 |
| ---------------- | -------- | ------- | ----------------------------------------------------------------------------------------------------------- |
| `APP_ENV`        | no       | `prod`  | Backend stage marker. Legal values: `dev`, `preprod`, `prod`. Gates mounting of `/api/admin/danger/*`.      |
| `PUBLIC_APP_ENV` | no       | `prod`  | Frontend stage marker. Must match `APP_ENV`. Gates rendering of the admin sidebar "Danger zone" nav entry. |

### URLs & routing

| Variable             | Required | Default | Description                                                                                                                                                                                                  |
| -------------------- | -------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `FRONTEND_URL`       | yes      | —       | Full public URL, e.g. `https://meme.example.com` — used for CORS, cookie domain, and magic link base URL                                                                                                     |
| `BACKEND_URL`        | yes      | —       | Full API URL, e.g. `https://meme.example.com/api`                                                                                                                                                            |
| `BACKEND_PORT`       | no       | `8080`  | Backend HTTP listen port                                                                                                                                                                                     |
| `TRUSTED_WS_ORIGINS` | no       | —       | Comma-separated extra origins accepted by the WebSocket upgrader (in addition to `FRONTEND_URL`). Trailing slashes are normalized. Example: `https://admin.meme.example.com,https://mobile.meme.example.com` |

### Email / SMTP

| Variable        | Required | Default | Description                                                        |
| --------------- | -------- | ------- | ------------------------------------------------------------------ |
| `SMTP_HOST`     | yes      | —       | SMTP server hostname                                               |
| `SMTP_PORT`     | no       | `587`   | SMTP port (587 = STARTTLS)                                         |
| `SMTP_USERNAME` | no       | —       | SMTP auth username (omit for unauthenticated servers like Mailpit) |
| `SMTP_PASSWORD` | no       | —       | SMTP auth password (omit for unauthenticated servers like Mailpit) |
| `SMTP_FROM`     | yes      | —       | From address, e.g. `noreply@example.com`                           |

> **GDPR note:** your SMTP provider is a data processor under GDPR Art. 28(3). A Data Processing Agreement must be in place before production use.

### Auth & sessions

| Variable                 | Required | Default | Description                                                                          |
| ------------------------ | -------- | ------- | ------------------------------------------------------------------------------------ |
| `SEED_ADMIN_EMAIL`       | no       | —       | Email for first-boot admin bootstrap. Idempotent: skipped if an admin already exists |
| `MAGIC_LINK_TTL`         | no       | `15m`   | How long a magic link is valid                                                       |
| `SESSION_TTL`            | no       | `720h`  | Session expiry (30 days); renewed on each authenticated request                      |
| `SESSION_RENEW_INTERVAL` | no       | `60m`   | How often the hub renews sessions for long-running WebSocket connections             |

### WebSocket

| Variable                 | Required | Default | Description                                                                     |
| ------------------------ | -------- | ------- | ------------------------------------------------------------------------------- |
| `RECONNECT_GRACE_WINDOW` | no       | `30s`   | How long a disconnected player can be absent before being removed from the room |
| `WS_RATE_LIMIT`          | no       | `20`    | Maximum messages per second per WebSocket connection                            |
| `WS_READ_LIMIT_BYTES`    | no       | `4096`  | Maximum WebSocket frame size; exceeding this disconnects the client             |
| `WS_PING_INTERVAL`       | no       | `25s`   | Client-side ping interval                                                       |
| `WS_READ_DEADLINE`       | no       | `60s`   | Server-side read deadline per connection; reset on each pong                    |

### Rate limits

All rate limits are enforced in-memory per backend process. This is correct for a single-host deployment. For multi-instance deployments, rate limit state must be externalized to Redis.

| Variable                           | Required | Default | Description                                                         |
| ---------------------------------- | -------- | ------- | ------------------------------------------------------------------- |
| `RATE_LIMIT_AUTH_RPM`              | no       | `10`    | Max requests/minute per IP on all `POST /api/auth/*` endpoints      |
| `RATE_LIMIT_INVITE_VALIDATION_RPH` | no       | `20`    | Max registration attempts/hour per IP (prevents invite brute-force) |
| `RATE_LIMIT_ROOMS_RPH`             | no       | `10`    | Max room creations/hour per user                                    |
| `RATE_LIMIT_UPLOADS_RPH`           | no       | `50`    | Max upload URL requests/hour per admin                              |
| `RATE_LIMIT_GLOBAL_RPM`            | no       | `100`   | Max requests/minute per IP on all other `GET /api/*` endpoints      |

### Uploads

| Variable                | Required | Default   | Description                        |
| ----------------------- | -------- | --------- | ---------------------------------- |
| `MAX_UPLOAD_SIZE_BYTES` | no       | `2097152` | Maximum upload size (default 2 MB) |

### Logging

| Variable    | Required | Default | Description                                    |
| ----------- | -------- | ------- | ---------------------------------------------- |
| `LOG_LEVEL` | no       | `info`  | Verbosity: `debug` / `info` / `warn` / `error` |

### Legal / privacy policy

The Privacy Policy served at `/privacy` is rendered from a Svelte template at [`frontend/src/routes/(public)/privacy/+page.svelte`](../frontend/src/routes/(public)/privacy/+page.svelte), with the operator-specific fields injected at runtime via these `PUBLIC_*` environment variables. They are read through SvelteKit's `$env/dynamic/public`, so updating any of them only requires `docker compose up -d` — no rebuild.

| Variable                       | Required | Default          | Description                                                                                                                                                                                                  |
| ------------------------------ | -------- | ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `PUBLIC_OPERATOR_NAME`         | yes      | —                | How you want to be identified as the data controller under GDPR Art. 13(1)(a). Free-form — write what matches your legal status. Examples: `Jane Doe (natural person, non-commercial)`, `Acme Corp S.A.S.`. |
| `PUBLIC_OPERATOR_CONTACT_EMAIL`| yes      | —                | Inbox you monitor for all privacy requests (access, erasure, rectification, objection, complaints). Rendered as a `mailto:` link on the page.                                                                |
| `PUBLIC_OPERATOR_URL`          | no       | `${FRONTEND_URL}`| Public URL of this instance, shown under "Hosted at". Defaults to `FRONTEND_URL` which is almost always what you want; override only if your privacy policy needs to advertise a different canonical URL.    |
| `PUBLIC_OPERATOR_SMTP_PROVIDER`| yes      | —                | Free-form description of the SMTP provider handling magic-link delivery, for the Art. 28 processor disclosure. Include company + country. Example: `OVHcloud (OVH SAS, France) — EU-only transactional relay`. |

> **GDPR compliance gate.** If any of the three required variables above is unset, the `/privacy` page renders a visible red banner at the top warning that operator configuration is incomplete. Do not launch your instance until these values are set — the consent checkbox at registration links to this page, so incomplete content is a registration-time compliance failure.

See [`docs/reference/privacy-policy.md`](reference/privacy-policy.md) for the full reference copy of the rendered policy content (templates, tables, and exact legal wording) — that file is the source of truth for the wording and structure of the Svelte page.

---

## Secrets rotation

| Secret                                    | Impact when rotated                         | Procedure                                                           |
| ----------------------------------------- | ------------------------------------------- | ------------------------------------------------------------------- |
| `POSTGRES_PASSWORD`                       | All backend connections drop                | Update `.env`, restart backend + postgres                           |
| `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY` | All asset operations fail                   | Rotate in RustFS admin console, update `.env`, restart backend      |
| `SMTP_PASSWORD`                           | Email delivery fails (magic links broken)   | Update `.env`, restart backend                                      |
| Session tokens (in DB)                    | None — tokens are random and self-contained | Invalidate individual sessions by deleting rows in `sessions` table |

---

## Danger zone

The admin UI exposes a set of destructive actions under **Admin → Danger zone** that permanently delete data and cannot be undone. These actions exist to reset a dev or preprod deployment to a known state without tearing down the whole Docker stack (which would also lose the database volume).

**They are unmounted entirely in production.** When `APP_ENV=prod` (the default), the backend does not register the `/api/admin/danger/*` routes at all — requests return `404 not found`, not `403 forbidden`. The frontend mirrors this: the sidebar nav link is hidden and the `/admin/danger` page throws a 404 load error when `PUBLIC_APP_ENV=prod`. Both variables default to `prod` when unset, so a partially configured `.env` cannot accidentally expose the feature.

To enable the danger zone in dev or preprod, set **both** `APP_ENV` and `PUBLIC_APP_ENV` to `dev` or `preprod` in the corresponding `.env` file (see [Stage gate](#stage-gate) above). Each action requires typing an exact confirmation phrase in a modal; the phrase is also sent as a JSON body field and validated server-side, so a forged request without the exact phrase is rejected with `400 invalid_confirmation`.

| Action                    | Deletes                                                                                                                                         | Preserves                                                                                                               |
| ------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| Wipe game history         | All rooms, rounds, submissions, and votes                                                                                                       | Packs, items, users, invites, sessions, object storage                                                                  |
| Wipe packs and media      | All packs, items, pack media rows, AND all game history that depends on them. Empties the entire object storage bucket via `storage.Purge("")` | Users, invites, sessions                                                                                                |
| Wipe invites              | All invite tokens                                                                                                                               | Everything else — existing users stay logged in and keep their accounts. Only future sign-ups are affected.             |
| Force logout everyone     | All sessions and magic-link tokens **except the acting admin's own**                                                                            | The acting admin's session. All other users are bounced to login on their next request.                                |
| Full reset to first boot  | Everything above AND all non-protected users                                                                                                    | The acting admin, the bootstrap admin, the sentinel user (for GDPR-anonymized records), and game type seed data        |

Every invocation is written to the admin audit log with the action name, the acting admin ID, and a JSON summary of what was deleted. Check [`docs/reference/decisions.md`](reference/decisions.md) (ADR-012) for the design rationale — in particular why routes are unmounted rather than returning 403.

---

## Network topology

| Service  | Internal network | Pangolin (external) |
| -------- | ---------------- | ------------------- |
| postgres | ✓                |                     |
| backend  | ✓                |                     |
| frontend | ✓                | ✓                   |

PostgreSQL and the backend are not reachable outside the Docker internal network. The frontend sits on the `pangolin` external network so the reverse proxy can route traffic to it. RustFS lives in a separate Docker stack on the same `pangolin` network; the backend reaches it via `RUSTFS_ENDPOINT`.
