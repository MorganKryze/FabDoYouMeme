<script lang="ts">
  import { enhance } from '$app/forms';
  import { goto } from '$app/navigation';
  import { untrack } from 'svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { reveal } from '$lib/actions/reveal';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Search, Shield, UserX, Mail, Gamepad2, Clock } from '$lib/icons';
  import type { ActionData, PageData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  // Seed the input from the URL query once; thereafter the user owns it
  // and a debounced goto() propagates edits back to the URL.
  let searchTerm = $state(untrack(() => data.q ?? ''));
  let searchTimeout: ReturnType<typeof setTimeout>;
  let confirmDeleteId = $state<string | null>(null);
  let confirmSendLinkId = $state<string | null>(null);

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
    if (!iso) return m.admin_users_relative_none();
    const then = new Date(iso).getTime();
    if (Number.isNaN(then)) return m.admin_users_relative_none();
    const diff = Math.max(0, Date.now() - then);
    const sec = Math.floor(diff / 1000);
    if (sec < 60) return m.admin_users_relative_just_now();
    const min = Math.floor(sec / 60);
    if (min < 60) return m.admin_users_relative_minutes({ count: min });
    const hr = Math.floor(min / 60);
    if (hr < 24) return m.admin_users_relative_hours({ count: hr });
    const day = Math.floor(hr / 24);
    if (day < 30) return m.admin_users_relative_days({ count: day });
    const mo = Math.floor(day / 30);
    if (mo < 12) return m.admin_users_relative_months({ count: mo });
    return m.admin_users_relative_years({ count: Math.floor(mo / 12) });
  }

  $effect(() => {
    if (form === lastForm) return;
    lastForm = form;
    if (form?.error) toast.show(form.error, 'error');
    if (form?.success) toast.show(m.admin_users_toast_updated(), 'success');
    if (form?.deleted) {
      toast.show(m.admin_users_toast_deleted(), 'success');
      confirmDeleteId = null;
    }
    if (form?.link_sent) {
      toast.show(m.admin_users_toast_link_sent(), 'success');
      confirmSendLinkId = null;
    }
  });
</script>

<svelte:head>
  <title>{m.admin_users_page_title()}</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4" use:reveal>
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">{m.admin_users_heading()}</h1>
    <div class="relative">
      <Search size={14} strokeWidth={2.5} class="absolute left-3 top-1/2 -translate-y-1/2 text-brand-text-muted" />
      <input
        type="search"
        placeholder={m.admin_users_search_placeholder()}
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
          <th class="text-left px-4 py-3">{m.admin_users_col_username()}</th>
          <th class="text-left px-4 py-3">{m.admin_users_col_email()}</th>
          <th class="text-left px-4 py-3">{m.admin_users_col_role()}</th>
          <th class="text-left px-4 py-3">{m.admin_users_col_active()}</th>
          <th class="text-left px-4 py-3">
            <span class="inline-flex items-center gap-1">
              <Gamepad2 size={12} strokeWidth={2.5} />
              {m.admin_users_col_games()}
            </span>
          </th>
          <th class="text-left px-4 py-3">{m.admin_users_col_joined()}</th>
          <th class="text-left px-4 py-3">
            <span class="inline-flex items-center gap-1">
              <Clock size={12} strokeWidth={2.5} />
              {m.admin_users_col_last_login()}
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
                    title={m.admin_users_protected_tooltip()}
                    aria-label={m.admin_users_protected_aria()}
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
                  title={u.is_protected ? m.admin_users_role_locked_tooltip() : undefined}
                  class="h-7 rounded border border-transparent hover:border-brand-border px-1 text-sm bg-transparent focus:outline-none focus:border-ring cursor-pointer disabled:cursor-not-allowed disabled:opacity-60"
                >
                  <option value="player">{m.admin_users_role_player()}</option>
                  <option value="admin">{m.admin_users_role_admin()}</option>
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
                  title={u.is_protected ? m.admin_users_deactivate_locked_tooltip() : undefined}
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
                : m.admin_users_last_login_none()}
            >
              {formatRelative(u.last_login_at)}
            </td>
            <td class="px-4 py-3 text-right">
              <div class="flex gap-1 justify-end items-center">
                {#if confirmSendLinkId === u.id}
                  <form method="POST" action="?/sendMagicLink" use:enhance>
                    <input type="hidden" name="user_id" value={u.id} />
                    <button type="submit" class="text-xs text-brand-text underline hover:text-brand-text">
                      {m.admin_users_send_link_confirm()}
                    </button>
                  </form>
                  <button type="button" onclick={() => (confirmSendLinkId = null)}
                    class="text-xs text-brand-text-muted underline">
                    {m.admin_users_cancel()}
                  </button>
                {:else if confirmDeleteId === u.id}
                  <form method="POST" action="?/deleteUser" use:enhance>
                    <input type="hidden" name="user_id" value={u.id} />
                    <button type="submit" class="text-xs text-red-600 underline hover:text-red-800">
                      {m.admin_users_confirm_delete()}
                    </button>
                  </form>
                  <button type="button" onclick={() => (confirmDeleteId = null)}
                    class="text-xs text-brand-text-muted underline">
                    {m.admin_users_cancel()}
                  </button>
                {:else}
                  <button type="button"
                    onclick={() => { confirmDeleteId = null; confirmSendLinkId = u.id; }}
                    use:hoverEffect={'swap'}
                    class="text-brand-text-muted hover:text-brand-text transition-colors inline-flex items-center p-1 rounded-full"
                    aria-label={m.admin_users_send_link_aria()}>
                    <Mail size={16} strokeWidth={2.5} />
                  </button>
                  {#if !u.is_protected}
                    <button type="button"
                      onclick={() => { confirmSendLinkId = null; confirmDeleteId = u.id; }}
                      use:hoverEffect={'swap'}
                      class="text-brand-text-muted hover:text-red-600 transition-colors inline-flex items-center p-1 rounded-full"
                      aria-label={m.admin_users_delete_aria()}>
                      <UserX size={16} strokeWidth={2.5} />
                    </button>
                  {/if}
                {/if}
              </div>
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
      {m.admin_users_load_more()}
    </a>
  {/if}
</div>
