<script lang="ts">
  import { page } from '$app/stores';
  import { user } from '$lib/state/user.svelte';
  import { room } from '$lib/state/room.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { guest } from '$lib/state/guest.svelte';
  import { stage } from '$lib/motion/stage.svelte';
  import MemeCaptionSubmitForm from '$lib/games/meme-freestyle/SubmitForm.svelte';
  import MemeCaptionVoteForm from '$lib/games/meme-freestyle/VoteForm.svelte';
  import MemeCaptionResultsView from '$lib/games/meme-freestyle/ResultsView.svelte';
  import MemeVoteSubmitForm from '$lib/games/meme-showdown/SubmitForm.svelte';
  import MemeVoteVoteForm from '$lib/games/meme-showdown/VoteForm.svelte';
  import MemeVoteResultsView from '$lib/games/meme-showdown/ResultsView.svelte';
  import WaitingStage from '$lib/components/room/WaitingStage.svelte';
  import EndStage from '$lib/components/room/EndStage.svelte';
  import RoomHeader from '$lib/components/room/RoomHeader.svelte';
  import RoomRail from '$lib/components/room/RoomRail.svelte';

  // Player identity — registered users match via user.id; guest identity
  // is recovered from sessionStorage (set by the /join flow in F4).
  const playerId = $derived.by(() => {
    if (user.id) return user.id;
    return room.code ? guest.playerId(room.code) : null;
  });

  const isHost = $derived(
    playerId !== null && room.hostUserId === playerId
  );

  // host_paced is a room-creation setting; it doesn't change during gameplay.
  // Read it from the SSR-loaded page data (room.config) rather than adding it
  // to RoomState to avoid over-widening the reactive singleton.
  const hostPaced = $derived(($page.data as any)?.room?.config?.host_paced ?? false);
  const totalRounds = $derived(
    (($page.data as any)?.room?.config?.round_count as number | undefined) ?? null
  );

  // Per-game-type component trios. Each plugin supplies a Submit/Vote/Results
  // component that understands the slug's payload shape. Keyed by slug so new
  // game types slot in without touching the room shell's render logic.
  const plugins = {
    'meme-freestyle': {
      Submit: MemeCaptionSubmitForm,
      Vote: MemeCaptionVoteForm,
      Results: MemeCaptionResultsView,
    },
    'meme-showdown': {
      Submit: MemeVoteSubmitForm,
      Vote: MemeVoteVoteForm,
      Results: MemeVoteResultsView,
    },
  } as const;

  const plugin = $derived(
    plugins[(room.gameType?.slug ?? 'meme-freestyle') as keyof typeof plugins] ?? plugins['meme-freestyle']
  );

  let prefersReducedMotion = $state(false);

  $effect(() => {
    if (typeof window !== 'undefined') {
      prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    }
  });

  $effect(() => {
    stage.sync(room.phase);
  });

  function nextRound() {
    ws.send('next_round');
  }

  // Table chrome (sticky header + felt table + right rail) only makes sense
  // during active play. Lobby and end stages render against the bare page.
  const showTable = $derived(
    (room.state === 'playing' || room.state === 'finished') &&
    (stage.displayPhase === 'submitting' ||
      stage.displayPhase === 'voting' ||
      stage.displayPhase === 'results')
  );
</script>

<!-- Countdown overlay. Uses global class so all styles land reliably
     (inline style + action combos were fragile on Firefox). -->
{#if room.countdown !== null}
  <div class="countdown-overlay" aria-live="assertive">
    <div class="countdown-number" class:no-bounce={prefersReducedMotion}>
      {room.countdown > 0 ? room.countdown : 'GO!'}
    </div>
  </div>
{/if}

{#if showTable}
  <!-- Active-play shell: sticky header + felt table + right rail.
       `content-blur` applies `filter: blur()` to the whole shell while
       the countdown is running — more reliable across browsers than
       `backdrop-filter` on the overlay (Firefox drops the latter when
       any ancestor has a `transform`, which `.stage-wrap` always does). -->
  <div
    class="max-w-[1280px] mx-auto px-4 md:px-6 pt-4 pb-10 flex flex-col gap-5"
    class:content-blur={room.countdown !== null}
  >
    <RoomHeader {totalRounds} />

    <div class="grid gap-5 lg:grid-cols-[1fr_288px]">
      <section class="table-panel">
        <div class="stage-wrap relative z-[1]" class:hidden={!stage.visible}>
          {#if stage.displayPhase === 'submitting' && room.currentRound}
            {@const Submit = plugin.Submit}
            <Submit round={room.currentRound} />
          {:else if stage.displayPhase === 'voting'}
            {@const Vote = plugin.Vote}
            <Vote submissions={room.submissions} round={room.currentRound} />
          {:else if stage.displayPhase === 'results'}
            {@const Results = plugin.Results}
            <Results
              submissions={room.submissions}
              leaderboard={room.leaderboard}
              {isHost}
              {hostPaced}
              onNextRound={nextRound}
            />
          {/if}
        </div>
      </section>

      <div class="hidden lg:block">
        <RoomRail />
      </div>
    </div>
  </div>
{:else}
  <!-- Lobby / end / idle — pages render their own chrome -->
  <div class="p-6 flex flex-col gap-6" class:content-blur={room.countdown !== null}>
    <div class="stage-wrap" class:hidden={!stage.visible}>
      {#if stage.displayPhase === 'idle' && room.state === 'lobby'}
        <WaitingStage {isHost} />
      {:else if room.state === 'finished'}
        <EndStage />
      {/if}
    </div>
  </div>
{/if}
