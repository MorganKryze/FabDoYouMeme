# GDPR Compliance

This document records how FabDoYouMeme meets GDPR obligations. Update it when the data model, retention policy, or lawful basis changes.

**Scope:** FabDoYouMeme processes personal data of EU residents. It runs on a single self-hosted machine. The SMTP provider used for magic-link delivery is the only external data processor.

**Two-controller model (phase-5 groups paradigm).** Responsibility is split rather than joint:

- The **platform admin** (the person or entity operating the self-hosted instance) is controller for platform-level data: user accounts, email addresses, consent + age-affirmation records, the invite budget, and the SFW/NSFW taxonomy itself. The platform admin operates under a **notice-and-takedown** posture with respect to content inside groups — they do not routinely moderate group content.
- Each **group admin** is a separate controller for group-level moderation data about their group: membership, kicks/bans, pack evictions, and the group's declared SFW/NSFW classification. Group admins are not joint controllers with the platform; they act independently within the authority the platform exposes.

Both controllers' decisions are recorded in the shared `audit_logs` table (phase-5 wiring) so the platform admin retains pull-read visibility for structural invariants (classification breaches, age-gate enforcement) and incident response.

---

## Lawful basis (Art. 6)

| Processing activity                           | Lawful basis                        | Details                                                                                |
| --------------------------------------------- | ----------------------------------- | -------------------------------------------------------------------------------------- |
| Account creation and authentication           | Consent — Art. 6(1)(a)              | Captured at registration via `users.consent_at` checkbox                               |
| Magic link email delivery                     | Contract performance — Art. 6(1)(b) | Email is the authentication channel; no email = no access                              |
| Game history (submissions, votes, scores)     | Consent — Art. 6(1)(a)              | Covered by registration consent; game participation is voluntary                       |
| Operational logs (no IP at info level)        | Legitimate interest — Art. 6(1)(f)  | Security monitoring and incident response; 30-day retention                            |
| Admin audit log                               | Legitimate interest — Art. 6(1)(f)  | Accountability for admin actions; retained indefinitely (anonymised on admin deletion) |
| First-boot admin account (`SEED_ADMIN_EMAIL`) | Legitimate interest — Art. 6(1)(f)  | Bootstrapping the platform; `consent_at` set to bootstrap timestamp                    |

---

## Personal data inventory (ROPA-lite, Art. 30)

| Data element              | Table / location                               | Controller           | Purpose                                        | Retention                                                                     |
| ------------------------- | ---------------------------------------------- | -------------------- | ---------------------------------------------- | ----------------------------------------------------------------------------- |
| Email address             | `users.email`, `users.pending_email`           | Platform             | Authentication                                 | Until erasure                                                                 |
| Username                  | `users.username`                               | Platform             | In-game display                                | Until erasure                                                                 |
| Consent timestamp         | `users.consent_at`                             | Platform             | GDPR Art. 7 record                             | Until erasure                                                                 |
| Last-login timestamp      | `users.last_login_at`                          | Platform             | Dormancy detection for group auto-promotion    | Until erasure                                                                 |
| Session token hash        | `sessions.token_hash`                          | Platform             | Authentication state                           | 30 days (auto-expire) or until logout                                         |
| Magic link token hash     | `magic_link_tokens.token_hash`                 | Platform             | One-time auth                                  | 15 minutes (auto-expire); used tokens cleaned after 7 days                    |
| Invite email restriction  | `invites.restricted_email`                     | Platform             | Platform-level invite targeting                | Until invite revoked                                                          |
| Group invite email limit  | `group_invites.restricted_email`               | Group admin          | Per-group invite targeting                     | Until invite revoked                                                          |
| Group membership          | `group_memberships (group_id, user_id, role)`  | Group admin          | Who is in the group, admin vs member           | Until group hard-delete OR user erasure                                       |
| Group ban list            | `group_bans (group_id, user_id, banned_by)`    | Group admin          | Block redemption for banned users              | Until unban OR group hard-delete OR user erasure                              |
| Group moderation audit    | `audit_logs` rows with `group.*` actions       | Group admin (+ platform pull-read) | Accountability for group-level actions | 3 years, then anonymised (username/email → SHA-256)                      |
| Group classification      | `groups.classification`                        | Group admin          | SFW/NSFW declaration for content invariants    | Until group hard-delete                                                       |
| Submissions (captions)    | `submissions.payload`                          | Platform             | Game content                                   | Until room data is purged (2 years after game)                                |
| Votes                     | `votes.voter_id`                               | Platform             | Game scoring                                   | Until erasure (voter replaced by sentinel on hard-delete)                     |
| Game scores               | `room_players.score`                           | Platform             | Leaderboard                                    | Until erasure (row deleted on hard-delete via CASCADE)                        |
| Audit log PII snapshot    | `audit_logs.changes` (`hard_delete_user` only) | Platform             | Admin accountability                           | Retained 3 years, then anonymised (username/email replaced with SHA-256 hash) |
| Operational logs          | Docker stdout/stderr                           | Platform             | Security monitoring                            | ≤ 30 days (Docker log rotation)                                               |
| Database backups          | Full PostgreSQL dump                           | Platform             | Disaster recovery                              | 7 days; may contain recently-deleted user data for up to 7 days after erasure |

---

## Data subject rights

| Right                     | Article   | How it is fulfilled                                                               |
| ------------------------- | --------- | --------------------------------------------------------------------------------- |
| Right of access           | Art. 15   | `GET /api/auth/me` (current profile); `GET /api/users/me/export` (full data dump) |
| Right to rectification    | Art. 16   | `PATCH /api/users/me` (username, email change)                                    |
| Right to erasure          | Art. 17   | `DELETE /api/admin/users/:id` — admin action on user request                      |
| Right to portability      | Art. 20   | `GET /api/users/me/export` returns machine-readable JSON                          |
| Right to object           | Art. 21   | Contact admin; no automated profiling or marketing — not practically applicable   |
| Right to withdraw consent | Art. 7(3) | Contact admin for account deletion (consent withdrawal = erasure request)         |

> **Self-serve deletion is not available** to prevent mid-game abuse. The admin must process erasure requests within 30 days per GDPR Art. 12(3). Document the admin's contact method in the Privacy Policy at `/privacy`.

---

## Consent record (Art. 7)

- `users.consent_at` is set once at registration and never updated.
- The registration form presents a mandatory checkbox: "I have read and agree to the Privacy Policy."
- `POST /api/auth/register` rejects `consent != true` with `400 {"code":"consent_required"}`.
- The consent covers: account data, authentication emails, game history storage.
- Withdrawing consent = erasure request (account deletion).

---

## Minor protection (Art. 8)

FabDoYouMeme is intended for users aged **16 and above** (the GDPR default threshold).

- The registration form includes a mandatory age affirmation: "I am at least 16 years old."
- `POST /api/auth/register` rejects `age_affirmation != true` with `400 age_affirmation_required`.
- Admins must not invite users known to be under 16 unless parental consent is obtained separately.
- No automated age verification is implemented — enforcement relies on invite-only gating and admin judgement.
- This policy must be stated explicitly in the Privacy Policy at `/privacy`.

---

## Data minimisation (Art. 5(1)(c))

- Only `email` and `username` are stored — no phone, DOB, gender, location, or payment data.
- IP addresses are not written to structured logs at `info` level.
- Magic link tokens are stored as SHA-256 hashes only.
- Session tokens are stored as SHA-256 hashes only.
- Game submissions store user-provided content only — no metadata beyond `created_at`.

---

## Data processors (Art. 28)

GDPR Art. 28 requires a written Data Processing Agreement (DPA) with every third party that processes personal data on your behalf.

| Processor                   | Personal data handled              | DPA required             | Action                                              |
| --------------------------- | ---------------------------------- | ------------------------ | --------------------------------------------------- |
| SMTP provider (`SMTP_HOST`) | User email address, magic-link URL | Yes — sign before launch | Obtain from provider (Mailgun, SES, Sendgrid, etc.) |

All other processing is on-premises (PostgreSQL, RustFS). No other data processors exist.

---

## Groups paradigm (phase 5+)

### Controller split

The platform admin and each group admin act as **separate controllers** over different slices of the data flow. The split is deliberate so the platform operator can maintain a **safe-harbour / notice-and-takedown** posture toward group content without inheriting liability for every moderation call a group admin makes.

Platform-controller responsibilities:
- Account lifecycle (registration, age affirmation, erasure).
- SFW/NSFW **taxonomy integrity** — the rules themselves, not the labels on any given group.
- Platform-wide invite budget (`user_invite_quotas`).
- Structural invariants: classification breach escalation, age-gate enforcement, per-group quota enforcement.
- Pull-read audit visibility over `audit_logs` rows scoped to `group:*` resources.

Group-admin-controller responsibilities:
- Membership decisions (admits, kicks, bans) within the group.
- Pack evictions and group-pack moderation.
- Classification **declaration** for their specific group (the label SFW or NSFW; the meaning of the label is platform-defined).
- Responding to group-internal member reports.

Platform admins **do not** routinely review group content, do not auto-receive member reports, and do not curate or recommend. They intervene on notice (classification breach, a report about a group admin themselves) or escalation.

### Retention alignment

Group **soft-delete** terminates live state only: packs, memberships, quotas, and active invite codes enter a 30-day recovery window, then hard-delete (see ADR-011 once written; mechanics live in `backend/cmd/server/main.go` cleanup). Historical game-play data that referenced the group's assets is **not** accelerated by group deletion — it follows the existing platform retention clock (2 years game data, 3 years audit PII). Replay-redaction renders deleted-pack content as `[deleted]` in historical rooms.

Group admins cannot override platform retention policy. A group admin deleting their group does not force earlier erasure of submissions or votes authored inside it — those remain under the platform controller's purpose.

### NSFW / age-gate record

Joining an NSFW group requires an explicit one-time age affirmation at redemption time (on top of the platform-level age affirmation from registration). The affirmation is recorded in `audit_logs` with a `group.invite_redeemed_with_nsfw_affirmation` action; it is **not** persisted on the user row because the consent is scoped to that join action, not an ongoing status.

### Data subject erasure under groups

Hard-deleting a user cascades into the groups layer:
- All `group_memberships` rows for the user are removed.
- If any group falls to zero admins as a result, the auto-promotion job runs immediately for that group instead of waiting the 90-day dormancy window.
- All pending invites the user minted are revoked.
- Personal packs are deleted (same as pre-phase-5); group-owned packs (duplicated) survive because they belong to the group, not the user.
- Replay-redaction already handles submissions/votes via the sentinel UUID (unchanged).

Platform-ban (`users.is_active = false`) triggers the same cascade (see `backend/internal/groupjobs/CascadePlatformBan`).

---

## Breach notification procedure (Art. 33/34)

In the event of a suspected personal data breach (e.g., DB dump leaked, SMTP credentials compromised):

1. **Contain** — revoke affected credentials immediately. Rotate `POSTGRES_PASSWORD`, `SMTP_PASSWORD`, or `RUSTFS_SECRET_KEY` as applicable. Restart affected services.
2. **Assess within 24 hours** — determine which personal data was exposed. Check audit logs for unauthorised admin actions.
3. **Notify supervisory authority within 72 hours** (Art. 33) — if the breach is likely to result in a risk to rights and freedoms. File with the relevant EU data protection authority (e.g., CNIL for France, BfDI for Germany). Include: nature of breach, categories and approximate number of affected data subjects, likely consequences, measures taken.
4. **Notify affected users without undue delay** (Art. 34) — if the breach is likely to result in a high risk. Explain what happened and what to do.
5. **Document the breach internally** regardless of notification outcome (Art. 33(5)) — record in a breach register with date, nature, assessment, and actions taken.

> The 72-hour clock starts when you become **aware** of the breach — not when it occurred.

---

## Data subject request procedure (Art. 12(3))

**SLA:** respond to all requests within 30 calendar days of receipt.

| Right                   | How fulfilled                                                  | Who acts                      |
| ----------------------- | -------------------------------------------------------------- | ----------------------------- |
| Access (Art. 15)        | User: `GET /api/users/me/export`                               | Self-service                  |
| Portability (Art. 20)   | User: `GET /api/users/me/export`                               | Self-service                  |
| Rectification (Art. 16) | User: `PATCH /api/users/me`                                    | Self-service                  |
| Erasure (Art. 17)       | User contacts admin → admin runs `DELETE /api/admin/users/:id` | Admin within 30 days          |
| Objection (Art. 21)     | Contact admin                                                  | Admin assesses within 30 days |

**Admin erasure procedure:**

1. Verify requester identity (email matches `users.email`).
2. Check for overriding legal basis (active legal dispute, ongoing abuse investigation).
3. Execute hard-delete via `DELETE /api/admin/users/:id`.
4. Confirm deletion to requester by email.
5. Note: backups ≤7 days old may still contain the deleted data — this 7-day lag is acceptable under Art. 17(3)(b).

---

## Cookie notice (Art. 13)

A single `HttpOnly`, `Secure`, `SameSite=Strict` session cookie is set after successful authentication. It is:

- **Functional only** — required for authentication; no tracking or analytics
- **Not subject to cookie banner requirements** under most EU guidance (strictly necessary)
- **Disclosed in the Privacy Policy** at `/privacy`

No third-party cookies are set by this application.
