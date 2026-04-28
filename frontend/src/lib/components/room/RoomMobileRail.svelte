<script lang="ts">
  import { room } from '$lib/state/room.svelte';
  import RoomRail from './RoomRail.svelte';
  import { ChevronDown, Users } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';

  // Mobile counterpart to the desktop right-rail. Renders as a slim pill
  // anchored beneath the (sticky) RoomHeader and drops down a panel with
  // the full roster + readiness + leaderboard. Keeping it at the top —
  // rather than a permanent bottom sheet — frees the lower viewport for
  // the composer/textarea and the sticky vote bar during play.
  let open = $state(false);

  const playerCount = $derived(room.players.length);
  const progress = $derived.by(() => {
    if (room.phase === 'submitting') {
      return room.submittedPlayerIds.size + room.skippedSubmitIds.size;
    }
    if (room.phase === 'voting') {
      return room.votedPlayerIds.size + room.skippedVoteIds.size;
    }
    return null;
  });

  function close() {
    open = false;
  }

  function handleKey(e: KeyboardEvent) {
    if (open && e.key === 'Escape') close();
  }
</script>

<svelte:window onkeydown={handleKey} />

<div class="lg:hidden relative z-10">
  <button
    type="button"
    onclick={() => (open = !open)}
    aria-expanded={open}
    aria-controls="room-mobile-rail-panel"
    class="w-full inline-flex items-center justify-between gap-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-2.5 text-sm font-bold cursor-pointer"
    style="box-shadow: 0 4px 0 rgba(0,0,0,0.10);"
  >
    <span class="inline-flex items-center gap-2 min-w-0">
      <Users size={16} strokeWidth={2.5} />
      <span class="truncate">
        {#if room.phase === 'results'}
          {m.room_leaderboard()}
        {:else}
          {m.room_players_title()}
        {/if}
      </span>
      <span class="font-mono tabular-nums text-brand-text-mid">{playerCount}</span>
      {#if progress !== null}
        <span class="text-xs font-bold text-brand-text-muted">· {progress}/{playerCount}</span>
      {/if}
    </span>
    <ChevronDown
      size={16}
      strokeWidth={2.5}
      class="transition-transform duration-200 {open ? 'rotate-180' : ''}"
    />
  </button>

  {#if open}
    <!-- Backdrop swallows taps outside the panel and dismisses it. -->
    <button
      type="button"
      aria-label={m.room_close_dialog_aria()}
      onclick={close}
      class="fixed inset-0 z-20 bg-black/0 cursor-default"
    ></button>
    <div
      id="room-mobile-rail-panel"
      class="absolute top-full left-0 right-0 mt-2 z-30 rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-white p-3 max-h-[70dvh] overflow-y-auto"
      style="box-shadow: 0 8px 0 rgba(0,0,0,0.12);"
    >
      <RoomRail variant="sheet" />
    </div>
  {/if}
</div>
