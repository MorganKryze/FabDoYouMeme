<script lang="ts">
  import { page } from '$app/stores';
  import { invalidateAll } from '$app/navigation';
  import { room } from '$lib/state/room.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { roomsApi } from '$lib/api/rooms';
  import {
    Play,
    Clock,
    ListChecks,
    Copy,
    Link2,
    CheckCircle,
    Users,
    Sparkles,
    Settings,
    X
  } from '$lib/icons';
  import type { GameType, Player, Room, RoomConfig } from '$lib/api/types';
  import { fade, scale } from 'svelte/transition';
  import { backOut } from 'svelte/easing';
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
  // Hand-size is opt-in per game type (manifest must declare bounds). When
  // the manifest doesn't, both the server and this UI treat hand_size as
  // absent — the stepper never renders.
  const handSizeMax = $derived(gameType?.config.max_hand_size ?? 0);
  const handSize = $derived(
    pageRoom?.config.hand_size ??
      gameType?.config.default_hand_size ??
      0
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
  // Dynamic staging: always surface one empty "Waiting…" slot so joiners can
  // see the room has room, and round up to pairs so the 2-column grid never
  // leaves an orphan. Capped at maxPlayers — once the room is full, no empty
  // slot is shown.
  const slotsToShow = $derived(
    Math.min(maxPlayers, Math.max(2, Math.ceil((playerCount + 1) / 2) * 2))
  );
  const emptySlots = $derived(
    Array.from({ length: Math.max(0, slotsToShow - playerCount) })
  );

  // ─── Host-only room settings ────────────────────────────────────────
  // Backed by PATCH /api/rooms/{code}/config (lobby only, host only).
  // Inputs hold a local draft that re-syncs from the server room on every
  // load/invalidate — the effect below is the single source of truth for
  // "what the server currently thinks the config is".
  let settingsRoundCount = $state(0);
  let settingsSubmitDuration = $state(0);
  let settingsVoteDuration = $state(0);
  let settingsHostPaced = $state(false);
  let settingsJokerCount = $state(0);
  let settingsAllowSkipVote = $state(true);
  let settingsHandSize = $state(0);
  let settingsSaving = $state(false);

  $effect(() => {
    settingsRoundCount = roundCount;
    settingsSubmitDuration = submitDuration;
    settingsVoteDuration = voteDuration;
    settingsHostPaced = pageRoom?.config.host_paced ?? false;
    settingsJokerCount = pageRoom?.config.joker_count ?? Math.ceil(roundCount / 5);
    settingsAllowSkipVote = pageRoom?.config.allow_skip_vote ?? true;
    settingsHandSize = handSize;
  });

  // Bounds come from the game type manifest (backend/internal/game/types/
  // <slug>/manifest.yaml), exposed here via gameType.config. We never
  // hardcode these in the UI — a host editing meme-freestyle sees meme
  // bounds, match-the-meme sees match bounds, etc. The server enforces
  // the same bounds on every PATCH, so the clamps below are just UX.
  const roundCountMin = $derived(gameType?.config.min_round_count ?? 1);
  const roundCountMax = $derived(gameType?.config.max_round_count ?? 50);
  const submitMin = $derived(gameType?.config.min_round_duration_seconds ?? 15);
  const submitMax = $derived(gameType?.config.max_round_duration_seconds ?? 300);
  const voteMin = $derived(gameType?.config.min_voting_duration_seconds ?? 10);
  const voteMax = $derived(gameType?.config.max_voting_duration_seconds ?? 120);
  const handSizeMin = $derived(gameType?.config.min_hand_size ?? 1);
  const handSizeMaxBound = $derived(gameType?.config.max_hand_size ?? 1);

  // The server merges this partial patch over the room's current config
  // and re-validates against the manifest bounds, so we only send the
  // field that changed.
  async function saveSettings(patch: Partial<RoomConfig>) {
    if (!room.code) return;
    settingsSaving = true;
    try {
      await roomsApi.updateConfig(room.code, patch);
      await invalidateAll();
    } catch {
      toast.show('Could not save settings', 'error');
    } finally {
      settingsSaving = false;
    }
  }

  function clamp(v: number, lo: number, hi: number): number {
    return Math.max(lo, Math.min(hi, Math.round(v || 0)));
  }

  function commitRoundCount() {
    const v = clamp(settingsRoundCount, roundCountMin, roundCountMax);
    settingsRoundCount = v;
    if (v !== roundCount) void saveSettings({ round_count: v });
  }

  function commitSubmitDuration() {
    const v = clamp(settingsSubmitDuration, submitMin, submitMax);
    settingsSubmitDuration = v;
    if (v !== submitDuration) void saveSettings({ round_duration_seconds: v });
  }

  function commitVoteDuration() {
    const v = clamp(settingsVoteDuration, voteMin, voteMax);
    settingsVoteDuration = v;
    if (v !== voteDuration) void saveSettings({ voting_duration_seconds: v });
  }

  function toggleHostPaced(e: Event) {
    const checked = (e.currentTarget as HTMLInputElement).checked;
    settingsHostPaced = checked;
    void saveSettings({ host_paced: checked });
  }

  function commitJokerCount() {
    const v = clamp(settingsJokerCount, 0, settingsRoundCount);
    settingsJokerCount = v;
    if (v !== (pageRoom?.config.joker_count ?? Math.ceil(roundCount / 5))) {
      void saveSettings({ joker_count: v });
    }
  }

  function toggleAllowSkipVote(e: Event) {
    const checked = (e.currentTarget as HTMLInputElement).checked;
    settingsAllowSkipVote = checked;
    void saveSettings({ allow_skip_vote: checked });
  }

  function commitHandSize() {
    const v = clamp(settingsHandSize, handSizeMin, handSizeMaxBound);
    settingsHandSize = v;
    if (v !== handSize) void saveSettings({ hand_size: v });
  }

  // ─── Kick flow (host-only, lobby-only) ──────────────────────────────
  let kickTarget = $state<{ id: string; name: string; isGuest: boolean } | null>(null);
  let kickPending = $state(false);
  let kickError = $state<string | null>(null);

  function openKick(player: Player) {
    kickTarget = { id: player.user_id, name: player.username, isGuest: !!player.is_guest };
    kickError = null;
  }

  function closeKick() {
    if (kickPending) return;
    kickTarget = null;
  }

  async function confirmKick() {
    if (!room.code || !kickTarget || kickPending) return;
    kickPending = true;
    kickError = null;
    try {
      await roomsApi.kick(
        room.code,
        kickTarget.isGuest ? { guestPlayerId: kickTarget.id } : { userId: kickTarget.id }
      );
      toast.show(`${kickTarget.name} was removed`, 'success');
      kickTarget = null;
      await invalidateAll();
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Could not remove player';
      kickError = message;
      toast.show(message, 'error');
    } finally {
      kickPending = false;
    }
  }

  function handleKickKey(e: KeyboardEvent) {
    if (kickTarget && e.key === 'Escape') closeKick();
  }
</script>

<style>
  .stepper {
    display: inline-flex;
    align-items: center;
    justify-content: space-between;
    height: 2.5rem;
    padding: 0 0.25rem;
    border-radius: 9999px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white);
    gap: 0.25rem;
  }
  .stepper button {
    flex-shrink: 0;
    height: 1.75rem;
    width: 1.75rem;
    border-radius: 9999px;
    background: var(--brand-surface);
    color: var(--brand-text);
    font-weight: 700;
    font-size: 1rem;
    line-height: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition:
      background-color 150ms ease,
      opacity 150ms ease,
      transform 80ms ease;
  }
  .stepper button:hover:not(:disabled) {
    background: var(--brand-border);
  }
  .stepper button:active:not(:disabled) {
    transform: scale(0.92);
  }
  .stepper button:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }
  .stepper input {
    flex: 1;
    min-width: 0;
    background: transparent;
    text-align: center;
    font-size: 0.9rem;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    color: var(--brand-text);
    outline: none;
    border: none;
    padding: 0;
    -moz-appearance: textfield;
    appearance: textfield;
  }
  .stepper input::-webkit-inner-spin-button,
  .stepper input::-webkit-outer-spin-button {
    -webkit-appearance: none;
    margin: 0;
  }

  .brand-toggle {
    position: relative;
    display: inline-block;
    width: 44px;
    height: 24px;
    flex-shrink: 0;
    margin-top: 2px;
    border-radius: 9999px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white);
    cursor: pointer;
    transition: background-color 180ms ease;
  }
  .brand-toggle.is-on {
    background: var(--brand-text);
  }
  .brand-toggle input {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    margin: 0;
    padding: 0;
    opacity: 0;
    cursor: pointer;
  }
  .brand-toggle .thumb {
    position: absolute;
    top: 50%;
    left: 2px;
    width: 14px;
    height: 14px;
    border-radius: 9999px;
    background: var(--brand-text);
    transform: translateY(-50%);
    transition:
      left 180ms cubic-bezier(0.4, 0, 0.2, 1),
      background-color 180ms ease;
    pointer-events: none;
  }
  .brand-toggle.is-on .thumb {
    left: calc(100% - 14px - 2px);
    background: var(--brand-white);
  }

  .status-chip {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.25rem 0.6rem 0.25rem 0.5rem;
    border-radius: 9999px;
    border: 2px solid var(--brand-border-heavy);
    font-size: 0.625rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.12em;
    line-height: 1;
    white-space: nowrap;
    box-shadow: 0 2px 0 rgba(0, 0, 0, 0.08);
  }
  .status-chip .status-dot {
    width: 0.45rem;
    height: 0.45rem;
    border-radius: 9999px;
    flex-shrink: 0;
  }
  .status-chip.is-online {
    background: var(--brand-success-soft);
    border-color: var(--brand-success);
    color: var(--brand-success);
  }
  .status-chip.is-online .status-dot {
    background: var(--brand-success);
    box-shadow: 0 0 0 3px rgba(47, 133, 102, 0.18);
    animation: status-pulse 1.8s ease-in-out infinite;
  }
  .status-chip.is-away {
    background: var(--brand-grad-1);
    border-color: var(--brand-accent);
    color: var(--brand-text);
  }
  .status-chip.is-away .status-dot {
    background: var(--brand-accent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--brand-accent) 22%, transparent);
  }

  @keyframes status-pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.65; transform: scale(0.85); }
  }
  @media (prefers-reduced-motion: reduce) {
    .status-chip.is-online .status-dot { animation: none; }
  }

  .readiness-chip {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.35rem 0.75rem 0.35rem 0.6rem;
    border-radius: 9999px;
    border: 2px solid var(--brand-border-heavy);
    font-size: 0.65rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.14em;
    line-height: 1;
    white-space: nowrap;
    box-shadow: 0 2px 0 rgba(0, 0, 0, 0.08);
  }
  .readiness-dot {
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 9999px;
    flex-shrink: 0;
  }
  .readiness-sep {
    opacity: 0.4;
  }
  .readiness-count {
    font-variant-numeric: tabular-nums;
  }
  .readiness-chip.is-ready {
    background: var(--brand-success-soft);
    border-color: var(--brand-success);
    color: var(--brand-success);
  }
  .readiness-chip.is-ready .readiness-dot {
    background: var(--brand-success);
    box-shadow: 0 0 0 3px rgba(47, 133, 102, 0.18);
    animation: status-pulse 1.8s ease-in-out infinite;
  }
  .readiness-chip.is-waiting {
    background: var(--brand-grad-1);
    border-color: var(--brand-accent);
    color: var(--brand-text);
  }
  .readiness-chip.is-waiting .readiness-dot {
    background: var(--brand-accent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--brand-accent) 22%, transparent);
    animation: status-pulse 1.4s ease-in-out infinite;
  }
  @media (prefers-reduced-motion: reduce) {
    .readiness-chip .readiness-dot { animation: none; }
  }
</style>

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
  </div>

  <!-- ═══════════════════════════════════════════════════════════════
       ROOM SETTINGS — host-only. Edits apply via PATCH /rooms/{code}/config
       while the room is still in lobby. Future settings get added here.
       ═══════════════════════════════════════════════════════════════ -->
  {#if isHost}
    <section
      class="rounded-3xl border-[2.5px] border-brand-border-heavy bg-brand-surface px-5 py-5 flex flex-col gap-4"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
    >
      <div class="flex items-center justify-between gap-3">
        <div class="inline-flex items-center gap-2">
          <Settings size={16} strokeWidth={2.5} />
          <h2 class="text-[0.7rem] font-bold uppercase tracking-[0.2em] text-brand-text-mid">
            Room settings
          </h2>
        </div>
        {#if settingsSaving}
          <span class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
            Saving…
          </span>
        {/if}
      </div>

      <div
        class={handSizeMax > 0
          ? 'grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3'
          : 'grid grid-cols-2 sm:grid-cols-4 gap-3'}
      >
        <div class="flex flex-col gap-1.5">
          <span class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
            Rounds
          </span>
          <div class="stepper">
            <button
              type="button"
              aria-label="Decrease rounds"
              disabled={settingsRoundCount <= roundCountMin}
              onclick={() => { settingsRoundCount = Math.max(roundCountMin, settingsRoundCount - 1); commitRoundCount(); }}
            >−</button>
            <input
              type="number"
              min={roundCountMin}
              max={roundCountMax}
              bind:value={settingsRoundCount}
              onblur={commitRoundCount}
              onchange={commitRoundCount}
              aria-label="Rounds"
            />
            <button
              type="button"
              aria-label="Increase rounds"
              disabled={settingsRoundCount >= roundCountMax}
              onclick={() => { settingsRoundCount = Math.min(roundCountMax, settingsRoundCount + 1); commitRoundCount(); }}
            >+</button>
          </div>
          <span class="text-[0.6rem] font-semibold text-brand-text-muted tabular-nums">
            {roundCountMin}–{roundCountMax}
          </span>
        </div>
        <div class="flex flex-col gap-1.5">
          <span class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
            Submit time (s)
          </span>
          <div class="stepper">
            <button
              type="button"
              aria-label="Decrease submit time"
              disabled={settingsSubmitDuration <= submitMin}
              onclick={() => { settingsSubmitDuration = Math.max(submitMin, settingsSubmitDuration - 5); commitSubmitDuration(); }}
            >−</button>
            <input
              type="number"
              min={submitMin}
              max={submitMax}
              bind:value={settingsSubmitDuration}
              onblur={commitSubmitDuration}
              onchange={commitSubmitDuration}
              aria-label="Submit time in seconds"
            />
            <button
              type="button"
              aria-label="Increase submit time"
              disabled={settingsSubmitDuration >= submitMax}
              onclick={() => { settingsSubmitDuration = Math.min(submitMax, settingsSubmitDuration + 5); commitSubmitDuration(); }}
            >+</button>
          </div>
          <span class="text-[0.6rem] font-semibold text-brand-text-muted tabular-nums">
            {submitMin}–{submitMax}s
          </span>
        </div>
        <div class="flex flex-col gap-1.5">
          <span class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
            Vote time (s)
          </span>
          <div class="stepper">
            <button
              type="button"
              aria-label="Decrease vote time"
              disabled={settingsVoteDuration <= voteMin}
              onclick={() => { settingsVoteDuration = Math.max(voteMin, settingsVoteDuration - 5); commitVoteDuration(); }}
            >−</button>
            <input
              type="number"
              min={voteMin}
              max={voteMax}
              bind:value={settingsVoteDuration}
              onblur={commitVoteDuration}
              onchange={commitVoteDuration}
              aria-label="Vote time in seconds"
            />
            <button
              type="button"
              aria-label="Increase vote time"
              disabled={settingsVoteDuration >= voteMax}
              onclick={() => { settingsVoteDuration = Math.min(voteMax, settingsVoteDuration + 5); commitVoteDuration(); }}
            >+</button>
          </div>
          <span class="text-[0.6rem] font-semibold text-brand-text-muted tabular-nums">
            {voteMin}–{voteMax}s
          </span>
        </div>
        <div class="flex flex-col gap-1.5">
          <span class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
            Jokers per player
          </span>
          <div class="stepper">
            <button
              type="button"
              aria-label="Decrease jokers"
              disabled={settingsJokerCount <= 0}
              onclick={() => { settingsJokerCount = Math.max(0, settingsJokerCount - 1); commitJokerCount(); }}
            >−</button>
            <input
              type="number"
              min={0}
              max={settingsRoundCount}
              bind:value={settingsJokerCount}
              onblur={commitJokerCount}
              onchange={commitJokerCount}
              aria-label="Jokers per player"
            />
            <button
              type="button"
              aria-label="Increase jokers"
              disabled={settingsJokerCount >= settingsRoundCount}
              onclick={() => { settingsJokerCount = Math.min(settingsRoundCount, settingsJokerCount + 1); commitJokerCount(); }}
            >+</button>
          </div>
          <span class="text-[0.6rem] font-semibold text-brand-text-muted tabular-nums">
            0 disables · rec. {Math.ceil(settingsRoundCount / 5)}
          </span>
        </div>
        {#if handSizeMax > 0}
          <div class="flex flex-col gap-1.5">
            <span class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
              Hand size
            </span>
            <div class="stepper">
              <button
                type="button"
                aria-label="Decrease hand size"
                disabled={settingsHandSize <= handSizeMin}
                onclick={() => { settingsHandSize = Math.max(handSizeMin, settingsHandSize - 1); commitHandSize(); }}
              >−</button>
              <input
                type="number"
                min={handSizeMin}
                max={handSizeMaxBound}
                bind:value={settingsHandSize}
                onblur={commitHandSize}
                onchange={commitHandSize}
                aria-label="Hand size"
              />
              <button
                type="button"
                aria-label="Increase hand size"
                disabled={settingsHandSize >= handSizeMaxBound}
                onclick={() => { settingsHandSize = Math.min(handSizeMaxBound, settingsHandSize + 1); commitHandSize(); }}
              >+</button>
            </div>
            <span class="text-[0.6rem] font-semibold text-brand-text-muted tabular-nums">
              {handSizeMin}–{handSizeMaxBound} cards
            </span>
          </div>
        {/if}
      </div>

      <label class="flex items-start gap-3 cursor-pointer rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3">
        <span class="brand-toggle" class:is-on={settingsHostPaced}>
          <input
            type="checkbox"
            checked={settingsHostPaced}
            onchange={toggleHostPaced}
            aria-label="Host-paced rounds"
          />
          <span class="thumb"></span>
        </span>
        <span class="flex flex-col gap-0.5">
          <span class="text-sm font-bold text-brand-text">Host-paced rounds</span>
          <span class="text-xs font-semibold text-brand-text-muted">
            You advance each round manually instead of auto-advancing.
          </span>
        </span>
      </label>

      <label class="flex items-start gap-3 cursor-pointer rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3">
        <span class="brand-toggle" class:is-on={settingsAllowSkipVote}>
          <input
            type="checkbox"
            checked={settingsAllowSkipVote}
            onchange={toggleAllowSkipVote}
            aria-label="Allow skip vote"
          />
          <span class="thumb"></span>
        </span>
        <span class="flex flex-col gap-0.5">
          <span class="text-sm font-bold text-brand-text">Allow skip button</span>
          <span class="text-xs font-semibold text-brand-text-muted">
            Players can skip voting if no caption landed. If everyone skips, the round ends with no points.
          </span>
        </span>
      </label>
    </section>
  {/if}

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
        <span
          class="readiness-chip"
          class:is-ready={canStart}
          class:is-waiting={!canStart}
        >
          <span class="readiness-dot"></span>
          <span class="readiness-count tabular-nums">{playerCount} / {maxPlayers}</span>
          <span class="readiness-sep" aria-hidden="true">·</span>
          {#if canStart}
            Ready
          {:else}
            Need {minPlayers - playerCount} more
          {/if}
        </span>
      </div>

      <ul class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        {#each room.players as player, i (player.user_id)}
          {@const isOnline = player.connected ?? true}
          <li
            class="relative flex items-center gap-3 rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-3 py-3"
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
            <span
              class="status-chip shrink-0"
              class:is-online={isOnline}
              class:is-away={!isOnline}
              aria-label={isOnline ? 'Online' : 'Away'}
            >
              <span class="status-dot"></span>
              {isOnline ? 'Online' : 'Away'}
            </span>
            {#if isHost && !player.is_host}
              <button
                type="button"
                use:pressPhysics={'ghost'}
                onclick={() => openKick(player)}
                aria-label={`Remove ${player.username}`}
                title={`Remove ${player.username}`}
                class="absolute -top-2 -right-2 h-7 w-7 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text-mid hover:text-red-600 hover:border-red-600 inline-flex items-center justify-center cursor-pointer transition-colors"
                style="box-shadow: 0 2px 0 rgba(0,0,0,0.1);"
              >
                <X size={12} strokeWidth={3} />
              </button>
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

<svelte:window onkeydown={handleKickKey} />

{#if kickTarget}
  <div
    class="fixed inset-0 z-50 bg-black/0"
    aria-hidden="true"
    transition:fade={{ duration: 120 }}
  >
    <button
      type="button"
      aria-label="Close dialog"
      class="absolute inset-0 w-full h-full cursor-default"
      onclick={closeKick}
    ></button>
    <div class="absolute inset-0 flex items-center justify-center p-4 pointer-events-none">
      <div
        class="w-[min(26rem,100%)] bg-brand-white border-[2.5px] border-brand-border-heavy rounded-3xl p-6 flex flex-col gap-4 pointer-events-auto"
        style="box-shadow: 0 10px 0 rgba(0,0,0,0.15);"
        role="dialog"
        tabindex="-1"
        aria-modal="true"
        aria-labelledby="kick-title"
        transition:scale={{ duration: 180, start: 0.85, easing: backOut }}
      >
        <h2 id="kick-title" class="text-xl font-bold">Remove {kickTarget.name}?</h2>
        <p class="text-sm font-semibold text-brand-text-mid">
          They'll be disconnected immediately and won't be able to rejoin this room.
        </p>
        {#if kickError}
          <p class="text-sm font-semibold text-red-600">{kickError}</p>
        {/if}
        <div class="flex gap-3 justify-end mt-2">
          <button
            use:pressPhysics={'ghost'}
            type="button"
            onclick={closeKick}
            disabled={kickPending}
            class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-sm font-bold disabled:opacity-50 cursor-pointer"
          >
            Cancel
          </button>
          <button
            use:pressPhysics={'dark'}
            type="button"
            onclick={confirmKick}
            disabled={kickPending}
            class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-red-600 text-white text-sm font-bold disabled:opacity-50 cursor-pointer"
          >
            {kickPending ? 'Removing…' : 'Remove player'}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
