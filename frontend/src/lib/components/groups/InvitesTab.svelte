<script lang="ts">
  // Phase 2 of the groups paradigm. Admin-only tab for minting and managing
  // group_join + platform_plus_group invite codes. Non-admins never see
  // this tab (the parent gates rendering on selfIsAdmin).
  import { onMount } from 'svelte';
  import { groupsApi, type GroupInvite } from '$lib/api/groups';
  import { toast } from '$lib/state/toast.svelte';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Plus, Copy, Trash2 } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';
  import { page } from '$app/stores';

  let { gid }: { gid: string } = $props();

  let invites = $state<GroupInvite[]>([]);
  let loading = $state(false);
  let busy = $state<string | null>(null);

  // Mint form state. Two paths share the form; the kind toggle determines
  // which endpoint is hit. platform_plus is single-use by definition.
  let kind = $state<'group_join' | 'platform_plus_group'>('group_join');
  let maxUses = $state(1);
  let ttlDays = $state(7);
  let restrictedEmail = $state('');

  async function load() {
    loading = true;
    try {
      invites = await groupsApi.listInvites(gid);
    } catch (e) {
      toast.show((e as Error).message, 'error');
    } finally {
      loading = false;
    }
  }

  onMount(load);

  async function mint(e: Event) {
    e.preventDefault();
    busy = 'mint';
    try {
      const body = {
        max_uses: kind === 'platform_plus_group' ? 1 : Math.max(1, maxUses),
        ttl_seconds: Math.max(60, Math.round(ttlDays * 86400)),
        restricted_email: restrictedEmail.trim() || undefined
      };
      if (kind === 'platform_plus_group') {
        await groupsApi.mintPlatformPlus(gid, body);
      } else {
        await groupsApi.mintGroupJoin(gid, body);
      }
      restrictedEmail = '';
      await load();
      toast.show(m.groups_invite_minted(), 'success');
    } catch (e) {
      toast.show((e as Error).message, 'error');
    } finally {
      busy = null;
    }
  }

  async function revoke(inv: GroupInvite) {
    if (!confirm(m.groups_invite_revoke_confirm())) return;
    busy = `revoke:${inv.id}`;
    try {
      await groupsApi.revokeInvite(gid, inv.id);
      await load();
    } catch (e) {
      toast.show((e as Error).message, 'error');
    } finally {
      busy = null;
    }
  }

  function inviteURL(inv: GroupInvite) {
    const origin = $page.url.origin;
    if (inv.kind === 'platform_plus_group') {
      return `${origin}/auth/register?group_invite_token=${encodeURIComponent(inv.token)}`;
    }
    return `${origin}/join/group/${encodeURIComponent(inv.token)}`;
  }

  async function copyInviteURL(inv: GroupInvite) {
    try {
      await navigator.clipboard.writeText(inviteURL(inv));
      toast.show(m.groups_invite_copied(), 'success');
    } catch {
      toast.show(m.groups_invite_copy_failed(), 'error');
    }
  }

  function inviteState(inv: GroupInvite): { label: string; tone: 'ok' | 'warn' | 'dead' } {
    if (inv.revoked_at) return { label: m.groups_invite_state_revoked(), tone: 'dead' };
    if (inv.uses_count >= inv.max_uses)
      return { label: m.groups_invite_state_exhausted(), tone: 'dead' };
    if (inv.expires_at && new Date(inv.expires_at) < new Date())
      return { label: m.groups_invite_state_expired(), tone: 'dead' };
    return { label: m.groups_invite_state_active(), tone: 'ok' };
  }
</script>

<div class="flex flex-col gap-6">
  <section
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-4"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
      {m.groups_invite_mint_heading()}
    </h2>
    <form class="flex flex-col gap-4" onsubmit={mint}>
      <div class="flex flex-col gap-2">
        <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
          {m.groups_invite_kind_label()}
        </p>
        <div class="flex gap-2">
          <label class="flex-1">
            <input
              type="radio"
              bind:group={kind}
              value="group_join"
              class="peer sr-only"
            />
            <span
              class="block text-center h-11 leading-[44px] rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-sm font-bold cursor-pointer peer-checked:bg-brand-text peer-checked:text-brand-white transition-colors"
            >
              {m.groups_invite_kind_group_join()}
            </span>
          </label>
          <label class="flex-1">
            <input
              type="radio"
              bind:group={kind}
              value="platform_plus_group"
              class="peer sr-only"
            />
            <span
              class="block text-center h-11 leading-[44px] rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-sm font-bold cursor-pointer peer-checked:bg-brand-text peer-checked:text-brand-white transition-colors"
            >
              {m.groups_invite_kind_platform_plus()}
            </span>
          </label>
        </div>
        <p class="text-xs text-brand-text-muted">
          {kind === 'platform_plus_group'
            ? m.groups_invite_kind_hint_platform_plus()
            : m.groups_invite_kind_hint_group_join()}
        </p>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        {#if kind === 'group_join'}
          <label class="flex flex-col gap-1">
            <span class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
              {m.groups_invite_max_uses_label()}
            </span>
            <input
              type="number"
              bind:value={maxUses}
              min={1}
              max={1000}
              class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
            />
          </label>
        {/if}
        <label class="flex flex-col gap-1">
          <span class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
            {m.groups_invite_ttl_label()}
          </span>
          <input
            type="number"
            bind:value={ttlDays}
            min={1}
            max={30}
            class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
          />
        </label>
      </div>

      <label class="flex flex-col gap-1">
        <span class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
          {m.groups_invite_restricted_email_label()}
        </span>
        <input
          type="email"
          bind:value={restrictedEmail}
          placeholder={m.groups_invite_restricted_email_placeholder()}
          class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
        />
      </label>

      <div class="flex justify-end">
        <button
          type="submit"
          use:pressPhysics={'dark'}
          use:hoverEffect={'swap'}
          disabled={busy !== null}
          class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <Plus size={16} strokeWidth={2.5} />
          {m.groups_invite_mint_submit()}
        </button>
      </div>
    </form>
  </section>

  <section class="flex flex-col gap-3">
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
      {m.groups_invite_list_heading()}
    </h2>
    {#if loading}
      <p class="text-sm font-semibold text-brand-text-muted">{m.groups_loading()}</p>
    {:else if invites.length === 0}
      <p class="text-sm font-semibold text-brand-text-muted">{m.groups_invite_list_empty()}</p>
    {:else}
      <ul class="flex flex-col gap-2 list-none p-0 m-0">
        {#each invites as inv (inv.id)}
          {@const state = inviteState(inv)}
          <li
            class="rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-2"
            style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
          >
            <div class="flex items-center justify-between gap-3">
              <div class="flex items-center gap-2 min-w-0">
                <span class="text-[0.6rem] font-bold uppercase tracking-[0.15em] rounded-full px-2 py-1 border-[2px] border-brand-border-heavy">
                  {inv.kind === 'platform_plus_group'
                    ? m.groups_invite_kind_platform_plus()
                    : m.groups_invite_kind_group_join()}
                </span>
                <span
                  class="text-[0.6rem] font-bold uppercase tracking-[0.15em] rounded-full px-2 py-1 border-[2px]"
                  class:border-green-500={state.tone === 'ok'}
                  class:text-green-700={state.tone === 'ok'}
                  class:border-orange-400={state.tone === 'warn'}
                  class:text-orange-700={state.tone === 'warn'}
                  class:border-brand-border-heavy={state.tone === 'dead'}
                  class:text-brand-text-muted={state.tone === 'dead'}
                >
                  {state.label}
                </span>
              </div>
              <div class="flex items-center gap-2">
                {#if state.tone === 'ok'}
                  <button
                    type="button"
                    onclick={() => copyInviteURL(inv)}
                    class="h-9 px-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-xs font-bold cursor-pointer inline-flex items-center gap-1"
                  >
                    <Copy size={14} strokeWidth={2.5} />
                    {m.groups_invite_copy_link()}
                  </button>
                  <button
                    type="button"
                    disabled={busy !== null}
                    onclick={() => revoke(inv)}
                    class="h-9 px-3 rounded-full border-[2.5px] border-brand-border-heavy bg-red-50 text-red-700 text-xs font-bold cursor-pointer inline-flex items-center gap-1 disabled:opacity-50"
                  >
                    <Trash2 size={14} strokeWidth={2.5} />
                    {m.groups_invite_revoke()}
                  </button>
                {/if}
              </div>
            </div>
            <p class="text-xs text-brand-text-muted">
              {m.groups_invite_meta({
                used: String(inv.uses_count),
                max: String(inv.max_uses),
                expires: inv.expires_at ?? '—'
              })}
            </p>
            {#if inv.restricted_email}
              <p class="text-xs text-brand-text-muted">
                {m.groups_invite_restricted_email_value({ email: inv.restricted_email })}
              </p>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>
