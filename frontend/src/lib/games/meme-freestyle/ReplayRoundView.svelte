<script lang="ts">
  import { reveal } from '$lib/actions/reveal';
  import type { ReplayRound, ReplayLeaderboardRow, Submission, LeaderboardEntry } from '$lib/api/types';
  import { mediaUrl } from '$lib/api/media';
  import RoundReveal from './RoundReveal.svelte';
  import * as m from '$lib/paraglide/messages';

  let {
    round,
    leaderboard,
  }: {
    round: ReplayRound;
    leaderboard: ReplayLeaderboardRow[];
  } = $props();

  const submissions = $derived<Submission[]>(
    round.submissions.map((s) => ({
      id: s.id,
      user_id: '',
      username: s.author.display_name,
      caption: (s.payload as { caption?: string }).caption ?? '',
      votes_received: s.votes_received,
      points_awarded: s.points_awarded,
    }))
  );

  const lb = $derived<LeaderboardEntry[]>(
    leaderboard.map((e) => ({
      player_id: e.display_name,
      display_name: e.display_name,
      is_guest: e.kind === 'guest',
      score: e.score,
      rank: e.rank,
    }))
  );

  const promptImage = $derived.by(() => {
    if (round.prompt.image_url) return round.prompt.image_url;
    if (round.prompt.media_key) return mediaUrl(round.prompt.media_key);
    return null;
  });
</script>

<div use:reveal class="flex flex-col gap-4">
  <div
    class="relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface overflow-hidden"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    {#if promptImage}
      <div
        aria-hidden="true"
        class="absolute inset-0 scale-110 bg-center bg-cover blur-2xl"
        style={`background-image: url("${promptImage}");`}
      ></div>
      <img
        src={promptImage}
        alt=""
        class="relative block mx-auto h-auto max-h-[45vh] max-w-full w-auto"
      />
    {/if}
    {#if round.prompt.prompt}
      <p class="relative px-4 py-3 text-sm font-bold text-brand-text m-0 bg-brand-surface">
        {round.prompt.prompt}
      </p>
    {/if}
  </div>

  {#if submissions.length === 0}
    <p class="text-center text-sm font-bold text-brand-text-muted py-8 m-0">
      {m.game_round_all_skipped()}
    </p>
  {:else}
    <RoundReveal submissions={submissions} leaderboard={lb} />
  {/if}
</div>
