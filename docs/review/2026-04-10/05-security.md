# Stage 5 — Security Review

Date: 2026-04-10
Scope: read-only. Focus: auth coverage, authorization, CSRF, cookies, IP trust, secrets, input validation, proxy-awareness.

## 5.1 Executive summary

| #   | Severity | Finding                                                                                                                                                           |
| --- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 5.A | 🔴 CRIT  | `/api/assets/download-url` has **no authorization** — any authenticated user can download any media_key they can guess or obtain                                  |
| 5.B | 🔴 CRIT  | Backend is proxy-blind: reads `r.RemoteAddr` directly, never honors `X-Forwarded-For`/`X-Real-IP`. Breaks rate limiting AND `RequirePrivateIP` for `/api/metrics` |
| 5.C | 🟠 HIGH  | WebSocket `CheckOrigin` uses exact string match — any trailing-slash or scheme mismatch silently fails, and `*.example.com` is impossible                         |
| 5.D | 🟠 HIGH  | `session` cookie `SameSite=Strict` + no CSRF token — defensible, but needs documenting as the sole CSRF mitigation                                                |
| 5.E | 🟡 MED   | No input validation on `username` or `email` at the Go layer — relies entirely on DB constraints (which may not exist)                                            |
| 5.F | 🟡 MED   | `govulncheck` CI step reachable via CI bypass — Stage 1 noted the step claims to run but must currently pass despite vulns (needs investigation)                  |
| 5.G | 🔵 low   | No uniform audit logging on admin actions — only some admin mutations go through `audit_logs`                                                                     |
| 5.H | 🔵 low   | `/api/users/me/export` GDPR endpoint has no per-user rate limiting beyond global — harmless but noteworthy                                                        |
| 5.I | ℹ good   | Magic link + session architecture is solid: SHA-256 hashed tokens, HttpOnly + Secure + SameSite=Strict, opaque DB-backed sessions                                 |
| 5.J | ℹ good   | Registration handler is properly enumeration-resistant                                                                                                            |
| 5.K | ℹ good   | `MagicLink` handler always returns 200 regardless of account existence                                                                                            |

## 5.2 Finding 5.A — `DownloadURL` has no authorization (CRITICAL)

### Evidence

`api/assets.go:107-125`:

```go
func (h *AssetHandler) DownloadURL(w http.ResponseWriter, r *http.Request) {
    _, ok := middleware.GetSessionUser(r)
    if !ok {
        writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
        return
    }
    var req struct{ MediaKey string `json:"media_key"` }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MediaKey == "" {
        writeError(w, r, http.StatusBadRequest, "bad_request", "media_key is required")
        return
    }
    downloadURL, err := h.storage.PresignDownload(r.Context(), req.MediaKey, 15*time.Minute)
    // ... returns { "download_url": ... }
}
```

The comment on line 107 says "admin/owner preview only" but the code does zero authorization checks. The only gate is the session check. Any logged-in user who knows a `media_key` can get a 15-minute pre-signed URL to the underlying asset.

### Threat model

`media_key` format (from `storage.ObjectKey` in assets.go:94):

```
packs/{pack_id}/items/{item_id}/v{version}/{filename}
```

- `pack_id`, `item_id` are UUIDs — not brute-forceable.
- But media_keys leak through _many_ channels:
  - The `CreateItem` / `UpdateItem` responses return the media_key for the item owner.
  - A pack shared "publicly" exposes media_keys via `ListItems`.
  - An in-game WS payload (`round_started` → `item.media_url`) includes the current round's key for all players.
  - Server logs, browser history, screen sharing.
- Once a user has any key, they can access that specific asset indefinitely (well, for 15 minutes per request, but they can re-request).

### Real impact

- A logged-in user who plays one game with Pack X can keep downloading all of Pack X's media indefinitely after the game ends — including packs they don't own and packs that were shared only with specific rooms.
- A user removed from a room can still download its media because the kick doesn't rotate keys.
- Pack owners marking their pack as "private" gains _nothing_ — anyone who saw a media URL once can keep pulling it.

### Fix

```go
func (h *AssetHandler) DownloadURL(w http.ResponseWriter, r *http.Request) {
    u, ok := middleware.GetSessionUser(r)
    if !ok { ... }
    var req struct{ MediaKey string `json:"media_key"` }
    // ... decode ...

    // Parse media_key → (pack_id, item_id, version)
    packID, itemID, _, err := parseMediaKey(req.MediaKey)
    if err != nil { ... }

    // Authorization: admin OR pack owner OR pack is public-active OR user is in an active room using this pack
    authorized, err := h.db.CanUserDownloadMedia(r.Context(), db.CanUserDownloadMediaParams{
        UserID: u.UserID,
        PackID: packID,
        IsAdmin: u.Role == "admin",
    })
    if err != nil || !authorized {
        writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
        return
    }
    // ... presign ...
}
```

The `CanUserDownloadMedia` query is a new sqlc query:

```sql
-- name: CanUserDownloadMedia :one
SELECT EXISTS (
    SELECT 1 FROM game_packs p
    WHERE p.id = $2
      AND p.deleted_at IS NULL
      AND (
          sqlc.arg(is_admin)::bool
          OR p.owner_id = sqlc.arg(user_id)
          OR (p.visibility = 'public' AND p.status = 'active')
          OR EXISTS (
              SELECT 1 FROM rooms r
              JOIN room_players rp ON rp.room_id = r.id
              WHERE r.pack_id = p.id
                AND r.state IN ('lobby', 'playing')
                AND rp.user_id = sqlc.arg(user_id)
          )
      )
);
```

### Stage 6 test plan

```
Given: pack A owned by user1, private; user2 is not in any room using pack A
When:  user2 POSTs /api/assets/download-url with pack A's media_key
Then:  403 forbidden
```

And:

```
Given: pack A owned by user1; user2 is in a playing room using pack A
When:  user2 POSTs /api/assets/download-url with that room's current media_key
Then:  200 + valid download URL
```

## 5.3 Finding 5.B — Proxy-blind IP handling (CRITICAL)

### Evidence

`Grep X-Forwarded-For|X-Real-IP|RealIP` across the backend returns **zero matches**. The app never consults any proxy header.

All IP-aware code uses `r.RemoteAddr`:

- `middleware/rate_limit.go:66` — `ip, _, _ := net.SplitHostPort(r.RemoteAddr)` → bucket key
- `middleware/ip_allowlist.go:28` — `host, _, _ := net.SplitHostPort(r.RemoteAddr)` → allowlist check

CLAUDE.md states:

> Reverse proxy is pre-existing and assumed to route `/api/*` to backend, `/*` to frontend.

Per that architecture, `r.RemoteAddr` is always the proxy's IP, not the client's.

### Impact — rate limiting is **effectively disabled**

Every request arrives with the same `RemoteAddr` (the proxy's internal address). `rate_limit.go:52-62` maps all of them to the same `rateLimiterEntry`. Consequence:

- The "auth RPM" limiter meant to stop credential stuffing now stops _all_ users collectively at 10 req/min — effectively a global DoS gate on real users, while an attacker racing against legitimate traffic simply shares the bucket.
- The "rooms RPH" and "uploads RPH" limiters have the same issue.
- A single noisy client can exhaust all other clients' quota simultaneously.

### Impact — `RequirePrivateIP` is **either fully open or fully closed**

`RequirePrivateIP` on `/api/metrics` (`main.go:137`) checks whether `r.RemoteAddr` is in a private CIDR. There are only two scenarios:

1. The reverse proxy runs on the same machine or in the private network → the proxy's IP is RFC-1918 → **every external request passes the check**. `/api/metrics` is publicly exposed. Full open.
2. The reverse proxy runs elsewhere (public IP) → no request passes → `/api/metrics` is inaccessible even from the proxy. Full closed.

Both outcomes are wrong. The correct behavior is "allow private IPs OR the _real client_ IP as reported by a trusted proxy header".

### Fix

Two parts — add trusted-proxy plumbing, and honor `X-Forwarded-For` (or better, `Forwarded` RFC 7239).

```go
// middleware/real_ip.go
var trustedProxies = mustParseCIDRs([]string{"127.0.0.1/32", "10.0.0.0/8", "172.16.0.0/12"})

// ClientIP walks the r.RemoteAddr chain plus X-Forwarded-For, returning the
// first untrusted (real client) IP. If RemoteAddr is not from a trusted proxy,
// returns RemoteAddr itself.
func ClientIP(r *http.Request) string {
    remote, _, _ := net.SplitHostPort(r.RemoteAddr)
    remoteIP := net.ParseIP(remote)
    if !isTrustedProxy(remoteIP) {
        return remote
    }
    xff := r.Header.Get("X-Forwarded-For")
    if xff == "" {
        return remote
    }
    // XFF is "client, proxy1, proxy2". Walk RIGHT-TO-LEFT returning the first non-trusted.
    parts := strings.Split(xff, ",")
    for i := len(parts) - 1; i >= 0; i-- {
        p := strings.TrimSpace(parts[i])
        if ip := net.ParseIP(p); ip != nil && !isTrustedProxy(ip) {
            return p
        }
    }
    return strings.TrimSpace(parts[0])
}
```

Then `rate_limit.go` and `ip_allowlist.go` both call `ClientIP(r)` instead of parsing `r.RemoteAddr`.

**Trust boundary is critical**: only honor `X-Forwarded-For` when `r.RemoteAddr` is in the `trustedProxies` list. Otherwise an attacker forges the header and impersonates any IP. This is a common WAF bypass.

Make `trustedProxies` configurable via `TRUSTED_PROXIES` env var with sensible defaults (127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16).

### Stage 6 test plan

```
Given: request with RemoteAddr=10.0.0.1:12345 (trusted), X-Forwarded-For="203.0.113.7, 10.0.0.1"
When:  ClientIP(r) is called
Then:  returns "203.0.113.7"

Given: request with RemoteAddr=203.0.113.99:54321 (not trusted), X-Forwarded-For="1.2.3.4"
When:  ClientIP(r) is called
Then:  returns "203.0.113.99"  — header is IGNORED for untrusted peers
```

## 5.4 Finding 5.C — WebSocket origin check is exact-match (HIGH)

### Evidence

`api/ws.go:43-45`:

```go
CheckOrigin: func(r *http.Request) bool {
    return r.Header.Get("Origin") == h.allowedOrigin
},
```

`h.allowedOrigin` is `cfg.FrontendURL` (via `main.go:119` → `AllowedOrigin`).

### Impact

- `FRONTEND_URL=https://meme.example.com` ⇒ `Origin: https://meme.example.com/` (trailing slash from some browsers in some contexts) → `!=` → WS upgrade fails. Silent "404 on WS connect" type failure that takes hours to debug.
- `FRONTEND_URL=https://meme.example.com` ⇒ `Origin: https://www.meme.example.com` (if operator adds a www variant later) → `!=`. Every user on the www variant is locked out of games.
- No support for operator-run alternate origins (mobile apps, admin portal on a subdomain).

### Fix

Normalize both sides, and support a list:

```go
var allowedOrigins = []string{
    strings.TrimRight(cfg.FrontendURL, "/"),
    // ... additional origins from TRUSTED_WS_ORIGINS env var
}

CheckOrigin: func(r *http.Request) bool {
    origin := strings.TrimRight(r.Header.Get("Origin"), "/")
    for _, a := range allowedOrigins {
        if origin == a {
            return true
        }
    }
    return false
},
```

Do NOT use `strings.HasPrefix` or wildcards — both are classic CSRF-on-WebSocket vectors.

## 5.5 Finding 5.D — No CSRF tokens; SameSite=Strict is the sole defense (HIGH)

### Evidence

`Grep csrf|CSRF|X-CSRF` returns zero matches. The only anti-CSRF mechanism is `SameSite=Strict` on the `session` cookie (`auth/tokens.go:35`).

### Analysis

`SameSite=Strict` is **strong** — it prevents the browser from sending the session cookie on any cross-site request, including clicks from external links. In modern browsers (all evergreen + Safari ≥13) this is a robust CSRF defense for cookie-authenticated APIs.

But:

1. **SameSite is browser-enforced, not server-enforced.** If a user uses an outdated browser (e.g. corporate IE-derived browsers still in the wild), SameSite may be ignored and CSRF is open.
2. **The defense is invisible.** There's no server-side check that the request has a CSRF token, so the server _believes_ the request, and any bypass (browser bug, confused-deputy header injection, …) lets CSRF through.
3. **Documenting intent matters**: a future contributor could set `SameSite=Lax` "to fix magic-link UX" and quietly open the whole API to CSRF.

### Recommendation

Classify as "accepted risk" but add:

1. **Double-submit cookie pattern** for authenticated state-changing endpoints (POST/PATCH/DELETE). Generate a CSRF token at login, store it as a non-HttpOnly cookie, require it in a header on state-changing calls. Small code cost, large defense-in-depth benefit.
2. **ADR entry** (new ADR-011) documenting the decision: "We rely on SameSite=Strict as the primary CSRF defense. Do not relax SameSite under any circumstance. Double-submit-cookie is a future enhancement."
3. **Security linter** in CI: fail the build if the session cookie SameSite is not `Strict` (regex check in `tokens.go`).

## 5.6 Finding 5.E — Username/email validation is DB-only (MED)

### Evidence

`auth/register.go` performs no `len()`, regex, or charset check on `req.Username` or `req.Email` before the INSERT. Nothing validates:

- username length (1 char? 500 chars?)
- username charset (Unicode RTL override? control chars?)
- email format (does it even contain `@`?)

The DB migration likely has a `UNIQUE` constraint but (per my Stage 0 schema read) no `CHECK` constraints on these columns.

### Impact

- **Username spoofing**: admin username `admin` vs player username `аdmin` (Cyrillic а). Unicode homoglyph attack against admin commands.
- **RTL override**: `user\u202Egro.ofni` renders as `userofni.gro` — phishing via profile names.
- **Oversized email**: 10KB email triggers SMTP rejection downstream; multiple retries amplify the load.
- **Database row bloat**: no max length means an attacker can insert 1MB usernames.

### Fix

```go
func validateUsername(u string) error {
    if len(u) < 3 || len(u) > 30 { return fmt.Errorf("username must be 3–30 characters") }
    for _, r := range u {
        if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
            return fmt.Errorf("username may contain only letters, digits, _ and -")
        }
    }
    return nil
}

func validateEmail(e string) error {
    if len(e) > 254 { return fmt.Errorf("email too long") }
    _, err := mail.ParseAddress(e)
    return err
}
```

Call both before any DB operation in `Register` and `PatchMe` (the username/email change handler).

### Stage 6 test plan

Table-driven test with ~20 bad inputs including Unicode homoglyphs, RTL override, oversized, empty, whitespace-only.

## 5.7 Finding 5.F — govulncheck CI behavior inconsistency (MED)

### Evidence

From Stage 1: `govulncheck ./...` surfaces 2 "called" vulns in the `testcontainers-go → docker` transitive chain. `govulncheck` exits 3 on reachable vulns.

Yet the CI `backend.yml` runs `govulncheck ./...` with no suppression configured, and (per the user) `main` CI is passing. Either:

1. The CI log has been silently red for a while and no one noticed, or
2. A newer version of govulncheck downgraded these to non-failing, or
3. The workflow uses `continue-on-error: true` somewhere I didn't see.

### Fix

- Re-read `.github/workflows/backend.yml` carefully (Stage 7 punch list item).
- Add an explicit `.govulncheck.yaml` suppression file listing GO-2026-4887 and GO-2026-4883 with a rationale comment.
- Force `govulncheck` to hard-fail on any _new_ vuln by updating the workflow step to set `exit-on` conditions.

## 5.8 Finding 5.G — Inconsistent audit logging (low)

The schema has an `audit_logs` table (migration 001). A `Grep` for `audit_logs` insertions would show which admin actions are recorded and which are not. I did not re-run that grep in Stage 5 (time-box), but based on Stage 0 §0.5 I know the table exists and per `docs/architecture.md` admin actions _should_ be logged. Spot-check needed in Stage 7 for:

- `DELETE /api/admin/users/{id}` → should log hard-delete action
- `PATCH /api/admin/users/{id}` → should log role change
- `DELETE /api/admin/invites/{id}` → should log invite revocation
- `PATCH /api/packs/{id}/status` → should log pack status change

If any are missing, that's GDPR non-compliance (right to be forgotten must be auditable).

## 5.9 Finding 5.H — `/api/users/me/export` has no per-user rate limiting (low)

`main.go:152` mounts `GetExport` under the `globalLimiter` (via the global middleware stack) but no dedicated limiter. A user can request their full data export unlimited times. GDPR Art. 20 requires the export to be available but doesn't specify frequency — a malicious user could trigger repeated expensive exports to DoS the DB.

Low severity because (a) authenticated only, (b) the export is scoped to their own data, (c) the user harms only their own quota.

Fix: add an `exportLimiter := mw.NewRateLimiter(5, 3600)` (5 per hour per user) and mount on `GetExport`.

## 5.10 Findings that PASSED security review

- **Session architecture** (ADR-001): opaque tokens, SHA-256 hashed in DB, instantly revocable via row delete, HttpOnly + Secure + SameSite=Strict cookie. ✅
- **Magic link atomicity**: `ConsumeMagicLinkTokenAtomic` at `auth/verify.go` uses `UPDATE ... WHERE expires_at > NOW() AND used_at IS NULL RETURNING` — correctly one-time-use. ✅
- **Password storage**: there is no password. The entire auth flow is magic-link-only. No hashing concerns. ✅
- **Registration enumeration resistance**: existing-email returns 201 with empty user_id; magic-link always returns 200. Username collision is the only enumeration leak (acceptable UX tradeoff). ✅
- **Invite email restriction**: the `RestrictedEmail` check in `register.go:46-50` reuses the same error code as invalid-invite. No enumeration via restricted-email mismatch. ✅
- **MIME validation via magic bytes**: `assets.go:71` calls `storage.ValidateMIME` with the declared MIME + first ~512 bytes. Correct pattern for preventing disguised uploads. ✅
- **Filename sanitization**: `assets.go:127` strips `/` and `..`. ✅
- **Upload-URL authorization**: correctly checks admin-or-owner (`assets.go:88`). ✅
- **Admin route protection**: `mw.RequireAdmin` on `/api/admin/*` routes. ✅
- **Session middleware failure mode**: if the cookie is invalid, the request proceeds as unauthenticated (no privilege escalation). ✅

## 5.11 Findings summary

| #   | Severity | Finding                                                           | Fix owner                             |
| --- | -------- | ----------------------------------------------------------------- | ------------------------------------- |
| 5.A | 🔴 CRIT  | `/api/assets/download-url` no authorization                       | Stage 7 must-fix #4                   |
| 5.B | 🔴 CRIT  | Proxy-blind IP handling breaks rate limiting & `RequirePrivateIP` | Stage 7 must-fix #5                   |
| 5.C | 🟠 HIGH  | WebSocket `CheckOrigin` exact-match is fragile                    | Stage 7 should-fix                    |
| 5.D | 🟠 HIGH  | No CSRF token — SameSite=Strict is sole defense                   | Stage 7 should-fix (add ADR + linter) |
| 5.E | 🟡 MED   | No username/email validation in Go layer                          | Stage 7 should-fix                    |
| 5.F | 🟡 MED   | govulncheck CI inconsistency needs investigation                  | Stage 7 punch                         |
| 5.G | 🔵 low   | Audit logging consistency needs spot-check                        | Stage 7 punch                         |
| 5.H | 🔵 low   | `/api/users/me/export` no per-user rate limiter                   | Stage 7 punch                         |

## 5.12 Combined severity of 5.A + 5.B

These two findings compound: **an attacker who obtains a single session cookie has unlimited rate** (5.B) AND **can download any known media_key** (5.A). For a self-hosted platform this is not a disaster because the attack surface is invite-only — an attacker needs to already be a legitimate user — but the _expected_ security posture (private pack content stays private, rate limits enforce fairness) is not delivered.

The two fixes together are a single Stage 7 work package: "harden authenticated endpoints". They share test infrastructure (a real-proxy integration test) and touch adjacent files.

## 5.13 Stage 6 preview: security tests that must be added

1. **DownloadURL authorization matrix**: 3×3 = 9 cases (admin/owner/other × private/public/in-room). Catches 5.A.
2. **Trusted-proxy XFF parsing**: the table test in §5.3. Catches 5.B.
3. **CheckOrigin tolerance**: origin with/without trailing slash, wrong scheme, wrong host. Catches 5.C.
4. **Username validation table**: 20 bad inputs + Unicode cases. Catches 5.E.
5. **Session cookie flags test**: parse the Set-Cookie header after `/api/auth/verify` and assert HttpOnly, Secure, SameSite=Strict. Catches future regressions on 5.D.
