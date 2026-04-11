# P5.1 Frontend Test Bootstrap — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up vitest + `@testing-library/svelte` in `frontend/` and land ~10 passing state/component smoke tests so CI can gate regressions.

**Architecture:** vitest with `happy-dom` environment; a standalone `vitest.config.ts` that merges the existing `vite.config.ts` (so the `sveltekit()` plugin compiles Svelte 5 runes identically to the app build). `$env/dynamic/public` is aliased to a static mock file to avoid depending on SvelteKit's runtime. State classes are tested by instantiating fresh instances of the class (not the exported module singletons); components are tested by importing the module singletons and resetting them in `beforeEach`.

**Tech Stack:** vitest, `@vitest/coverage-v8`, `@testing-library/svelte@^5.2`, `@testing-library/jest-dom@^6`, `happy-dom`, Svelte 5 runes, SvelteKit adapter-node.

**Spec:** `docs/superpowers/specs/2026-04-11-p5-frontend-test-bootstrap-design.md`

**Out of scope:** Playwright E2E (P5.2 — deferred), tests for `ws.svelte.ts` and `studio.svelte.ts`, route-level tests.

---

## File Structure

**New files:**

| Path | Responsibility |
|------|----------------|
| `frontend/vitest.config.ts` | Test runner config, merged with `vite.config.ts` |
| `frontend/src/test/env-mock.ts` | Static stub for `$env/dynamic/public` |
| `frontend/src/test/setup.ts` | Global test setup (jest-dom matchers) |
| `frontend/src/lib/state/toast.svelte.test.ts` | 3 unit tests for `ToastState` |
| `frontend/src/lib/state/user.svelte.test.ts` | 4 unit tests for `UserState` |
| `frontend/src/lib/state/room.svelte.test.ts` | 5 unit tests for `RoomState.handleMessage` |
| `frontend/src/lib/components/Toast.svelte.test.ts` | 2 smoke tests for `Toast.svelte` |
| `frontend/src/lib/games/meme-caption/SubmitForm.svelte.test.ts` | 2 smoke tests for `SubmitForm.svelte` |
| `frontend/src/lib/games/meme-caption/VoteForm.svelte.test.ts` | 2 smoke tests for `VoteForm.svelte` |

**Modified files:**

| Path | Change |
|------|--------|
| `frontend/package.json` | Add devDeps + `test` / `test:watch` scripts |
| `frontend/src/lib/state/user.svelte.ts` | **EXPORT** the `UserState` class so tests can instantiate fresh instances (currently only the singleton is exported) |
| `frontend/src/lib/state/room.svelte.ts` | **EXPORT** the `RoomState` class so tests can instantiate fresh instances |
| `frontend/src/lib/state/toast.svelte.ts` | **EXPORT** the `ToastState` class so tests can instantiate fresh instances |
| `docs/review/2026-04-10/99-punch-list.md` | Tick P5.1 as done |

**Why export the state classes?** The current files only export the `toast`, `user`, `room` singletons — the classes themselves are private. State unit tests need isolated instances per test (`new RoomState()`) to avoid cross-test contamination. Exporting the class is a one-word change per file and does not affect existing callers.

---

## Task 1: Install dev dependencies and add test scripts

**Files:**
- Modify: `frontend/package.json`

- [ ] **Step 1: Add dev dependencies**

Run from the `frontend/` directory:

```bash
cd frontend && npm install --save-dev \
  vitest@^3.2.0 \
  @vitest/coverage-v8@^3.2.0 \
  @testing-library/svelte@^5.2.8 \
  @testing-library/jest-dom@^6.6.0 \
  happy-dom@^18.0.0
```

Expected: the five packages appear under `devDependencies` in `frontend/package.json` and `frontend/package-lock.json` updates. No runtime dependency changes.

- [ ] **Step 2: Add `test` / `test:watch` scripts**

Edit `frontend/package.json` and add to the `"scripts"` block (after `"check:watch"`):

```json
"test": "vitest run",
"test:watch": "vitest"
```

After this change, the scripts block should look like:

```json
"scripts": {
    "dev": "vite dev",
    "build": "vite build",
    "preview": "vite preview",
    "prepare": "svelte-kit sync || echo ''",
    "check": "svelte-kit sync && svelte-check --tsconfig ./tsconfig.json",
    "check:watch": "svelte-kit sync && svelte-check --tsconfig ./tsconfig.json --watch",
    "test": "vitest run",
    "test:watch": "vitest"
}
```

- [ ] **Step 3: Verify install worked**

Run: `cd frontend && npx vitest --version`
Expected: prints a vitest version string matching `3.x.x`.

- [ ] **Step 4: Commit**

```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "chore(frontend): add vitest + testing-library devDeps and test scripts (P5.1)"
```

---

## Task 2: Test environment mock and setup file

**Files:**
- Create: `frontend/src/test/env-mock.ts`
- Create: `frontend/src/test/setup.ts`

- [ ] **Step 1: Create the `$env/dynamic/public` mock**

Create `frontend/src/test/env-mock.ts` with exactly:

```ts
// Static stub for SvelteKit's `$env/dynamic/public` so vitest does not
// need to spin up the SvelteKit runtime to resolve public env vars.
export const env = {
  PUBLIC_API_URL: 'http://localhost:8080'
};
```

- [ ] **Step 2: Create the global setup file**

Create `frontend/src/test/setup.ts` with exactly:

```ts
import '@testing-library/jest-dom/vitest';
```

This registers matchers like `toBeDisabled()`, `toBeInTheDocument()` on vitest's `expect`.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/test/env-mock.ts frontend/src/test/setup.ts
git commit -m "test(frontend): add env mock and jest-dom setup for vitest (P5.1)"
```

---

## Task 3: vitest.config.ts

**Files:**
- Create: `frontend/vitest.config.ts`

- [ ] **Step 1: Write `vitest.config.ts`**

Create `frontend/vitest.config.ts` with exactly:

```ts
import { defineConfig, mergeConfig } from 'vitest/config';
import { fileURLToPath } from 'node:url';
import viteConfig from './vite.config';

export default mergeConfig(
  viteConfig,
  defineConfig({
    resolve: {
      alias: {
        // Avoid SvelteKit's runtime `$env` resolver — use a static stub.
        '$env/dynamic/public': fileURLToPath(
          new URL('./src/test/env-mock.ts', import.meta.url)
        ),
        // Explicit `$lib` alias. The sveltekit() vite plugin normally
        // provides this, but its configResolved-time registration can
        // race with vitest's config merge — pinning it here avoids the
        // intermittent "Cannot find module '$lib/...'" failure.
        $lib: fileURLToPath(new URL('./src/lib', import.meta.url))
      }
    },
    test: {
      environment: 'happy-dom',
      globals: false,
      include: ['src/**/*.{test,spec}.{js,ts}'],
      setupFiles: ['src/test/setup.ts'],
      // Svelte 5 runes compile through @sveltejs/vite-plugin-svelte which is
      // already enabled by the sveltekit() plugin inherited from vite.config.
      clearMocks: true,
      restoreMocks: true
    }
  })
);
```

- [ ] **Step 2: Sanity check — run vitest against an empty suite**

Run: `cd frontend && npx vitest run`

Expected output contains:
```
No test files found
```
and exit code 0. This confirms the config loads without errors before any tests exist.

If you see an error about `$env/dynamic/public`, double-check the alias path in step 1.

- [ ] **Step 3: Commit**

```bash
git add frontend/vitest.config.ts
git commit -m "test(frontend): add vitest config extending vite.config (P5.1)"
```

---

## Task 4: Export the state classes for test isolation

**Files:**
- Modify: `frontend/src/lib/state/toast.svelte.ts`
- Modify: `frontend/src/lib/state/user.svelte.ts`
- Modify: `frontend/src/lib/state/room.svelte.ts`

- [ ] **Step 1: Export `ToastState`**

In `frontend/src/lib/state/toast.svelte.ts` find the line:

```ts
class ToastState {
```

and change it to:

```ts
export class ToastState {
```

No other changes. The module-level `export const toast = new ToastState();` stays.

- [ ] **Step 2: Export `UserState`**

In `frontend/src/lib/state/user.svelte.ts` find:

```ts
class UserState {
```

Change to:

```ts
export class UserState {
```

- [ ] **Step 3: Export `RoomState`**

In `frontend/src/lib/state/room.svelte.ts` find:

```ts
class RoomState {
```

Change to:

```ts
export class RoomState {
```

- [ ] **Step 4: Type-check still passes**

Run: `cd frontend && npm run check`

Expected: exit code 0, no new errors. Existing warnings about `state_referenced_locally` etc. are pre-existing and unrelated.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/state/toast.svelte.ts \
        frontend/src/lib/state/user.svelte.ts \
        frontend/src/lib/state/room.svelte.ts
git commit -m "refactor(frontend): export state classes for per-test isolation (P5.1)"
```

---

## Task 5: Toast state unit tests

**Files:**
- Create: `frontend/src/lib/state/toast.svelte.test.ts`

- [ ] **Step 1: Write the failing test file**

Create `frontend/src/lib/state/toast.svelte.test.ts` with exactly:

```ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ToastState } from './toast.svelte';

describe('ToastState', () => {
  let t: ToastState;

  beforeEach(() => {
    vi.useFakeTimers();
    t = new ToastState();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('caps visible items at 3 and drops the oldest', () => {
    t.show('a', 'success');
    t.show('b', 'success');
    t.show('c', 'success');
    t.show('d', 'success');

    expect(t.items).toHaveLength(3);
    expect(t.items.map((i) => i.message)).toEqual(['b', 'c', 'd']);
  });

  it('dismiss removes only the targeted item', () => {
    t.show('a', 'success');
    t.show('b', 'success');
    const firstId = t.items[0].id;

    t.dismiss(firstId);

    expect(t.items).toHaveLength(1);
    expect(t.items[0].message).toBe('b');
  });

  it('error type gets duration 0 (persistent)', () => {
    t.show('boom', 'error');

    expect(t.items).toHaveLength(1);
    expect(t.items[0].type).toBe('error');
    expect(t.items[0].duration).toBe(0);
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd frontend && npx vitest run src/lib/state/toast.svelte.test.ts`

Expected: **all 3 tests PASS**. (We are backfilling tests for already-working code, not red-then-green TDD.)

If a test fails, investigate whether the ToastState implementation matches the spec:
- cap is implemented at `if (this.#items.length >= 3)` — which trims to 2 before adding the 4th. The expected resulting length is 3.
- `show('msg', 'error')` sets `duration = 0`.

- [ ] **Step 3: Test sensitivity check (mutation check)**

Verify the test actually catches regressions. In `frontend/src/lib/state/toast.svelte.ts`, temporarily change:

```ts
if (this.#items.length >= 3) {
```

to:

```ts
if (this.#items.length >= 99) {
```

Then run: `cd frontend && npx vitest run src/lib/state/toast.svelte.test.ts`

Expected: the "caps visible items at 3" test FAILS with an assertion on length. This proves the test is sensitive to the cap logic.

Revert the source change. Re-run the tests to confirm they pass again.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/state/toast.svelte.test.ts
git commit -m "test(frontend): add unit tests for ToastState (P5.1)"
```

---

## Task 6: User state unit tests

**Files:**
- Create: `frontend/src/lib/state/user.svelte.test.ts`

- [ ] **Step 1: Write the test file**

Create `frontend/src/lib/state/user.svelte.test.ts` with exactly:

```ts
import { describe, it, expect, beforeEach } from 'vitest';
import { UserState } from './user.svelte';

describe('UserState', () => {
  let u: UserState;

  beforeEach(() => {
    u = new UserState();
  });

  it('is unauthenticated on a fresh instance', () => {
    expect(u.id).toBeNull();
    expect(u.isAuthenticated).toBe(false);
    expect(u.isAdmin).toBe(false);
  });

  it('becomes authenticated after setFrom', () => {
    u.setFrom({
      id: 'user-1',
      username: 'alice',
      email: 'alice@example.com',
      role: 'player'
    });

    expect(u.isAuthenticated).toBe(true);
    expect(u.id).toBe('user-1');
    expect(u.username).toBe('alice');
    expect(u.email).toBe('alice@example.com');
    expect(u.role).toBe('player');
    expect(u.isAdmin).toBe(false);
  });

  it('isAdmin is true when role is admin', () => {
    u.setFrom({
      id: 'admin-1',
      username: 'root',
      email: 'root@example.com',
      role: 'admin'
    });

    expect(u.isAdmin).toBe(true);
    expect(u.isAuthenticated).toBe(true);
  });

  it('clear() resets all fields', () => {
    u.setFrom({
      id: 'user-1',
      username: 'alice',
      email: 'alice@example.com',
      role: 'admin'
    });

    u.clear();

    expect(u.id).toBeNull();
    expect(u.username).toBeNull();
    expect(u.email).toBeNull();
    expect(u.role).toBeNull();
    expect(u.isAuthenticated).toBe(false);
    expect(u.isAdmin).toBe(false);
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd frontend && npx vitest run src/lib/state/user.svelte.test.ts`

Expected: all 4 tests PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/state/user.svelte.test.ts
git commit -m "test(frontend): add unit tests for UserState (P5.1)"
```

---

## Task 7: Room state unit tests

**Files:**
- Create: `frontend/src/lib/state/room.svelte.test.ts`

- [ ] **Step 1: Write the test file**

Create `frontend/src/lib/state/room.svelte.test.ts` with exactly:

```ts
import { describe, it, expect, beforeEach } from 'vitest';
import { RoomState } from './room.svelte';
import type { Player, Round, WsMessage } from '$lib/api/types';

function makePlayer(userId: string, username = userId): Player {
  return { user_id: userId, username, connected: true };
}

function makeRound(n: number): Round {
  return {
    round_number: n,
    ends_at: new Date(Date.now() + 60_000).toISOString(),
    duration_seconds: 60,
    item: { payload: {} }
  };
}

describe('RoomState.handleMessage', () => {
  let r: RoomState;

  beforeEach(() => {
    r = new RoomState();
    r.init({
      code: 'ABCD',
      game_type: {
        id: 'g1',
        slug: 'meme-caption',
        name: 'Meme Caption',
        description: null,
        version: '1',
        supports_solo: true,
        config: {
          min_round_duration_seconds: 10,
          max_round_duration_seconds: 120,
          default_round_duration_seconds: 60,
          min_voting_duration_seconds: 10,
          max_voting_duration_seconds: 60,
          default_voting_duration_seconds: 20,
          min_players: 2,
          max_players: 8,
          min_round_count: 1,
          max_round_count: 10,
          default_round_count: 3
        },
        supported_payload_versions: [1]
      },
      state: 'lobby',
      players: [makePlayer('u1', 'alice')],
      host_id: 'u1'
    });
  });

  it('player_joined appends a new player', () => {
    const msg: WsMessage = {
      type: 'player_joined',
      data: makePlayer('u2', 'bob')
    };

    r.handleMessage(msg);

    expect(r.players).toHaveLength(2);
    expect(r.players[1].user_id).toBe('u2');
    expect(r.players[1].is_host).toBe(false);
  });

  it('player_left removes the player by user_id', () => {
    r.handleMessage({ type: 'player_joined', data: makePlayer('u2') });
    expect(r.players).toHaveLength(2);

    r.handleMessage({ type: 'player_left', data: makePlayer('u2') });

    expect(r.players).toHaveLength(1);
    expect(r.players[0].user_id).toBe('u1');
  });

  it('round_started sets phase to submitting and clears submit/vote flags', () => {
    r.hasSubmitted = true;
    r.hasVoted = true;
    r.phase = 'results';
    r.submissions = [
      { id: 's-old', user_id: 'u1', username: 'alice', caption: 'stale' }
    ];

    r.handleMessage({ type: 'round_started', data: makeRound(1) });

    expect(r.phase).toBe('submitting');
    expect(r.hasSubmitted).toBe(false);
    expect(r.hasVoted).toBe(false);
    expect(r.submissions).toHaveLength(0);
    expect(r.currentRound?.round_number).toBe(1);
  });

  it('game_ended sets state to finished and captures leaderboard', () => {
    r.handleMessage({
      type: 'game_ended',
      data: {
        reason: 'normal',
        leaderboard: [
          { user_id: 'u1', username: 'alice', total_score: 10, rank: 1 }
        ]
      }
    });

    expect(r.state).toBe('finished');
    expect(r.phase).toBe('idle');
    expect(r.endReason).toBe('normal');
    expect(r.leaderboard).toHaveLength(1);
    expect(r.leaderboard[0].rank).toBe(1);
  });

  it('error with code already_submitted marks hasSubmitted', () => {
    expect(r.hasSubmitted).toBe(false);

    r.handleMessage({
      type: 'error',
      data: { code: 'already_submitted', message: 'nope' }
    });

    expect(r.hasSubmitted).toBe(true);
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd frontend && npx vitest run src/lib/state/room.svelte.test.ts`

Expected: all 5 tests PASS.

If the `$lib/api/types` import fails to resolve, confirm that the vitest config inherited `$lib` aliasing from `vite.config.ts` via the `sveltekit()` plugin. (SvelteKit's plugin provides `$lib` automatically.) If it still fails, add `'$lib': fileURLToPath(new URL('./src/lib', import.meta.url))` to the `resolve.alias` block in `vitest.config.ts`.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/state/room.svelte.test.ts
git commit -m "test(frontend): add unit tests for RoomState.handleMessage (P5.1)"
```

---

## Task 8: Toast component smoke test

**Files:**
- Create: `frontend/src/lib/components/Toast.svelte.test.ts`

- [ ] **Step 1: Write the test file**

Create `frontend/src/lib/components/Toast.svelte.test.ts` with exactly:

```ts
import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { render } from '@testing-library/svelte';
import Toast from './Toast.svelte';
import { toast } from '$lib/state/toast.svelte';

describe('Toast.svelte', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // Reset the module-level singleton between tests.
    for (const item of [...toast.items]) {
      toast.dismiss(item.id);
    }
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders no alerts when the toast list is empty', () => {
    const { queryAllByRole } = render(Toast);

    expect(queryAllByRole('alert')).toHaveLength(0);
  });

  it('renders a message after toast.show() is called', async () => {
    const { findByRole } = render(Toast);

    toast.show('hello world', 'success');

    const alert = await findByRole('alert');
    expect(alert).toHaveTextContent('hello world');
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd frontend && npx vitest run src/lib/components/Toast.svelte.test.ts`

Expected: both tests PASS.

Common issues:
- If you see "Cannot find module '@testing-library/svelte'", re-run `npm install` in `frontend/`.
- If Svelte 5 runes error with "Cannot access $state outside of a component", the vitest config did not inherit the Svelte plugin — re-check Task 3.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/Toast.svelte.test.ts
git commit -m "test(frontend): add smoke test for Toast component (P5.1)"
```

---

## Task 9: SubmitForm component smoke test

**Files:**
- Create: `frontend/src/lib/games/meme-caption/SubmitForm.svelte.test.ts`

- [ ] **Step 1: Write the test file**

Create `frontend/src/lib/games/meme-caption/SubmitForm.svelte.test.ts` with exactly:

```ts
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import SubmitForm from './SubmitForm.svelte';
import { room } from '$lib/state/room.svelte';
import type { Round } from '$lib/api/types';

function makeRound(): Round {
  return {
    round_number: 1,
    // 120s in the future — keeps the component in the submitting state.
    ends_at: new Date(Date.now() + 120_000).toISOString(),
    duration_seconds: 120,
    item: { payload: {} }
  };
}

describe('SubmitForm.svelte', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'setInterval', 'Date'] });
    room.reset();
    room.players = [
      { user_id: 'u1', username: 'alice', connected: true }
    ];
  });

  afterEach(() => {
    vi.useRealTimers();
    room.reset();
  });

  it('disables the submit button when caption is empty', () => {
    const { getByRole } = render(SubmitForm, { props: { round: makeRound() } });

    const button = getByRole('button', { name: /submit/i });
    expect(button).toBeDisabled();
  });

  it('enables the submit button once caption has non-whitespace text', async () => {
    const { getByRole, getByPlaceholderText } = render(SubmitForm, {
      props: { round: makeRound() }
    });

    const textarea = getByPlaceholderText(/write your caption/i);
    await fireEvent.input(textarea, { target: { value: 'funny' } });

    const button = getByRole('button', { name: /submit/i });
    expect(button).not.toBeDisabled();
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd frontend && npx vitest run src/lib/games/meme-caption/SubmitForm.svelte.test.ts`

Expected: both tests PASS.

Notes for debugging:
- `SubmitForm` uses `requestAnimationFrame` inside a `$effect`. happy-dom provides `requestAnimationFrame`, and the test never awaits animation frames, so the initial render is sufficient.
- `SubmitForm` iterates `room.players` to render submission status badges. The `beforeEach` seeds a single player so that loop does not crash.
- If the test fails with "ws is not defined" or similar, confirm that `$env/dynamic/public` is aliased in `vitest.config.ts` (`ws.svelte.ts` is transitively imported by `SubmitForm`).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/games/meme-caption/SubmitForm.svelte.test.ts
git commit -m "test(frontend): add smoke test for SubmitForm component (P5.1)"
```

---

## Task 10: VoteForm component smoke test

**Files:**
- Create: `frontend/src/lib/games/meme-caption/VoteForm.svelte.test.ts`

- [ ] **Step 1: Write the test file**

Create `frontend/src/lib/games/meme-caption/VoteForm.svelte.test.ts` with exactly:

```ts
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import VoteForm from './VoteForm.svelte';
import { room } from '$lib/state/room.svelte';
import { user } from '$lib/state/user.svelte';
import type { Submission } from '$lib/api/types';

const SUBMISSIONS: Submission[] = [
  { id: 's1', user_id: 'u1', username: 'alice', caption: 'first caption' },
  { id: 's2', user_id: 'u2', username: 'bob', caption: 'second caption' }
];

describe('VoteForm.svelte', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'setInterval', 'Date'] });
    room.reset();
    room.currentRound = {
      round_number: 1,
      ends_at: new Date(Date.now() + 60_000).toISOString(),
      duration_seconds: 60,
      item: { payload: {} }
    };
    user.setFrom({
      id: 'u3',
      username: 'carol',
      email: 'carol@example.com',
      role: 'player'
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    room.reset();
    user.clear();
  });

  it('renders one button per submission', () => {
    const { getAllByText } = render(VoteForm, { props: { submissions: SUBMISSIONS } });

    expect(getAllByText('first caption')).toHaveLength(1);
    expect(getAllByText('second caption')).toHaveLength(1);
  });

  it('disables the vote button until a card is selected', async () => {
    const { getByRole } = render(VoteForm, { props: { submissions: SUBMISSIONS } });

    const voteBtn = getByRole('button', { name: /^vote$/i });
    expect(voteBtn).toBeDisabled();

    // Click the first submission card (bob's — carol cannot vote for own).
    const card = getByRole('button', { name: /first caption/i });
    await fireEvent.click(card);

    expect(voteBtn).not.toBeDisabled();
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd frontend && npx vitest run src/lib/games/meme-caption/VoteForm.svelte.test.ts`

Expected: both tests PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/games/meme-caption/VoteForm.svelte.test.ts
git commit -m "test(frontend): add smoke test for VoteForm component (P5.1)"
```

---

## Task 11: Run the full suite and the CI test command

**Files:** none modified.

- [ ] **Step 1: Run the whole vitest suite**

Run: `cd frontend && npm test`

Expected output (exact counts):

```
 Test Files  6 passed (6)
      Tests  18 passed (18)
```

Count breakdown: 3 (toast) + 4 (user) + 5 (room) + 2 (Toast cmp) + 2 (SubmitForm) + 2 (VoteForm) = **18 tests across 6 files**.

- [ ] **Step 2: Run type check to confirm no TS regressions**

Run: `cd frontend && npm run check`

Expected: exits 0, no new TypeScript errors. Pre-existing `state_referenced_locally` warnings from other files are OK.

- [ ] **Step 3: Simulate the CI step**

The `frontend.yml` workflow calls `npm test --if-present`. Run it locally to confirm it passes:

```bash
cd frontend && npm test --if-present
```

Expected: same passing output as Step 1.

---

## Task 12: Tick P5.1 done in the punch list

**Files:**
- Modify: `docs/review/2026-04-10/99-punch-list.md`

- [ ] **Step 1: Mark P5.1 complete**

Find the line:

```
| P5.1 | ☐    | Add vitest + @testing-library/svelte; ~10 component/state tests | 1.5d   | `frontend/vitest.config.ts` (new), `frontend/src/**/*.test.ts` (new) |
```

and change `☐` to `✅`:

```
| P5.1 | ✅   | Add vitest + @testing-library/svelte; ~10 component/state tests | 1.5d   | `frontend/vitest.config.ts` (new), `frontend/src/**/*.test.ts` (new) |
```

- [ ] **Step 2: Commit**

```bash
git add docs/review/2026-04-10/99-punch-list.md
git commit -m "docs(review): tick P5.1 frontend test bootstrap complete"
```

---

## Done criteria

- `cd frontend && npm test` runs 18 passing tests across 6 files
- `cd frontend && npm run check` is still clean
- `frontend.yml` CI job now executes real vitest assertions (not the no-op)
- Punch list reflects P5.1 as done; P5.2 remains marked as deferred
