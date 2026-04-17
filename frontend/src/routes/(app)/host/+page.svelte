<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { ArrowLeft, Plus, Package, Check } from '$lib/icons';
  import type { ActionData, PageData } from './$types';
  import type { Pack, PaginatedResponse, GameType } from '$lib/api/types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  type Step = 'game' | 'pack';

  let selectedGameTypeId = $state(untrack(() => data.preselectedId));
  let selectedPackId = $state('');
  let isSolo = $state(false);
  let packs = $state<Pack[]>([]);
  let loadingPacks = $state(false);
  let step = $state<Step>(untrack(() => (data.preselectedId ? 'pack' : 'game')));

  const selectedGameType = $derived<GameType | null>(
    data.gameTypes.find((gt) => gt.id === selectedGameTypeId) ?? null
  );

  const officialPacks = $derived(packs.filter((p) => p.is_official || p.is_system));
  const personalPacks = $derived(packs.filter((p) => !(p.is_official || p.is_system)));

  let packAbort: AbortController | null = null;

  async function loadPacks() {
    if (!selectedGameTypeId) { packs = []; return; }
    packAbort?.abort();
    packAbort = new AbortController();
    loadingPacks = true;
    try {
      const res = await fetch(`/api/packs?game_type_id=${selectedGameTypeId}`, {
        signal: packAbort.signal
      });
      if (!res.ok) { packs = []; return; }
      const body: PaginatedResponse<Pack> = await res.json();
      packs = body.data ?? [];
      selectedPackId = '';
    } catch (err) {
      if ((err as Error).name !== 'AbortError') packs = [];
    } finally {
      loadingPacks = false;
    }
  }

  $effect(() => {
    if (step === 'pack') void loadPacks();
  });

  function pickGame(id: string) {
    selectedGameTypeId = id;
    step = 'pack';
  }
</script>

<svelte:head>
  <title>Host a game — FabDoYouMeme</title>
</svelte:head>

<div class="flex-1 flex items-start justify-center p-6 pt-8">
  <div class="w-full max-w-3xl flex flex-col gap-6">

    <header use:reveal class="flex items-center gap-3">
      <a
        href="/"
        use:pressPhysics={'ghost'}
        class="h-10 w-10 inline-flex items-center justify-center rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
        aria-label="Back to home"
      >
        <ArrowLeft size={18} strokeWidth={2.5} />
      </a>
      <h1 class="text-2xl font-bold">Host a game</h1>
    </header>

    {#if form?.error}
      <div
        use:reveal={{ delay: 1 }}
        class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold"
        style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
      >
        {form.error}
      </div>
    {/if}

    {#if step === 'game'}
      <section class="flex flex-col gap-4">
        <p use:reveal={{ delay: 1 }} class="text-sm font-semibold text-brand-text-muted uppercase tracking-[0.2em]">Pick a game</p>
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {#each data.gameTypes as gt, i}
            <button
              type="button"
              use:reveal={{ delay: i + 2 }}
              use:physCard
              use:hoverEffect={'gradient'}
              onclick={() => pickGame(gt.id)}
              class="text-left rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-3 cursor-pointer"
              style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
            >
              <div class="text-lg font-bold">{gt.name}</div>
              {#if gt.description}
                <p class="text-sm font-semibold text-brand-text-muted">{gt.description}</p>
              {/if}
              <div class="mt-auto text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
                {gt.supports_solo ? 'Solo + party' : 'Party'}
              </div>
            </button>
          {/each}
        </div>
      </section>
    {:else}
      <form method="POST" action="?/createRoom" use:enhance class="flex flex-col gap-5">
        <input type="hidden" name="game_type_id" value={selectedGameTypeId} />
        <input type="hidden" name="pack_id" value={selectedPackId} />
        <input type="hidden" name="is_solo" value={String(isSolo)} />

        <div class="flex items-center justify-between">
          <p class="text-sm font-semibold text-brand-text-muted uppercase tracking-[0.2em]">Pick a pack</p>
          <button
            type="button"
            onclick={() => (step = 'game')}
            class="text-xs font-bold underline decoration-2 underline-offset-4 cursor-pointer"
          >
            Change game
          </button>
        </div>

        {#snippet packCard(p: Pack, i: number)}
          {@const selected = selectedPackId === p.id}
          <button
            type="button"
            use:reveal={{ delay: i + 1 }}
            use:physCard
            onclick={() => (selectedPackId = p.id)}
            aria-pressed={selected}
            class="relative text-left rounded-[22px] border-[2.5px] p-5 flex flex-col gap-2 cursor-pointer transition-all
                   {selected
                     ? 'border-brand-text bg-brand-surface ring-4 ring-brand-text/15'
                     : 'border-brand-border-heavy bg-brand-white'}"
            style={selected
              ? 'box-shadow: 0 6px 0 rgba(0,0,0,0.15);'
              : 'box-shadow: 0 5px 0 rgba(0,0,0,0.08);'}
          >
            {#if selected}
              <span
                class="absolute -top-2 -right-2 h-7 w-7 inline-flex items-center justify-center rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white"
                style="box-shadow: 0 2px 0 rgba(0,0,0,0.12);"
                aria-hidden="true"
              >
                <Check size={14} strokeWidth={3} />
              </span>
            {/if}
            <div class="inline-flex items-center gap-2 pr-8">
              <Package size={16} strokeWidth={2.5} />
              <span class="font-bold">{p.name}</span>
            </div>
            {#if p.description}
              <p class="text-xs font-semibold text-brand-text-muted line-clamp-2">{p.description}</p>
            {/if}
            {#if typeof p.item_count === 'number'}
              <div class="mt-auto text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
                {p.item_count} items
              </div>
            {/if}
          </button>
        {/snippet}

        {#if loadingPacks}
          <p class="text-sm font-semibold text-brand-text-muted">Loading packs…</p>
        {:else if packs.length === 0}
          <p class="text-sm font-semibold text-brand-text-muted">No compatible packs for {selectedGameType?.name ?? 'this game'}.</p>
        {:else}
          <div class="flex flex-col gap-6">
            {#if officialPacks.length > 0}
              <div class="flex flex-col gap-3">
                <p class="text-[0.7rem] font-bold text-brand-text-muted uppercase tracking-[0.2em]">Official</p>
                <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {#each officialPacks as p, i (p.id)}
                    {@render packCard(p, i)}
                  {/each}
                </div>
              </div>
            {/if}
            {#if personalPacks.length > 0}
              <div class="flex flex-col gap-3">
                <p class="text-[0.7rem] font-bold text-brand-text-muted uppercase tracking-[0.2em]">Personal</p>
                <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {#each personalPacks as p, i (p.id)}
                    {@render packCard(p, i + officialPacks.length)}
                  {/each}
                </div>
              </div>
            {/if}
          </div>
        {/if}

        {#if selectedGameType?.supports_solo}
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" bind:checked={isSolo} class="h-4 w-4 rounded border-brand-border-heavy" />
            <span class="text-sm font-semibold">Solo mode</span>
          </label>
        {/if}

        <button
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          type="submit"
          disabled={!selectedPackId}
          class="h-14 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-50 cursor-pointer inline-flex items-center justify-center gap-2"
        >
          <Plus size={18} strokeWidth={2.5} />
          Spin up the room
        </button>
      </form>
    {/if}
  </div>
</div>

<footer class="border-t border-brand-border px-6 py-6 flex items-center justify-between text-xs font-semibold text-brand-text-muted">
  <p>© {new Date().getFullYear()} FabDoYouMeme</p>
  <a href="/privacy" class="hover:text-brand-text transition-colors">Privacy Policy</a>
</footer>
