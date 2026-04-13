<script lang="ts">
  import { room } from '$lib/state/room.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { Play, Clock, ListChecks } from '$lib/icons';
  import MemeCaptionGameRules from '$lib/games/meme-caption/GameRules.svelte';

  interface Props {
    isHost: boolean;
  }

  let { isHost }: Props = $props();

  function startGame() {
    ws.send('start');
  }

  const hostName = $derived(
    room.players.find((p) => p.is_host)?.username ?? 'the host'
  );
</script>

<div class="flex flex-col items-center gap-8 text-center max-w-2xl mx-auto" use:reveal>
  <div class="flex flex-col gap-2">
    <p class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Staging</p>
    <h2 class="text-3xl font-bold">{room.gameType?.name ?? 'Waiting'}</h2>
    {#if room.gameType?.description}
      <p class="text-sm font-semibold text-brand-text-muted max-w-md">{room.gameType.description}</p>
    {/if}
  </div>

  {#if room.gameType}
    <MemeCaptionGameRules gameType={room.gameType} />
  {/if}

  <!--
    F3 v1: round/duration knobs are read-only; see plan deviation note —
    live config tuning needs a new backend WS message and is deferred.
    The host still sees the current defaults so there are no surprises.
  -->
  <div class="flex flex-wrap items-center justify-center gap-2">
    <span
      class="inline-flex items-center gap-1.5 h-9 px-4 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-xs font-bold"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      <ListChecks size={14} strokeWidth={2.5} />
      5 rounds
    </span>
    <span
      class="inline-flex items-center gap-1.5 h-9 px-4 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-xs font-bold"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      <Clock size={14} strokeWidth={2.5} />
      60s submit / 30s vote
    </span>
  </div>

  {#if isHost}
    <div class="flex flex-col gap-3 w-full max-w-xs">
      <button
        use:pressPhysics={'dark'}
        type="button"
        onclick={startGame}
        disabled={room.players.length < 2}
        class="h-14 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold text-lg disabled:opacity-50 transition-colors cursor-pointer inline-flex items-center justify-center gap-2"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.12);"
      >
        <Play size={18} strokeWidth={2.5} />
        Start game
      </button>
      {#if room.players.length < 2}
        <p class="text-xs font-semibold text-brand-text-muted">Need at least 2 players to start.</p>
      {/if}
    </div>
  {:else}
    <p class="text-sm font-semibold text-brand-text-muted">
      Waiting for <span class="font-bold text-brand-text">{hostName}</span> to start…
    </p>
  {/if}
</div>
