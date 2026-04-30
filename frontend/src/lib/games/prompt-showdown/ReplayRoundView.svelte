<script lang="ts">
  import { reveal } from '$lib/actions/reveal';
  import type { ReplayRound, ReplayLeaderboardRow, Submission, LeaderboardEntry, Round } from '$lib/api/types';
  import SentenceWithBlank from '../_shared/SentenceWithBlank.svelte';
  import RoundReveal from './RoundReveal.svelte';
  import * as m from '$lib/paraglide/messages';

  let {
    round,
    leaderboard,
  }: {
    round: ReplayRound;
    leaderboard: ReplayLeaderboardRow[];
  } = $props();

  const prefix = $derived(((round.prompt as { prefix?: string }).prefix ?? '') as string);
  const suffix = $derived(((round.prompt as { suffix?: string }).suffix ?? '') as string);

  const submissions = $derived<Submission[]>(
    round.submissions.map((s) => ({
      id: s.id,
      user_id: '',
      username: s.author.display_name,
      text: (s.payload as { text?: string }).text ?? '',
      votes_received: s.votes_received,
      points_awarded: s.points_awarded,
    } as Submission))
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

  const roundLike = $derived<Round>({
    round_number: round.round_number,
    ends_at: '',
    duration_seconds: 0,
    item: {
      id: '',
      payload_version: 4,
      payload: { prefix, suffix } as unknown as Round['item']['payload'],
    },
  } as unknown as Round);
</script>

<div use:reveal class="flex flex-col gap-4">
  <div
    class="relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface overflow-hidden p-5 sm:p-6"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <p class="m-0 text-brand-text leading-tight">
      <SentenceWithBlank {prefix} {suffix} placeholder={m.game_prompt_showdown_blank_placeholder()} size="md" />
    </p>
  </div>

  {#if submissions.length === 0}
    <p class="text-center text-sm font-bold text-brand-text-muted py-8 m-0">
      {m.game_round_all_skipped()}
    </p>
  {:else}
    <RoundReveal {submissions} leaderboard={lb} round={roundLike} />
  {/if}
</div>
