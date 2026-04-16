<script lang="ts">
  import { page } from '$app/stores';
  import { room } from '$lib/state/room.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import {
    Play,
    Clock,
    ListChecks,
    Copy,
    Link2,
    CheckCircle,
    Users,
    Sparkles
  } from '$lib/icons';
  import type { GameType, Room } from '$lib/api/types';
  import EndRoomButton from './EndRoomButton.svelte';

  interface Props {
    isHost: boolean;
  }

  let { isHost }: Props = $props();

  const AVATAR_COLORS = ['#E8937F', '#8BC9B1', '#D4A5C9', '#A8D8EA', '#FDDCB5', '#B5E2D0'];

  // Data sources: prefer the reactive room singleton; fall back to
  // page.data for SSR-first renders and stable config values.
  const pageRoom = $derived<Room | null>(($page.data as any)?.room ?? null);
  const gameType = $derived<GameType | null>(
    room.gameType ?? (pageRoom as any)?.game_type ?? null
  );

  const gameName = $derived(gameType?.name ?? 'Unrecognized game');
  const gameDescription = $derived(gameType?.description ?? '');
  const minPlayers = $derived(gameType?.config.min_players ?? 2);
  const maxPlayers = $derived(gameType?.config.max_players ?? 8);

  // Round / timing values come from the room's own config (the host's
  // selected values), not the game type's defaults.
  const roundCount = $derived(
    pageRoom?.config.round_count ?? gameType?.config.default_round_count ?? 5
  );
  const submitDuration = $derived(
    pageRoom?.config.round_duration_seconds ??
      gameType?.config.default_round_duration_seconds ??
      60
  );
  const voteDuration = $derived(
    pageRoom?.config.voting_duration_seconds ??
      gameType?.config.default_voting_duration_seconds ??
      30
  );

  let codeCopied = $state(false);
  let linkCopied = $state(false);
  let codeTimeout: ReturnType<typeof setTimeout> | null = null;
  let linkTimeout: ReturnType<typeof setTimeout> | null = null;

  function startGame() {
    ws.send('start');
  }

  function inviteLink(): string {
    if (typeof window === 'undefined' || !room.code) return '';
    return `${window.location.origin}/join/${room.code}`;
  }

  async function copyCode() {
    if (!room.code) return;
    try {
      await navigator.clipboard.writeText(room.code);
      codeCopied = true;
      toast.show('Room code copied', 'success');
      if (codeTimeout) clearTimeout(codeTimeout);
      codeTimeout = setTimeout(() => { codeCopied = false; }, 2000);
    } catch {
      toast.show('Could not copy room code', 'error');
    }
  }

  async function copyInviteLink() {
    const link = inviteLink();
    if (!link) return;
    try {
      if (typeof navigator !== 'undefined' && 'share' in navigator) {
        try {
          await navigator.share({
            title: 'Join my FabDoYouMeme game',
            text: `Join me with code ${room.code}`,
            url: link
          });
          return;
        } catch {
          // user cancelled share sheet → fall through to clipboard copy
        }
      }
      await navigator.clipboard.writeText(link);
      linkCopied = true;
      toast.show('Invite link copied', 'success');
      if (linkTimeout) clearTimeout(linkTimeout);
      linkTimeout = setTimeout(() => { linkCopied = false; }, 2000);
    } catch {
      toast.show('Could not copy invite link', 'error');
    }
  }

  const hostName = $derived(
    room.players.find((p) => p.is_host)?.username ?? 'the host'
  );

  const playerCount = $derived(room.players.length);
  const canStart = $derived(playerCount >= minPlayers);
  const slotsToShow = $derived(Math.max(maxPlayers, playerCount));
  const emptySlots = $derived(
    Array.from({ length: Math.max(0, slotsToShow - playerCount) })
  );
</script>

<div
  class="w-full max-w-6xl mx-auto px-6 py-6 flex flex-col gap-6"
  use:reveal
>
  <!-- ═══════════════════════════════════════════════════════════════
       HEADER — game type title + inline details (rounds, timings,
       player range). These are read-only for F3 v1, so surfacing them
       directly is clearer than hiding them behind a modal.
       ═══════════════════════════════════════════════════════════════ -->
  <div class="flex flex-col items-center gap-3 text-center">
    <h1 class="inline-flex items-center gap-2.5 text-2xl sm:text-3xl md:text-4xl font-bold text-brand-text">
      <Sparkles size={24} strokeWidth={2.5} class="shrink-0" />
      <span>{gameName}</span>
    </h1>

    {#if gameDescription}
      <p class="text-sm font-semibold text-brand-text-muted max-w-2xl">
        {gameDescription}
      </p>
    {/if}

    <div class="flex flex-wrap items-center justify-center gap-2">
      <span
        class="inline-flex items-center gap-1.5 h-9 px-4 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-xs font-bold tabular-nums"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
      >
        <ListChecks size={14} strokeWidth={2.5} />
        {roundCount} rounds
      </span>
      <span
        class="inline-flex items-center gap-1.5 h-9 px-4 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-xs font-bold tabular-nums"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
      >
        <Clock size={14} strokeWidth={2.5} />
        {submitDuration}s submit · {voteDuration}s vote
      </span>
      <span
        class="inline-flex items-center gap-1.5 h-9 px-4 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-xs font-bold tabular-nums"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
      >
        <Users size={14} strokeWidth={2.5} />
        {minPlayers}–{maxPlayers} players
      </span>
    </div>
  </div>

  <!-- ═══════════════════════════════════════════════════════════════
       START CTA — Cancel sits directly next to Start as a grouped pair.
       ═══════════════════════════════════════════════════════════════ -->
  <div class="flex flex-col items-center gap-2">
    <div class="flex items-center gap-3">
      <EndRoomButton />
      {#if isHost}
        <button
          use:pressPhysics={'dark'}
          type="button"
          onclick={startGame}
          disabled={!canStart}
          class="h-14 px-10 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold text-lg disabled:opacity-50 transition-colors cursor-pointer inline-flex items-center justify-center gap-2"
          style="box-shadow: 0 5px 0 rgba(0,0,0,0.22);"
        >
          <Play size={20} strokeWidth={2.5} />
          Start game
        </button>
      {:else}
        <div
          class="h-14 px-8 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-surface font-bold text-sm text-brand-text-mid inline-flex items-center justify-center gap-2"
          style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
        >
          Waiting for <span class="text-brand-text">{hostName}</span>…
        </div>
      {/if}
    </div>
    {#if isHost && !canStart}
      <p class="text-[0.65rem] font-semibold text-brand-text-muted">
        Need {minPlayers}+ players — you have {playerCount}
      </p>
    {/if}
  </div>

  <!-- ═══════════════════════════════════════════════════════════════
       MAIN BODY — two columns on lg+: left = code share, right = players.
       ═══════════════════════════════════════════════════════════════ -->
  <div class="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)] gap-5 items-start">

    <!-- LEFT: Room-code share card -->
    <section
      class="rounded-3xl border-[2.5px] border-brand-border-heavy bg-brand-surface px-6 py-6 flex flex-col items-center gap-5"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
    >
      <p class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
        Room code
      </p>

      <div class="font-mono font-bold tracking-[0.15em] text-brand-text select-all leading-none text-4xl">
        {room.code ?? '----'}
      </div>

      <p class="text-xs font-semibold text-brand-text-muted text-center max-w-xs">
        Share the 4-letter code — or tap the link to let friends join in one tap.
      </p>

      <div class="flex flex-wrap items-center justify-center gap-3 w-full">
        <button
          use:pressPhysics={'ghost'}
          type="button"
          onclick={copyCode}
          class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text text-sm font-bold inline-flex items-center justify-center gap-2 cursor-pointer"
        >
          {#if codeCopied}
            <CheckCircle size={16} strokeWidth={2.5} />
            Copied!
          {:else}
            <Copy size={16} strokeWidth={2.5} />
            Copy code
          {/if}
        </button>
        <button
          use:pressPhysics={'dark'}
          type="button"
          onclick={copyInviteLink}
          class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold inline-flex items-center justify-center gap-2 cursor-pointer"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.18);"
        >
          {#if linkCopied}
            <CheckCircle size={16} strokeWidth={2.5} />
            Link copied!
          {:else}
            <Link2 size={16} strokeWidth={2.5} />
            Copy invite link
          {/if}
        </button>
      </div>
    </section>

    <!-- RIGHT: Players grid -->
    <section
      class="rounded-3xl border-[2.5px] border-brand-border-heavy bg-brand-surface px-5 py-5 flex flex-col gap-4"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
    >
      <div class="flex items-center justify-between gap-3">
        <div class="inline-flex items-center gap-2">
          <Users size={16} strokeWidth={2.5} />
          <h2 class="text-[0.7rem] font-bold uppercase tracking-[0.2em] text-brand-text-mid">
            Players
          </h2>
        </div>
        <span class="text-xs font-bold tabular-nums text-brand-text-muted">
          {playerCount} / {maxPlayers}
        </span>
      </div>

      <ul class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        {#each room.players as player, i (player.user_id)}
          <li
            class="flex items-center gap-3 rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-3 py-3"
            style="box-shadow: 0 4px 0 rgba(0,0,0,0.1);"
          >
            <span
              class="h-11 w-11 shrink-0 rounded-full border-[2.5px] border-brand-border-heavy flex items-center justify-center text-xs font-bold text-white"
              style="background: {AVATAR_COLORS[i % AVATAR_COLORS.length]};"
            >
              {player.username.slice(0, 2).toUpperCase()}
            </span>
            <div class="flex-1 min-w-0 flex flex-col gap-0.5 text-left">
              <span class="truncate text-sm font-bold text-brand-text">
                {player.username}
              </span>
              <div class="flex items-center gap-1.5 text-[0.6rem] font-bold uppercase tracking-[0.1em] text-brand-text-muted">
                {#if player.is_host}
                  <span class="inline-flex items-center gap-1">
                    <Sparkles size={10} strokeWidth={2.5} />
                    Host
                  </span>
                {:else if player.is_guest}
                  <span>Guest</span>
                {:else}
                  <span>Player</span>
                {/if}
              </div>
            </div>
            {#if player.connected}
              <span
                class="h-2 w-2 rounded-full shrink-0"
                style="background: var(--brand-success, #4ade80);"
                title="Connected"
              ></span>
            {/if}
          </li>
        {/each}

        {#each emptySlots as _}
          <li
            class="flex items-center gap-3 rounded-2xl border-[2.5px] border-dashed border-brand-border px-3 py-3 text-brand-text-muted"
          >
            <span class="h-11 w-11 shrink-0 rounded-full border-[2.5px] border-dashed border-brand-border flex items-center justify-center">
              <Users size={16} strokeWidth={2.5} />
            </span>
            <span class="text-[0.65rem] font-bold uppercase tracking-[0.1em]">
              Waiting…
            </span>
          </li>
        {/each}
      </ul>
    </section>
  </div>

</div>
