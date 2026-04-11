# Security Policy

FabDoYouMeme is a small, self-hosted project. Your security reports help
keep every operator who runs it safer — thank you for taking the time.

## Supported versions

There are no tagged releases yet. The `main` branch is the only supported
line; `main` is always the intended stable target.

| Version | Supported |
| ------- | --------- |
| `main`  | yes       |
| older   | no        |

Self-hosters should redeploy from `main` to pick up security fixes.

## Reporting a vulnerability

**Do not open a public GitHub issue for security reports.**

Use one of:

1. **GitHub Private Security Advisory** (preferred):
   <https://github.com/MorganKryze/FabDoYouMeme/security/advisories/new>
2. **Email**: **<contact@libresoftware.cloud>**

Include what you can:

- What the issue is and how to reproduce it
- The commit SHA or date you tested against
- Impact — what an attacker could actually do
- A suggested fix, if you have one (optional)

A clear written description is enough. You don't need a working exploit.

## What to expect

- **Acknowledgement** within 72 hours.
- **Initial assessment** within 7 days.
- **Fix or written explanation** when I have one — the timeline depends on
  severity. Critical issues get triaged before anything else.
- **Credit** in the fix commit and release notes, unless you'd rather stay
  anonymous.

This is a one-person project, so response times are best-effort. If you
haven't heard back in 7 days, send a second email in case the first one
got lost in a filter.

## Scope

**In scope:**

- The `main` branch of this repository
- The default Docker Compose stack as documented in `docs/self-hosting.md`
- Authentication, sessions, invite and magic-link flows
- WebSocket protocol and rate-limiting
- File upload and asset handling

**Out of scope:**

- Third-party services you wire in yourself (SMTP provider, RustFS,
  reverse proxy, TLS termination)
- Your own reverse-proxy or network configuration
- Issues that require already-compromised host access to exploit
- Self-inflicted misconfiguration (weak `POSTGRES_PASSWORD`, leaked
  `.env`, exposing `/api/metrics` to the internet, etc.)
- Multi-instance concerns — FabDoYouMeme is single-process by design
  (see ADR-005 in `docs/reference/decisions.md`)

## Safe harbor

Good-faith security research is welcome. If you stay within scope, follow
responsible disclosure, and don't exfiltrate data beyond what's needed to
demonstrate the issue, I won't pursue legal action. Please coordinate with
me before any public disclosure.
