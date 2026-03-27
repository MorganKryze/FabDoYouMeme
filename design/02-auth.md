# 02 — Authentication & Invite System

## Principles

- No passwords — email + magic link only
- Email is the user's identity anchor; no PII stored beyond username and email
- Sessions stored in DB: instant revocation, simple audit trail
- Session TTL: **30 days**, renewed on each authenticated request so players stay logged in across game nights
- All session cookies: `HttpOnly`, `Secure`, `SameSite=Strict`
- Magic link tokens: one-time use, **15-minute TTL**, SHA-256 hash stored (never the raw token)
- Email enumeration protection: magic link endpoint always returns `200` regardless of whether the email exists
- Session tokens are random 32-byte values from `crypto/rand`, hex-encoded; SHA-256 stored in DB — no HMAC signing key required
- **Role and `is_active` are re-fetched from the DB on every authenticated request** — never read from the cookie or cached in memory. This ensures a deactivated user or demoted admin takes effect immediately with no grace period.

---

## Invite Token Model

An admin creates an invite with:

| Field              | Type                   | Notes                                                  |
| ------------------ | ---------------------- | ------------------------------------------------------ |
| `token`            | 12-char alphanumeric   | Human-typeable or embeddable in a URL                  |
| `max_uses`         | int                    | `0` = unlimited, `N` = exactly N registrations allowed |
| `restricted_email` | text (nullable)        | If set, only that address may use this invite          |
| `expires_at`       | timestamptz (nullable) | Null = never expires                                   |
| `label`            | text (nullable)        | Human note, e.g. `"gaming night 2026-03"`              |

Registration validates: token not expired, `uses_count < max_uses` (atomic check — see [03-database.md](03-database.md)), email matches `restricted_email` if set. Then creates user and atomically increments `uses_count`.

**Streamlined onboarding when `restricted_email` is set**: after the account is created, the backend immediately sends the first magic link to the registered email without the user needing a separate request. The player receives one email, clicks once, and is in. This collapses the 4-step flow into 2 steps (register → click link).

---

## Registration & Login Flow

```plain
POST /api/auth/register   { invite_token, username, email }
  → validates invite
  → creates user row
  → if invite.restricted_email matches: immediately sends magic link (auto-login path)
  → returns 201 { user_id }

POST /api/auth/magic-link  { email }
  → invalidates any existing unused tokens for that user (UPDATE used_at = now())
  → generates new token, stores SHA-256 hash
  → sends email (always returns 200, never reveals account existence)

  [email arrives]

GET  /auth/verify?token=xxx   (frontend route — intermediate page)
  → renders a "Log in" button
  → user clicks → browser POSTs token

POST /api/auth/verify   { token }
  → hashes incoming token with SHA-256
  → looks up hash in magic_link_tokens
  → checks: not expired, not already used
  → marks token as used (used_at = now())
  → if purpose = login:     creates session, sets HttpOnly cookie, returns 200
  → if purpose = email_change: swaps email ← pending_email, clears pending_email,
                               deletes all existing sessions, sends old-email notification,
                               sets new session cookie, returns 200

POST /api/auth/logout   (session cookie required)
  → deletes session row → returns 200

GET  /api/auth/me       (session cookie required)
  → returns current user { id, username, email, role }
```

**Why `POST /api/auth/verify` instead of `GET`**: security tools and email clients pre-fetch URLs in emails. A `GET` would consume the one-time token before the user clicks it. The email link targets a frontend route (`/auth/verify?token=xxx`) which renders an intermediate "Log in" button. That button `POST`s the token. One extra click eliminates the pre-fetch risk entirely.

**Why invalidate prior magic link tokens on re-request**: if a user requests a magic link twice (e.g., did not receive the first email), the old token becoming invalid prevents two active login paths existing simultaneously. Only the latest link is valid.

---

## Email Change Flow

```plain
Self-service path (user-initiated):
  PATCH /api/users/me { email: "new@example.com" }
    → stores new address in users.pending_email
    → sends magic link with purpose = email_change to pending_email

  User clicks link in email → POST /api/auth/verify { token }
    → purpose = email_change detected
    → users.email ← pending_email
    → pending_email cleared
    → all existing sessions invalidated
    → notification sent to old email address
    → new session cookie set

Admin path (forced change):
  PATCH /api/admin/users/:id { email: "new@example.com" }
    → updates users.email immediately (no verification step)
    → always sends notification to old email address (prevents silent lockout)
```

Username is changeable freely via `PATCH /api/users/me { username }` by the user, or `PATCH /api/admin/users/:id { username }` by an admin. No verification step — it is a display handle, not an auth credential.

---

## First Admin Bootstrap

On first boot, if `SEED_ADMIN_EMAIL` is set and no admin user exists in the DB:

1. Backend creates a user with that email, `role = 'admin'`, `username = 'admin'`, `is_active = true`, no `invited_by`.
2. Backend immediately sends a magic link to that address.
3. Admin clicks the link, is logged in, and can begin creating invites.

Subsequent restarts with the same env var are no-ops (idempotent check: admin exists → skip). This avoids the chicken-and-egg problem where no admin can create the first invite.

---

## Session Renewal on WebSocket Connections

Sessions are renewed on each authenticated HTTP request. A WebSocket connection is a single long-lived HTTP upgrade — there is no subsequent request to renew on. A player in a multi-hour game night could have their session expire mid-game if this is not handled.

**Strategy**: renew the session at WebSocket connect time, then renew periodically while the connection is alive.

1. **On WS upgrade**: the session lookup that authenticates the upgrade also extends `expires_at` to `now() + SESSION_TTL`. This covers any connected player.
2. **Periodic renewal**: the hub triggers a session renewal for each connected player every **60 minutes** (background goroutine per hub). This covers players in very long sessions.

If a session has already expired when the WS upgrade arrives (player was idle for 30 days with the tab open), the upgrade is rejected with `401` and the frontend redirects to `/auth/magic-link`.

---

## Email Templates

All emails are HTML with a plain-text fallback. Templates live in `internal/email/templates/`.

### Magic Link — Login

**Subject**: `Your FabDoYouMeme login link`

**Body**:

```plain
Hi [username],

Click the button below to log in. This link expires in 15 minutes and can only be used once.

  [ Log In → ]   (button linking to {FRONTEND_URL}/auth/verify?token=xxx)

If you didn't request this, you can safely ignore this email.

— FabDoYouMeme
```

**Plain text fallback**:

```plain
Hi [username],

Use this link to log in (expires in 15 minutes, one-time use):
{FRONTEND_URL}/auth/verify?token=xxx

If you didn't request this, ignore this email.
```

### Magic Link — Email Change Verification

**Subject**: `Confirm your new email address`

**Body**:

```plain
Hi [username],

Someone requested to change your FabDoYouMeme account email to this address.
Click below to confirm. This link expires in 15 minutes.

  [ Confirm Email Change → ]

If you didn't request this, your account is safe — the change will not take effect.
```

### Notification — Email Changed

Sent to the **old** email address when any email change takes effect.

**Subject**: `Your FabDoYouMeme email address was changed`

**Body**:

```plain
Hi [username],

Your FabDoYouMeme account email was changed to [new_email_masked].

If you made this change, no action is needed.

If you did NOT make this change, contact your admin immediately.
```

`new_email_masked` shows only the domain part for privacy, e.g., `***@example.com`.

---

## Token Verification Detail

On `POST /api/auth/verify`:

1. Hash the incoming token with SHA-256
2. Look up hash in `magic_link_tokens`
3. Check: not expired (`expires_at > now()`), not already used (`used_at IS NULL`)
4. Mark token as used: `UPDATE magic_link_tokens SET used_at = now() WHERE id = ?`
5. Re-fetch user: check `is_active = true` (deactivated users cannot log in even with a valid token)
6. If `purpose = login`: insert session row, set `HttpOnly`/`Secure`/`SameSite=Strict` cookie (30-day TTL), return 200
7. If `purpose = email_change`: update `users.email ← pending_email`, clear `pending_email`, `DELETE FROM sessions WHERE user_id = ?`, send old-email notification, set new session cookie, return 200
