# Frontend Admin Routes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement all admin routes under `(admin)` group: dashboard, users, invites, packs, pack-items, and game-types.

**Architecture:** SvelteKit `(admin)` route group with a shared sidebar layout and admin role guard. All pages use server-side form actions for mutations (SvelteKit `enhance` progressive enhancement). Pagination is cursor-based; search is debounced 300ms on the client then submits via form action.

**Tech Stack:** SvelteKit 2, Svelte 5 runes, Tailwind CSS v4, shadcn-svelte, `$lib/api/admin.ts` (already created in Phase 8)

---

## Files

| File                                                     | Role                                               |
| -------------------------------------------------------- | -------------------------------------------------- |
| `frontend/src/routes/(admin)/+layout.server.ts`          | Admin role guard → 403 if not admin                |
| `frontend/src/routes/(admin)/+layout.svelte`             | Admin sidebar layout with notification badge       |
| `frontend/src/routes/(admin)/+page.server.ts`            | Load dashboard stats + recent audit log            |
| `frontend/src/routes/(admin)/+page.svelte`               | Dashboard: stats cards + recent activity           |
| `frontend/src/routes/(admin)/users/+page.server.ts`      | Load paginated users; patch/delete actions         |
| `frontend/src/routes/(admin)/users/+page.svelte`         | User management table                              |
| `frontend/src/routes/(admin)/invites/+page.server.ts`    | Load invites; create/revoke actions                |
| `frontend/src/routes/(admin)/invites/+page.svelte`       | Invite table with create slide-over                |
| `frontend/src/routes/(admin)/packs/+page.server.ts`      | Load all packs; create/delete actions              |
| `frontend/src/routes/(admin)/packs/+page.svelte`         | Pack list table                                    |
| `frontend/src/routes/(admin)/packs/[id]/+page.server.ts` | Load pack + items; add/reorder/delete item actions |
| `frontend/src/routes/(admin)/packs/[id]/+page.svelte`    | Pack item manager                                  |
| `frontend/src/routes/(admin)/game-types/+page.server.ts` | Load game type registry                            |
| `frontend/src/routes/(admin)/game-types/+page.svelte`    | Game type read-only list                           |

---

## Task 1: Admin Layout + Role Guard

**Files:**

- Create: `frontend/src/routes/(admin)/+layout.server.ts`
- Create: `frontend/src/routes/(admin)/+layout.svelte`

- [ ] **Step 1: Write the admin role guard**

```ts
// frontend/src/routes/(admin)/+layout.server.ts
import { error } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ locals }) => {
  if (!locals.user) {
    throw error(401, 'Not authenticated');
  }
  if (locals.user.role !== 'admin') {
    throw error(403, 'Admin access required');
  }
  // Load unread notification count for badge
  // (notifications are fetched inline; badge count is cheap to compute)
  return { user: locals.user };
};
```

- [ ] **Step 2: Write the admin sidebar layout**

```svelte
<!-- frontend/src/routes/(admin)/+layout.svelte -->
<script lang="ts">
  import '../../../app.css';
  import Toast from '$lib/components/Toast.svelte';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  const navItems = [
    { href: '/admin', label: 'Dashboard' },
    { href: '/admin/users', label: 'Users' },
    { href: '/admin/invites', label: 'Invites' },
    { href: '/admin/packs', label: 'Packs' },
    { href: '/admin/game-types', label: 'Game Types' },
  ] as const;
</script>

<div class="min-h-screen flex bg-background text-foreground">
  <!-- Sidebar -->
  <nav class="w-48 shrink-0 border-r border-border flex flex-col py-4">
    <div class="px-4 mb-6">
      <a href="/" class="font-bold text-base">FabDoYouMeme</a>
      <p class="text-xs text-muted-foreground mt-0.5">Admin</p>
    </div>

    <ul class="flex flex-col gap-0.5 px-2">
      {#each navItems as item}
        <li>
          <a
            href={item.href}
            class="flex items-center gap-2 px-3 py-2 rounded-md text-sm text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
          >
            {item.label}
          </a>
        </li>
      {/each}
    </ul>

    <div class="mt-auto px-4 pt-4 border-t border-border">
      <a href="/profile" class="text-xs text-muted-foreground hover:text-foreground transition-colors">
        {data.user.username}
      </a>
    </div>
  </nav>

  <!-- Main content -->
  <main class="flex-1 overflow-y-auto">
    {@render children()}
  </main>

  <Toast />
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(admin\)/+layout.server.ts frontend/src/routes/\(admin\)/+layout.svelte
git commit -m "feat(frontend/admin): add admin layout with sidebar and role guard"
```

---

## Task 2: Admin Dashboard

**Files:**

- Create: `frontend/src/routes/(admin)/+page.server.ts`
- Create: `frontend/src/routes/(admin)/+page.svelte`

- [ ] **Step 1: Write the server load**

```ts
// frontend/src/routes/(admin)/+page.server.ts
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => {
  const [statsRes, auditRes] = await Promise.all([
    fetch('/api/admin/stats'),
    fetch('/api/admin/audit-log?limit=10')
  ]);

  const stats = statsRes.ok ? await statsRes.json() : null;
  const auditLog = auditRes.ok ? await auditRes.json() : [];

  return { stats, auditLog };
};
```

- [ ] **Step 2: Write the dashboard page**

```svelte
<!-- frontend/src/routes/(admin)/+page.svelte -->
<script lang="ts">
  import type { PageData } from './$types';
  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>Admin Dashboard — FabDoYouMeme</title>
</svelte:head>

<div class="p-6 flex flex-col gap-6">
  <h1 class="text-2xl font-bold">Dashboard</h1>

  {#if data.stats}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      {#each [
        { label: 'Active Rooms', value: data.stats.active_rooms ?? 0 },
        { label: 'Total Users', value: data.stats.total_users ?? 0 },
        { label: 'Total Packs', value: data.stats.total_packs ?? 0 },
        { label: 'Pending Invites', value: data.stats.pending_invites ?? 0 },
      ] as card}
        <div class="rounded-xl border border-border bg-card p-4">
          <p class="text-sm text-muted-foreground">{card.label}</p>
          <p class="text-3xl font-bold mt-1">{card.value}</p>
        </div>
      {/each}
    </div>
  {/if}

  <section class="flex flex-col gap-3">
    <h2 class="text-base font-semibold">Recent Activity</h2>
    {#if data.auditLog.length === 0}
      <p class="text-sm text-muted-foreground">No recent activity.</p>
    {:else}
      <ul class="flex flex-col gap-2">
        {#each data.auditLog as entry}
          <li class="text-sm flex items-center gap-2">
            <span class="text-muted-foreground shrink-0">
              {new Date(entry.created_at).toLocaleString()}
            </span>
            <span class="flex-1">{entry.description}</span>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(admin\)/+page.server.ts frontend/src/routes/\(admin\)/+page.svelte
git commit -m "feat(frontend/admin): add admin dashboard with stats cards and audit log"
```

---

## Task 3: Admin Users Page

**Files:**

- Create: `frontend/src/routes/(admin)/users/+page.server.ts`
- Create: `frontend/src/routes/(admin)/users/+page.svelte`

- [ ] **Step 1: Write the server load and actions**

```ts
// frontend/src/routes/(admin)/users/+page.server.ts
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url }) => {
  const q = url.searchParams.get('q') ?? '';
  const cursor = url.searchParams.get('cursor') ?? '';
  const qs = new URLSearchParams({ limit: '50' });
  if (q) qs.set('q', q);
  if (cursor) qs.set('cursor', cursor);

  const res = await fetch(`/api/admin/users?${qs}`);
  const data = res.ok ? await res.json() : { users: [], next_cursor: null };

  return { users: data.users ?? [], nextCursor: data.next_cursor ?? null, q };
};

export const actions: Actions = {
  updateUser: async ({ request, fetch }) => {
    const data = await request.formData();
    const userId = data.get('user_id') as string;
    const patch: Record<string, unknown> = {};
    if (data.has('role')) patch.role = data.get('role');
    if (data.has('is_active'))
      patch.is_active = data.get('is_active') === 'true';
    if (data.has('username')) patch.username = data.get('username');
    if (data.has('email')) patch.email = data.get('email');

    const res = await fetch(`/api/admin/users/${userId}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(patch)
    });

    if (!res.ok) {
      let code = 'error';
      try {
        const b = await res.json();
        code = b.code ?? code;
      } catch {
        /**/
      }
      return fail(res.status, {
        error:
          code === 'username_taken'
            ? 'Username already taken.'
            : 'Update failed.'
      });
    }
    return { success: true };
  },

  deleteUser: async ({ request, fetch }) => {
    const data = await request.formData();
    const userId = data.get('user_id') as string;

    const res = await fetch(`/api/admin/users/${userId}`, { method: 'DELETE' });
    if (!res.ok) return fail(res.status, { error: 'Failed to delete user.' });
    return { deleted: userId };
  }
};
```

- [ ] **Step 2: Write the users page**

```svelte
<!-- frontend/src/routes/(admin)/users/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import { goto } from '$app/navigation';
  import { toast } from '$lib/state/toast.svelte';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let searchTerm = $state(data.q ?? '');
  let searchTimeout: ReturnType<typeof setTimeout>;
  let confirmDeleteId = $state<string | null>(null);

  function onSearchInput() {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      goto(`?q=${encodeURIComponent(searchTerm)}`, { replaceState: true });
    }, 300);
  }

  $effect(() => {
    if (form?.error) toast.show(form.error, 'error');
    if (form?.success) toast.show('User updated.', 'success');
    if (form?.deleted) toast.show('User deleted.', 'success');
  });
</script>

<svelte:head>
  <title>Users — Admin</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4">
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">Users</h1>
    <input
      type="search"
      placeholder="Search users…"
      bind:value={searchTerm}
      oninput={onSearchInput}
      class="h-9 w-56 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
    />
  </div>

  <div class="rounded-xl border border-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-border bg-muted/40 text-xs font-medium text-muted-foreground">
          <th class="text-left px-4 py-3">Username</th>
          <th class="text-left px-4 py-3">Email</th>
          <th class="text-left px-4 py-3">Role</th>
          <th class="text-left px-4 py-3">Active</th>
          <th class="text-left px-4 py-3">Joined</th>
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each data.users as u}
          <tr class="border-b border-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3">
              <form method="POST" action="?/updateUser" use:enhance class="flex gap-1">
                <input type="hidden" name="user_id" value={u.id} />
                <input
                  name="username"
                  type="text"
                  value={u.username}
                  onblur={(e) => {
                    if ((e.target as HTMLInputElement).value !== u.username)
                      (e.target as HTMLInputElement).closest('form')?.requestSubmit();
                  }}
                  class="h-7 w-28 rounded border border-transparent hover:border-border px-1 text-sm bg-transparent focus:outline-none focus:border-ring"
                />
              </form>
            </td>
            <td class="px-4 py-3">
              <form method="POST" action="?/updateUser" use:enhance class="flex gap-1">
                <input type="hidden" name="user_id" value={u.id} />
                <input
                  name="email"
                  type="email"
                  value={u.email}
                  onblur={(e) => {
                    if ((e.target as HTMLInputElement).value !== u.email)
                      (e.target as HTMLInputElement).closest('form')?.requestSubmit();
                  }}
                  class="h-7 w-40 rounded border border-transparent hover:border-border px-1 text-sm bg-transparent focus:outline-none focus:border-ring"
                />
              </form>
            </td>
            <td class="px-4 py-3">
              <form method="POST" action="?/updateUser" use:enhance>
                <input type="hidden" name="user_id" value={u.id} />
                <select
                  name="role"
                  value={u.role}
                  onchange={(e) => (e.target as HTMLSelectElement).closest('form')?.requestSubmit()}
                  class="h-7 rounded border border-transparent hover:border-border px-1 text-sm bg-transparent focus:outline-none focus:border-ring cursor-pointer"
                >
                  <option value="player">player</option>
                  <option value="admin">admin</option>
                </select>
              </form>
            </td>
            <td class="px-4 py-3">
              <form method="POST" action="?/updateUser" use:enhance>
                <input type="hidden" name="user_id" value={u.id} />
                <input
                  type="checkbox"
                  name="is_active"
                  checked={u.is_active}
                  value="true"
                  onchange={(e) => {
                    const input = e.target as HTMLInputElement;
                    // Send false when unchecked
                    if (!input.checked) {
                      const hidden = document.createElement('input');
                      hidden.type = 'hidden';
                      hidden.name = 'is_active';
                      hidden.value = 'false';
                      input.closest('form')?.appendChild(hidden);
                      input.remove();
                    }
                    input.closest('form')?.requestSubmit();
                  }}
                  class="h-4 w-4 cursor-pointer"
                />
              </form>
            </td>
            <td class="px-4 py-3 text-muted-foreground text-xs">
              {new Date(u.created_at).toLocaleDateString()}
            </td>
            <td class="px-4 py-3 text-right">
              {#if confirmDeleteId === u.id}
                <div class="flex gap-1 justify-end">
                  <form method="POST" action="?/deleteUser" use:enhance>
                    <input type="hidden" name="user_id" value={u.id} />
                    <button type="submit" class="text-xs text-red-600 underline hover:text-red-800">
                      Confirm Delete
                    </button>
                  </form>
                  <button type="button" onclick={() => confirmDeleteId = null}
                    class="text-xs text-muted-foreground underline">
                    Cancel
                  </button>
                </div>
              {:else}
                <button type="button" onclick={() => confirmDeleteId = u.id}
                  class="text-muted-foreground hover:text-red-600 transition-colors text-lg leading-none"
                  aria-label="Delete user">
                  ×
                </button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  {#if data.nextCursor}
    <a
      href="?{data.q ? `q=${encodeURIComponent(data.q)}&` : ''}cursor={data.nextCursor}"
      class="self-center text-sm text-muted-foreground underline hover:text-foreground"
    >
      Load more
    </a>
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
git add frontend/src/routes/\(admin\)/users/
git commit -m "feat(frontend/admin): add users page with inline edit, role toggle, and delete"
```

---

## Task 4: Admin Invites Page

**Files:**

- Create: `frontend/src/routes/(admin)/invites/+page.server.ts`
- Create: `frontend/src/routes/(admin)/invites/+page.svelte`

- [ ] **Step 1: Write the server load and actions**

```ts
// frontend/src/routes/(admin)/invites/+page.server.ts
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => {
  const res = await fetch('/api/admin/invites');
  const invites = res.ok ? await res.json() : [];
  return { invites };
};

export const actions: Actions = {
  createInvite: async ({ request, fetch }) => {
    const data = await request.formData();
    const label = (data.get('label') as string | null) ?? '';
    const restricted_email =
      (data.get('restricted_email') as string | null) || null;
    const max_uses = Number(data.get('max_uses') ?? 0);
    const expires_at = (data.get('expires_at') as string | null) || null;

    const res = await fetch('/api/admin/invites', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ label, restricted_email, max_uses, expires_at })
    });

    if (!res.ok)
      return fail(res.status, { createError: 'Failed to create invite.' });
    const invite = await res.json();
    return { created: invite };
  },

  revokeInvite: async ({ request, fetch }) => {
    const data = await request.formData();
    const inviteId = data.get('invite_id') as string;

    const res = await fetch(`/api/admin/invites/${inviteId}`, {
      method: 'DELETE'
    });
    if (!res.ok)
      return fail(res.status, { revokeError: 'Failed to revoke invite.' });
    return { revoked: inviteId };
  }
};
```

- [ ] **Step 2: Write the invites page**

```svelte
<!-- frontend/src/routes/(admin)/invites/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import { toast } from '$lib/state/toast.svelte';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let showCreateForm = $state(false);
  let invites = $state(data.invites);
  let revealedTokens = $state<Set<string>>(new Set());

  $effect(() => {
    if (form?.created) {
      invites = [...invites, form.created];
      showCreateForm = false;
      toast.show('Invite created.', 'success');
    }
    if (form?.revoked) {
      invites = invites.filter((i: { id: string }) => i.id !== form.revoked);
      toast.show('Invite revoked.', 'success');
    }
    if (form?.createError || form?.revokeError) {
      toast.show(form.createError ?? form.revokeError, 'error');
    }
  });

  function copyLink(token: string) {
    const url = `${window.location.origin}/auth/register?invite=${token}`;
    navigator.clipboard.writeText(url).then(() => toast.show('Link copied to clipboard.', 'success'));
  }

  function toggleReveal(id: string) {
    const next = new Set(revealedTokens);
    if (next.has(id)) next.delete(id); else next.add(id);
    revealedTokens = next;
  }
</script>

<svelte:head>
  <title>Invites — Admin</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4">
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">Invites</h1>
    <button type="button" onclick={() => showCreateForm = !showCreateForm}
      class="h-9 px-4 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90">
      + Create Invite
    </button>
  </div>

  {#if showCreateForm}
    <form method="POST" action="?/createInvite" use:enhance
      class="rounded-xl border border-border bg-card p-4 flex flex-col gap-3">
      <h2 class="text-sm font-semibold">New Invite</h2>
      <div class="grid grid-cols-2 gap-3">
        <div class="flex flex-col gap-1">
          <label for="label" class="text-xs font-medium">Label</label>
          <input id="label" name="label" type="text" placeholder="Gaming night 2026"
            class="h-9 rounded border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
        </div>
        <div class="flex flex-col gap-1">
          <label for="restricted_email" class="text-xs font-medium">Restrict to email</label>
          <input id="restricted_email" name="restricted_email" type="email" placeholder="Optional"
            class="h-9 rounded border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
        </div>
        <div class="flex flex-col gap-1">
          <label for="max_uses" class="text-xs font-medium">Max uses (0 = unlimited)</label>
          <input id="max_uses" name="max_uses" type="number" min={0} value={0}
            class="h-9 rounded border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
        </div>
        <div class="flex flex-col gap-1">
          <label for="expires_at" class="text-xs font-medium">Expires at</label>
          <input id="expires_at" name="expires_at" type="datetime-local"
            class="h-9 rounded border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
        </div>
      </div>
      <div class="flex gap-2 justify-end">
        <button type="button" onclick={() => showCreateForm = false}
          class="h-9 px-4 rounded border border-border text-sm hover:bg-muted">Cancel</button>
        <button type="submit"
          class="h-9 px-4 rounded bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90">
          Create
        </button>
      </div>
    </form>
  {/if}

  <div class="rounded-xl border border-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-border bg-muted/40 text-xs font-medium text-muted-foreground">
          <th class="text-left px-4 py-3">Label</th>
          <th class="text-left px-4 py-3">Token</th>
          <th class="text-left px-4 py-3">Restricted Email</th>
          <th class="text-left px-4 py-3">Uses</th>
          <th class="text-left px-4 py-3">Expires</th>
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each invites as inv}
          <tr class="border-b border-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3">{inv.label ?? '—'}</td>
            <td class="px-4 py-3 font-mono">
              <button type="button" onclick={() => toggleReveal(inv.id)}
                class="text-muted-foreground hover:text-foreground transition-colors">
                {revealedTokens.has(inv.id) ? inv.token : `${inv.token.slice(0, 4)}…`}
              </button>
            </td>
            <td class="px-4 py-3 text-muted-foreground">{inv.restricted_email ?? '—'}</td>
            <td class="px-4 py-3 text-muted-foreground">
              {inv.uses_count}/{inv.max_uses === 0 ? '∞' : inv.max_uses}
            </td>
            <td class="px-4 py-3 text-muted-foreground text-xs">
              {inv.expires_at ? new Date(inv.expires_at).toLocaleDateString() : 'Never'}
            </td>
            <td class="px-4 py-3">
              <div class="flex gap-2 justify-end">
                <button type="button" onclick={() => copyLink(inv.token)}
                  class="text-xs text-muted-foreground underline hover:text-foreground">
                  Copy Link
                </button>
                <form method="POST" action="?/revokeInvite" use:enhance
                  onsubmit={(e) => !confirm('Revoke this invite?') && e.preventDefault()}>
                  <input type="hidden" name="invite_id" value={inv.id} />
                  <button type="submit" class="text-xs text-red-600 underline hover:text-red-800">
                    Revoke
                  </button>
                </form>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(admin\)/invites/
git commit -m "feat(frontend/admin): add invites page with create slide-over and revoke"
```

---

## Task 5: Admin Packs List Page

**Files:**

- Create: `frontend/src/routes/(admin)/packs/+page.server.ts`
- Create: `frontend/src/routes/(admin)/packs/+page.svelte`

- [ ] **Step 1: Write the server load and actions**

```ts
// frontend/src/routes/(admin)/packs/+page.server.ts
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => {
  const res = await fetch('/api/packs?include_all=true');
  const packs = res.ok ? await res.json() : [];
  return { packs };
};

export const actions: Actions = {
  createPack: async ({ request, fetch }) => {
    const data = await request.formData();
    const name = (data.get('name') as string | null) ?? '';
    const description = (data.get('description') as string | null) ?? '';

    const res = await fetch('/api/packs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description: description || undefined })
    });
    if (!res.ok)
      return fail(res.status, { createError: 'Failed to create pack.' });
    return { created: await res.json() };
  },

  deletePack: async ({ request, fetch }) => {
    const data = await request.formData();
    const packId = data.get('pack_id') as string;
    const res = await fetch(`/api/packs/${packId}`, { method: 'DELETE' });
    if (!res.ok)
      return fail(res.status, { deleteError: 'Failed to delete pack.' });
    return { deleted: packId };
  }
};
```

- [ ] **Step 2: Write the packs list page**

```svelte
<!-- frontend/src/routes/(admin)/packs/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import { toast } from '$lib/state/toast.svelte';
  import type { ActionData, PageData } from './$types';
  import type { Pack } from '$lib/api/types';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let packs = $state<Pack[]>(data.packs);
  let showNewRow = $state(false);

  $effect(() => {
    if (form?.created) { packs = [...packs, form.created]; showNewRow = false; toast.show('Pack created.', 'success'); }
    if (form?.deleted) { packs = packs.filter((p) => p.id !== form.deleted); toast.show('Pack deleted.', 'success'); }
    if (form?.createError || form?.deleteError) toast.show(form.createError ?? form.deleteError, 'error');
  });
</script>

<svelte:head>
  <title>Packs — Admin</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4">
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">Packs</h1>
    <button type="button" onclick={() => showNewRow = !showNewRow}
      class="h-9 px-4 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90">
      + New Pack
    </button>
  </div>

  {#if showNewRow}
    <form method="POST" action="?/createPack" use:enhance
      class="flex gap-3 items-end rounded-lg border border-dashed border-border p-3">
      <div class="flex flex-col gap-1 flex-1">
        <label class="text-xs font-medium">Name</label>
        <input name="name" type="text" required autofocus placeholder="Pack name"
          class="h-9 rounded border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
      </div>
      <div class="flex flex-col gap-1 flex-1">
        <label class="text-xs font-medium">Description</label>
        <input name="description" type="text" placeholder="Optional"
          class="h-9 rounded border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
      </div>
      <button type="submit"
        class="h-9 px-4 rounded bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 shrink-0">
        Create
      </button>
      <button type="button" onclick={() => showNewRow = false}
        class="h-9 px-4 rounded border border-border text-sm hover:bg-muted shrink-0">
        Cancel
      </button>
    </form>
  {/if}

  <div class="rounded-xl border border-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-border bg-muted/40 text-xs font-medium text-muted-foreground">
          <th class="text-left px-4 py-3">Name</th>
          <th class="text-left px-4 py-3">Description</th>
          <th class="text-left px-4 py-3">Items</th>
          <th class="text-left px-4 py-3">Status</th>
          <th class="text-left px-4 py-3">Created</th>
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each packs as pack}
          <tr class="border-b border-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3">
              <a href="/admin/packs/{pack.id}" class="font-medium hover:underline">{pack.name}</a>
            </td>
            <td class="px-4 py-3 text-muted-foreground">{pack.description ?? '—'}</td>
            <td class="px-4 py-3 text-muted-foreground">{pack.item_count ?? 0}</td>
            <td class="px-4 py-3">
              <span class="text-xs px-2 py-0.5 rounded-full
                {pack.status === 'active' ? 'bg-green-100 text-green-800' :
                 pack.status === 'flagged' ? 'bg-yellow-100 text-yellow-800' :
                 'bg-red-100 text-red-800'}">
                {pack.status}
              </span>
            </td>
            <td class="px-4 py-3 text-muted-foreground text-xs">
              {new Date(pack.created_at).toLocaleDateString()}
            </td>
            <td class="px-4 py-3 text-right">
              <form method="POST" action="?/deletePack" use:enhance
                onsubmit={(e) => {
                  if (!confirm('Delete this pack? In-use packs will no longer be available for new rooms.')) e.preventDefault();
                }}>
                <input type="hidden" name="pack_id" value={pack.id} />
                <button type="submit"
                  class="text-muted-foreground hover:text-red-600 transition-colors text-lg leading-none"
                  aria-label="Delete pack">
                  ×
                </button>
              </form>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(admin\)/packs/+page.server.ts frontend/src/routes/\(admin\)/packs/+page.svelte
git commit -m "feat(frontend/admin): add packs list page with inline create and delete"
```

---

## Task 6: Admin Pack Items Page

**Files:**

- Create: `frontend/src/routes/(admin)/packs/[id]/+page.server.ts`
- Create: `frontend/src/routes/(admin)/packs/[id]/+page.svelte`

- [ ] **Step 1: Write the server load and actions**

```ts
// frontend/src/routes/(admin)/packs/[id]/+page.server.ts
import { error, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, fetch }) => {
  const [packRes, itemsRes] = await Promise.all([
    fetch(`/api/packs/${params.id}`),
    fetch(`/api/packs/${params.id}/items`)
  ]);

  if (!packRes.ok) throw error(404, 'Pack not found');
  return {
    pack: await packRes.json(),
    items: itemsRes.ok ? await itemsRes.json() : []
  };
};

export const actions: Actions = {
  deleteItem: async ({ request, fetch, params }) => {
    const data = await request.formData();
    const itemId = data.get('item_id') as string;
    const res = await fetch(`/api/packs/${params.id}/items/${itemId}`, {
      method: 'DELETE'
    });
    if (!res.ok)
      return fail(res.status, { deleteError: 'Failed to delete item.' });
    return { deleted: itemId };
  }
};
```

- [ ] **Step 2: Write the pack items page**

```svelte
<!-- frontend/src/routes/(admin)/packs/[id]/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import { toast } from '$lib/state/toast.svelte';
  import { uploadImageItem } from '$lib/api/studio';
  import type { ActionData, PageData } from './$types';
  import type { GameItem } from '$lib/api/types';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let items = $state<GameItem[]>(data.items);
  let uploading = $state(false);

  $effect(() => {
    if (form?.deleted) { items = items.filter((i) => i.id !== form.deleted); toast.show('Item deleted.', 'success'); }
    if (form?.deleteError) toast.show(form.deleteError, 'error');
  });

  async function handleFileInput(e: Event) {
    const input = e.target as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    if (files.length === 0) return;
    uploading = true;
    for (const file of files) {
      try {
        const item = await uploadImageItem(data.pack.id, file.name.replace(/\.[^.]+$/, ''), file);
        items = [...items, item];
      } catch {
        toast.show(`Failed to upload ${file.name}.`, 'error');
      }
    }
    uploading = false;
    input.value = '';
    toast.show(`${files.length} item(s) uploaded.`, 'success');
  }
</script>

<svelte:head>
  <title>{data.pack.name} — Admin</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4">
  <div class="flex items-center gap-3">
    <a href="/admin/packs" class="text-sm text-muted-foreground hover:text-foreground">← Packs</a>
    <span class="text-muted-foreground">/</span>
    <h1 class="text-xl font-bold">{data.pack.name}</h1>
    <span class="text-sm text-muted-foreground ml-1">({items.length} items)</span>
    <div class="flex-1" />
    <label class="h-9 px-4 rounded-lg border border-border text-sm font-medium cursor-pointer hover:bg-muted flex items-center gap-1">
      <input type="file" accept="image/jpeg,image/png,image/webp" multiple class="sr-only" onchange={handleFileInput} disabled={uploading} />
      {uploading ? 'Uploading…' : 'Add Items'}
    </label>
  </div>

  {#if data.pack.description}
    <p class="text-sm text-muted-foreground">{data.pack.description}</p>
  {/if}

  <div class="rounded-xl border border-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-border bg-muted/40 text-xs font-medium text-muted-foreground">
          <th class="w-10 px-4 py-3">#</th>
          <th class="text-left px-4 py-3">Preview</th>
          <th class="text-left px-4 py-3">Name</th>
          <th class="text-left px-4 py-3">Type</th>
          <th class="text-left px-4 py-3">Version</th>
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each items as item, i}
          <tr class="border-b border-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3 text-muted-foreground text-xs">{i + 1}</td>
            <td class="px-4 py-3">
              {#if item.thumbnail_url}
                <img src={item.thumbnail_url} alt="" class="h-10 w-10 rounded object-cover" />
              {:else}
                <div class="h-10 w-10 rounded bg-muted flex items-center justify-center text-muted-foreground text-xs">
                  {item.type === 'image' ? '🖼' : 'T'}
                </div>
              {/if}
            </td>
            <td class="px-4 py-3 font-medium">{item.name}</td>
            <td class="px-4 py-3">
              <span class="text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground">{item.type}</span>
            </td>
            <td class="px-4 py-3 text-muted-foreground text-xs">v{item.version_number ?? 1}</td>
            <td class="px-4 py-3 text-right">
              <form method="POST" action="?/deleteItem" use:enhance
                onsubmit={(e) => !confirm(`Delete "${item.name}"?`) && e.preventDefault()}>
                <input type="hidden" name="item_id" value={item.id} />
                <button type="submit"
                  class="text-muted-foreground hover:text-red-600 transition-colors text-lg leading-none"
                  aria-label="Delete item">×</button>
              </form>
            </td>
          </tr>
        {/each}
        {#if items.length === 0}
          <tr>
            <td colspan={6} class="px-4 py-8 text-center text-muted-foreground text-sm">
              No items yet. Upload images to get started.
            </td>
          </tr>
        {/if}
      </tbody>
    </table>
  </div>
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(admin\)/packs/\[id\]/
git commit -m "feat(frontend/admin): add pack items page with upload and delete"
```

---

## Task 7: Admin Game Types Page

**Files:**

- Create: `frontend/src/routes/(admin)/game-types/+page.server.ts`
- Create: `frontend/src/routes/(admin)/game-types/+page.svelte`

- [ ] **Step 1: Write the server load**

```ts
// frontend/src/routes/(admin)/game-types/+page.server.ts
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => {
  const res = await fetch('/api/game-types');
  const gameTypes = res.ok ? await res.json() : [];
  return { gameTypes };
};
```

- [ ] **Step 2: Write the read-only game types page**

```svelte
<!-- frontend/src/routes/(admin)/game-types/+page.svelte -->
<script lang="ts">
  import type { PageData } from './$types';
  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>Game Types — Admin</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4">
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">Game Types</h1>
    <p class="text-xs text-muted-foreground">Read-only — game types are registered in code.</p>
  </div>

  <div class="rounded-xl border border-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-border bg-muted/40 text-xs font-medium text-muted-foreground">
          <th class="text-left px-4 py-3">Slug</th>
          <th class="text-left px-4 py-3">Name</th>
          <th class="text-left px-4 py-3">Description</th>
          <th class="text-left px-4 py-3">Payload Versions</th>
          <th class="text-left px-4 py-3">Supports Solo</th>
        </tr>
      </thead>
      <tbody>
        {#each data.gameTypes as gt}
          <tr class="border-b border-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3 font-mono text-xs">{gt.slug}</td>
            <td class="px-4 py-3 font-medium">{gt.name}</td>
            <td class="px-4 py-3 text-muted-foreground">{gt.description}</td>
            <td class="px-4 py-3 text-muted-foreground font-mono text-xs">
              [{(gt.supported_payload_versions ?? []).join(', ')}]
            </td>
            <td class="px-4 py-3">
              <span class="text-xs px-2 py-0.5 rounded-full {gt.supports_solo ? 'bg-green-100 text-green-800' : 'bg-muted text-muted-foreground'}">
                {gt.supports_solo ? 'Yes' : 'No'}
              </span>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(admin\)/game-types/
git commit -m "feat(frontend/admin): add game types read-only registry page"
```

---

## Task 8: Admin Integration Smoke Test

- [ ] **Step 1: Start the dev stack**

```bash
docker compose up --build
```

- [ ] **Step 2: Navigate to `/admin`**

Log in as admin user. Go to `http://localhost:5173/admin`. Expected:

- Stats cards show active rooms, users, packs, invites
- Recent activity list visible

- [ ] **Step 3: Non-admin user sees 403**

Log in as a non-admin user. Navigate to `http://localhost:5173/admin`. Expected: SvelteKit error page with 403 message.

- [ ] **Step 4: User management**

1. Search for a user by partial username — table filters
2. Change a user's role using the dropdown — should auto-save
3. Deactivate a user via the checkbox
4. Delete a user: confirm dialog appears, then user disappears from table

- [ ] **Step 5: Invite management**

1. Create an invite with a label and max_uses=1
2. Hover over the token field — reveal on click
3. Copy the invite link — toast confirms
4. Revoke the invite — confirm dialog, then removed from table

- [ ] **Step 6: Pack management**

1. Create a new pack
2. Click the pack name → navigate to pack items page
3. Upload an image → verify it appears in the table
4. Delete an item

- [ ] **Step 7: Commit if fixes needed**

```bash
git commit -m "fix(frontend/admin): resolve admin smoke test issues"
```
