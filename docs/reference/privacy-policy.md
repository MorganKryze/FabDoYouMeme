# Privacy Policy — Reference Copy

> **This markdown file is the reference copy of the privacy policy wording.** The live page served at `/privacy` is rendered by a Svelte template at [`frontend/src/routes/(public)/privacy/+page.svelte`](../../frontend/src/routes/(public)/privacy/+page.svelte) which mirrors the structure below. This file exists so legal wording can be reviewed, diffed, and versioned without reading JSX.

**Operator-specific fields are not hardcoded.** The fields below that reference a specific operator (name, contact email, deployment URL, SMTP provider) are injected at runtime into the Svelte template from these environment variables:

| Field in the page              | Environment variable            | Who sets it                                    |
| ------------------------------ | ------------------------------- | ---------------------------------------------- |
| §1 "Data controller"           | `PUBLIC_OPERATOR_NAME`          | Operator of the instance                      |
| §1, §5, §7 contact email       | `PUBLIC_OPERATOR_CONTACT_EMAIL` | Operator of the instance                      |
| §1 "Hosted at"                 | `PUBLIC_OPERATOR_URL`           | Operator (defaults to `FRONTEND_URL`)         |
| §9 SMTP processor name         | `PUBLIC_OPERATOR_SMTP_PROVIDER` | Operator of the instance                      |

So when a new operator self-hosts FabDoYouMeme (the project is GPLv3), **they do not need to edit this file or the Svelte page** — they set four env vars in their `.env` and their instance's `/privacy` page shows their own details automatically. See [`docs/self-hosting.md → Legal / privacy policy`](../self-hosting.md#legal--privacy-policy) for the full variable reference.

The wording that follows is intentionally **simple, readable, and accurate**: plain language over legalese, only the data actually collected, only the processors actually used. Nothing is padded for the sake of looking complete. It also serves as the reference configuration of the public instance operated by the maintainer — the concrete values below (Morgan Kryze, `contact@libresoftware.cloud`, OVHcloud, etc.) are what that instance sets its env vars to, and show what a filled-in policy looks like. GDPR Art. 13(1) requires this information to be provided at the time personal data is collected (registration). The consent checkbox on the registration page links to the rendered page — incomplete or inaccurate content is a compliance failure.

---

## 1. Controller identity (Art. 13(1)(a))

**Data controller:** Morgan Kryze — natural person, acting as an individual non-commercial operator. No company, no legal entity, no revenue; the instance is hosted and maintained personally as a hobby project.  
**Contact email:** <contact@libresoftware.cloud> — used for all privacy requests (access, erasure, rectification, objection, complaints). This is the same address used for the project repository.  
**Hosted at:** [DEPLOYMENT URL — the public URL of this specific instance, e.g. `https://meme.libresoftware.cloud`]

---

## 2. Purposes and lawful bases (Art. 13(1)(c))

| Purpose                           | Data used                              | Lawful basis                        |
| --------------------------------- | -------------------------------------- | ----------------------------------- |
| Authentication (magic link login) | Email address                          | Contract performance — Art. 6(1)(b) |
| Account management                | Username, email                        | Consent — Art. 6(1)(a)              |
| Game history and leaderboards     | Submissions, votes, scores             | Consent — Art. 6(1)(a)              |
| Security monitoring               | Operational logs (no IP at info level) | Legitimate interest — Art. 6(1)(f)  |
| Admin accountability              | Audit log                              | Legitimate interest — Art. 6(1)(f)  |

---

## 3. Categories of personal data collected

- **Email address** — used to send authentication links; not shared with other players
- **Username** — displayed in-game and on leaderboards; visible to all players in a room
- **Consent timestamp** — recorded when you accept this policy at registration
- **Game submissions** — captions or answers you submit during games; visible to room players
- **Game scores** — points earned per game; visible on leaderboards
- **Session cookie** — one `HttpOnly` functional cookie for authentication; no tracking

---

## 4. Data retention (Art. 13(2)(a))

| Data                                      | Retained for                                                  |
| ----------------------------------------- | ------------------------------------------------------------- |
| Account (email, username)                 | Until you request deletion                                    |
| Game history (rooms, submissions, scores) | 2 years after the game ends, then deleted                     |
| Session cookie                            | 30 days, renewed on each visit                                |
| Authentication tokens                     | 15 minutes, single-use                                        |
| Operational logs                          | Up to 30 days (automatic rotation)                            |
| Database backups                          | 7 days; may contain your data for up to 7 days after deletion |

---

## 5. Your rights (Art. 13(2)(b))

| Right                            | How to exercise                                      |
| -------------------------------- | ---------------------------------------------------- |
| **Access** (Art. 15)             | Download your data at Profile → "Download My Data"   |
| **Portability** (Art. 20)        | Same as above — downloads as JSON                    |
| **Rectification** (Art. 16)      | Update username or email at Profile → edit fields    |
| **Erasure** (Art. 17)            | Email contact@libresoftware.cloud — processed within 30 days    |
| **Objection** (Art. 21)          | Email contact@libresoftware.cloud                               |
| **Withdraw consent** (Art. 7(3)) | Withdrawal = erasure request; email contact@libresoftware.cloud |

---

## 6. How to lodge a complaint (Art. 13(2)(d))

If you believe your data is being processed unlawfully, you have the right to lodge a complaint with your national data protection authority:

- **France:** CNIL — <https://www.cnil.fr>
- **Germany:** BfDI — <https://www.bfdi.bund.de>
- **UK:** ICO — <https://ico.org.uk>
- **Other EU/EEA:** <https://edpb.europa.eu/about-edpb/about-edpb/members_en>

---

## 7. Minimum age (Art. 8)

This platform is intended for users aged **16 and above**. By registering, you confirm that you meet this requirement. If you are under 16, parental consent must be obtained — contact contact@libresoftware.cloud.

---

## 8. Cookies (Art. 13)

This platform sets **one** cookie:

| Name      | Type                                    | Purpose                                     | Duration |
| --------- | --------------------------------------- | ------------------------------------------- | -------- |
| `session` | `HttpOnly`, `Secure`, `SameSite=Strict` | Authentication — required to stay logged in | 30 days  |

No tracking, analytics, or advertising cookies are used. No third-party cookies are set by this application.

---

## 9. Data processors (Art. 28)

Your email address is transmitted to our email provider to send authentication links:

| Processor                    | Role                           | Data sent                                | DPA in place                                           |
| ---------------------------- | ------------------------------ | ---------------------------------------- | ------------------------------------------------------ |
| OVHcloud (OVH SAS, France)   | Transactional SMTP relay       | Your email address, authentication link  | Yes — OVHcloud publishes a standard GDPR DPA (Art. 28) |

OVHcloud is a European (French) hosting provider, so your email address stays within the EU when authentication links are sent. No data is transferred outside the EU/EEA.

All other data (username, submissions, votes, scores, session, logs, backups) is stored on the operator's own infrastructure and is not shared with any third party.

---

## 10. Backup disclosure

Database backups are retained for 7 days for disaster recovery. If you request erasure, your data is deleted from the live database immediately, but may persist in backups for up to 7 days. This is permitted under GDPR Art. 17(3)(b) (legitimate interest — incident recovery).

---

_Last updated: 2026-04-13_  
_Version: 1.0_
