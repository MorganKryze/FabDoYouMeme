# P5.1 Frontend Test Bootstrap — Design Spec

Date: 2026-04-11  
Scope: vitest + @testing-library/svelte setup and ~10 unit/component tests  
Out of scope: Playwright E2E (P5.2 — deferred; see punch-list P5.2 note)

---

## 1. Goal

Add a viable frontend test layer that CI can enforce. The primary target is state-logic unit tests (fast, deterministic, no DOM) plus a small set of component smoke tests. No production code changes required.

---

## 2. Infrastructure

### 2.1 New dependencies (`frontend/package.json` devDependencies)

| Package                     | Purpose                                      |
| --------------------------- | -------------------------------------------- |
| `vitest`                    | Test runner (Vite-native)                    |
| `@vitest/coverage-v8`       | Coverage reporting                           |
| `@testing-library/svelte`   | Component mounting + queries                 |
| `@testing-library/jest-dom` | Extended DOM matchers (`toBeDisabled`, etc.) |
| `happy-dom`                 | Lightweight browser environment for vitest   |

### 2.2 New scripts (`frontend/package.json`)

```json
"test":       "vitest run",
"test:watch": "vitest"
```

### 2.3 `frontend/vitest.config.ts`

Extends `vite.config.ts` via `mergeConfig` so the same `@sveltejs/vite-plugin-svelte` pipeline compiles Svelte 5 runes identically to the app build.

Key settings:

- `environment: 'happy-dom'`
- `resolve.alias` maps `$env/dynamic/public` → `src/test/env-mock.ts`
- `resolve.alias` maps `$lib` → `src/lib` (mirrors SvelteKit)
- `setupFiles: ['src/test/setup.ts']`

### 2.4 `frontend/src/test/env-mock.ts`

```ts
export const env = { PUBLIC_API_URL: 'http://localhost:8080' };
```

### 2.5 `frontend/src/test/setup.ts`

```ts
import '@testing-library/jest-dom';
```

---

## 3. Test Suite

All tests instantiate **fresh class instances** (not the module-level singletons) to prevent inter-test state leakage.

### 3.1 State unit tests

#### `src/lib/state/toast.svelte.test.ts` (3 tests)

| Test                       | Assertion                                                              |
| -------------------------- | ---------------------------------------------------------------------- |
| caps at 3 items            | After 4 `show()` calls, `items.length === 3` and the oldest is dropped |
| dismiss removes by id      | `dismiss(id)` removes only the targeted item                           |
| error type gets duration 0 | `show('msg', 'error')` → `items[0].duration === 0`                     |

#### `src/lib/state/user.svelte.test.ts` (4 tests)

| Test                               | Assertion                                              |
| ---------------------------------- | ------------------------------------------------------ |
| isAuthenticated false initially    | `user.isAuthenticated === false` on fresh instance     |
| isAuthenticated true after setFrom | After `setFrom({...})`, `isAuthenticated === true`     |
| isAdmin derived correctly          | `setFrom({ role: 'admin' })` → `isAdmin === true`      |
| clear resets all fields            | After `clear()`, all fields null, derived values false |

#### `src/lib/state/room.svelte.test.ts` (5 tests)

| Test                                   | Assertion                                                                                  |
| -------------------------------------- | ------------------------------------------------------------------------------------------ |
| player_joined appends player           | Player added to `players` array                                                            |
| player_left removes player             | Player removed by `user_id`                                                                |
| round_started resets submit/vote flags | `hasSubmitted` and `hasVoted` reset to false; `phase === 'submitting'`                     |
| game_ended sets finished state         | `state === 'finished'`; `leaderboard` populated                                            |
| error already_submitted sets flag      | `handleMessage({type:'error', data:{code:'already_submitted'}})` → `hasSubmitted === true` |

### 3.2 Component smoke tests

`Toast.svelte`, `SubmitForm`, and `VoteForm` import the module-level state singletons directly — they do not accept state as props. Component tests therefore import the same singletons (`toast`, `room`, `user`) and mutate them before mounting. A `beforeEach` resets all relevant singleton fields to prevent inter-test leakage.

#### `src/lib/components/Toast.svelte.test.ts` (2 tests)

| Test                             | Assertion                         |
| -------------------------------- | --------------------------------- |
| renders nothing with empty items | No toast elements in DOM          |
| renders message when item added  | Toast text appears after `show()` |

#### `src/lib/games/meme-caption/SubmitForm.svelte.test.ts` (2 tests)

| Test                                      | Assertion                                            |
| ----------------------------------------- | ---------------------------------------------------- |
| submit button disabled when caption empty | `getByRole('button', {name: /submit/i})` is disabled |
| submit button enabled with caption text   | Typing into textarea enables the button              |

#### `src/lib/games/meme-caption/VoteForm.svelte.test.ts` (2 tests)

| Test                                     | Assertion                                                       |
| ---------------------------------------- | --------------------------------------------------------------- |
| renders one card per submission          | N submissions → N vote cards in DOM                             |
| vote button disabled until card selected | Button disabled before selection; enabled after clicking a card |

---

## 4. CI integration

The existing `frontend.yml` workflow (updated in P4.2) already has a vitest step. The new `"test"` script in `package.json` is what that step calls. No further CI changes required.

---

## 5. Not in scope

- Playwright E2E tests (P5.2 — deferred; requires full Docker stack)
- Tests for `ws.svelte.ts` (imports `WebSocket` global; deferred until a `FakeWebSocket` test helper exists)
- Tests for `studio.svelte.ts` (depends on many API calls; lower ROI than game state)
- Any test for SvelteKit route files (`+page.svelte`, `+layout.svelte`)

---

## 6. Future work (P5.2 — Playwright)

When the team is ready to add Playwright E2E:

- Flow: `register → magic-link verify → create room → join → start → submit caption → vote → results`
- Requires: full Docker stack (`make dev`), a seeded invite token, a test admin account
- Files: `frontend/tests/e2e/smoke.spec.ts`
- Config: `frontend/playwright.config.ts`
- Effort: ~0.5d once the stack is stable
