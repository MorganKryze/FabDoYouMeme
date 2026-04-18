# Error Codes

Canonical reference for every `snake_case` error code emitted by the system.

- **REST** errors use: `{ "error": "human message", "code": "snake_case_code", "request_id": "..." }`
- **WebSocket** errors use: `{ "type": "error", "data": { "code": "snake_case_code", "message": "..." } }`

---

## REST error codes

| Code                       | HTTP | Emitted by                                       | Meaning                                                                                                                    |
| -------------------------- | ---- | ------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------- |
| `consent_required`         | 400  | `POST /api/auth/register`                        | `consent` field is missing or not `true` — user must explicitly accept the privacy policy                                  |
| `age_affirmation_required` | 400  | `POST /api/auth/register`                        | `age_affirmation` field is missing or not `true` — user must confirm they are 16+                                          |
| `invalid_username`         | 400  | `POST /api/auth/register`, `PATCH /api/users/me` | Username fails shape validation (length 3–30 bytes, ASCII letters/digits/`_`/`-` only) — prevents Unicode homoglyphs and RTL overrides |
| `invalid_email`            | 400  | `POST /api/auth/register`, `PATCH /api/users/me` | Email fails shape validation (non-empty, ≤254 bytes, parseable as bare address per RFC 5321)                               |
| `invalid_invite`           | 400  | `POST /api/auth/register`                        | Token does not exist, is expired, or is exhausted — deliberately generic to prevent enumeration                            |
| `email_mismatch`           | 400  | `POST /api/auth/register`                        | Invite has `restricted_email` and provided email does not match                                                            |
| `username_taken`           | 409  | `POST /api/auth/register`, `PATCH /api/users/me` | Username already in use by another account                                                                                 |
| `token_expired`            | 400  | `POST /api/auth/verify`                          | Magic link token past its TTL                                                                                              |
| `token_used`               | 400  | `POST /api/auth/verify`                          | Magic link token already consumed                                                                                          |
| `token_not_found`          | 400  | `POST /api/auth/verify`                          | Token hash not found in DB                                                                                                 |
| `user_inactive`            | 403  | `POST /api/auth/verify`                          | User account deactivated by admin — valid token, but user may not log in                                                   |
| `unauthorized`             | 401  | All authenticated routes                         | No valid session cookie present                                                                                            |
| `forbidden`                | 403  | Admin routes, host-only actions                  | Valid session but insufficient role or ownership                                                                           |
| `not_found`                | 404  | Any route                                        | Requested resource does not exist                                                                                          |
| `conflict`                 | 409  | Generic                                          | Duplicate key or invalid state transition                                                                                  |
| `invalid_mime_type`        | 422  | `POST /api/assets/upload-url`                    | MIME type not in the allowlist (JPEG, PNG, WebP)                                                                           |
| `magic_bytes_mismatch`     | 422  | `POST /api/assets/upload-url`                    | File magic bytes do not match declared MIME type                                                                           |
| `upload_too_large`         | 422  | `POST /api/assets/upload-url`                    | `size_bytes` exceeds `MAX_UPLOAD_SIZE_BYTES`                                                                               |
| `pack_insufficient_items`  | 422  | `POST /api/rooms`                                | Pack has compatible items but fewer than `config.round_count`                                                              |
| `pack_no_supported_items`  | 422  | `POST /api/rooms`                                | Pack has zero items with a `payload_version` supported by the chosen game type                                             |
| `solo_mode_not_supported`  | 422  | `POST /api/rooms`                                | `mode='solo'` requested but `game_types.supports_solo = false`                                                             |
| `room_not_lobby`           | 409  | `PATCH /api/rooms/:code/config`                  | Room state is not `lobby`; config is locked once the game starts                                                           |
| `room_already_finished`    | 409  | `POST /api/rooms/:code/end`                      | Room is already in `finished` state — terminal, cannot be ended again                                                      |
| `display_name_taken`       | 409  | `POST /api/rooms/:code/guest-join`               | Another guest in this room already uses that display name — pick a different one                                           |
| `already_in_active_room`   | 409  | `POST /api/rooms`, `GET /api/ws/rooms/:code`     | Caller is already in a lobby/playing room (as host or participant) — must leave it before creating or joining another      |
| `banned_from_room`         | 409  | `GET /api/ws/rooms/:code`                        | Caller (user or guest) is on this room's `room_bans` list — connection is rejected pre-upgrade so the join form can surface it |
| `cannot_kick_self`         | 409  | `POST /api/rooms/:code/kick`                     | Host targeted themselves — use `POST /api/rooms/:code/end` to close their own room instead                                |
| `room_not_in_lobby`        | 409  | `POST /api/rooms/:code/kick`                     | Room is not in `lobby` state — kick is only available before the game starts                                              |
| `positions_invalid`        | 422  | `PATCH /api/packs/:id/items/reorder`             | Positions array does not cover all items, contains duplicates, does not start at 1, or contains item IDs from another pack |
| `system_pack_readonly`     | 403  | Every mutating pack/item handler                 | Attempt to modify a pack managed by `systempack` (bundled demo pack). The filesystem is the only way to update it.         |
| `rate_limited`             | 429  | Rate-limit middleware                            | Too many requests — see [self-hosting.md](../self-hosting.md) for per-route limits                                         |

### Special: smtp_failure (201 + warning)

When `POST /api/auth/register` is used with a `restricted_email` invite and SMTP fails to deliver the auto-sent magic link, the response is `201 Created` with a `warning` field:

```json
{
  "user_id": "uuid",
  "warning": "smtp_failure",
  "message": "Account created but login email could not be sent. Contact your admin."
}
```

The user record **is** created and the invite slot **is** consumed. The user must request a magic link manually once SMTP is restored. User creation is not rolled back — doing so would release the invite slot for re-use by a different person, creating a race condition.

---

## WebSocket error codes

Sent to the originating connection as `{ "type": "error", "data": { "code": "...", "message": "..." } }`.

| Code                   | Emitted by                   | Meaning                                                                                    |
| ---------------------- | ---------------------------- | ------------------------------------------------------------------------------------------ |
| `not_host`             | Hub                          | Sender attempted a host-only action (`start`, `next_round`, kick) but is not the room host |
| `game_already_started` | Hub                          | `join` received when room state is not `lobby`                                             |
| `invalid_submission`   | Handler `ValidateSubmission` | Submission payload is invalid for this game type (e.g. caption exceeds 200 chars)          |
| `invalid_vote`         | Handler `ValidateVote`       | Vote payload is invalid for this game type                                                 |
| `cannot_vote_for_self` | Handler `ValidateVote`       | `voterID == submission.UserID`                                                             |
| `submission_closed`    | Hub                          | Submit message received after the submission window closed                                 |
| `voting_closed`        | Hub                          | Vote message received outside the voting window                                            |
| `duplicate_vote`       | Hub                          | Player already cast a vote this round                                                      |
| `unknown_game_type`    | Registry `Dispatch`          | Prefixed message slug does not match the room's game type                                  |
| `skip_submit_disabled` | Hub                          | `skip_submit` received while host has set `joker_count == 0` — the skip-turn joker is unavailable for this room |
| `skip_vote_disabled`   | Hub                          | `skip_vote` received while host has set `allow_skip_vote == false` — abstention is unavailable for this room    |
| `jokers_exhausted`     | Hub                          | `skip_submit` received after the player has used every joker for this game — no further skips accepted          |
| `already_submitted`    | Hub                          | Submit or skip_submit received after the player already submitted or skipped this round                         |
| `already_voted`        | Hub                          | Vote or skip_vote received after the player already voted or abstained this round                               |

### WebSocket `room_closed` reasons

`room_closed` is not a `type: "error"` frame — it is its own top-level server → client message (see `docs/api.md`). The `data.reason` string is one of:

- `ended_by_host` — the room's host terminated the room via `POST /api/rooms/:code/end`
- `ended_by_admin` — a platform admin (not the host) terminated the room
