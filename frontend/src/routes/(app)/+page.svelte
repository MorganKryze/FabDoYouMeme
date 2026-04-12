<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import type { ActionData, PageData } from './$types';
  import type { Pack, PaginatedResponse } from '$lib/api/types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  // Default to the first loaded game type; user can change it afterwards.
  let selectedGameTypeId = $state(untrack(() => data.gameTypes[0]?.id ?? ''));
  let packs = $state<Pack[]>([]);
  let selectedPackId = $state('');
  let isSolo = $state(false);
  let roundCount = $state(5);
  let roundDuration = $state(60);
  let votingDuration = $state(30);
  let loadingPacks = $state(false);
  let currentAbortController: AbortController | null = null;
  // Imperative focus for the Join-room code input — replaces the raw
  // `autofocus` attribute so screen readers announce the focus change
  // (a11y_autofocus).
  let roomCodeInput = $state<HTMLInputElement | null>(null);
  $effect(() => {
    if (roomCodeInput) roomCodeInput.focus();
  });

  const selectedGameType = $derived(
    data.gameTypes.find((gt) => gt.id === selectedGameTypeId) ?? null
  );

  async function loadPacks() {
    if (!selectedGameTypeId) return;
    currentAbortController?.abort();
    currentAbortController = new AbortController();
    const signal = currentAbortController.signal;
    loadingPacks = true;
    try {
      const res = await fetch(`/api/packs?game_type_id=${selectedGameTypeId}`, { signal });
      if (!res.ok) { packs = []; return; }
      const body: PaginatedResponse<Pack> = await res.json();
      packs = body.data ?? [];
      selectedPackId = packs[0]?.id ?? '';
    } catch (err) {
      if ((err as Error).name !== 'AbortError') { packs = []; }
    } finally {
      loadingPacks = false;
    }
  }

  $effect(() => {
    void loadPacks();
  });
</script>

<svelte:head>
  <title>Lobby — FabDoYouMeme</title>
</svelte:head>

<div class="flex-1 flex items-start justify-center p-6 pt-8">
  <div class="w-full max-w-3xl grid grid-cols-1 md:grid-cols-2 gap-5">

    <!-- Create Room Card -->
    <div
      use:reveal
      class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-6 flex flex-col gap-4 cursor-default"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
    >
      <h2 class="text-xl font-bold">Create a Room</h2>

      {#if form?.error}
        <div
          class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
        >
          {form.error}
        </div>
      {/if}

      <form method="POST" action="?/createRoom" use:enhance class="flex flex-col gap-4">
        <div class="flex flex-col gap-1">
          <label for="game_type" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Game Type</label>
          <select
            id="game_type"
            name="game_type_id"
            bind:value={selectedGameTypeId}
            class="h-11 rounded-lg border-[2.5px] border-brand-border-heavy bg-brand-white px-3 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
          >
            {#each data.gameTypes as gt}
              <option value={gt.id}>{gt.name}</option>
            {/each}
          </select>
        </div>

        <div class="flex flex-col gap-1">
          <label for="pack" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Pack</label>
          {#if loadingPacks}
            <p class="text-sm font-semibold text-brand-text-muted">Loading packs…</p>
          {:else if packs.length === 0}
            <p class="text-sm font-semibold text-brand-text-muted">No compatible packs found for this game type.</p>
          {:else}
            <select
              id="pack"
              name="pack_id"
              bind:value={selectedPackId}
              class="h-11 rounded-lg border-[2.5px] border-brand-border-heavy bg-brand-white px-3 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
            >
              {#each packs as p}
                <option value={p.id}>{p.name}</option>
              {/each}
            </select>
          {/if}
        </div>

        {#if selectedGameType?.supports_solo}
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" bind:checked={isSolo} class="h-4 w-4 rounded border-brand-border-heavy" />
            <span class="text-sm font-semibold">Solo mode</span>
          </label>
          <input type="hidden" name="is_solo" value={String(isSolo)} />
        {:else}
          <input type="hidden" name="is_solo" value="false" />
        {/if}

        <div class="flex flex-col gap-1">
          <label for="round_count" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Rounds: {roundCount}</label>
          <input id="round_count" type="range" name="round_count" min={1} max={50} bind:value={roundCount}
            class="accent-brand-accent" />
        </div>

        <div class="flex flex-col gap-1">
          <label for="round_duration_seconds" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Submission time: {roundDuration}s</label>
          <input id="round_duration_seconds" type="range" name="round_duration_seconds" min={15} max={300} step={5}
            bind:value={roundDuration} class="accent-brand-accent" />
        </div>

        <div class="flex flex-col gap-1">
          <label for="voting_duration_seconds" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Voting time: {votingDuration}s</label>
          <input id="voting_duration_seconds" type="range" name="voting_duration_seconds" min={10} max={120} step={5}
            bind:value={votingDuration} class="accent-brand-accent" />
        </div>

        <button
          use:pressPhysics={'dark'}
          type="submit"
          disabled={!selectedPackId}
          class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-50 transition-colors cursor-pointer"
        >
          Create Room
        </button>
      </form>
    </div>

    <!-- Join Room Card -->
    <div
      use:reveal={{ delay: 1 }}
      class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-6 flex flex-col gap-4 cursor-default"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
    >
      <h2 class="text-xl font-bold">Join a Room</h2>

      {#if form?.joinError}
        <div
          class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
        >
          {form.joinError}
        </div>
      {/if}

      <form method="POST" action="?/joinRoom" use:enhance class="flex flex-col gap-4">
        <div class="flex flex-col gap-1">
          <label for="room-code" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Room Code</label>
          <input
            id="room-code"
            name="code"
            bind:this={roomCodeInput}
            type="text"
            inputmode="text"
            autocapitalize="characters"
            maxlength={4}
            placeholder="WXYZ"
            class="h-16 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-6 text-center text-2xl font-mono font-bold tracking-widest uppercase focus:outline-none focus:border-brand-text transition-colors"
            style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
          />
        </div>

        <button
          use:pressPhysics={'primary'}
          type="submit"
          class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text font-bold hover:bg-brand-white transition-colors cursor-pointer"
        >
          Join Game
        </button>
      </form>
    </div>
  </div>
</div>
