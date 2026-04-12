# FabDoYouMeme — Documentation

FabDoYouMeme is a self-hosted, invite-only multiplayer party game platform. It ships with meme-captioning games and is built to support any turn-based party game without schema or protocol changes.

---

## Choose your path

### I want to run this platform

| Document                              | What it covers                                                |
| ------------------------------------- | ------------------------------------------------------------- |
| [Self-hosting guide](self-hosting.md) | Prerequisites, first boot, environment variables, admin setup |
| [Operations](operations.md)           | Monitoring, logs, backups, CI, production checklist           |

### I want to understand how it works

| Document                                | What it covers                                     |
| --------------------------------------- | -------------------------------------------------- |
| [Overview](overview.md)                 | Project goals, philosophy, tech stack choices      |
| [Architecture](architecture.md)         | System components, data layer, storage, middleware |
| [Auth & Identity](auth-and-identity.md) | Registration, magic links, sessions, GDPR          |
| [Game Engine](game-engine.md)           | How rooms, rounds, and game types work             |
| [API](api.md)                           | REST endpoints, WebSocket protocol, error model    |
| [Frontend](frontend.md)                 | SvelteKit structure, reactive state, routing       |

---

## Reference

| Document                                               | What it covers                                                                  |
| ------------------------------------------------------ | ------------------------------------------------------------------------------- |
| [Brand & identity](brand.md)                           | Name rationale, voice, vocabulary, visual direction, namespace audit — living   |
| [Error codes](reference/error-codes.md)                | Every `snake_case` error code — REST and WebSocket                              |
| [Architectural decisions](reference/decisions.md)      | ADR-001–ADR-010: why each non-obvious choice was made                           |
| [GDPR compliance](reference/gdpr.md)                   | Lawful basis, data inventory, subject rights, breach procedure                  |
| [Privacy policy template](reference/privacy-policy.md) | Art. 13(1) stub — complete before inviting first users                          |

---

## Quick flows

**First boot:** set `SEED_ADMIN_EMAIL` → backend creates admin user and sends magic link on startup.

**Player onboarding:** admin creates invite → player registers with token → receives magic link → clicks once → in.

**Game session:** create room → players join via WebSocket → host starts → rounds of submit/vote/results → game ends.

**Asset upload:** admin creates item → requests pre-signed upload URL → uploads directly to RustFS → confirms item with version.
