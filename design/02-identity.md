# 02 — Identity & Trust

Authentication, session management, invite system, rate limits, and security policies.

---

## Principles

- No passwords — email + magic link only
- Email is the user's identity anchor; no PII stored beyond username and email
- Sessions stored in DB: instant revocation, simple audit trail
- Session TTL: **30 days**, renewed on each authenticated request so players stay logged in across game nights
- All session cookies: `HttpOnly`, `Secure`, `SameSite=Strict`
- Magic link tokens: one-time use, **15-minute TTL**, SHA-256 hash stored (never the raw token)
- **Email enumeration protection**: both the magic-link endpoint AND the registration endpoint always return `200`/`201` regardless of whether the email exists or the account is already registered
- Session tokens are random 32-byte values from `crypto/rand`, hex-encoded; SHA-256 stored in DB — no HMAC signing key required
- **Role and `is_active` are re-fetched from the DB on every authenticated request** — never read from the cookie or cached in memory. A deactivated user or demoted admin takes effect immediately with no grace period.

See `ref-decisions.md` ADR-001 (magic links) and ADR-002 (DB sessions) for rationale.

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

Registration validates: token not expired, `uses_count < max_uses` (atomic check — see `03-data.md`), email matches `restricted_email` if set. Then creates user and atomically increments `uses_count`.

**Streamlined onboarding when `restricted_email` is set**: after the account is created, the backend immediately sends the first magic link to the registered email without the user needing a separate request. The player receives one email, clicks once, and is in. This collapses the 4-step flow into 2 steps (register → click link).

**Invite enumeration defense**: `POST /api/auth/register` applies a dedicated per-IP rate limit of `RATE_LIMIT_INVITE_VALIDATION_RPH` (default: 20/hour). This is stricter than the general auth rate limit because invite token brute-forcing is the primary enumeration vector at registration (62^12 ≈ 475 trillion combinations — unfeasible to enumerate, but rate-limited anyway). Registration errors always return generic `400 {"code":"invalid_invite"}` regardless of whether the token exists, is expired, or is exhausted — no timing or status-code oracle.

---

## Registration & Login Flow

```plain
POST /api/auth/register   { invite_token, username, email }
  → applies invite-validation rate limit (RATE_LIMIT_INVITE_VALIDATION_RPH)
  → validates invite (generic invalid_invite on any failure)
  → if email already registered: still returns 201 (no enumeration)
  → creates user row (if email is new)
  → if invite.restricted_email matches: immediately sends magic link (auto-login path)
  → returns 201 { user_id }
  → if restricted_email path AND SMTP fails: returns 201 { user_id, "warning": "smtp_failure" }
    (user IS created; admin must trigger manual magic link send)

POST /api/auth/magic-link  { email }
  → invalidates any existing unused tokens for that user (UPDATE used_at = now())
  → generates new token, stores SHA-256 hash
  → sends email
  → always returns 200 (never reveals account existence)

  [email arrives]

GET  /auth/verify?token=xxx   (frontend route — intermediate page)
  → renders a "Log in" button
  → user clicks → browser POSTs token

POST /api/auth/verify   { token }
  → hashes incoming token with SHA-256
  → looks up hash in magic_link_tokens
  → checks: not expired, not already used
  → marks token as used (used_at = now())
  → if purpose = login:        creates session, sets HttpOnly cookie, returns 200
  → if purpose = email_change: swaps email ← pending_email, clears pending_email,
                               invalidates all existing sessions, sends old-email notification,
                               sets new session cookie, returns 200

POST /api/auth/logout   (session cookie required)
  → deletes session row → returns 200

GET  /api/auth/me       (session cookie required)
  → returns current user { id, username, email, role }
```

**Why `POST /api/auth/verify` instead of `GET`**: email clients and security tools pre-fetch URLs. A `GET` would consume the one-time token before the user clicks it. The email link targets a frontend route (`/auth/verify?token=xxx`) which renders an intermediate "Log in" button — one extra click eliminates the pre-fetch risk entirely.

**Why invalidate prior magic link tokens on re-request**: if a user requests a magic link twice (e.g., did not receive the first), the old token is immediately invalidated. Only the latest link is ever valid — no two simultaneous active login paths.

**SMTP failure on restricted-email auto-send**: user creation and invite `uses_count` increment are committed regardless. If SMTP fails, return `201` with `"warning": "smtp_failure"`. Do NOT rollback — rolling back would release the invite slot for re-use by another person, creating a race. The user exists and can request a magic link manually once SMTP is restored.

---

## Email Change Flow

```plain
Self-service path (user-initiated):
  PATCH /api/users/me { email: "new@example.com" }
    → stores new address in users.pending_email
    → sends magic link (purpose=email_change) to pending_email
    → OLD email address REMAINS active for login until confirmation
    → returns 200 { "message": "Verification link sent to new address" }

  [user clicks link at the NEW email address]

  POST /api/auth/verify { token }   (purpose=email_change)
    → users.email ← pending_email
    → pending_email cleared
    → ALL existing sessions invalidated
    → notification sent to OLD email address (the one just replaced)
    → new session cookie set
    → returns 200

Admin path (forced change):
  PATCH /api/admin/users/:id { email: "new@example.com" }
    → updates users.email IMMEDIATELY (no verification step)
    → sends notification to OLD email address (prevents silent lockout)
    → does NOT invalidate existing sessions (admin can do that separately if needed)
    → returns 200
```

**Contract**: in the self-service path, the old email remains the active login address until the user confirms at the new address. This prevents account lockout if the user mistypes the new email. The admin path is authoritative and immediate — no confirmation required, because the admin acts with authority.

Username is changeable freely via `PATCH /api/users/me { username }` — no verification step; it is a display handle, not an auth credential. Returns `409 username_taken` if the new username conflicts.

---

## First Admin Bootstrap

On first boot, if `SEED_ADMIN_EMAIL` is set and no admin user exists in the DB:

1. Backend creates a user with that email, `role = 'admin'`, `username = 'admin'`, `is_active = true`, no `invited_by`.
2. Backend immediately sends a magic link to that address.
3. Admin clicks the link, is logged in, and can begin creating invites.

Subsequent restarts with the same env var are no-ops (idempotent check: admin exists → skip). This avoids the chicken-and-egg problem where no admin can create the first invite.

---

## Session Renewal on WebSocket Connections

Sessions are renewed on each authenticated HTTP request. A WebSocket connection is a single long-lived HTTP upgrade — there is no subsequent request to renew on.

**Strategy**:

1. **On WS upgrade**: the session lookup that authenticates the upgrade extends `expires_at` to `now() + SESSION_TTL`.
2. **Periodic renewal**: the hub triggers a session renewal for each connected player every `SESSION_RENEW_INTERVAL` (default: 60 minutes) via a background goroutine per hub. This covers players in very long sessions.

If a session has already expired when the WS upgrade arrives, the upgrade is rejected with `401` and the frontend redirects to `/auth/magic-link`.

---

## Email Templates

**Template engine**: Go's `html/template` package. Templates live in `internal/email/templates/` as `.html` and `.txt` paired files.

| Template file                              | Purpose                                     |
| ------------------------------------------ | ------------------------------------------- |
| `magic_link_login.html` / `.txt`           | Login magic link                            |
| `magic_link_email_change.html` / `.txt`    | Email-change confirmation link              |
| `notification_email_changed.html` / `.txt` | Notification to old address on email change |

**Template variables** (passed as a Go struct to `template.Execute`):

| Variable              | Type   | Used in                            |
| --------------------- | ------ | ---------------------------------- |
| `{{.Username}}`       | string | All templates                      |
| `{{.MagicLinkURL}}`   | string | Login, email-change                |
| `{{.FrontendURL}}`    | string | All templates (footer link)        |
| `{{.NewEmailMasked}}` | string | Notification email                 |
| `{{.ExpiryMinutes}}`  | int    | Login, email-change (currently 15) |

**Masking rule for `NewEmailMasked`**: replace all characters before `@` with `***`. E.g. `alice@example.com` → `***@example.com`.

### Magic Link — Login

**Subject**: `Your FabDoYouMeme login link`

```plain
Hi {{.Username}},

Click the button below to log in. This link expires in {{.ExpiryMinutes}} minutes and can only be used once.

  [ Log In → ]   (links to {{.MagicLinkURL}})

If you didn't request this, you can safely ignore this email.

— FabDoYouMeme
```

**Plain-text fallback**:

```plain
Hi {{.Username}},

Use this link to log in (expires in {{.ExpiryMinutes}} minutes, one-time use):
{{.MagicLinkURL}}

If you didn't request this, ignore this email.
```

### Magic Link — Email Change Verification

**Subject**: `Confirm your new email address`

```plain
Hi {{.Username}},

Someone requested to change your FabDoYouMeme account email to this address.
Click below to confirm. This link expires in {{.ExpiryMinutes}} minutes.

  [ Confirm Email Change → ]   (links to {{.MagicLinkURL}})

If you didn't request this, your account is safe — the change will not take effect.
Your current email address is still active.
```

### Notification — Email Changed

Sent to the **old** email address after any email change takes effect.

**Subject**: `Your FabDoYouMeme email address was changed`

```plain
Hi {{.Username}},

Your FabDoYouMeme account email was changed to {{.NewEmailMasked}}.

If you made this change, no action is needed.

If you did NOT make this change, contact your admin immediately.

— FabDoYouMeme
```

---

## Token Verification Detail

On `POST /api/auth/verify`:

1. Hash the incoming token with SHA-256
2. Look up hash in `magic_link_tokens`
3. Check: not expired (`expires_at > now()`), not already used (`used_at IS NULL`)
4. Mark token as used: `UPDATE magic_link_tokens SET used_at = now() WHERE id = ?`
5. Re-fetch user: check `is_active = true` (deactivated users cannot log in even with a valid token)
6. If `purpose = login`: insert session row, set `HttpOnly`/`Secure`/`SameSite=Strict` cookie (TTL = `SESSION_TTL`), return 200
7. If `purpose = email_change`: update `users.email ← pending_email`, clear `pending_email`, `DELETE FROM sessions WHERE user_id = ?`, send old-email notification, set new session cookie, return 200

---

## Rate Limits

All rate limits are owned exclusively by this document. See `ref-env-vars.md` for the corresponding env variables and `ref-decisions.md` ADR-005 for the single-instance caveat.

| Route / Context                                          | Limit                     | Env variable                       |
| -------------------------------------------------------- | ------------------------- | ---------------------------------- |
| `POST /api/auth/*` (all auth endpoints)                  | 10 req/min per IP         | `RATE_LIMIT_AUTH_RPM`              |
| Invite token validation during `POST /api/auth/register` | 20 req/hour per IP        | `RATE_LIMIT_INVITE_VALIDATION_RPH` |
| `POST /api/rooms`                                        | 10 rooms/hour per user    | `RATE_LIMIT_ROOMS_RPH`             |
| `POST /api/assets/upload-url`                            | 50 req/hour per admin     | `RATE_LIMIT_UPLOADS_RPH`           |
| All other `GET /api/*`                                   | 100 req/min per IP        | `RATE_LIMIT_GLOBAL_RPM`            |
| WebSocket messages                                       | 20 msg/sec per connection | `WS_RATE_LIMIT`                    |

**Error code**: `429 {"code":"rate_limited","error":"Too many requests"}`.

**In-memory caveat**: rate limit counters are held in Go process memory. On a single Docker Compose host this is correct and sufficient. For multi-instance deployment, state must be externalized — see ADR-005 in `ref-decisions.md`.

---

## Security Policies

### Authentication Controls

| Concern                         | Mitigation                                                                                                                                                                               |
| ------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Credential attacks**          | No passwords — magic link only; eliminates brute force, credential stuffing, and password storage breaches entirely                                                                      |
| **Magic link prefetch**         | Verify endpoint is `POST`; email link routes to intermediate frontend page that renders a "Log in" button — one extra click prevents pre-fetch bots from consuming the token             |
| **Magic link replay**           | One-time use (`used_at` set on consume); 15-minute TTL                                                                                                                                   |
| **Magic link leak**             | Token stored as SHA-256 hash only; raw token sent exclusively by email; hash is useless without the preimage                                                                             |
| **Multiple active magic links** | On new magic link request, all existing unused tokens for that user are immediately invalidated                                                                                          |
| **Email enumeration**           | Magic link endpoint always returns `200`; registration endpoint always returns `201` — neither reveals whether an email exists                                                           |
| **Invite enumeration**          | 12-char alphanumeric token (62^12 ≈ 475 trillion combinations); dedicated lower rate limit (`RATE_LIMIT_INVITE_VALIDATION_RPH`); all registration errors return generic `invalid_invite` |
| **Silent email change**         | Notification sent to old email on any change; self-service change requires confirmation at new address before taking effect                                                              |
| **Session hijacking**           | `HttpOnly` + `Secure` + `SameSite=Strict` cookies; 30-day TTL renewed on each authenticated request                                                                                      |
| **Stale admin/active status**   | `role` and `is_active` re-fetched from DB on every session lookup — never cached; deactivated or demoted users are blocked immediately                                                   |

### Transport Controls

| Concern               | Mitigation                                                                                                                                                                                                                                                                                                                                                       |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **CSRF**              | `SameSite=Strict` cookie policy prevents cross-site form submission; origin check enforced on WebSocket handshake                                                                                                                                                                                                                                                |
| **WS authentication** | Session cookie validated during HTTP upgrade; unauthenticated upgrade requests rejected with `401` before handshake completes                                                                                                                                                                                                                                    |
| **SMTP in transit**   | `go-mail` enforces STARTTLS/TLS; plaintext SMTP disabled in production config                                                                                                                                                                                                                                                                                    |
| **CSP — frontend**    | Nonce-based CSP via SvelteKit `csp` option (`mode: 'nonce'`). Nonce generated per request in `src/hooks.server.ts`. Baseline: `default-src 'self'; script-src 'self' 'nonce-{n}'; style-src 'self' 'nonce-{n}'; font-src 'self'; img-src 'self' data: blob:; connect-src 'self' wss: ws:; frame-ancestors 'none'`. In production: replace `ws:` with `wss:` only |
| **CSP — API**         | Go middleware sets `default-src 'none'` on all `/api/*` responses                                                                                                                                                                                                                                                                                                |
| **XSS**               | Svelte's default HTML escaping prevents reflected/stored XSS; CSP nonce provides defence-in-depth                                                                                                                                                                                                                                                                |

### Input Controls

| Concern                 | Mitigation                                                                                                                                                 |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **SQL injection**       | `sqlc` parameterised queries — no string concatenation in DB access code                                                                                   |
| **File upload abuse**   | Server validates declared `mime_type` against allowlist (JPEG, PNG, WebP) **and** validates magic bytes via `image.DecodeConfig` before issuing upload URL |
| **Upload size**         | `size_bytes` validated server-side (`≤ MAX_UPLOAD_SIZE_BYTES`) before issuing upload URL                                                                   |
| **WS message flooding** | Per-connection rate limit (`WS_RATE_LIMIT`, default 20 msg/s); connections exceeding the limit are dropped                                                 |
| **WS payload bombs**    | `SetReadLimit(WS_READ_LIMIT_BYTES)` on every connection (default 4 KB); frames exceeding this disconnect the client                                        |
| **WS dead connections** | Server sets `WS_READ_DEADLINE` (default 60s); reset on each received pong. Client sends ping every `WS_PING_INTERVAL` (default 25s)                        |

### Access Controls

| Concern                  | Mitigation                                                                                                                                                |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Privilege escalation** | Role checked server-side in middleware on every admin route; `is_active` checked on every session lookup                                                  |
| **Self-vote**            | Rejected server-side by game type handler (`cannot_vote_for_self`); hub pre-validates before calling handler                                              |
| **Asset leakage**        | No public bucket; pre-signed download URLs embedded in WS events (15-min TTL, `response-content-disposition=attachment`); session required for all access |
| **Internal services**    | Postgres and backend not reachable outside Docker internal network; RustFS accessible only via reverse proxy                                              |

### Secrets Management

| Concern              | Mitigation                                                                                                         |
| -------------------- | ------------------------------------------------------------------------------------------------------------------ |
| **Secrets in repo**  | All secrets via `.env`; `.gitignore` includes `.env`; no signing keys required (raw random tokens, SHA-256 stored) |
| **Dependency drift** | `govulncheck ./...` in CI; `npm audit` on frontend dependencies                                                    |

---

## Threat Model Boundaries

This platform is **invite-only** and runs on **personal hardware**. The following threats are out of scope:

- **Public account enumeration**: no public registration or user listing
- **DDoS at infrastructure level**: assumed mitigated at the network layer or reverse proxy; application-layer rate limits cover abuse by authenticated users only
- **Physical access to the host**: disk encryption and physical security are the operator's responsibility
- **Email interception**: the magic link email channel is assumed secure (TLS in transit); link compromise after delivery is mitigated by the 15-minute TTL and one-time-use constraint
- **Distributed rate-limit bypass**: rate limits are enforced in-memory per backend instance. On a single-host deployment this is correct. A coordinated multi-IP attack or a multi-instance deployment could bypass per-IP limits. This limitation is intentional and documented in ADR-005 — not treated as an open bug at this deployment scale.

---

## Secrets Rotation

| Secret                                    | Rotation impact                             | Procedure                                                           |
| ----------------------------------------- | ------------------------------------------- | ------------------------------------------------------------------- |
| `POSTGRES_PASSWORD`                       | All backend connections drop                | Update `.env`, restart backend + postgres                           |
| `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY` | All asset operations fail                   | Rotate keys in RustFS admin console, update `.env`, restart backend |
| `SMTP_PASSWORD`                           | Email delivery fails (magic links broken)   | Update `.env`, restart backend                                      |
| Session tokens (in DB)                    | None — tokens are random and self-contained | N/A; invalidate individual sessions by deleting rows                |
