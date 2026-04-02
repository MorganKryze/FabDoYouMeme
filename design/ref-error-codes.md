# ref — Error Codes

Canonical reference for every `snake_case` error code emitted by the system. All REST error responses use the shape `{ "error": "human message", "code": "snake_case_code" }`. WebSocket error frames use `{ "type": "error", "data": { "code": "snake_case_code", "message": "..." } }`.

All endpoint-specific codes link back here. Do not define error codes inline in other docs — add them here first.

---

## REST Error Codes

| Code                       | HTTP | Emitted by                                       | Meaning                                                                                                                    |
| -------------------------- | ---- | ------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------- |
| `consent_required`         | 400  | `POST /api/auth/register`                        | `consent` field is missing or not `true` — user must explicitly accept the privacy policy                                  |
| `age_affirmation_required` | 400  | `POST /api/auth/register`                        | `age_affirmation` field is missing or not `true` — user must confirm they are 16+                                          |
| `invalid_invite`           | 400  | `POST /api/auth/register`                        | Token does not exist, is expired, or is exhausted — deliberately generic to prevent enumeration oracle                     |
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
| `pack_no_supported_items`  | 422  | `POST /api/rooms`                                | Pack has zero items with a `payload_version` supported by the chosen game type handler                                     |
| `solo_mode_not_supported`  | 422  | `POST /api/rooms`                                | `mode='solo'` requested but `game_types.supports_solo = false`                                                             |
| `room_not_lobby`           | 409  | `PATCH /api/rooms/:code/config`                  | Room state is not `lobby`; config is locked during play                                                                    |
| `positions_invalid`        | 422  | `PATCH /api/packs/:id/items/reorder`             | Positions array does not cover all items, contains duplicates, does not start at 1, or contains item IDs from another pack |
| `rate_limited`             | 429  | Rate-limit middleware                            | Too many requests — see `02-identity.md` for per-route limits                                                              |

### Special: smtp_failure (201 + warning)

When `POST /api/auth/register` is used with a `restricted_email` invite and SMTP fails to deliver the auto-sent magic link, the response is:

```json
201 Created
{
  "user_id": "uuid",
  "warning": "smtp_failure",
  "message": "Account created but login email could not be sent. Contact your admin."
}
```

The user record **is** created and the invite slot **is** consumed. The user must request a magic link manually once SMTP is restored. Rolling back user creation is not done — it would allow the same invite slot to be re-used by a different person, creating a race condition.

---

## WebSocket Error Codes

Sent as `{ "type": "error", "data": { "code": "...", "message": "..." } }` to the originating connection.

| Code                   | Emitted by                   | Meaning                                                                                    |
| ---------------------- | ---------------------------- | ------------------------------------------------------------------------------------------ |
| `not_host`             | Hub                          | Sender attempted a host-only action (`start`, `next_round`, kick) but is not the room host |
| `game_already_started` | Hub                          | `join` received when room state is not `lobby`                                             |
| `invalid_submission`   | Handler `ValidateSubmission` | Submission payload is invalid for this game type (e.g. caption exceeds 300 chars)          |
| `invalid_vote`         | Handler `ValidateVote`       | Vote payload is invalid for this game type                                                 |
| `cannot_vote_for_self` | Handler `ValidateVote`       | `voterID == submission.UserID`                                                             |
| `submission_closed`    | Hub                          | Submit message received after the submission window closed                                 |
| `voting_closed`        | Hub                          | Vote message received outside the voting window                                            |
| `duplicate_vote`       | Hub                          | Player already cast a vote this round                                                      |
| `unknown_game_type`    | Registry `Dispatch`          | Prefixed message slug does not match the room's game type                                  |
