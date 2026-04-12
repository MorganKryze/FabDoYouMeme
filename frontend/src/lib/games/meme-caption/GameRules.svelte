<script lang="ts">
  import type { GameType } from '$lib/api/types';
  let { gameType }: { gameType: GameType } = $props();
</script>

<div
  class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 text-sm text-brand-text-mid text-left max-w-sm"
  style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
>
  <p class="font-bold text-brand-text mb-1">{gameType.name}</p>
  <p class="font-semibold">{gameType.description}</p>
</div>
