<script lang="ts">
  import { enhance } from '$app/forms';
  import { goto } from '$app/navigation';
  import { untrack } from 'svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { reveal } from '$lib/actions/reveal';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Search, Shield, UserX, Gamepad2, Clock } from '$lib/icons';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  // Seed the input from the URL query once; thereafter the user owns it
  // and a debounced goto() propagates edits back to the URL.
  let searchTerm = $state(untrack(() => data.q ?? ''));
  let searchTimeout: ReturnType<typeof setTimeout>;
  let confirmDeleteId = $state<string | null>(null);

  // `use:enhance` updates the `form` prop several times per submission
  // (pending → result → post-invalidate refetch), each update firing the
  // effect. Without this guard we got 3× toasts. A plain `let` (not
  // `$state`) skips reactivity, so writing from inside the effect is safe.
  let lastForm: ActionData | undefined;

  function onSearchInput() {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      goto(`?q=${encodeURIComponent(searchTerm)}`, { replaceState: true });
    }, 300);
  }

  // Compact relative-time for the "Last login" column. "—" for null
  // (logged out / never active), otherwise the largest unit that still
  // reads as a small number: "5m", "2h", "3d", "6mo", "2y".
  function formatRelative(iso: string | null): string {
    if (!iso) return '—';
    const then = new Date(iso).getTime();
    if (Number.isNaN(then)) return '—';
    const diff = Math.max(0, Date.now() - then);
    const sec = Math.floor(diff / 1000);
    if (sec < 60) return 'just now';
    const min = Math.floor(sec / 60);
    if (min < 60) return `${min}m`;
    const hr = Math.floor(min / 60);
    if (hr < 24) return `${hr}h`;
    const day = Math.floor(hr / 24);
    if (day < 30) return `${day}d`;
    const mo = Math.floor(day / 30);
    if (mo < 12) return `${mo}mo`;
    return `${Math.floor(mo / 12)}y`;
  }

  $effect(() => {
    if (form === lastForm) return;
    lastForm = form;
    if (form?.error) toast.show(form.error, 'error');
    if (form?.success) toast.show('User updated.', 'success');
    if (form?.deleted) toast.show('User deleted.', 'success');
  });
</script>

<svelte:head>
  <title>Users — Admin</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4" use:reveal>
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">Users</h1>
    <div class="relative">
      <Search size={14} strokeWidth={2.5} class="absolute left-3 top-1/2 -translate-y-1/2 text-brand-text-muted" />
      <input
        type="search"
        placeholder="Search users…"
        bind:value={searchTerm}
        oninput={onSearchInput}
        class="h-9 w-56 rounded-md border border-brand-border-heavy bg-brand-white pl-8 pr-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
      />
    </div>
  </div>

  <div class="rounded-xl border border-brand-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-brand-border bg-muted/40 text-xs font-medium text-brand-text-muted">
          <th class="text-left px-4 py-3">Username</th>
          <th class="text-left px-4 py-3">Email</th>
          <th class="text-left px-4 py-3">Role</th>
          <th class="text-left px-4 py-3">Active</th>
          <th class="text-left px-4 py-3">
            <span class="inline-flex items-center gap-1">
              <Gamepad2 size={12} strokeWidth={2.5} />
              Games
            </span>
          </th>
          <th class="text-left px-4 py-3">Joined</th>
          <th class="text-left px-4 py-3">
            <span class="inline-flex items-center gap-1">
              <Clock size={12} strokeWidth={2.5} />
              Last login
            </span>
          </th>
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each data.users as u}
          <tr class="border-b border-brand-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3">
              <form method="POST" action="?/updateUser" use:enhance class="flex items-center gap-1">
                <input type="hidden" name="user_id" value={u.id} />
                <input
                  name="username"
                  type="text"
                  value={u.username}
                  onblur={(e) => {
                    if ((e.target as HTMLInputElement).value !== u.username)
                      (e.target as HTMLInputElement).closest('form')?.requestSubmit();
                  }}
                  class="h-7 w-28 rounded border border-transparent hover:border-brand-border px-1 text-sm bg-transparent focus:outline-none focus:border-ring"
                />
                {#if u.is_protected}
                  <span
                    class="inline-flex items-center text-brand-text-muted"
                    title="Protected bootstrap admin — cannot be deleted or have its role changed"
                    aria-label="Protected account"
                  >
                    <Shield size={14} strokeWidth={2.5} />
                  </span>
                {/if}
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
                  class="h-7 w-40 rounded border border-transparent hover:border-brand-border px-1 text-sm bg-transparent focus:outline-none focus:border-ring"
                />
              </form>
            </td>
            <td class="px-4 py-3">
              <form method="POST" action="?/updateUser" use:enhance>
                <input type="hidden" name="user_id" value={u.id} />
                <select
                  name="role"
                  value={u.role}
                  disabled={u.is_protected}
                  onchange={(e) => (e.target as HTMLSelectElement).closest('form')?.requestSubmit()}
                  title={u.is_protected ? 'Protected bootstrap admin — role is locked' : undefined}
                  class="h-7 rounded border border-transparent hover:border-brand-border px-1 text-sm bg-transparent focus:outline-none focus:border-ring cursor-pointer disabled:cursor-not-allowed disabled:opacity-60"
                >
                  <option value="player">player</option>
                  <option value="admin">admin</option>
                </select>
              </form>
            </td>
            <td class="px-4 py-3">
              <form method="POST" action="?/updateUser" use:enhance class="inline-flex items-center">
                <input type="hidden" name="user_id" value={u.id} />
                <input type="hidden" name="is_active" value={(!u.is_active).toString()} />
                <input
                  type="checkbox"
                  checked={u.is_active}
                  disabled={u.is_protected}
                  onchange={(e) => (e.target as HTMLInputElement).closest('form')?.requestSubmit()}
                  title={u.is_protected ? 'Protected bootstrap admin — cannot be deactivated' : undefined}
                  class="h-4 w-4 cursor-pointer disabled:cursor-not-allowed disabled:opacity-60"
                />
              </form>
            </td>
            <td class="px-4 py-3 text-xs tabular-nums">
              {u.games_played}
            </td>
            <td class="px-4 py-3 text-brand-text-muted text-xs">
              {new Date(u.created_at).toLocaleDateString()}
            </td>
            <td
              class="px-4 py-3 text-brand-text-muted text-xs tabular-nums"
              title={u.last_login_at
                ? new Date(u.last_login_at).toLocaleString()
                : 'No live session — user is logged out'}
            >
              {formatRelative(u.last_login_at)}
            </td>
            <td class="px-4 py-3 text-right">
              {#if u.is_protected}
                <!-- Delete intentionally absent for protected rows; the Shield
                     next to the username already signals the locked state. -->
              {:else if confirmDeleteId === u.id}
                <div class="flex gap-1 justify-end">
                  <form method="POST" action="?/deleteUser" use:enhance>
                    <input type="hidden" name="user_id" value={u.id} />
                    <button type="submit" class="text-xs text-red-600 underline hover:text-red-800">
                      Confirm Delete
                    </button>
                  </form>
                  <button type="button" onclick={() => confirmDeleteId = null}
                    class="text-xs text-brand-text-muted underline">
                    Cancel
                  </button>
                </div>
              {:else}
                <button type="button" onclick={() => confirmDeleteId = u.id}
                  use:hoverEffect={'swap'}
                  class="text-brand-text-muted hover:text-red-600 transition-colors inline-flex items-center p-1 rounded-full"
                  aria-label="Delete user">
                  <UserX size={16} strokeWidth={2.5} />
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
      class="self-center text-sm text-brand-text-muted underline hover:text-brand-text"
    >
      Load more
    </a>
  {/if}
</div>
