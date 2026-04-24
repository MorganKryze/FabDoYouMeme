# Auth & Identity

## Model: invite-only, no passwords

Every account requires an admin-issued invite. There is no public registration. Authentication uses magic links only — no passwords are stored or managed. The only database record of a login token is its SHA-256 hash; the raw token is sent once by email and then discarded.

---

## Invite system

An admin creates invites with the following fields:

| Field              | Description                                                            |
| ------------------ | ---------------------------------------------------------------------- |
| `token`            | 12-character alphanumeric code (human-typeable or embeddable in a URL) |
| `max_uses`         | `0` = unlimited; `N` = exactly N registrations                         |
| `restricted_email` | If set, only that exact address may register with this invite          |
| `expires_at`       | Optional expiry timestamp; null means never expires                    |
| `label`            | Optional admin note, e.g. `"gaming night 2026-03"`                     |

Invite tokens are rate-limited at 20 attempts per hour per IP to prevent brute-force enumeration. All registration failures return a generic `400 invalid_invite` regardless of whether the token exists, is expired, or is exhausted — no oracle for the attacker.

**Streamlined onboarding:** when `restricted_email` is set on an invite and a player registers, the backend immediately sends the first magic link without requiring a second request. The player receives one email, clicks once, and is authenticated.

**Platform+group invites (phase 2).** A `group_invite_token` (kind `platform_plus_group`, minted at `/api/groups/{gid}/invites/platform_plus`) replaces the `invite_token` field and enrols the new user into the target group in the same registration transaction. NSFW groups additionally require a `nsfw_age_affirmation: true` field. The platform-registration slot is debited at mint time from the issuing admin's `user_invite_quotas` row, not at redemption. See `backend/internal/auth/register.go`.

---

## Registration flow

```plain
POST /api/auth/register  { invite_token, username, email,
                           consent: true, age_affirmation: true }
```

- Both `consent` and `age_affirmation` must be explicitly `true`. Any other value returns `400`.
- The timestamp of consent is stored in `users.consent_at` at registration. It is never changed.
- If the email is already registered, the response is still `201` — no enumeration.
- On success, the invite's `uses_count` is atomically incremented.
- **Phase 2+**: if `group_invite_token` is sent instead of `invite_token`, registration consumes a `group_invites` row (kind `platform_plus_group`) and inserts a `group_memberships` row (role=`member`) in the same transaction. `nsfw_age_affirmation: true` is required when the target group is classified NSFW. Sending both tokens is a 400.

---

## Login flow

```plain
POST /api/auth/magic-link  { email }
  → always returns 200
  → invalidates any prior unused tokens for this user (only one active link at a time)
  → generates a new token, stores SHA-256 hash, sends email

GET  /auth/verify?token=xxx     ← frontend intermediate page, renders a "Log in" button
POST /api/auth/verify  { token }
  → hashes token with SHA-256, looks it up
  → if not found: 400 token_not_found
  → if expired:   400 token_expired
  → if used:      400 token_used
  → marks token used, checks user.is_active
  → if inactive:  403 user_inactive
  → creates session row, sets HttpOnly/Secure/SameSite=Strict cookie
  → returns 200
```

**Why the intermediate page?** Email security scanners and link-preview tools pre-fetch URLs. If the verify endpoint were a `GET`, these bots would silently consume the one-time token before the user ever clicks. The intermediate page renders a button the user must click, triggering a `POST` that the bots will not replicate.

**Why invalidate prior tokens?** If a user requests a magic link twice (e.g., the first email was delayed), the old token is immediately invalidated. Only the most recently issued link is ever valid. There is never more than one active login path for a user at a time.

---

## Session management

Sessions are stored in the `sessions` table as a SHA-256 hash of a random 32-byte token. On every authenticated request, the backend:

1. Reads the `session` cookie
2. Hashes it with SHA-256
3. Looks up the hash in the `sessions` table
4. Re-fetches the user's `role` and `is_active` from the `users` table

Role and active status are never cached — a deactivated user or demoted admin is locked out immediately on the next request.

Sessions expire after 30 days (`SESSION_TTL`) but are renewed on each authenticated request, so an active player stays logged in indefinitely. For long-running WebSocket connections (where no HTTP request renews the session), the hub renews sessions in the background every `SESSION_RENEW_INTERVAL` (default 60 minutes).

Logout deletes the session row. The cookie is cleared on the client. All other sessions for the same user remain valid until explicitly deleted.

**Last-login stamping (phase 2).** Every successful magic-link verify stamps `users.last_login_at` with `now()`. The column drives the 90-day dormant-sole-admin scan in `backend/internal/groupjobs/PromoteDormantAdmins`; failure to stamp is logged but does not fail login.

### Platform-ban cascade into groups (phase 2)

Setting `users.is_active = false` via `PATCH /api/admin/users/{id}` is treated as a platform ban. In addition to the existing session-invalidation effect, the admin handler invokes `groupjobs.CascadePlatformBan` which:

1. Snapshots the groups where the banned user is the sole admin.
2. Deletes every `group_memberships` row for the user.
3. Revokes every outstanding `group_invites` row the user minted (sets `revoked_at = now()`).
4. For each sole-admin group in the snapshot, promotes the longest-tenured active member of that group to admin. When no candidate exists, a `group.auto_promote_no_candidate` audit-log entry is written so the platform admin can intervene.

Failures inside steps 2–4 are logged but never block the admin's PATCH response. See `backend/internal/groupjobs/promote.go`.

---

## Email change flow

Users can request an email change from their profile settings. The old address remains the active login address until the user confirms at the new address:

1. `PATCH /api/users/me  { email: "new@example.com" }` — stores `new@example.com` in `users.pending_email`, sends a confirmation magic link to the new address.
2. User receives the email and clicks verify.
3. On `POST /api/auth/verify` with the `email_change` token: the new address replaces the old one, all existing sessions are invalidated (re-authentication required), and a notification is sent to the old address.

If the user types the wrong new address, they can just request another change — the old email stays active and is never locked out.

Admins can change any user's email immediately, without a confirmation step. A notification is always sent to the old address to prevent silent lockout.

---

## Security controls summary

| Concern                  | How it's handled                                                               |
| ------------------------ | ------------------------------------------------------------------------------ |
| Credential attacks       | No passwords exist; eliminated entirely                                        |
| Magic link prefetch      | Verify is `POST` behind an intermediate page button                            |
| Magic link replay        | One-time use + 15-minute TTL                                                   |
| Magic link leak          | Only SHA-256 hash stored; raw token never persisted                            |
| Email enumeration        | All auth endpoints return 200/201 regardless of account existence              |
| Invite brute-force       | 12-char token (475 trillion combinations) + 20 req/hour rate limit             |
| Session hijacking        | `HttpOnly` + `Secure` + `SameSite=Strict` cookies                              |
| Stale role/active status | Re-fetched from DB on every request; never cached                              |
| Silent email change      | Old address always notified; self-service requires confirmation at new address |
| CSRF                     | `SameSite=Strict` + WebSocket origin check                                     |
| SQL injection            | `sqlc` parameterised queries throughout                                        |
| File upload abuse        | MIME allowlist + magic byte validation before issuing upload URL               |

---

## GDPR posture

**Lawful basis:** consent (Art. 6(1)(a)) for regular users, recorded via `users.consent_at` at registration. For the bootstrap admin, legitimate interest (Art. 6(1)(f)).

**Hard delete:** when a user requests account deletion, the sequence is:

1. All sessions explicitly deleted (no race window with token lookup)
2. Submissions and votes re-attributed to the sentinel user (`00000000-0000-0000-0000-000000000001`)
3. User row deleted

This preserves round history for other players while removing all personal data.

**Data portability:** `GET /api/users/me/export` returns all personal data as JSON (GDPR Art. 20): profile, game history, and submissions.

**Retention limits:**

- Game data (rounds, submissions, votes): purged 2 years after room completion
- Audit log PII: anonymised after 3 years

**Data processor:** the SMTP provider receives the user's email address to deliver magic links. A Data Processing Agreement with the SMTP provider is required before production use.

See `docs/reference/gdpr.md` for the full GDPR documentation.
