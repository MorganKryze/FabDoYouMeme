<script lang="ts">
  import type { PageData } from './$types';
  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>Game Types — Admin</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4">
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">Game Types</h1>
    <p class="text-xs text-muted-foreground">Read-only — game types are registered in code.</p>
  </div>

  <div class="rounded-xl border border-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-border bg-muted/40 text-xs font-medium text-muted-foreground">
          <th class="text-left px-4 py-3">Slug</th>
          <th class="text-left px-4 py-3">Name</th>
          <th class="text-left px-4 py-3">Description</th>
          <th class="text-left px-4 py-3">Payload Versions</th>
          <th class="text-left px-4 py-3">Supports Solo</th>
        </tr>
      </thead>
      <tbody>
        {#each data.gameTypes as gt}
          <tr class="border-b border-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3 font-mono text-xs">{gt.slug}</td>
            <td class="px-4 py-3 font-medium">{gt.name}</td>
            <td class="px-4 py-3 text-muted-foreground">{gt.description}</td>
            <td class="px-4 py-3 text-muted-foreground font-mono text-xs">
              [{(gt.supported_payload_versions ?? []).join(', ')}]
            </td>
            <td class="px-4 py-3">
              <span class="text-xs px-2 py-0.5 rounded-full {gt.supports_solo ? 'bg-green-100 text-green-800' : 'bg-muted text-muted-foreground'}">
                {gt.supports_solo ? 'Yes' : 'No'}
              </span>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
