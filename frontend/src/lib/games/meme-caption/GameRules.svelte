<script lang="ts">
  import type { GameType } from '$lib/api/types';
  let { gameType }: { gameType: GameType } = $props();
</script>

<div class="rounded-lg border border-border bg-muted/40 p-4 text-sm text-muted-foreground text-left max-w-sm">
  <p class="font-medium text-foreground mb-1">{gameType.name}</p>
  <p>{gameType.description}</p>
</div>
