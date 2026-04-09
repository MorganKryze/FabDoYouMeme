<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';
  import type { Pack, PaginatedResponse } from '$lib/api/types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let selectedGameTypeId = $state(data.gameTypes[0]?.id ?? '');
  let packs = $state<Pack[]>([]);
  let selectedPackId = $state('');
  let isSolo = $state(false);
  let roundCount = $state(5);
  let roundDuration = $state(60);
  let votingDuration = $state(30);
  let loadingPacks = $state(false);
  let currentAbortController: AbortController | null = null;

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

<div class="flex-1 flex items-start justify-center p-6 pt-12">
  <div class="w-full max-w-3xl grid grid-cols-1 md:grid-cols-2 gap-6">

    <!-- Create Room Card -->
    <div class="rounded-xl border border-border bg-card p-6 flex flex-col gap-4">
      <h2 class="text-lg font-semibold">Create a Room</h2>

      {#if form?.error}
        <div class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {form.error}
        </div>
      {/if}

      <form method="POST" action="?/createRoom" use:enhance class="flex flex-col gap-4">
        <div class="flex flex-col gap-1">
          <label for="game_type" class="text-sm font-medium">Game Type</label>
          <select
            id="game_type"
            name="game_type_id"
            bind:value={selectedGameTypeId}
            class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          >
            {#each data.gameTypes as gt}
              <option value={gt.id}>{gt.name}</option>
            {/each}
          </select>
        </div>

        <div class="flex flex-col gap-1">
          <label for="pack" class="text-sm font-medium">Pack</label>
          {#if loadingPacks}
            <p class="text-sm text-muted-foreground">Loading packs…</p>
          {:else if packs.length === 0}
            <p class="text-sm text-muted-foreground">No compatible packs found for this game type.</p>
          {:else}
            <select
              id="pack"
              name="pack_id"
              bind:value={selectedPackId}
              class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
            >
              {#each packs as p}
                <option value={p.id}>{p.name}</option>
              {/each}
            </select>
          {/if}
        </div>

        {#if selectedGameType?.supports_solo}
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" bind:checked={isSolo} class="h-4 w-4 rounded border-input" />
            <span class="text-sm">Solo mode</span>
          </label>
          <input type="hidden" name="is_solo" value={String(isSolo)} />
        {:else}
          <input type="hidden" name="is_solo" value="false" />
        {/if}

        <div class="flex flex-col gap-1">
          <label for="round_count" class="text-sm font-medium">Rounds: {roundCount}</label>
          <input id="round_count" type="range" name="round_count" min={1} max={50} bind:value={roundCount}
            class="accent-primary" />
        </div>

        <div class="flex flex-col gap-1">
          <label for="round_duration_seconds" class="text-sm font-medium">Submission time: {roundDuration}s</label>
          <input id="round_duration_seconds" type="range" name="round_duration_seconds" min={15} max={300} step={5}
            bind:value={roundDuration} class="accent-primary" />
        </div>

        <div class="flex flex-col gap-1">
          <label for="voting_duration_seconds" class="text-sm font-medium">Voting time: {votingDuration}s</label>
          <input id="voting_duration_seconds" type="range" name="voting_duration_seconds" min={10} max={120} step={5}
            bind:value={votingDuration} class="accent-primary" />
        </div>

        <button
          type="submit"
          disabled={!selectedPackId}
          class="h-11 rounded-lg bg-primary text-primary-foreground font-semibold disabled:opacity-50 hover:bg-primary/90 transition-colors"
        >
          Create Room
        </button>
      </form>
    </div>

    <!-- Join Room Card -->
    <div class="rounded-xl border border-border bg-card p-6 flex flex-col gap-4">
      <h2 class="text-lg font-semibold">Join a Room</h2>

      {#if form?.joinError}
        <div class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {form.joinError}
        </div>
      {/if}

      <form method="POST" action="?/joinRoom" use:enhance class="flex flex-col gap-4">
        <div class="flex flex-col gap-1">
          <label for="room-code" class="text-sm font-medium">Room Code</label>
          <input
            id="room-code"
            name="code"
            type="text"
            inputmode="text"
            autocapitalize="characters"
            maxlength={4}
            placeholder="WXYZ"
            autofocus
            class="h-14 rounded-lg border border-input bg-background px-4 text-center text-2xl font-mono tracking-widest uppercase focus:outline-none focus:ring-2 focus:ring-ring"
          />
        </div>

        <button
          type="submit"
          class="h-11 rounded-lg bg-secondary text-secondary-foreground font-semibold hover:bg-secondary/80 transition-colors"
        >
          Join Game
        </button>
      </form>
    </div>
  </div>
</div>
