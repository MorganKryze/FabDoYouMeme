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
| `POST`  | `/api/rooms`                   | Create a room with game type, pack(s), mode, and config. Body requires one `pack_id` per role declared by the game type: `pack_id` (role `image`, required by every game type) and `text_pack_id` (role `text`, required only by `meme-vote`). Config accepts `round_count`, `round_duration_seconds`, `voting_duration_seconds`, `host_paced`, `joker_count` (default `ceil(round_count/5)`, `0` disables), `allow_skip_vote` (default `true`), and — for `meme-vote` — `hand_size` |
| `GET`   | `/api/rooms/:code`             | Get room info — state, players, config, game type, `pack_id`, and `text_pack_id` (nullable) |
| `PATCH` | `/api/rooms/:code/config`      | Update room config (host only, lobby state only). Accepts a **partial patch** — send only the fields you changed. `joker_count` must satisfy `0 ≤ joker_count ≤ round_count`; violations return `422 invalid_config` |
| `POST`  | `/api/rooms/:code/leave`       | Leave the room (lobby only; host leaving closes the room) |
| `POST`  | `/api/rooms/:code/kick`        | Remove a player (host or admin, **lobby only**). Body: exactly one of `{ "user_id": "<uuid>" }` or `{ "guest_player_id": "<uuid>" }`. Writes a `room_bans` row so the player cannot rejoin (WS handshake returns `409 banned_from_room`). Errors: `403 forbidden`, `409 room_not_in_lobby`, `409 cannot_kick_self` (host used `/end` instead), `400 bad_request` (neither or both id fields supplied) |
| `POST`  | `/api/rooms/:code/end`         | End the room (host or admin): lobby→hard-delete, playing→persist as finished |
| `GET`   | `/api/rooms/:code/leaderboard` | Final leaderboard (finished rooms only)                   |

### Game types (`/api/game-types/*`)

Requires session.

| Method | Path                    | Description                                                         |
| ------ | ----------------------- | ------------------------------------------------------------------- |
| `GET`  | `/api/game-types`       | List all registered game types. Each entry includes `required_packs: [{ role, payload_versions }]` so the UI knows which pack slots to render per type |
| `GET`  | `/api/game-types/:slug` | Details, config schema, and `required_packs` for one type           |

### Packs, items, and versions

Requires session (read); owner or admin (write).

| Method   | Path                                                  | Description                                     |
| -------- | ----------------------------------------------------- | ----------------------------------------------- |
| `GET`    | `/api/packs`                                          | List packs — visibility rules apply (see below). Supports filtering via `?game_type=<slug>&role=<image\|text>`; the handler drops packs that contain zero items compatible with the role's payload-version set |
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
| `skip_submit`   | Player spends a joker to skip the submit phase (platform-level; valid only during submit phase) |
| `skip_vote`     | Player abstains from voting (platform-level; valid only during voting phase)                    |

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
| `player_submitted`   | A player submitted (anonymous — only `user_id`/`player_id`)            |
| `player_skipped_submit` | A player spent a joker — includes `jokers_remaining` for the sender |
| `submissions_closed` | Submission phase ended, voting opens                                   |
| `player_skipped_vote` | A player abstained from voting                                        |
| `vote_results`       | Scores for the completed round; server-paced mode also includes `next_round_at` and `results_duration_seconds` |
| `game_ended`         | Final leaderboard with `reason` field                                  |
| `room_closed`        | Host or admin terminated the room — clients must disconnect            |
| `room_state`         | Full snapshot sent on reconnect                                        |
| `error`              | Server-side error with `code` and `message`                            |

#### `room_closed`

Sent when a host or admin terminates the room via `POST /api/rooms/:code/end`.
Clients must treat this message as terminal: disconnect the socket (do not
auto-reconnect), surface the reason to the user, and navigate away from the
room.

The server's DB action after broadcasting `room_closed` is state-dependent: a
lobby room is hard-deleted (disappears from history), while a playing room is
marked `finished` so gameplay data is preserved for the leaderboard.

```json
{ "type": "room_closed", "data": { "reason": "ended_by_host" } }
```

**Reasons:**

- `ended_by_host` — the room's host clicked End Room.
- `ended_by_admin` — an admin (not the host) clicked End Room.

### Timer contract

Every timed event includes both `duration_seconds` (server-authoritative) and `ends_at` (ISO 8601 timestamp for client countdown display). The server enforces the timer; the client uses `ends_at` to render a countdown without clock-drift accumulation.

### Rate limits and message limits

- Max message size: `WS_READ_LIMIT_BYTES` (default 4 KB) — exceeding this disconnects the client
- Max message rate: `WS_RATE_LIMIT` per second (default 20) — exceeding this disconnects the client

For game-specific message payloads and the room/round lifecycle behind these messages, see `docs/game-engine.md`.
