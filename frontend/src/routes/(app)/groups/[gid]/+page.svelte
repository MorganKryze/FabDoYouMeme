<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { groupDetailState } from '$lib/state/groups.svelte';
  import { reveal } from '$lib/actions/reveal';
  import MembersTab from '$lib/components/groups/MembersTab.svelte';
  import SettingsTab from '$lib/components/groups/SettingsTab.svelte';
  import InvitesTab from '$lib/components/groups/InvitesTab.svelte';
  import PacksTab from '$lib/components/groups/PacksTab.svelte';
  import { user } from '$lib/state/user.svelte';
  import * as m from '$lib/paraglide/messages';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();
  let activeTab = $state<'packs' | 'members' | 'invites' | 'settings'>('packs');

  // Phase 2 — invites tab is admin-only. We compute admin status from the
  // members list (loaded by groupDetailState below) so the tab disappears
  // automatically when the actor is demoted.
  const selfIsAdmin = $derived(
    groupDetailState.members.some((mem) => mem.user_id === user.id && mem.role === 'admin')
  );

  onMount(() => {
    void groupDetailState.load(data.gid);
  });

  // 404 path: backend returns 404 for both "absent" and "soft-deleted from
  // a non-restore caller". Bouncing back to /groups keeps the user out of
  // an empty shell page and matches the destructive-flow convention used
  // elsewhere in the app.
  $effect(() => {
    if (groupDetailState.error?.toLowerCase().includes('not found')) {
      goto('/groups');
    }
  });
</script>

<svelte:head>
  <title>{groupDetailState.group?.name ?? m.groups_detail_loading_title()}</title>
</svelte:head>

{#if groupDetailState.loading && !groupDetailState.group}
  <p class="p-6 text-sm font-semibold text-brand-text-muted">{m.groups_loading()}</p>
{:else if groupDetailState.group}
  {@const group = groupDetailState.group}
  <div class="w-full max-w-3xl mx-auto p-6 flex flex-col gap-6" use:reveal>
    <header class="flex items-start justify-between gap-4">
      <div class="flex flex-col gap-2 min-w-0">
        <h1 class="text-2xl font-bold m-0 truncate">{group.name}</h1>
        <p class="text-sm text-brand-text-muted m-0">{group.description}</p>
      </div>
      <span
        class="shrink-0 text-[0.6rem] font-bold uppercase tracking-[0.15em] rounded-full px-2 py-1 border-[2px] border-brand-border-heavy"
      >
        {group.classification}
      </span>
    </header>

    <nav
      class="flex gap-2 border-b-[2.5px] border-brand-border-heavy overflow-x-auto -mx-6 px-6"
      style="scrollbar-width: thin;"
    >
      <button
        type="button"
        class="px-4 py-3 -mb-[2.5px] border-b-[2.5px] text-sm font-bold cursor-pointer transition-colors"
        class:border-brand-text={activeTab === 'packs'}
        class:border-transparent={activeTab !== 'packs'}
        class:text-brand-text-muted={activeTab !== 'packs'}
        onclick={() => (activeTab = 'packs')}
      >
        {m.groups_tab_packs()}
      </button>
      <button
        type="button"
        class="px-4 py-3 -mb-[2.5px] border-b-[2.5px] text-sm font-bold cursor-pointer transition-colors"
        class:border-brand-text={activeTab === 'members'}
        class:border-transparent={activeTab !== 'members'}
        class:text-brand-text-muted={activeTab !== 'members'}
        onclick={() => (activeTab = 'members')}
      >
        {m.groups_tab_members()}
      </button>
      {#if selfIsAdmin}
        <button
          type="button"
          class="px-4 py-3 -mb-[2.5px] border-b-[2.5px] text-sm font-bold cursor-pointer transition-colors"
          class:border-brand-text={activeTab === 'invites'}
          class:border-transparent={activeTab !== 'invites'}
          class:text-brand-text-muted={activeTab !== 'invites'}
          onclick={() => (activeTab = 'invites')}
        >
          {m.groups_tab_invites()}
        </button>
      {/if}
      <button
        type="button"
        class="px-4 py-3 -mb-[2.5px] border-b-[2.5px] text-sm font-bold cursor-pointer transition-colors"
        class:border-brand-text={activeTab === 'settings'}
        class:border-transparent={activeTab !== 'settings'}
        class:text-brand-text-muted={activeTab !== 'settings'}
        onclick={() => (activeTab = 'settings')}
      >
        {m.groups_tab_settings()}
      </button>
    </nav>

    {#if activeTab === 'packs'}
      <PacksTab gid={data.gid} />
    {:else if activeTab === 'members'}
      <MembersTab gid={data.gid} />
    {:else if activeTab === 'invites' && selfIsAdmin}
      <InvitesTab gid={data.gid} />
    {:else}
      <SettingsTab gid={data.gid} />
    {/if}
  </div>
{/if}
