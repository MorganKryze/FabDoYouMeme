# FabDoYouMeme — Implementation Status

Master tracker. Check boxes as phases complete. Each phase links to a detailed sub-plan.

> Start each session by reading this file, picking the first unchecked phase, and opening its sub-plan.

---

## Phases

| #   | Phase                                                       | Sub-plan                                                                                | Status |
| --- | ----------------------------------------------------------- | --------------------------------------------------------------------------------------- | ------ |
| 1   | Infrastructure & project bootstrap                          | [01-infrastructure.md](superpowers/plans/2026-04-03-01-infrastructure.md)               | `[x]`  |
| 2   | Database — migrations + sqlc                                | [02-backend-db.md](superpowers/plans/2026-04-03-02-backend-db.md)                       | `[x]`  |
| 3   | Backend — config + middleware                               | [03-backend-core.md](superpowers/plans/2026-04-03-03-backend-core.md)                   | `[x]`  |
| 4   | Backend — auth (sessions, magic links, invites)             | [04-backend-auth.md](superpowers/plans/2026-04-03-04-backend-auth.md)                   | `[x]`  |
| 5   | Backend — storage (RustFS) + email                          | [05-backend-storage-email.md](superpowers/plans/2026-04-03-05-backend-storage-email.md) | `[x]`  |
| 6   | Backend — game engine (registry, hub, meme-caption)         | [06-backend-game.md](superpowers/plans/2026-04-03-06-backend-game.md)                   | `[x]`  |
| 7   | Backend — REST API wiring + observability                   | [07-backend-api.md](superpowers/plans/2026-04-03-07-backend-api.md)                     | `[x]`  |
| 8   | Frontend — project setup + state layer + API client         | [08-frontend-setup.md](superpowers/plans/2026-04-03-08-frontend-setup.md)               | `[ ]`  |
| 9   | Frontend — auth routes (register, magic-link, verify)       | [09-frontend-auth.md](superpowers/plans/2026-04-03-09-frontend-auth.md)                 | `[ ]`  |
| 10  | Frontend — app routes (lobby, rooms, profile) + game plugin | [10-frontend-app.md](superpowers/plans/2026-04-03-10-frontend-app.md)                   | `[ ]`  |
| 11  | Frontend — studio                                           | [11-frontend-studio.md](superpowers/plans/2026-04-03-11-frontend-studio.md)             | `[ ]`  |
| 12  | Frontend — admin routes                                     | [12-frontend-admin.md](superpowers/plans/2026-04-03-12-frontend-admin.md)               | `[ ]`  |
| 13  | CI pipelines + production checklist                         | [13-ci-production.md](superpowers/plans/2026-04-03-13-ci-production.md)                 | `[ ]`  |

---

## Dependency order

```
Phase 1 (infra)
  └── Phase 2 (DB)
        └── Phase 3 (core)
              ├── Phase 4 (auth)
              │     ├── Phase 5 (storage/email) ← needed by auth email templates
              │     └── Phase 6 (game engine)
              │           └── Phase 7 (API wiring) ← brings it all together
              └── Phase 8 (frontend setup)
                    ├── Phase 9 (frontend auth)
                    ├── Phase 10 (frontend app) ← needs Phase 7 running
                    ├── Phase 11 (studio)
                    └── Phase 12 (admin)
                          └── Phase 13 (CI)
```

Phases 8–12 can start in parallel with Phase 7 if the backend API contract is stable.

---

## Design docs reference

All specs live in `design/`. Key files per phase:

| Phase           | Primary design doc(s)                                                             |
| --------------- | --------------------------------------------------------------------------------- |
| 2 (DB)          | `design/03-data.md` — full schema + cleanup SQL                                   |
| 3 (core)        | `design/06-operations.md` — logging schema; `design/ref-env-vars.md`              |
| 4 (auth)        | `design/02-identity.md` — full flow; `design/ref-error-codes.md`                  |
| 5 (storage)     | `design/03-data.md` — RustFS setup; `design/02-identity.md` — email templates     |
| 6 (game)        | `design/04-protocol.md` — GameTypeHandler interface + WS protocol                 |
| 7 (API)         | `design/04-protocol.md` — all REST endpoints; `design/06-operations.md` — metrics |
| 8–12 (frontend) | `design/05-frontend.md` — full UX spec                                            |
| GDPR            | `design/ref-gdpr.md`, `design/02-identity.md`, `design/03-data.md`                |
