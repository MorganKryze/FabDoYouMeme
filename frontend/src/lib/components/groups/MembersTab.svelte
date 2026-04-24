<script lang="ts">
  // Phase 1 of the groups paradigm. Lists the current member roster and
  // surfaces the admin-only Promote / Kick / Ban controls. The actor's
  // own row never shows action buttons (admin uses Settings → Leave for
  // self-departure so the last-admin guard kicks in).
  import { groupDetailState } from '$lib/state/groups.svelte';
  import { groupsApi } from '$lib/api/groups';
  import { user } from '$lib/state/user.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { Shield, UserX, Ban } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';

  let { gid }: { gid: string } = $props();

  let busy = $state<string | null>(null);

  const selfMembership = $derived(
    groupDetailState.members.find((mem) => mem.user_id === user.id) ?? null
  );
  const selfIsAdmin = $derived(selfMembership?.role === 'admin');

  async function withBusy(key: string, fn: () => Promise<unknown>) {
    if (busy) return;
    busy = key;
    try {
      await fn();
      await groupDetailState.load(gid);
    } catch (e) {
      toast.show((e as Error).message, 'error');
    } finally {
      busy = null;
    }
  }

  function promote(userID: string) {
    return withBusy(`promote:${userID}`, () => groupsApi.promote(gid, userID));
  }
  function kick(userID: string) {
    if (!confirm(m.groups_kick_confirm())) return Promise.resolve();
    return withBusy(`kick:${userID}`, () => groupsApi.kick(gid, userID));
  }
  function ban(userID: string) {
    if (!confirm(m.groups_ban_confirm())) return Promise.resolve();
    return withBusy(`ban:${userID}`, () => groupsApi.ban(gid, userID));
  }
</script>

<section class="flex flex-col gap-3">
  {#if groupDetailState.members.length === 0}
    <p class="text-sm font-semibold text-brand-text-muted">{m.groups_members_empty()}</p>
  {:else}
    <ul class="flex flex-col gap-2 list-none p-0 m-0">
      {#each groupDetailState.members as member (member.user_id)}
        <li
          class="flex items-center justify-between gap-3 rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-4 py-3"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
        >
          <div class="flex items-center gap-3 min-w-0">
            {#if member.role === 'admin'}
              <Shield size={16} strokeWidth={2.5} />
            {/if}
            <div class="min-w-0">
              <p class="text-sm font-bold m-0 truncate">{member.username}</p>
              <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted m-0">
                {member.role === 'admin' ? m.groups_role_admin() : m.groups_role_member()}
              </p>
            </div>
          </div>

          {#if selfIsAdmin && member.user_id !== user.id}
            <div class="flex items-center gap-2 shrink-0">
              {#if member.role !== 'admin'}
                <button
                  type="button"
                  disabled={busy !== null}
                  onclick={() => promote(member.user_id)}
                  class="h-9 px-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-xs font-bold cursor-pointer inline-flex items-center gap-1 disabled:opacity-50"
                  aria-label={m.groups_promote()}
                >
                  <Shield size={14} strokeWidth={2.5} />
                  {m.groups_promote()}
                </button>
                <button
                  type="button"
                  disabled={busy !== null}
                  onclick={() => kick(member.user_id)}
                  class="h-9 px-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-xs font-bold cursor-pointer inline-flex items-center gap-1 disabled:opacity-50"
                  aria-label={m.groups_kick()}
                >
                  <UserX size={14} strokeWidth={2.5} />
                  {m.groups_kick()}
                </button>
              {/if}
              <button
                type="button"
                disabled={busy !== null}
                onclick={() => ban(member.user_id)}
                class="h-9 px-3 rounded-full border-[2.5px] border-brand-border-heavy bg-red-50 text-red-600 text-xs font-bold cursor-pointer inline-flex items-center gap-1 disabled:opacity-50"
                aria-label={m.groups_ban()}
              >
                <Ban size={14} strokeWidth={2.5} />
                {m.groups_ban()}
              </button>
            </div>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</section>
