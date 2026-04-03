# Frontend Auth Routes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement all public-facing auth pages: landing `/`, register `/auth/register`, magic-link `/auth/magic-link`, verify `/auth/verify`, and privacy `/privacy`.

**Architecture:** SvelteKit (public) group layout — minimal chrome, no nav bar, centered content. All pages are server-rendered; auth state is loaded in `hooks.server.ts` (Phase 8). Forms call backend REST endpoints and redirect on success.

**Tech Stack:** SvelteKit 2, Svelte 5, Tailwind CSS v4, shadcn-svelte, lucide-svelte, typed API client from `$lib/api`

---

## Files

| File                                                           | Role                                                                       |
| -------------------------------------------------------------- | -------------------------------------------------------------------------- |
| `frontend/src/routes/(public)/+layout.svelte`                  | Minimal public layout — no nav, centered, footer with privacy link         |
| `frontend/src/routes/(public)/+page.svelte`                    | Landing page: room code join + hosting link                                |
| `frontend/src/routes/(public)/auth/register/+page.svelte`      | Registration form: invite token, username, email, consent + age checkboxes |
| `frontend/src/routes/(public)/auth/register/+page.server.ts`   | Server action: POST /api/auth/register                                     |
| `frontend/src/routes/(public)/auth/magic-link/+page.svelte`    | Magic-link request form: email field                                       |
| `frontend/src/routes/(public)/auth/magic-link/+page.server.ts` | Server action: POST /api/auth/magic-link                                   |
| `frontend/src/routes/(public)/auth/verify/+page.svelte`        | Verify intermediate page: "Log In" button                                  |
| `frontend/src/routes/(public)/auth/verify/+page.server.ts`     | Server action: POST /api/auth/verify + redirect                            |
| `frontend/src/routes/(public)/privacy/+page.svelte`            | Static privacy policy page                                                 |

---

## Task 1: Public Layout

**Files:**

- Create: `frontend/src/routes/(public)/+layout.svelte`

- [ ] **Step 1: Write the component**

```svelte
<!-- frontend/src/routes/(public)/+layout.svelte -->
<script lang="ts">
  import '../../../app.css';
  let { children } = $props();
</script>

<div class="min-h-screen flex flex-col items-center justify-center bg-background text-foreground px-4">
  <main class="w-full max-w-sm flex flex-col gap-6">
    {@render children()}
  </main>
  <footer class="mt-8 text-xs text-muted-foreground">
    <a href="/privacy" class="underline hover:text-foreground">Privacy Policy</a>
  </footer>
</div>
```

- [ ] **Step 2: Verify the layout renders**

```bash
cd frontend && npm run check
```

Expected: no type errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/\(public\)/+layout.svelte
git commit -m "feat(frontend): add public layout with footer privacy link"
```

---

## Task 2: Landing Page `/`

**Files:**

- Create: `frontend/src/routes/(public)/+page.svelte`

- [ ] **Step 1: Write the landing page**

```svelte
<!-- frontend/src/routes/(public)/+page.svelte -->
<script lang="ts">
  import { user } from '$lib/state/user.svelte';

  let code = $state('');

  function handleJoin() {
    const trimmed = code.trim().toUpperCase();
    if (trimmed.length !== 4) return;
    if (user.isAuthenticated) {
      window.location.href = `/rooms/${trimmed}`;
    } else {
      window.location.href = `/auth/magic-link?next=/rooms/${trimmed}`;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') handleJoin();
  }
</script>

<svelte:head>
  <title>FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col items-center gap-8 text-center">
  <h1 class="text-4xl font-bold tracking-tight">FabDoYouMeme</h1>

  <div class="w-full flex flex-col gap-3">
    <label for="room-code" class="text-sm font-medium text-muted-foreground">Enter a room code to join</label>
    <input
      id="room-code"
      type="text"
      inputmode="text"
      autocomplete="off"
      autocapitalize="characters"
      maxlength={4}
      placeholder="WXYZ"
      class="h-14 w-full rounded-lg border border-input bg-background px-4 text-center text-2xl font-mono tracking-widest uppercase focus:outline-none focus:ring-2 focus:ring-ring"
      bind:value={code}
      onkeydown={handleKeydown}
      autofocus
    />
    <button
      type="button"
      onclick={handleJoin}
      disabled={code.trim().length !== 4}
      class="h-12 rounded-lg bg-primary text-primary-foreground font-semibold text-base disabled:opacity-50 disabled:cursor-not-allowed hover:bg-primary/90 transition-colors"
    >
      Join Game
    </button>
  </div>

  <a
    href={user.isAuthenticated ? '/' : '/auth/magic-link?next=/'}
    class="text-sm text-muted-foreground hover:text-foreground transition-colors"
  >
    I'm hosting →
  </a>
</div>
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/\(public\)/+page.svelte
git commit -m "feat(frontend): add landing page with room code join"
```

---

## Task 3: Registration Page — UI

**Files:**

- Create: `frontend/src/routes/(public)/auth/register/+page.svelte`

- [ ] **Step 1: Write the registration form**

```svelte
<!-- frontend/src/routes/(public)/auth/register/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData } from './$types';

  let { form }: { form: ActionData } = $props();

  // Pre-fill invite token from ?invite= query param (set on server via load)
  let smtpWarning = $derived(form?.warning === 'smtp_failure');
  let success = $derived(form?.success === true);
</script>

<svelte:head>
  <title>Register — FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col gap-6">
  <div class="text-center">
    <h1 class="text-2xl font-bold">Create your account</h1>
    <p class="text-sm text-muted-foreground mt-1">You need an invite to join.</p>
  </div>

  {#if success && !smtpWarning}
    <div class="rounded-lg border border-green-200 bg-green-50 p-4 text-sm text-green-800">
      Account created! Check your email for your login link.
    </div>
  {/if}

  {#if smtpWarning}
    <div class="rounded-lg border border-yellow-200 bg-yellow-50 p-4 text-sm text-yellow-800">
      Account created, but the login email couldn't be sent. Ask your admin to resend.
    </div>
  {/if}

  {#if !success}
    <form method="POST" use:enhance class="flex flex-col gap-4">
      {#if form?.error}
        <div class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {form.error}
        </div>
      {/if}

      <div class="flex flex-col gap-1">
        <label for="invite_token" class="text-sm font-medium">Invite Token</label>
        <input
          id="invite_token"
          name="invite_token"
          type="text"
          required
          value={form?.invite_token ?? ''}
          class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          placeholder="abc123xyz789"
        />
      </div>

      <div class="flex flex-col gap-1">
        <label for="username" class="text-sm font-medium">Username</label>
        <input
          id="username"
          name="username"
          type="text"
          required
          minlength={3}
          maxlength={30}
          autocomplete="username"
          value={form?.username ?? ''}
          class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          placeholder="your_username"
        />
        <p class="text-xs text-muted-foreground">3–30 characters, letters, numbers, underscore.</p>
      </div>

      <div class="flex flex-col gap-1">
        <label for="email" class="text-sm font-medium">Email</label>
        <input
          id="email"
          name="email"
          type="email"
          required
          autocomplete="email"
          value={form?.email ?? ''}
          class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          placeholder="you@example.com"
        />
      </div>

      <label class="flex items-start gap-3 cursor-pointer">
        <input
          type="checkbox"
          name="consent"
          value="true"
          class="mt-0.5 h-4 w-4 rounded border-input"
          required
        />
        <span class="text-sm leading-snug">
          I have read and agree to the
          <a href="/privacy" class="underline hover:text-foreground" target="_blank">Privacy Policy</a>.
        </span>
      </label>

      <label class="flex items-start gap-3 cursor-pointer">
        <input
          type="checkbox"
          name="age_affirmation"
          value="true"
          class="mt-0.5 h-4 w-4 rounded border-input"
          required
        />
        <span class="text-sm leading-snug">I confirm I am at least 16 years old.</span>
      </label>

      <button
        type="submit"
        class="h-11 rounded-lg bg-primary text-primary-foreground font-semibold hover:bg-primary/90 transition-colors"
      >
        Create Account
      </button>
    </form>

    <p class="text-center text-sm text-muted-foreground">
      Already have an account?
      <a href="/auth/magic-link" class="underline hover:text-foreground">Sign in</a>
    </p>
  {/if}
</div>
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors (ActionData may be `null` initially — that's expected).

---

## Task 4: Registration Page — Server Action

**Files:**

- Create: `frontend/src/routes/(public)/auth/register/+page.server.ts`

- [ ] **Step 1: Write the server load + action**

```ts
// frontend/src/routes/(public)/auth/register/+page.server.ts
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const ERROR_MESSAGES: Record<string, string> = {
  invalid_invite: 'That invite token is invalid, expired, or already used.',
  consent_required: 'You must agree to the Privacy Policy to register.',
  age_affirmation_required: 'You must confirm you are at least 16 years old.',
  invalid_username:
    'Username must be 3–30 characters using letters, numbers, and underscores only.',
  invalid_email: 'Please enter a valid email address.'
};

export const load: PageServerLoad = async ({ url }) => {
  return {
    inviteToken: url.searchParams.get('invite') ?? ''
  };
};

export const actions: Actions = {
  default: async ({ request, fetch }) => {
    const data = await request.formData();
    const invite_token = (data.get('invite_token') as string | null) ?? '';
    const username = (data.get('username') as string | null) ?? '';
    const email = (data.get('email') as string | null) ?? '';
    const consent = data.get('consent') === 'true';
    const age_affirmation = data.get('age_affirmation') === 'true';

    const res = await fetch('/api/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        invite_token,
        username,
        email,
        consent,
        age_affirmation
      })
    });

    if (!res.ok) {
      let code = 'unknown_error';
      try {
        const body = await res.json();
        code = body.code ?? code;
      } catch {
        // ignore parse failure
      }
      return fail(res.status, {
        invite_token,
        username,
        email,
        error: ERROR_MESSAGES[code] ?? 'Registration failed. Please try again.'
      });
    }

    const body = await res.json();
    return {
      success: true,
      warning: body.warning ?? null
    };
  }
};
```

- [ ] **Step 2: Update the page to use load data for invite pre-fill**

Edit `+page.svelte` — replace the invite_token `value` binding with the server-loaded value:

```svelte
<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let smtpWarning = $derived(form?.warning === 'smtp_failure');
  let success = $derived(form?.success === true);
</script>
```

And update the invite_token field:

```svelte
        <input
          id="invite_token"
          name="invite_token"
          type="text"
          required
          value={form?.invite_token ?? data.inviteToken}
          class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          placeholder="abc123xyz789"
        />
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(public\)/auth/register/
git commit -m "feat(frontend): add registration page with invite pre-fill and GDPR checkboxes"
```

---

## Task 5: Magic-Link Request Page

**Files:**

- Create: `frontend/src/routes/(public)/auth/magic-link/+page.svelte`
- Create: `frontend/src/routes/(public)/auth/magic-link/+page.server.ts`

- [ ] **Step 1: Write the UI**

```svelte
<!-- frontend/src/routes/(public)/auth/magic-link/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData } from './$types';

  let { form }: { form: ActionData } = $props();
  let sent = $derived(form?.sent === true);
</script>

<svelte:head>
  <title>Sign In — FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col gap-6">
  <div class="text-center">
    <h1 class="text-2xl font-bold">Sign in</h1>
    <p class="text-sm text-muted-foreground mt-1">We'll email you a magic link.</p>
  </div>

  {#if sent}
    <div class="rounded-lg border border-green-200 bg-green-50 p-4 text-sm text-green-800 text-center">
      If that email is registered, a link is on its way.
    </div>
    <p class="text-center text-sm text-muted-foreground">
      <a href="/auth/magic-link" class="underline hover:text-foreground">Send another link</a>
    </p>
  {:else}
    <form method="POST" use:enhance class="flex flex-col gap-4">
      <div class="flex flex-col gap-1">
        <label for="email" class="text-sm font-medium">Email</label>
        <input
          id="email"
          name="email"
          type="email"
          required
          autocomplete="email"
          autofocus
          class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          placeholder="you@example.com"
        />
      </div>

      <button
        type="submit"
        class="h-11 rounded-lg bg-primary text-primary-foreground font-semibold hover:bg-primary/90 transition-colors"
      >
        Send Magic Link
      </button>
    </form>

    <p class="text-center text-sm text-muted-foreground">
      Don't have an account?
      <a href="/auth/register" class="underline hover:text-foreground">Register with an invite</a>
    </p>
  {/if}
</div>
```

- [ ] **Step 2: Write the server action**

```ts
// frontend/src/routes/(public)/auth/magic-link/+page.server.ts
import type { Actions } from './$types';

export const actions: Actions = {
  default: async ({ request, fetch }) => {
    const data = await request.formData();
    const email = (data.get('email') as string | null) ?? '';

    // Fire-and-forget — always show success (no enumeration)
    await fetch('/api/auth/magic-link', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email })
    }).catch(() => {
      // Silently ignore network errors — user sees "link is on its way" regardless
    });

    return { sent: true };
  }
};
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(public\)/auth/magic-link/
git commit -m "feat(frontend): add magic-link request page (no-enumeration pattern)"
```

---

## Task 6: Verify Page

**Files:**

- Create: `frontend/src/routes/(public)/auth/verify/+page.svelte`
- Create: `frontend/src/routes/(public)/auth/verify/+page.server.ts`

- [ ] **Step 1: Write the server load + action**

```ts
// frontend/src/routes/(public)/auth/verify/+page.server.ts
import { redirect, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ url }) => {
  return {
    token: url.searchParams.get('token') ?? '',
    next: url.searchParams.get('next') ?? '/'
  };
};

export const actions: Actions = {
  default: async ({ request, fetch, cookies }) => {
    const data = await request.formData();
    const token = (data.get('token') as string | null) ?? '';
    const next = (data.get('next') as string | null) ?? '/';

    const res = await fetch('/api/auth/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      // Cookie forwarding is automatic — SvelteKit passes the Set-Cookie back via event.cookies
      body: JSON.stringify({ token })
    });

    if (!res.ok) {
      let code = 'invalid_token';
      try {
        const body = await res.json();
        code = body.code ?? code;
      } catch {
        // ignore
      }
      return fail(400, { error: code });
    }

    // The backend response includes Set-Cookie header which SvelteKit forwards automatically
    throw redirect(303, next.startsWith('/') ? next : '/');
  }
};
```

- [ ] **Step 2: Write the page UI**

```svelte
<!-- frontend/src/routes/(public)/auth/verify/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let isExpired = $derived(
    form?.error === 'invalid_token' ||
    form?.error === 'token_expired' ||
    form?.error === 'token_used'
  );
</script>

<svelte:head>
  <title>Log In — FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col gap-6 text-center">
  <div>
    <h1 class="text-2xl font-bold">Welcome back</h1>
    {#if !isExpired}
      <p class="text-sm text-muted-foreground mt-1">Click below to log in to your account.</p>
    {/if}
  </div>

  {#if isExpired}
    <div class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
      This link has expired or already been used.
    </div>
    <a
      href="/auth/magic-link"
      class="inline-block h-11 rounded-lg bg-primary text-primary-foreground font-semibold leading-[2.75rem] hover:bg-primary/90 transition-colors px-6"
    >
      Request a new link →
    </a>
  {:else}
    {#if data.token}
      <form method="POST" use:enhance>
        <input type="hidden" name="token" value={data.token} />
        <input type="hidden" name="next" value={data.next} />
        <button
          type="submit"
          class="w-full h-11 rounded-lg bg-primary text-primary-foreground font-semibold hover:bg-primary/90 transition-colors"
        >
          Log In
        </button>
      </form>
    {:else}
      <div class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
        No token found in this link. Please check the email and try again.
      </div>
      <a href="/auth/magic-link" class="underline text-sm hover:text-foreground">
        Request a new link →
      </a>
    {/if}
  {/if}
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(public\)/auth/verify/
git commit -m "feat(frontend): add magic-link verify page with expired state handling"
```

---

## Task 7: Privacy Policy Page

**Files:**

- Create: `frontend/src/routes/(public)/privacy/+page.svelte`

- [ ] **Step 1: Write the static privacy page**

```svelte
<!-- frontend/src/routes/(public)/privacy/+page.svelte -->
<svelte:head>
  <title>Privacy Policy — FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col gap-6 text-left w-full max-w-prose mx-auto py-8">
  <h1 class="text-3xl font-bold">Privacy Policy</h1>
  <p class="text-sm text-muted-foreground">Last updated: [operator fills in date]</p>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">Who we are</h2>
    <p class="text-sm text-muted-foreground leading-relaxed">
      This service is operated by [operator name and contact — fill in before deployment].
      To contact us about privacy matters, email: [contact email].
    </p>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">What data we collect</h2>
    <ul class="text-sm text-muted-foreground leading-relaxed list-disc list-inside space-y-1">
      <li><strong>Email address</strong> — used to send magic links for authentication.</li>
      <li><strong>Username</strong> — displayed in-game and on leaderboards.</li>
      <li><strong>Game history</strong> — your submissions, votes, and scores.</li>
      <li><strong>One session cookie</strong> — functional only, no tracking. 30-day TTL. HttpOnly.</li>
    </ul>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">Why we collect it (lawful basis)</h2>
    <ul class="text-sm text-muted-foreground leading-relaxed list-disc list-inside space-y-1">
      <li><strong>Consent (Art. 6(1)(a) GDPR)</strong> — you give explicit consent at registration for account data processing.</li>
      <li><strong>Legitimate interest (Art. 6(1)(f) GDPR)</strong> — operational logs (up to 30 days) kept for security and reliability.</li>
    </ul>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">How long we keep it</h2>
    <ul class="text-sm text-muted-foreground leading-relaxed list-disc list-inside space-y-1">
      <li>Account data (email, username): until you request deletion.</li>
      <li>Game history (rooms, rounds, submissions, votes): 2 years after the game ends.</li>
      <li>Operational logs: up to 30 days.</li>
      <li>Backups: up to 7 days. Deleted user data may persist in backups for up to 7 days after erasure.</li>
    </ul>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">Your rights</h2>
    <ul class="text-sm text-muted-foreground leading-relaxed list-disc list-inside space-y-1">
      <li><strong>Access</strong> — view your data at any time from your profile page.</li>
      <li><strong>Portability</strong> — download your data as JSON from your profile page.</li>
      <li><strong>Rectification</strong> — update your username or email from your profile page.</li>
      <li><strong>Erasure</strong> — contact your admin to request deletion. We respond within 30 days.</li>
      <li><strong>Objection</strong> — contact your admin to object to any processing.</li>
      <li><strong>Supervisory authority</strong> — you may lodge a complaint with your local data protection authority.</li>
    </ul>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">Minimum age</h2>
    <p class="text-sm text-muted-foreground leading-relaxed">
      This service is intended for users aged <strong>16 and above</strong>. Registration
      requires explicit age affirmation. If you believe a user under 16 is registered,
      contact your admin immediately.
    </p>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">Third-party processors</h2>
    <p class="text-sm text-muted-foreground leading-relaxed">
      Magic-link emails are sent via our SMTP provider ([operator: fill in provider name]).
      Your email address is shared with this provider solely for delivery purposes.
      A Data Processing Agreement (DPA) is in place with the provider per GDPR Art. 28.
    </p>
    <p class="text-sm text-muted-foreground leading-relaxed">
      All other data remains on-premises. No analytics, tracking, or advertising services are used.
    </p>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-xl font-semibold">Cookies</h2>
    <p class="text-sm text-muted-foreground leading-relaxed">
      We use exactly one cookie: a session cookie set when you log in via magic link.
      It is <strong>HttpOnly</strong> (not accessible to JavaScript), <strong>Secure</strong>
      (HTTPS only), and <strong>SameSite=Strict</strong> (CSRF protection).
      It expires after 30 days of inactivity. No third-party cookies are set.
    </p>
  </section>

  <div class="pt-4">
    <a href="/" class="text-sm underline hover:text-foreground">← Back to home</a>
  </div>
</div>
```

- [ ] **Step 2: Override the layout for the privacy page (wider content)**

The public layout constrains width to `max-w-sm` (mobile-first). The privacy page needs wider prose. Since the layout already includes `w-full`, the page's own `max-w-prose mx-auto` override is sufficient — the layout `max-w-sm` wraps the `<main>`, so the privacy page should use its own layout. Create a `+layout@.svelte` to escape the public layout:

```svelte
<!-- frontend/src/routes/(public)/privacy/+layout.svelte -->
<script lang="ts">
  import '../../../../app.css';
  let { children } = $props();
</script>

<div class="min-h-screen bg-background text-foreground px-4 py-8">
  {@render children()}
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(public\)/privacy/
git commit -m "feat(frontend): add static privacy policy page (GDPR Art. 13 template)"
```

---

## Task 8: End-to-End Smoke Test

- [ ] **Step 1: Start the dev server**

```bash
cd frontend && npm run dev
```

- [ ] **Step 2: Verify landing page**

Open `http://localhost:5173`. Expected:

- App logo + room code input autofocused
- Typing 4 chars enables "Join Game" button
- "I'm hosting →" link visible

- [ ] **Step 3: Verify registration page**

Navigate to `http://localhost:5173/auth/register?invite=testtoken123`. Expected:

- Invite token pre-filled from URL
- Username, email fields present
- Two checkboxes (Privacy Policy + age affirmation) with links
- Submit button

- [ ] **Step 4: Verify magic-link page**

Navigate to `http://localhost:5173/auth/magic-link`. Expected:

- Email field autofocuses (in a browser — SvelteKit dev doesn't always autofocus)
- Form shows "If that email is registered, a link is on its way." after submit

- [ ] **Step 5: Verify verify page**

Navigate to `http://localhost:5173/auth/verify?token=abc123&next=/profile`. Expected:

- "Welcome back" heading + "Log In" button visible
- Without a token in URL: "No token found" error state

- [ ] **Step 6: Verify privacy page**

Navigate to `http://localhost:5173/privacy`. Expected:

- Full prose privacy policy with all sections
- No nav bar; back link to `/`

- [ ] **Step 7: Commit**

No new code. If smoke test reveals issues, fix and commit with:

```bash
git commit -m "fix(frontend): resolve auth page smoke test issues"
```
