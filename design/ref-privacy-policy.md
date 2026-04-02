# ref — Privacy Policy Template

This document is a **stub template** for the Privacy Policy served at `/privacy`. It must be completed by the operator before inviting the first users. All `[PLACEHOLDER]` fields must be replaced with real content.

GDPR Art. 13(1) requires this information to be provided at the time personal data is collected (i.e., at registration). The consent checkbox on the registration page links here — incomplete content is a compliance failure.

---

## 1. Controller Identity (Art. 13(1)(a))

**Data controller**: [OPERATOR NAME]
**Contact email**: [OPERATOR EMAIL — used for erasure and access requests]
**Hosted at**: [DEPLOYMENT URL, e.g. https://meme.example.com]

---

## 2. Purposes and Lawful Bases (Art. 13(1)(c))

| Purpose                           | Data used                              | Lawful basis                        |
| --------------------------------- | -------------------------------------- | ----------------------------------- |
| Authentication (magic link login) | Email address                          | Contract performance — Art. 6(1)(b) |
| Account management                | Username, email                        | Consent — Art. 6(1)(a)              |
| Game history and leaderboards     | Submissions, votes, scores             | Consent — Art. 6(1)(a)              |
| Security monitoring               | Operational logs (no IP at info level) | Legitimate interest — Art. 6(1)(f)  |
| Admin accountability              | Audit log                              | Legitimate interest — Art. 6(1)(f)  |

---

## 3. Categories of Personal Data Collected (Art. 13(1))

- **Email address** — used to send authentication links; not shared with other players
- **Username** — displayed in-game and on leaderboards; visible to all players in a room
- **Consent timestamp** — recorded when you accept this policy at registration
- **Game submissions** — captions or answers you submit during games; visible to room players
- **Game scores** — points earned per game; visible on leaderboards
- **Session cookie** — one `HttpOnly` functional cookie for authentication; no tracking

---

## 4. Data Retention (Art. 13(2)(a))

| Data                                      | Retained for                                                  |
| ----------------------------------------- | ------------------------------------------------------------- |
| Account (email, username)                 | Until you request deletion                                    |
| Game history (rooms, submissions, scores) | 2 years after the game ends, then deleted                     |
| Session cookie                            | 30 days, renewed on each visit                                |
| Authentication tokens                     | 15 minutes, single-use                                        |
| Operational logs                          | Up to 30 days (automatic rotation)                            |
| Database backups                          | 7 days; may contain your data for up to 7 days after deletion |

---

## 5. Your Rights (Art. 13(2)(b))

You have the following rights under GDPR:

| Right                            | How to exercise                                      |
| -------------------------------- | ---------------------------------------------------- |
| **Access** (Art. 15)             | Download your data at Profile → "Download My Data"   |
| **Portability** (Art. 20)        | Same as above — downloads as JSON                    |
| **Rectification** (Art. 16)      | Update username or email at Profile → edit fields    |
| **Erasure** (Art. 17)            | Email [OPERATOR EMAIL] — processed within 30 days    |
| **Objection** (Art. 21)          | Email [OPERATOR EMAIL]                               |
| **Withdraw consent** (Art. 7(3)) | Withdrawal = erasure request; email [OPERATOR EMAIL] |

---

## 6. How to Lodge a Complaint (Art. 13(2)(d))

If you believe your data is being processed unlawfully, you have the right to lodge a complaint with your national data protection authority:

- **France**: CNIL — <https://www.cnil.fr>
- **Germany**: BfDI — <https://www.bfdi.bund.de>
- **UK**: ICO — <https://ico.org.uk>
- **Other EU/EEA**: <https://edpb.europa.eu/about-edpb/about-edpb/members_en>

---

## 7. Minimum Age (Art. 8)

This platform is intended for users aged **16 and above**. By registering, you confirm that you meet this requirement. If you are under 16, parental consent must be obtained — contact [OPERATOR EMAIL].

---

## 8. Cookies (Art. 13)

This platform sets **one** cookie:

| Name      | Type                                    | Purpose                                     | Duration |
| --------- | --------------------------------------- | ------------------------------------------- | -------- |
| `session` | `HttpOnly`, `Secure`, `SameSite=Strict` | Authentication — required to stay logged in | 30 days  |

No tracking, analytics, or advertising cookies are used. No third-party cookies are set by this application.

---

## 9. Data Processors (Art. 28)

Your email address is transmitted to our email provider to send authentication links:

| Processor                          | Data sent                               | DPA in place        |
| ---------------------------------- | --------------------------------------- | ------------------- |
| [SMTP PROVIDER NAME, e.g. Mailgun] | Your email address, authentication link | [YES / LINK TO DPA] |

All other data is stored on-premises and not shared with third parties.

---

## 10. Backup Disclosure

Database backups are retained for 7 days for disaster recovery. If you request erasure, your data is deleted from the live database immediately, but may persist in backups for up to 7 days. This is permitted under GDPR Art. 17(3)(b) (legitimate interest — incident recovery).

---

_Last updated: [DATE]_
_Version: 1.0_
