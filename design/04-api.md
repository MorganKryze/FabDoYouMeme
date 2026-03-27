# 04 — API Surface

## REST (Go + chi)

All endpoints are prefixed with `/api`. The backend serves on port `8080` (internal only; reverse proxy routes `/api/*` externally).

### Authentication

| Method | Path                   | Auth    | Description                                  |
| ------ | ---------------------- | ------- | -------------------------------------------- |
| `POST` | `/api/auth/register`   | —       | Register with invite token                   |
| `POST` | `/api/auth/magic-link` | —       | Request magic link (always 200)              |
| `POST` | `/api/auth/verify`     | —       | Verify magic link token, create session      |
| `POST` | `/api/auth/logout`     | session | Logout (deletes session row)                 |
| `GET`  | `/api/auth/me`         | session | Current user `{ id, username, email, role }` |

### User Profile

| Method  | Path                    | Auth    | Description                                                                               |
| ------- | ----------------------- | ------- | ----------------------------------------------------------------------------------------- |
| `PATCH` | `/api/users/me`         | session | Update own `username` or request email change via `{ email }`                             |
| `GET`   | `/api/users/me/history` | session | Past rooms + final scores `[{ room_code, game_type, pack_name, score, rank, played_at }]` |

### Admin — User & Invite Management

| Method   | Path                     | Auth  | Description                                        |
| -------- | ------------------------ | ----- | -------------------------------------------------- |
| `GET`    | `/api/admin/users`       | admin | List all users (paginated — see Pagination)        |
| `PATCH`  | `/api/admin/users/:id`   | admin | Update `role`, `is_active`, `email`, or `username` |
| `DELETE` | `/api/admin/users/:id`   | admin | Hard-delete a user — see GDPR note below           |
| `GET`    | `/api/admin/invites`     | admin | List all invites (paginated)                       |
| `POST`   | `/api/admin/invites`     | admin | Create invite                                      |
| `DELETE` | `/api/admin/invites/:id` | admin | Revoke invite                                      |

### Game Types

| Method | Path                    | Auth    | Description                                      |
| ------ | ----------------------- | ------- | ------------------------------------------------ |
| `GET`  | `/api/game-types`       | session | List all available game types                    |
| `GET`  | `/api/game-types/:slug` | session | Details + config schema for a specific game type |

### Packs & Items

| Method   | Path                            | Auth    | Description                                                                                                                   |
| -------- | ------------------------------- | ------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `GET`    | `/api/packs`                    | session | List active packs (id, name, description, item count) — paginated                                                             |
| `POST`   | `/api/packs`                    | admin   | Create pack                                                                                                                   |
| `GET`    | `/api/packs/:id`                | admin   | Get pack details                                                                                                              |
| `DELETE` | `/api/packs/:id`                | admin   | Soft-delete pack                                                                                                              |
| `GET`    | `/api/packs/:id/items`          | session | List items in a pack — players see `id, position, media_url?, payload`; admins additionally see `payload_version, created_at` |
| `POST`   | `/api/packs/:id/items`          | admin   | Add item (payload only, no image)                                                                                             |
| `PATCH`  | `/api/packs/:id/items/:item_id` | admin   | Edit item metadata, payload, or confirm `media_key` after upload                                                              |
| `DELETE` | `/api/packs/:id/items/:item_id` | admin   | Remove item                                                                                                                   |
| `PATCH`  | `/api/packs/:id/items/reorder`  | admin   | Bulk reorder items `{ positions: [{ id, position }] }` — see reorder note below                                               |

### Rooms

| Method  | Path                           | Auth           | Description                                                                             |
| ------- | ------------------------------ | -------------- | --------------------------------------------------------------------------------------- |
| `POST`  | `/api/rooms`                   | session        | Create room `{ game_type_id, pack_id, mode, config }`                                   |
| `GET`   | `/api/rooms/:code`             | session        | Get room info (state, players, config, game type)                                       |
| `PATCH` | `/api/rooms/:code/config`      | session (host) | Update room config — only while `state = 'lobby'`                                       |
| `POST`  | `/api/rooms/:code/leave`       | session        | Leave the room — only while `state = 'lobby'`; host leaving closes the room             |
| `POST`  | `/api/rooms/:code/kick`        | session (host) | Kick a player `{ user_id }` — valid in `lobby` or `playing`; broadcasts `player_kicked` |
| `GET`   | `/api/rooms/:code/leaderboard` | session        | Fetch final leaderboard — only when `state = 'finished'`                                |

### Assets

| Method | Path                       | Auth    | Description                                                                            |
| ------ | -------------------------- | ------- | -------------------------------------------------------------------------------------- |
| `POST` | `/api/assets/upload-url`   | admin   | Request pre-signed upload URL `{ pack_id, item_id, filename, mime_type, size_bytes }`  |
| `POST` | `/api/assets/download-url` | session | Get pre-signed download URL for an item (admin preview only; in-game URLs come via WS) |

### Observability

| Method | Path               | Auth | Description                                                                                          |
| ------ | ------------------ | ---- | ---------------------------------------------------------------------------------------------------- |
| `GET`  | `/api/health`      | —    | Liveness: always `200 {"status":"ok"}` if process is up                                              |
| `GET`  | `/api/health/deep` | —    | Readiness: checks DB ping + Rustfs HEAD request; `200` or `503 {"status":"degraded","checks":{...}}` |
| `GET`  | `/api/metrics`     | —    | Prometheus-format metrics (bind to internal port or IP-restrict — never expose publicly)             |

---

## Error Response Shape

All REST errors return JSON:

```json
{ "error": "human-readable message", "code": "snake_case_error_code" }
```

| HTTP Code | Meaning                                                    |
| --------- | ---------------------------------------------------------- |
| `400`     | Bad request (malformed JSON, missing required fields)      |
| `401`     | Unauthenticated (no valid session)                         |
| `403`     | Forbidden (valid session, insufficient role)               |
| `404`     | Not found                                                  |
| `409`     | Conflict (duplicate username, invite already used/expired) |
| `422`     | Validation error (field out of range, invalid enum value)  |
| `500`     | Internal server error (never exposes internal details)     |

---

## WebSocket

```plain
WS /api/ws/rooms/:code
```

All messages are JSON with a `type` field and an optional `data` object. Authentication happens during the HTTP upgrade: the session cookie is validated against the `sessions` table. Unauthenticated upgrade requests are rejected with `401` before the WebSocket handshake completes.

### Type Namespace

- **Lifecycle** types (`join`, `start`, `game_ended`, …) are shared across all game types.
- **Game-specific** types are prefixed with the game type slug: `meme-caption:submit`, `trivia:answer`, etc.
- The backend rejects any game-type-prefixed message whose prefix does not match `room.game_type.slug`.

---

### Client → Server Messages

| Type            | When                                                 | Payload                                                                           |
| --------------- | ---------------------------------------------------- | --------------------------------------------------------------------------------- |
| `join`          | Player enters lobby                                  | `{}` — rejected if `room.state != 'lobby'` with `{"code":"game_already_started"}` |
| `reconnect`     | Player re-establishes connection within grace window | `{}` — triggers `room_state` snapshot instead of `player_joined` broadcast        |
| `start`         | Host starts the game                                 | `{}` — only host; rejected for non-host with `{"code":"not_host"}`                |
| `next_round`    | Host advances to next round                          | `{}` — only host; no-op if current round not yet ended                            |
| `ping`          | Keepalive heartbeat                                  | `{}` — sent every 25 seconds by client                                            |
| `{slug}:submit` | Player submits their answer                          | Game-type-specific; see game type protocols below                                 |
| `{slug}:vote`   | Player casts vote                                    | Game-type-specific; see game type protocols below                                 |

### Server → Client Messages

| Type                 | When                                      | Payload                                                                                                                                                                                                                                                      |
| -------------------- | ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `pong`               | Response to client `ping`                 | `{}`                                                                                                                                                                                                                                                         |
| `player_joined`      | Player joins the lobby                    | `{ user_id, username }`                                                                                                                                                                                                                                      |
| `player_left`        | Player disconnects (grace window expired) | `{ user_id, username }`                                                                                                                                                                                                                                      |
| `player_kicked`      | Host kicks a player                       | `{ user_id, username, reason: "kicked" }` — sent to entire room; kicked player's connection is closed                                                                                                                                                        |
| `reconnecting`       | Player enters 30s grace window            | `{ user_id, username }` — other players see "reconnecting…"                                                                                                                                                                                                  |
| `game_started`       | Host starts the game                      | `{ round_count, config }`                                                                                                                                                                                                                                    |
| `round_started`      | New round begins                          | `{ round_number, item: { payload, media_url? }, duration_seconds, ends_at: ISO8601 }` — `media_url` is a pre-signed 15-min Rustfs URL; `ends_at` is an absolute server-clock deadline clients must use for the countdown display (see Timer Synchronization) |
| `submissions_closed` | Submission phase ends, voting begins      | `{ voting_duration_seconds, ends_at: ISO8601 }` — `ends_at` is the absolute voting deadline                                                                                                                                                                  |
| `vote_results`       | Round scores revealed                     | Game-type-specific; see game type protocols below                                                                                                                                                                                                            |
| `game_ended`         | Final leaderboard                         | `{ reason: "completed" \| "host_disconnected" \| "all_players_disconnected", leaderboard: [{ user_id, username, total_score, rank }] }`                                                                                                                      |
| `room_state`         | Full snapshot sent on reconnect           | `{ state, players, current_round?, config }`                                                                                                                                                                                                                 |
| `error`              | Any error from server                     | `{ code: "snake_case_code", message: "..." }`                                                                                                                                                                                                                |

---

### Security & Reliability

**Message size limit**: `SetReadLimit(4096)` — 4 KB per message. Clients exceeding this are disconnected.

**Per-connection rate limit**: 20 messages/second. Clients exceeding this are disconnected.

**Heartbeat**: Client sends `{"type":"ping"}` every 25 seconds. Server replies `{"type":"pong"}`. Server sets a 60-second read deadline on each connection, reset on every received pong. A connection that does not pong within 60 seconds is considered dead and cleaned up.

**Reconnection & grace window**:

- On disconnect, player is marked `reconnecting` for **30 seconds** (not removed).
- Other players receive `{"type":"reconnecting", "data":{"user_id":"..."}}` instead of `player_left`.
- If player reconnects within the window: server sends `room_state` snapshot; client sends `reconnect` (not `join`).
- If grace window expires: `player_left` is broadcast and their pending turn is skipped.

**Quorum on all-disconnect**: if all players simultaneously enter the grace window (e.g., host's network drops everyone), the round timer is paused. If at least one player reconnects within the window, the game resumes. If none reconnect, the server broadcasts `game_ended` with `reason: "all_players_disconnected"` and marks the room `finished`.

**Host disconnect**:

- During `lobby`: if host's grace window expires, room moves to `finished`. No game started; players see "host left".
- During `playing`: if host's grace window expires, `game_ended` is broadcast with `reason: "host_disconnected"`. Room moves to `finished`. No host transfer — simplicity over completeness.

---

### Game Type Protocol: `meme-caption`

Gameplay: host shows an image with a prompt → players write captions → all captions shown anonymously → players vote for the best → author revealed in results.

#### Client → Server

| Type                  | Payload                   | Notes                                                                                                                    |
| --------------------- | ------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `meme-caption:submit` | `{ caption: string }`     | Max 300 chars; trimmed of leading/trailing whitespace; rejected after `submissions_closed`                               |
| `meme-caption:vote`   | `{ submission_id: uuid }` | One vote per player per round; server rejects if `submission.user_id == voter_id` with `{"code":"cannot_vote_for_self"}` |

#### Server → Client

| Type                             | Payload                                                                                                                              | Notes                                                                                                                           |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------- |
| `meme-caption:submissions_shown` | `{ submissions: [{ id, caption }] }`                                                                                                 | Authors **hidden** during voting to prevent bias; own submission included in the list but client should visually distinguish it |
| `meme-caption:vote_results`      | `{ submissions: [{ id, caption, author_username, votes_received, points_awarded }], round_scores: [{ user_id, username, points }] }` | Authors revealed; cumulative scores updated in `game_ended`                                                                     |

---

## Rate Limits

Applied per-IP and per-user as appropriate:

| Route                         | Limit                             |
| ----------------------------- | --------------------------------- |
| `POST /api/auth/*`            | 10 requests/minute per IP         |
| `POST /api/rooms`             | 10 rooms/hour per user            |
| `POST /api/assets/upload-url` | 50 requests/hour per admin        |
| All other `GET /api/*`        | 100 requests/minute per IP        |
| WebSocket messages            | 20 messages/second per connection |

---

## Pagination

All list endpoints that can return unbounded rows support cursor-based pagination. Cursor pagination avoids the "page drift" problem (items inserted during browsing cause rows to appear twice or be skipped with offset pagination).

### Query Parameters

| Parameter | Type   | Description                                          |
| --------- | ------ | ---------------------------------------------------- |
| `limit`   | int    | Max rows to return (default 50, max 100)             |
| `after`   | string | Opaque cursor from previous response's `next_cursor` |

### Response Envelope

```json
{
  "data": [ ... ],
  "next_cursor": "opaque_base64_string_or_null",
  "total": 142
}
```

`next_cursor` is `null` when there are no more pages. `total` is the full count (before pagination), useful for displaying "42 of 142".

The cursor encodes `{ id, created_at }` of the last row in the current page, base64-encoded. Clients treat it as opaque.

### Paginated Endpoints

- `GET /api/admin/users?limit=50&after=...`
- `GET /api/admin/invites?limit=50&after=...`
- `GET /api/packs?limit=50&after=...`
- `GET /api/packs/:id/items?limit=100&after=...` — ordered by `position ASC`

---

## Timer Synchronization

Client and server clocks can drift by several seconds. If the client counts down from a local start time, the submission window may appear open on the client while the server has already closed it — causing a confusing "too late" error on submit.

**Solution**: every timed event includes an `ends_at` field — an absolute ISO 8601 timestamp in the server's clock.

```json
{
  "type": "round_started",
  "data": {
    "round_number": 3,
    "duration_seconds": 60,
    "ends_at": "2026-03-27T14:05:00.000Z",
    "item": { ... }
  }
}
```

The client calculates remaining time as `ends_at - Date.now()` on each tick rather than counting down from a stored start time. This means clock drift is bounded to whatever skew exists between server and client — typically under 2 seconds.

**UI implication**: the `Submit` button is disabled when `Date.now() >= Date.parse(ends_at)` on the client, matching the server's enforcement.

---

## GDPR / User Deletion

`DELETE /api/admin/users/:id` hard-deletes the user row. Due to foreign key cascades:

- `sessions` rows are deleted (cascade)
- `magic_link_tokens` rows are deleted (cascade)
- `room_players` rows are deleted (cascade)
- `submissions` are orphaned — `user_id` on `submissions` is a non-cascading FK; the backend sets `user_id = NULL` (requires making the column nullable) before deleting the user, or reassigns to a sentinel "deleted user" UUID

**Recommended approach**: introduce a soft-delete path first (`is_active = false`) and reserve hard delete for explicit GDPR erasure requests. Document this clearly in the admin UI: "Deactivate prevents login. Delete permanently removes all personal data."

Audit log entry for `DELETE /api/admin/users/:id` records the action but **must not** store the deleted user's email or username in `changes` JSONB after deletion.

---

## Item Reorder

`PATCH /api/packs/:id/items/reorder` accepts a full position map for all items in the pack:

```json
{
  "positions": [
    { "id": "uuid-a", "position": 1 },
    { "id": "uuid-b", "position": 2 }
  ]
}
```

The backend executes the update in a single transaction using a **two-pass strategy** to avoid the `UNIQUE (pack_id, position)` constraint violation during intermediate states:

1. Shift all items in the pack to large temporary positions (`position + 10000`) in one `UPDATE`
2. Set the final positions as specified

This is safe because both steps run inside the same serializable transaction. See [03-database.md](03-database.md) for the deferred constraint alternative.

Validation: the request must include **all** items in the pack (no partial reorder), positions must be unique and start at 1, and no item IDs may belong to a different pack.
