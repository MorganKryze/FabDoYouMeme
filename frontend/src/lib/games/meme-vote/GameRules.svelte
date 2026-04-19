<script lang="ts">
  import { reveal } from '$lib/actions/reveal';
  import type { GameType } from '$lib/api/types';
  let { gameType }: { gameType: GameType } = $props();
</script>

<div
  use:reveal
  class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 text-sm text-brand-text-mid text-left max-w-sm"
  style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
>
  <p class="font-bold text-brand-text mb-1">{gameType.name}</p>
  <p class="font-semibold">{gameType.description}</p>
  <ul class="mt-3 flex flex-col gap-1.5 text-[11px] font-bold uppercase tracking-[0.15em] text-brand-text-muted list-none p-0">
    <li>• Pick a caption from your hand</li>
    <li>• Everyone plays one card per round</li>
    <li>• Vote the funniest · 1 point per vote received</li>
  </ul>
</div>
