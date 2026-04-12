<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import PackNavigator from '$lib/components/studio/PackNavigator.svelte';
  import ItemTable from '$lib/components/studio/ItemTable.svelte';
  import ItemEditor from '$lib/components/studio/ItemEditor.svelte';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  $effect(() => {
    studio.packs = data.packs;
  });
</script>

<svelte:head>
  <title>Studio — FabDoYouMeme</title>
</svelte:head>

<div class="flex-1 flex overflow-hidden h-[calc(100vh-3.5rem)]">
  <!-- Left: Pack Navigator (fixed width) -->
  <div class="w-52 shrink-0 border-r border-brand-border overflow-y-auto">
    <PackNavigator />
  </div>

  <!-- Center: Item Table (flexible) -->
  <div class="flex-1 min-w-0 border-r border-brand-border overflow-y-auto">
    {#if studio.selectedPackId}
      <ItemTable />
    {:else}
      <div class="flex items-center justify-center h-full text-brand-text-muted text-sm">
        Select a pack to view items.
      </div>
    {/if}
  </div>

  <!-- Right: Item Editor (fixed width) -->
  <div class="w-80 shrink-0 overflow-y-auto">
    {#if studio.selectedItemId}
      <ItemEditor />
    {:else}
      <div class="flex items-center justify-center h-full text-brand-text-muted text-sm p-4 text-center">
        Select an item to edit.
      </div>
    {/if}
  </div>
</div>
