# API

## Overview

The backend serves a REST API at `/api/*` and a WebSocket endpoint at `/api/ws/rooms/:code`. All API endpoints are prefixed with `/api` and served on port 8080 (internal only; the reverse proxy routes externally).

Authentication uses a `session` cookie set by `POST /api/auth/verify`. All authenticated endpoints require this cookie to be present and valid.

---

## REST endpoint groups

### Auth (`/api/auth/*`)

Public endpoints — no session required.

| Method | Path                   | Description                                                            |
| ------ | ---------------------- | ---------------------------------------------------------------------- |
| `POST` | `/api/auth/register`   | Register with invite token, username, email, and explicit consent      |
| `POST` | `/api/auth/magic-link` | Request a magic login link — always returns `200`                      |
| `POST` | `/api/auth/verify`     | Verify a magic link token and create a session                         |
| `POST` | `/api/auth/logout`     | Delete the current session (session required)                          |
| `GET`  | `/api/auth/me`         | Return current user `{ id, username, email, role }` (session required) |

See [auth-and-identity.md](auth-and-identity.md) for full flow documentation.

### User profile (`/api/users/*`)

Requires session.

| Method  | Path                    | Description                                     |
| ------- | ----------------------- | ----------------------------------------------- |
| `PATCH` | `/api/users/me`         | Update own username or request email change     |
| `GET`   | `/api/users/me/history` | Paginated past rooms and final scores           |
| `GET`   | `/api/users/me/export`  | Export all personal data as JSON (GDPR Art. 20) |

### Rooms (`/api/rooms/*`)

Requires session.

| Method  | Path                           | Description                                               |
| ------- | ------------------------------ | --------------------------------------------------------- |
| `POST`  | `/api/rooms`                   | Create a room with game type, pack, mode, and config      |
| `GET`   | `/api/rooms/:code`             | Get room info — state, players, config, game type         |
| `PATCH` | `/api/rooms/:code/config`      | Update room config (host only, lobby state only)          |
| `POST`  | `/api/rooms/:code/leave`       | Leave the room (lobby only; host leaving closes the room) |
| `POST`  | `/api/rooms/:code/kick`        | Kick a player by `user_id` (host only, lobby or playing)  |
| `GET`   | `/api/rooms/:code/leaderboard` | Final leaderboard (finished rooms only)                   |

### Game types (`/api/game-types/*`)

Requires session.

| Method | Path                    | Description                                                         |
| ------ | ----------------------- | ------------------------------------------------------------------- |
| `GET`  | `/api/game-types`       | List all registered game types                                      |
| `GET`  | `/api/game-types/:slug` | Details, config schema, and supported payload versions for one type |

### Packs, items, and versions

Requires session (read); owner or admin (write).

| Method   | Path                                                  | Description                                     |
| -------- | ----------------------------------------------------- | ----------------------------------------------- |
| `GET`    | `/api/packs`                                          | List packs — visibility rules apply (see below) |
| `POST`   | `/api/packs`                                          | Create a pack                                   |
| `GET`    | `/api/packs/:id`                                      | Get pack details                                |
| `PATCH`  | `/api/packs/:id`                                      | Update name, description, or visibility         |
| `DELETE` | `/api/packs/:id`                                      | Soft-delete a pack                              |
| `GET`    | `/api/packs/:id/items`                                | List items in a pack                            |
| `POST`   | `/api/packs/:id/items`                                | Add an item                                     |
| `PATCH`  | `/api/packs/:id/items/:item_id`                       | Edit item metadata or payload                   |
| `DELETE` | `/api/packs/:id/items/:item_id`                       | Remove an item                                  |
| `PATCH`  | `/api/packs/:id/items/reorder`                        | Bulk reorder — full position map required       |
| `GET`    | `/api/packs/:id/items/:item_id/versions`              | List all versions of an item                    |
| `POST`   | `/api/packs/:id/items/:item_id/versions`              | Save a new version                              |
| `POST`   | `/api/packs/:id/items/:item_id/versions/:vid/restore` | Restore a previous version as current           |
| `DELETE` | `/api/packs/:id/items/:item_id/versions/:vid`         | Move version to 30-day deletion bin             |
| `DELETE` | `/api/packs/:id/items/:item_id/versions/:vid/purge`   | Hard-purge immediately (admin only)             |

**Pack visibility rules:**

- Admins see all packs regardless of visibility or status
- Authenticated users see: their own packs + all `public` packs with `status = 'active'`
- `banned` packs are excluded from all listings except for the owning admin

### Assets (`/api/assets/*`)

Requires owner or admin.

| Method | Path                       | Description                                                                               |
| ------ | -------------------------- | ----------------------------------------------------------------------------------------- |
| `POST` | `/api/assets/upload-url`   | Request a pre-signed RustFS upload URL (after MIME + magic byte validation)               |
| `POST` | `/api/assets/download-url` | Get a pre-signed download URL (for admin/owner preview — in-game URLs come via WebSocket) |

### Admin (`/api/admin/*`)

Requires session + admin role.

| Method   | Path                           | Description                                         |
| -------- | ------------------------------ | --------------------------------------------------- |
| `GET`    | `/api/admin/users`             | List users (paginated, searchable)                  |
| `PATCH`  | `/api/admin/users/:id`         | Update role, is_active, email, or username          |
| `DELETE` | `/api/admin/users/:id`         | Hard-delete a user (GDPR erasure)                   |
| `GET`    | `/api/admin/invites`           | List all invites (paginated)                        |
| `POST`   | `/api/admin/invites`           | Create an invite                                    |
| `DELETE` | `/api/admin/invites/:id`       | Revoke an invite                                    |
| `GET`    | `/api/admin/notifications`     | List admin notifications (pack published/modified)  |
| `PATCH`  | `/api/admin/notifications/:id` | Mark notification as read                           |
| `PATCH`  | `/api/packs/:id/status`        | Set pack status to `active`, `flagged`, or `banned` |

### Observability

| Method | Path               | Auth         | Description                                                 |
| ------ | ------------------ | ------------ | ----------------------------------------------------------- |
| `GET`  | `/api/health`      | None         | Liveness — always `200` if the process is running           |
| `GET`  | `/api/health/deep` | None         | Readiness — checks DB and RustFS; returns `503` if degraded |
| `GET`  | `/api/metrics`     | IP allowlist | Prometheus-format metrics — never expose publicly           |

---

## Error response format

All errors return the same shape:

```plain
{ "error": "Human-readable message", "code": "snake_case_error_code", "request_id": "..." }
```

| HTTP status | Meaning                                                             |
| ----------- | ------------------------------------------------------------------- |
| `400`       | Bad request — malformed JSON, missing fields, or validation failure |
| `401`       | No valid session                                                    |
| `403`       | Valid session but insufficient role                                 |
| `404`       | Resource not found                                                  |
| `409`       | Conflict — duplicate, or invalid state transition                   |
| `422`       | Validation error — field out of range, pack compatibility failure   |
| `429`       | Rate limited                                                        |
| `500`       | Internal error — details are never exposed                          |

The `request_id` field matches the `X-Request-ID` response header, which is logged on the server side. Use it to correlate client errors with server logs.

Full error code table: `docs/reference/error-codes.md`.

---

## Pagination

List endpoints that can return unbounded rows use cursor-based pagination.

**Request parameters:**

- `limit` — max rows per page (default 50, max 100)
- `after` — opaque cursor from the previous response's `next_cursor`

**Response envelope:**

```json
{ "data": [...], "next_cursor": "opaque_or_null", "total": 142 }
```

`next_cursor` is `null` when there are no more pages. The cursor is treated as opaque by clients.

---

## WebSocket

```plain
WS /api/ws/rooms/:code
```

Authentication happens during the HTTP upgrade — the session cookie is validated against the database. Unauthenticated upgrades are rejected with `401` before the handshake completes. Cross-origin upgrades are rejected via origin check.

All messages are JSON with a `type` field and an optional `data` object.

### Message types

**Client → server:**

| Type            | When                                                     |
| --------------- | -------------------------------------------------------- |
| `join`          | Player enters the lobby                                  |
| `reconnect`     | Player re-establishes connection within the grace window |
| `start`         | Host starts the game                                     |
| `next_round`    | Host advances to the next round                          |
| `ping`          | Keepalive heartbeat                                      |
| `{slug}:submit` | Player submits their answer (game-type-specific payload) |
| `{slug}:vote`   | Player casts a vote (game-type-specific payload)         |

**Server → client:**

| Type                 | When                                                                   |
| -------------------- | ---------------------------------------------------------------------- |
| `pong`               | Response to `ping`                                                     |
| `player_joined`      | A player joins the lobby                                               |
| `player_left`        | A player's grace window expired                                        |
| `player_kicked`      | Host kicked a player                                                   |
| `reconnecting`       | A player entered the grace window                                      |
| `game_started`       | Host started the game                                                  |
| `round_started`      | New round begins — includes timer info (`duration_seconds`, `ends_at`) |
| `submissions_closed` | Submission phase ended, voting opens                                   |
| `vote_results`       | Scores for the completed round                                         |
| `game_ended`         | Final leaderboard with `reason` field                                  |
| `room_state`         | Full snapshot sent on reconnect                                        |
| `error`              | Server-side error with `code` and `message`                            |

### Timer contract

Every timed event includes both `duration_seconds` (server-authoritative) and `ends_at` (ISO 8601 timestamp for client countdown display). The server enforces the timer; the client uses `ends_at` to render a countdown without clock-drift accumulation.

### Rate limits and message limits

- Max message size: `WS_READ_LIMIT_BYTES` (default 4 KB) — exceeding this disconnects the client
- Max message rate: `WS_RATE_LIMIT` per second (default 20) — exceeding this disconnects the client

For game-specific message payloads and the room/round lifecycle behind these messages, see `docs/game-engine.md`.
