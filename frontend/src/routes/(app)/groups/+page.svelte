<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { groupsState } from '$lib/state/groups.svelte';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { Plus, Users } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';

  onMount(() => groupsState.load());
</script>

<svelte:head>
  <title>{m.groups_page_title()}</title>
</svelte:head>

<div class="w-full max-w-3xl mx-auto p-6 flex flex-col gap-6" use:reveal>
  <header class="flex items-center justify-between gap-4">
    <h1 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
      {m.groups_heading()}
    </h1>
    <button
      type="button"
      use:pressPhysics={'dark'}
      use:hoverEffect={'swap'}
      onclick={() => goto('/groups/new')}
      class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2"
    >
      <Plus size={16} strokeWidth={2.5} />
      {m.groups_new_cta()}
    </button>
  </header>

  {#if groupsState.loading}
    <p class="text-sm font-semibold text-brand-text-muted">{m.groups_loading()}</p>
  {:else if groupsState.error}
    <p class="text-sm font-bold text-red-600">{groupsState.error}</p>
  {:else if groupsState.groups.length === 0}
    <section
      class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-8 text-center"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
    >
      <Users size={32} strokeWidth={2.5} class="mx-auto mb-3 text-brand-text-muted" />
      <p class="text-sm font-semibold text-brand-text-muted">{m.groups_empty_state()}</p>
    </section>
  {:else}
    <ul class="grid grid-cols-1 sm:grid-cols-2 gap-4 list-none p-0 m-0">
      {#each groupsState.groups as g (g.id)}
        <li>
          <button
            type="button"
            onclick={() => goto(`/groups/${g.id}`)}
            use:hoverEffect={'glow'}
            class="w-full text-left rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 cursor-pointer"
            style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
          >
            <div class="flex items-center justify-between gap-2">
              <h2 class="text-base font-bold m-0 truncate">{g.name}</h2>
              <span
                class="shrink-0 text-[0.6rem] font-bold uppercase tracking-[0.15em] rounded-full px-2 py-1 border-[2px] border-brand-border-heavy"
              >
                {g.classification}
              </span>
            </div>
            <p class="text-sm text-brand-text-muted mt-2 line-clamp-2">{g.description}</p>
            <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted mt-3">
              {g.member_role === 'admin' ? m.groups_role_admin() : m.groups_role_member()}
            </p>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>
