<script lang="ts">
  import { room } from '$lib/state/room.svelte';
  import { user } from '$lib/state/user.svelte';
  import { guest } from '$lib/state/guest.svelte';

  // Player identity — same logic as /rooms/[code]/+page.svelte. Used to
  // highlight "you" in the roster.
  const selfId = $derived.by(() => {
    if (user.id) return user.id;
    return room.code ? guest.playerId(room.code) : null;
  });

  // During submit/vote we show the roster from room.players (connection +
  // guest/host flags), sorted with host first then alphabetically. During
  // results the leaderboard takes over (ranked, with scores).
  const rosterRows = $derived.by(() => {
    if (room.phase === 'results' && room.leaderboard.length > 0) {
      return room.leaderboard.map((e) => ({
        id: e.player_id,
        name: e.display_name,
        score: e.score,
        rank: e.rank,
        connected: true,
        isHost: false,
        hasSubmitted: false,
        hasSkippedSubmit: false,
        hasVoted: false,
        hasSkippedVote: false,
      }));
    }
    return [...room.players]
      .sort((a, b) => {
        if (a.is_host && !b.is_host) return -1;
        if (b.is_host && !a.is_host) return 1;
        return a.username.localeCompare(b.username);
      })
      .map((p) => ({
        id: p.user_id,
        name: p.username,
        score: null as number | null,
        rank: null as number | null,
        connected: p.connected ?? true,
        isHost: p.is_host ?? false,
        hasSubmitted: room.submittedPlayerIds.has(p.user_id),
        hasSkippedSubmit: room.skippedSubmitIds.has(p.user_id),
        hasVoted: room.votedPlayerIds.has(p.user_id),
        hasSkippedVote: room.skippedVoteIds.has(p.user_id),
      }));
  });

  // Aggregate progress — honest counts from state we already keep.
  const progress = $derived.by(() => {
    const total = room.players.length;
    if (room.phase === 'submitting') {
      return {
        label: 'Submitted',
        done: room.submittedPlayerIds.size + room.skippedSubmitIds.size,
        total,
      };
    }
    if (room.phase === 'voting') {
      return {
        label: 'Voted',
        done: room.votedPlayerIds.size + room.skippedVoteIds.size,
        total,
      };
    }
    return null;
  });

  function initials(name: string): string {
    const parts = name.trim().split(/\s+/);
    if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
    return name.slice(0, 2).toUpperCase();
  }

  // Avatar color cycle — deterministic by name so a given player keeps the
  // same color across renders.
  function avatarClass(id: string | null | undefined, isSelf: boolean): string {
    if (isSelf) return 'av-self';
    const hash = (id ?? '').split('').reduce((acc, c) => acc + c.charCodeAt(0), 0);
    const cycle = ['av-accent', 'av-accent-2', 'av-accent-3', 'av-grad-4'];
    return cycle[hash % cycle.length];
  }
</script>

<aside class="flex flex-col gap-4" aria-label="Room roster">
  <!-- Roster / leaderboard -->
  <div
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-3"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <h3 class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-mid flex items-center gap-2 m-0">
      <span style="color: var(--brand-accent);">♠</span>
      {#if room.phase === 'results'}
        Leaderboard
      {:else}
        Players
      {/if}
      <span class="ml-auto font-mono tabular-nums">{rosterRows.length}</span>
    </h3>
    <ul class="flex flex-col gap-1.5 m-0 p-0 list-none">
      {#each rosterRows as row (row.id)}
        {@const isSelf = row.id === selfId}
        <li
          class="flex items-center gap-2.5 rounded-full border-[2.5px] px-3 py-1.5 text-sm font-bold transition-colors"
          class:is-self={isSelf}
          style="background: {isSelf ? 'var(--brand-accent)' : 'var(--brand-white)'}; color: {isSelf ? '#1A1A1A' : 'var(--brand-text)'}; border-color: var(--brand-border-heavy);"
        >
          <span
            class="avatar inline-grid place-items-center h-8 w-8 rounded-full border-[2.5px] border-brand-border-heavy text-[11px] font-bold shrink-0 {avatarClass(row.id, isSelf)}"
            aria-hidden="true"
          >
            {initials(row.name)}
          </span>
          <div class="flex flex-col min-w-0 flex-1">
            <span class="leading-tight truncate">
              {isSelf ? `${row.name} (you)` : row.name}
            </span>
            {#if row.rank !== null}
              <span class="text-[9px] font-bold uppercase tracking-[0.15em] opacity-70">
                Rank {row.rank}
              </span>
            {:else if room.phase === 'voting' && row.hasSkippedVote}
              <span
                class="text-[9px] font-bold uppercase tracking-[0.15em]"
                style="color: {isSelf ? '#1A1A1A' : 'var(--brand-accent-3)'}; opacity: {isSelf ? 0.75 : 1};"
              >
                ✗ Skipped
              </span>
            {:else if room.phase === 'voting' && row.hasVoted}
              <span
                class="text-[9px] font-bold uppercase tracking-[0.15em]"
                style="color: {isSelf ? '#1A1A1A' : 'var(--brand-accent-2)'}; opacity: {isSelf ? 0.75 : 1};"
              >
                ✓ Voted
              </span>
            {:else if room.phase === 'submitting' && row.hasSkippedSubmit}
              <span
                class="text-[9px] font-bold uppercase tracking-[0.15em]"
                style="color: {isSelf ? '#1A1A1A' : 'var(--brand-accent-3)'}; opacity: {isSelf ? 0.75 : 1};"
              >
                ♠ Joker
              </span>
            {:else if room.phase === 'submitting' && row.hasSubmitted}
              <span
                class="text-[9px] font-bold uppercase tracking-[0.15em]"
                style="color: {isSelf ? '#1A1A1A' : 'var(--brand-accent-2)'}; opacity: {isSelf ? 0.75 : 1};"
              >
                ✓ Submitted
              </span>
            {:else if row.isHost}
              <span class="text-[9px] font-bold uppercase tracking-[0.15em] opacity-70">
                Host
              </span>
            {:else if !row.connected}
              <span class="text-[9px] font-bold uppercase tracking-[0.15em] opacity-60">
                Away
              </span>
            {/if}
          </div>
          {#if row.score !== null}
            <span
              class="font-mono font-bold text-xs tabular-nums shrink-0"
              style="color: {isSelf ? '#1A1A1A' : 'var(--brand-text-mid)'};"
            >
              {row.score}
            </span>
          {/if}
          <span
            class="h-2 w-2 rounded-full shrink-0"
            style="background: {row.connected ? 'var(--brand-accent-2)' : 'var(--brand-text-muted)'};"
            aria-label={row.connected ? 'Connected' : 'Away'}
          ></span>
        </li>
      {/each}
    </ul>
  </div>

  <!-- Phase hint — fills rail trough with useful copy, not empty felt. -->
  {#if room.phase === 'submitting' || room.phase === 'voting'}
    <div
      class="rounded-[22px] border-[2.5px] border-brand-border bg-brand-white/60 p-4 flex flex-col gap-1.5"
    >
      <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-mid inline-flex items-center gap-2">
        <span style="color: var(--brand-accent);">
          {room.phase === 'submitting' ? '♣' : '♥'}
        </span>
        How it works
      </span>
      <p class="text-xs font-semibold text-brand-text-mid leading-snug m-0">
        {#if room.phase === 'submitting'}
          Write the funniest caption for the prompt. Everyone submits blind — captions stay anonymous until the vote closes.
        {:else}
          Pick the caption that made you laugh hardest. You can't vote for your own, and votes reveal at the end of the round.
        {/if}
      </p>
    </div>
  {/if}

  <!-- Progress -->
  {#if progress}
    <div
      class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-2.5"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
    >
      <div class="flex items-center justify-between">
        <h3 class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-mid m-0">
          {progress.label}
        </h3>
        <span class="font-mono font-bold text-xs tabular-nums">
          {progress.done} / {progress.total}
        </span>
      </div>
      <div
        class="h-2.5 rounded-full overflow-hidden border-[2.5px] border-brand-border-heavy bg-brand-white"
      >
        <div
          class="h-full transition-[width] duration-500"
          style="width: {progress.total > 0 ? (progress.done / progress.total) * 100 : 0}%; background: linear-gradient(90deg, var(--brand-accent-2), var(--brand-accent));"
        ></div>
      </div>
    </div>
  {/if}
</aside>

<style>
  /* Avatar color cycle — stays constant across time bands by design, so the
     same player doesn't appear to change identity at dusk. */
  :global(.avatar.av-accent)   { background: var(--brand-accent); color: #ffffff; }
  :global(.avatar.av-accent-2) { background: var(--brand-accent-2); color: #ffffff; }
  :global(.avatar.av-accent-3) { background: var(--brand-accent-3); color: #ffffff; }
  :global(.avatar.av-grad-4)   { background: var(--brand-grad-4); color: var(--brand-text); }
  /* Self-row avatar sits on a coral pill — invert to a cream disc with dark
     ink so it reads as a "chip" rather than blending with the bg. */
  :global(.avatar.av-self)     { background: #FEFEFE; color: #1A1A1A; }
</style>
