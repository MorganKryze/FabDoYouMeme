<script lang="ts">
  import { room } from '$lib/state/room.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Play, LogOut, Trophy } from '$lib/icons';

  interface Props {
    isHost: boolean;
  }

  let { isHost }: Props = $props();

  // Rematch countdown driven by server-provided ISO timestamp.
  let now = $state(Date.now());
  let tickHandle: ReturnType<typeof setInterval> | null = null;

  $effect(() => {
    tickHandle = setInterval(() => { now = Date.now(); }, 1000);
    return () => { if (tickHandle) clearInterval(tickHandle); };
  });

  const windowExpiresMs = $derived(
    room.rematchWindowExpiresAt ? new Date(room.rematchWindowExpiresAt).getTime() : null
  );
  const secondsLeft = $derived(
    windowExpiresMs !== null ? Math.max(0, Math.floor((windowExpiresMs - now) / 1000)) : null
  );
  const windowExpired = $derived(secondsLeft !== null && secondsLeft <= 0);

  const mmss = $derived.by(() => {
    if (secondsLeft === null) return '';
    const m = Math.floor(secondsLeft / 60);
    const s = secondsLeft % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  });

  const hostName = $derived(
    room.players.find((p) => p.is_host)?.username ?? 'the host'
  );

  function requestRematch() {
    ws.send('rematch_request', {});
  }
</script>

<div class="flex flex-col items-center gap-6 text-center max-w-2xl mx-auto" use:reveal>
  <div class="flex flex-col gap-1">
    <p class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Game over</p>
    <h2 style="font-size: clamp(2.5rem, 6vw, 4rem); font-weight: 700; line-height: 1; letter-spacing: -0.03em;">
      Final scores
    </h2>
  </div>

  <div
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 w-full max-w-sm"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <ol class="flex flex-col gap-2">
      {#each room.leaderboard as entry, i}
        <li
          class="flex items-center gap-3 rounded-full border-[2.5px] border-brand-border bg-brand-white px-4 py-2.5 text-sm"
        >
          <span class="w-6 font-bold text-brand-text-muted text-right">
            {i === 0 ? '\u{1F947}' : i === 1 ? '\u{1F948}' : i === 2 ? '\u{1F949}' : `${i + 1}.`}
          </span>
          <span class="flex-1 text-left font-bold inline-flex items-center gap-2">
            {entry.username}
          </span>
          <span class="font-bold tabular-nums text-brand-text-mid">{entry.total_score} pts</span>
        </li>
      {/each}
    </ol>
  </div>

  <!-- Rematch block -->
  <div class="flex flex-col gap-3 w-full max-w-xs">
    {#if !windowExpired}
      {#if isHost}
        <button
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          type="button"
          onclick={requestRematch}
          class="h-14 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold text-lg cursor-pointer inline-flex items-center justify-center gap-2"
          style="box-shadow: 0 5px 0 rgba(0,0,0,0.12);"
        >
          <Play size={18} strokeWidth={2.5} />
          Rematch
        </button>
      {:else}
        <p class="text-sm font-semibold text-brand-text-muted">
          Waiting for <span class="font-bold text-brand-text">{hostName}</span> to start a rematch…
        </p>
      {/if}

      {#if mmss}
        <p class="text-xs font-bold text-brand-text-muted inline-flex items-center justify-center gap-1.5">
          <Trophy size={12} strokeWidth={2.5} />
          Rematch window: <span class="tabular-nums">{mmss}</span> remaining
        </p>
      {/if}
    {:else}
      <p class="text-xs font-bold text-brand-text-muted">
        Rematch window expired — host a new game instead.
      </p>
    {/if}

    <a
      href="/"
      use:pressPhysics={'ghost'}
      class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-sm font-bold cursor-pointer inline-flex items-center justify-center gap-2"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      <LogOut size={16} strokeWidth={2.5} />
      Leave
    </a>
  </div>
</div>
