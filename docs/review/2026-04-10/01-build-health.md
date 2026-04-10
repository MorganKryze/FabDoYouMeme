# Stage 1 — Build & Static Health

Date: 2026-04-10
Scope: Verification-only commands. No source files modified.

## 1.1 Summary

| Check                                     | Result                                    | Status                  |
| ----------------------------------------- | ----------------------------------------- | ----------------------- |
| `go build ./...`                          | exit 0, no output                         | ✅ pass                 |
| `go vet ./...`                            | exit 0, no output                         | ✅ pass                 |
| `go test -race -count=1 ./...`            | 10/10 packages pass, ~30 s total          | ✅ pass                 |
| `npm run check` (frontend)                | **0 errors, 18 warnings** across 11 files | 🟡 warn                 |
| `npm run build` (frontend)                | built in 1.65 s                           | ✅ pass                 |
| `npm audit --audit-level=high` (frontend) | **3 low, 0 high** — flag passes           | ✅ pass (but see below) |
| `govulncheck ./...` (backend)             | **2 vulns flagged, both test-path**       | 🟡 warn                 |
| `sqlc` drift                              | **1 potential drift** via mtime heuristic | 🟡 warn                 |

**Bottom line**: the code compiles cleanly, tests pass with the race detector on, and neither `vet` nor the build step surface anything. The warnings below are all non-blocking, but each one deserves attention.

## 1.2 Backend test run

```
ok  github.com/MorganKryze/FabDoYouMeme/backend/db                                 4.390s
ok  github.com/MorganKryze/FabDoYouMeme/backend/internal/api                       3.618s
ok  github.com/MorganKryze/FabDoYouMeme/backend/internal/auth                      3.761s
ok  github.com/MorganKryze/FabDoYouMeme/backend/internal/config                    1.662s
ok  github.com/MorganKryze/FabDoYouMeme/backend/internal/email                     1.866s
ok  github.com/MorganKryze/FabDoYouMeme/backend/internal/game                      4.585s
ok  github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_caption   2.240s
ok  github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware                2.426s
ok  github.com/MorganKryze/FabDoYouMeme/backend/internal/storage                   1.214s
```

No race detector warnings. `cmd/server`, `db/migrations`, `db/sqlc`, and `internal/testutil` correctly have no test files.

**Crucial consequence for this audit**: the test suite is real and exercises testcontainers-backed Postgres. Stage 6 can build on this, not replace it.

## 1.3 Frontend `svelte-check` warnings (18)

All 0 errors, all warnings fall into three buckets:

### 1.3.1 Svelte 5 `state_referenced_locally` (9 occurrences)

Affected files:

- `src/lib/components/studio/TextEditor.svelte:6`
- `src/lib/games/meme-caption/SubmitForm.svelte:15`
- `src/routes/(admin)/admin/invites/+page.svelte:8`
- `src/routes/(admin)/admin/packs/+page.svelte:8`
- `src/routes/(admin)/admin/packs/[id]/+page.svelte:9`
- `src/routes/(admin)/admin/users/+page.svelte:9`
- `src/routes/(app)/+page.svelte:8`
- `src/routes/(public)/auth/register/+page.svelte:10`
- `src/routes/(public)/auth/register/+page.svelte:11`

Each warning says: _"This reference only captures the initial value of `X`. Did you mean to reference it inside a derived instead?"_

This is a classic Svelte 5 migration trap: in runes mode, capturing a prop or `$state` at the top of the component body yields a snapshot, not a reactive reference. When the prop updates later, the captured variable keeps showing the old value. If any of these components ever get re-mounted with new data (e.g. the admin list page after a form submission), the UI will appear stale.

**The compiler cannot tell whether the code is actually broken** — you might be relying on the snapshot intentionally — but this is extremely rarely what you want. Each site should be audited and wrapped in `$derived(...)` or accessed via the reactive binding directly.

**Impact**: potential data-freshness bugs in every listed component. Non-fatal but user-visible.

### 1.3.2 A11y: `autofocus` (5 occurrences)

- `src/lib/components/studio/PackNavigator.svelte:113`
- `src/routes/(admin)/admin/packs/+page.svelte:36`
- `src/routes/(app)/+page.svelte:157`
- `src/routes/(app)/profile/+page.svelte:68`
- `src/routes/(app)/profile/+page.svelte:103`
- `src/routes/(public)/auth/magic-link/+page.svelte:37`

Svelte's a11y rule against `autofocus` exists because it breaks screen readers and mobile keyboards. In a modal form these can be defended (user expects focus), but they should be focused imperatively with `onMount` + `element.focus()` so assistive technology picks up the transition properly.

### 1.3.3 A11y: unassociated labels (2 occurrences)

- `src/routes/(admin)/admin/packs/+page.svelte:35` and `:40`

`<label>` without a `for=` or wrapping the input. Screen readers won't announce the control.

### 1.3.4 Other (2 occurrences)

- `src/routes/(admin)/admin/packs/[id]/+page.svelte:46` — ambiguous self-closing `<div ... />`. Cosmetic.

## 1.4 Potential `sqlc` drift

- `sqlc.yaml` lives at `backend/sqlc.yaml` (not noted in CLAUDE.md — minor docs drift).
- `sqlc` CLI is not installed on this machine, so I cannot run `sqlc generate` (which would also write files and violate the read-only constraint).
- As a heuristic I compared mtimes of `db/queries/*.sql` vs. `db/sqlc/<basename>.sql.go`:

```
STALE: db/sqlc/users.sql.go older than db/queries/users.sql
```

One suspect: `users.sql`. Every other query file's generated counterpart is up-to-date. This could be:

1. A real drift — someone modified `users.sql` without running `sqlc generate`. Since the build passes, any drift would have to be semantically-compatible (e.g. a comment change, an added column that isn't yet returned, a reformatted WHERE clause).
2. A mtime artefact of `git checkout` order on this clone.

**Recommendation**: Stage 7 punch list should include "install `sqlc` in CI and run `sqlc diff` / `sqlc verify` as a build step" to turn this heuristic into a hard gate. The current CI has no sqlc check at all.

## 1.5 `govulncheck` findings

Two "called" vulnerabilities:

| ID           | Package                                         | Title                                                | Fixed in |
| ------------ | ----------------------------------------------- | ---------------------------------------------------- | -------- |
| GO-2026-4887 | `github.com/docker/docker@v28.5.2+incompatible` | Moby AuthZ plugin bypass on oversized request bodies | N/A      |
| GO-2026-4883 | `github.com/docker/docker@v28.5.2+incompatible` | Moby off-by-one in plugin privilege validation       | N/A      |

### Reality check

Both vulns are in the Docker engine library, which is transitively pulled in by `testcontainers-go` via `internal/testutil`. The traces show the reachability runs only through `testutil.SetupSuite` → `testcontainers.Run` → docker client code.

**`internal/testutil` is not imported by `cmd/server/main.go` or any production handler.** Production binary exposure is zero.

### Why govulncheck still flags it

1. `testutil` is a regular (non-test) package, so `-test=false` doesn't exclude it.
2. govulncheck reports at the module level; once any code path through the symbol is reachable from _any_ buildable package in the module, it's flagged.
3. Several traces are false-positives even within that module scope — e.g. `#66: hub.KickPlayer calls json.Marshal, which eventually calls stdcopy.init` — the analyzer's trace inference is heuristic.

### Recommendation

Three options, ranked:

1. **Accept & document**: add a `.govulncheck.yaml` (or equivalent suppression) listing these two IDs with a comment linking to this review. The CI step then passes until a fix becomes available upstream. Lowest effort, honest outcome.
2. **Isolate testutil**: split `internal/testutil` into two packages — one pure-Go helpers (rollback, naming, etc.) that remain in the normal tree, and one `internal/testutil/tc` that is referenced only from `_test.go` files via build tags. This gets govulncheck to stop seeing the docker deps at all. Moderate effort.
3. **Ignore**: remove govulncheck from CI. **Not recommended** — we lose coverage of real production vulns.

**Also noted**: the CI workflow at `.github/workflows/backend.yml:68` currently runs `govulncheck ./...` with no opts. The current state of that CI run must be either passing (because the scanner exit code handling is lenient) or silently failing and the team hasn't noticed.

Actually, re-reading the CI step: `go install ... govulncheck@latest && govulncheck ./...` — govulncheck exits 3 on reachable vulns, which fails the step. **If the team's most recent `main` CI run is green, something is suppressing this.** Needs investigation (Stage 5 punch-list item).

## 1.6 Frontend `npm audit`

```
3 low severity vulnerabilities
cookie <0.7.0  —  via @sveltejs/kit → @sveltejs/adapter-node
```

All low-severity, so the `--audit-level=high` gate in CI passes. The underlying issue is that `@sveltejs/kit` depends on a `cookie` version that has a name/path/domain out-of-bounds-character bug (GHSA-pxg6-pf52-xh8x). A non-breaking fix path isn't available (`npm audit fix --force` would downgrade to kit `0.0.30`).

**Recommendation**: leave as-is, revisit when SvelteKit ships a version with a bumped `cookie` dep.

## 1.7 Config loading edge cases spotted during reading

Noted while reading `backend/internal/config/config.go` in Stage 0, recorded here because they're static-analysis-adjacent:

- `CookieDomain` derivation (`config.go:82-84`) silently discards `url.Parse` errors. A malformed `FRONTEND_URL` leaves `CookieDomain` empty — sessions set with `Domain=""` rather than erroring out. Suggest fail-loud: `return nil, err`.
- No bounds validation on duration env vars. `RECONNECT_GRACE_WINDOW=-10s` is accepted and would make grace expire immediately, causing instant player removal on disconnect. Suggest a post-parse sanity check (`if d <= 0 { return error }`).
- `Config.CookieDomain` never used in production as far as I can tell — needs Stage 5 verification (search for usage).

## 1.8 CI health observations

The existing CI uses a **Postgres service container** it doesn't actually need, because `testutil.SetupSuite` spins its own testcontainer:

```yaml
# .github/workflows/backend.yml
services:
  postgres:
    image: postgres:17-alpine
    ...
env:
  DATABASE_URL: postgres://fabyoumeme:testpassword@localhost:5432/fabyoumeme_test
steps:
  - run: migrate -path ./db/migrations -database "$DATABASE_URL" up
  - run: go test -race -count=1 ./...
```

The `migrate ... up` step runs against that external service, but the tests ignore `DATABASE_URL` and start their own container. Consequences:

1. ~5–10 seconds per CI run wasted on the unused service.
2. The `migrate up` step only verifies "migrations don't explode against an empty DB on Linux" — a weak signal. It doesn't verify the schema matches `sqlc` or that tests pass against it.
3. Cognitive load: a reader of the workflow would reasonably assume the tests use the service and may make wrong changes.

**Recommendation**: delete the `services:` block and the `Run migrations` step. Let testcontainers do its thing. Document in the workflow that Docker-in-Docker is available on `ubuntu-latest` runners (it is by default).

## 1.9 Findings summary

| #    | Severity | Finding                                                                    | Stage owner    |
| ---- | -------- | -------------------------------------------------------------------------- | -------------- |
| 1.1  | 🟡 med   | 9 Svelte 5 `state_referenced_locally` warnings — potential stale-data bugs | Stage 3        |
| 1.2  | 🟡 med   | 7 a11y warnings (5 autofocus + 2 unassociated labels)                      | Stage 3        |
| 1.3  | 🟡 low   | 1 potential sqlc drift on `users.sql` (mtime heuristic)                    | Stage 7        |
| 1.4  | 🟡 med   | CI has no `sqlc diff` / `sqlc verify` gate                                 | Stage 6        |
| 1.5  | 🔵 info  | 2 govulncheck vulns — test-path only, but CI should pass cleanly           | Stage 5        |
| 1.6  | 🔵 info  | 3 low-severity npm audit (cookie <0.7.0 via SvelteKit) — gate passes       | Stage 5        |
| 1.7  | 🟡 low   | `config.Load()` swallows `url.Parse` error for `CookieDomain`              | Stage 4        |
| 1.8  | 🟡 low   | `config.Load()` has no bounds validation on duration env vars              | Stage 4        |
| 1.9  | 🟡 med   | CI `backend.yml` starts a Postgres service container the tests never use   | Stage 6        |
| 1.10 | 🟡 low   | CI `frontend.yml` runs zero tests — type-check only                        | Stage 6        |
| 1.11 | 🟡 low   | `sqlc.yaml` path not mentioned in CLAUDE.md                                | Stage 7 (docs) |

No 🔴 high-severity findings at this stage. Build health is **good**.
