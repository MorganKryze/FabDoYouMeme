# ref — GDPR Compliance

This document records how FabDoYouMeme meets GDPR obligations. It is the authoritative reference for data protection decisions. Update it when the data model, retention policy, or lawful basis changes.

> **Scope**: FabDoYouMeme processes personal data of EU residents as a controller. It runs on a single self-hosted machine. No data processors (third-party SaaS) handle personal data except the SMTP provider used for magic link delivery.

---

## Lawful Basis (Art. 6)

| Processing activity                           | Lawful basis                        | Details                                                                                |
| --------------------------------------------- | ----------------------------------- | -------------------------------------------------------------------------------------- |
| Account creation and authentication           | Consent — Art. 6(1)(a)              | Captured at registration via `users.consent_at` checkbox                               |
| Magic link email delivery                     | Contract performance — Art. 6(1)(b) | Email is the authentication channel; no email = no access                              |
| Game history (submissions, votes, scores)     | Consent — Art. 6(1)(a)              | Covered by registration consent; game participation is voluntary                       |
| Operational logs (no IP at info level)        | Legitimate interest — Art. 6(1)(f)  | Security monitoring and incident response; 30-day retention                            |
| Admin audit log                               | Legitimate interest — Art. 6(1)(f)  | Accountability for admin actions; retained indefinitely (anonymised on admin deletion) |
| First-boot admin account (`SEED_ADMIN_EMAIL`) | Legitimate interest — Art. 6(1)(f)  | Bootstrapping the platform; consent_at set to bootstrap timestamp                      |

---

## Personal Data Inventory (ROPA-lite, Art. 30)

| Data element             | Table / location                               | Purpose              | Retention                                                                                                         |
| ------------------------ | ---------------------------------------------- | -------------------- | ----------------------------------------------------------------------------------------------------------------- |
| Email address            | `users.email`, `users.pending_email`           | Authentication       | Until erasure                                                                                                     |
| Username                 | `users.username`                               | In-game display      | Until erasure                                                                                                     |
| Consent timestamp        | `users.consent_at`                             | GDPR Art. 7 record   | Until erasure                                                                                                     |
| Session token hash       | `sessions.token_hash`                          | Authentication state | 30 days (auto-expire) or until logout                                                                             |
| Magic link token hash    | `magic_link_tokens.token_hash`                 | One-time auth        | 15 minutes (auto-expire); used tokens cleaned after 7 days                                                        |
| Invite email restriction | `invites.restricted_email`                     | Invite targeting     | Until invite revoked                                                                                              |
| Submissions (captions)   | `submissions.payload`                          | Game content         | Until room data is purged (indefinite)                                                                            |
| Votes                    | `votes.voter_id`                               | Game scoring         | Until erasure (voter replaced by sentinel on hard-delete)                                                         |
| Game scores              | `room_players.score`                           | Leaderboard          | Until erasure (row deleted on hard-delete via CASCADE)                                                            |
| Audit log PII snapshot   | `audit_logs.changes` (`hard_delete_user` only) | Admin accountability | Retained 3 years, then anonymized (username/email replaced with SHA-256 hash) — see `03-data.md` Cleanup Strategy |
| Operational logs         | Docker stdout/stderr                           | Security monitoring  | ≤ 30 days (Docker log rotation)                                                                                   |
| Database backups         | Full PostgreSQL dump                           | Disaster recovery    | 7 days (auto-deleted); may contain recently-deleted user data for up to 7 days after erasure                      |

---

## Data Subject Rights

| Right                     | Article   | How it is fulfilled                                                                                        |
| ------------------------- | --------- | ---------------------------------------------------------------------------------------------------------- |
| Right of access           | Art. 15   | `GET /api/auth/me` (current profile); `GET /api/users/me/export` (full data dump)                          |
| Right to rectification    | Art. 16   | `PATCH /api/users/me` (username, email change)                                                             |
| Right to erasure          | Art. 17   | `DELETE /api/admin/users/:id` — admin action on user request; see Hard-Delete Protocol in `04-protocol.md` |
| Right to portability      | Art. 20   | `GET /api/users/me/export` returns machine-readable JSON                                                   |
| Right to object           | Art. 21   | Contact admin; no automated profiling or marketing — not practically applicable                            |
| Right to withdraw consent | Art. 7(3) | Contact admin for account deletion (consent withdrawal = erasure request)                                  |

> **Self-serve deletion is not available** to prevent mid-game abuse. The admin must process erasure requests promptly — within 30 days per GDPR Art. 12(3). Document the admin's contact method in the Privacy Policy page (`/privacy`).

---

## Consent Record (Art. 7)

- `users.consent_at` is set once at registration and never updated.
- The registration form presents a mandatory checkbox: "I have read and agree to the [Privacy Policy]."
- `POST /api/auth/register` rejects `consent != true` with `400 {"code":"consent_required"}`.
- The consent covers: account data, authentication emails, game history storage.
- Withdrawing consent = erasure request (account deletion).

---

## Breach Notification Procedure (Art. 33 / 34)

In the event of a suspected personal data breach (e.g., DB dump leaked, SMTP credentials compromised):

1. **Contain**: revoke affected credentials immediately. Rotate `POSTGRES_PASSWORD`, `SMTP_PASSWORD`, or `RUSTFS_SECRET_KEY` as applicable (see `02-identity.md` Secrets Rotation table). Restart affected services.
2. **Assess within 24 hours**: determine which personal data was exposed (email addresses? session hashes? submissions?). Check audit logs for unauthorised admin actions.
3. **Notify supervisory authority within 72 hours** (Art. 33): if the breach is likely to result in a risk to rights and freedoms. File with the relevant EU data protection authority (e.g., CNIL for France, BfDI for Germany). Include: nature of breach, categories and approximate number of data subjects affected, likely consequences, measures taken.
4. **Notify affected users without undue delay** (Art. 34): if the breach is likely to result in a high risk. Send notification to affected email addresses explaining what happened and what to do.
5. **Document the breach** internally regardless of notification outcome (Art. 33(5)): record in a breach register with date, nature, assessment, and actions taken.

> **Practical note**: this is a self-hosted personal project with a small, known user base. Most breach scenarios will be contained quickly. The 72-hour clock starts when you become **aware** of the breach — not when it occurred.

---

## Cookie Notice (Art. 13)

A single `HttpOnly`, `Secure`, `SameSite=Strict` session cookie is set after successful authentication. It is:

- **Functional only** — required for authentication; no tracking or analytics.
- **Not subject to cookie banner requirements** under most EU guidance (strictly necessary).
- **Disclosed in the Privacy Policy** at `/privacy`.

No third-party cookies are set by this application.

---

## Data Minimisation Compliance (Art. 5(1)(c))

- Only `email` and `username` are stored — no phone, DOB, gender, location, or payment data.
- IP addresses are not written to structured logs at `info` level.
- Magic link tokens are stored as SHA-256 hashes only.
- Session tokens are stored as SHA-256 hashes only.
- Game submissions store user-provided content only (captions, votes) — no metadata beyond `created_at`.

---

## Data Processors (Art. 28)

GDPR Art. 28 requires a written Data Processing Agreement (DPA) with every third party that processes personal data on your behalf.

| Processor                   | Personal data handled              | DPA required             | Action                                              |
| --------------------------- | ---------------------------------- | ------------------------ | --------------------------------------------------- |
| SMTP provider (`SMTP_HOST`) | User email address, magic-link URL | Yes — sign before launch | Obtain from provider (Mailgun, SES, Sendgrid, etc.) |

> All other processing is on-premises (PostgreSQL, RustFS). No other data processors exist.

---

## Minor Protection (Art. 8)

FabDoYouMeme is intended for users aged **16 and above** (the GDPR default threshold).

- The registration form includes a mandatory age affirmation: "I am at least 16 years old."
- `POST /api/auth/register` rejects `age_affirmation != true` with `400 age_affirmation_required`.
- Admins must not invite users known to be under 16 unless parental consent is obtained separately.
- No automated age verification is implemented — enforcement relies on invite-only gating and admin judgment.
- This policy must be stated explicitly in the Privacy Policy at `/privacy`.

---

## Data Subject Request Procedure (Art. 12(3))

**SLA**: Respond to all requests within 30 calendar days of receipt.

| Right                   | How fulfilled                                                  | Who acts                      |
| ----------------------- | -------------------------------------------------------------- | ----------------------------- |
| Access (Art. 15)        | User: `GET /api/users/me/export`                               | Self-service                  |
| Portability (Art. 20)   | User: `GET /api/users/me/export`                               | Self-service                  |
| Rectification (Art. 16) | User: `PATCH /api/users/me`                                    | Self-service                  |
| Erasure (Art. 17)       | User contacts admin → admin runs `DELETE /api/admin/users/:id` | Admin within 30 days          |
| Objection (Art. 21)     | Contact admin                                                  | Admin assesses within 30 days |

**Admin erasure procedure**:

1. Verify requester identity (email matches `users.email`).
2. Check for overriding legal basis (active legal dispute, ongoing abuse investigation).
3. Execute hard-delete via `DELETE /api/admin/users/:id`.
4. Confirm deletion to requester by email.
5. Note: backups ≤7 days old may still contain the deleted data — see `06-operations.md` Backup Strategy.
