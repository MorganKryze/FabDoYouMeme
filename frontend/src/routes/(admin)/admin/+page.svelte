<script lang="ts">
  import type { PageData } from './$types';
  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>Admin Dashboard — FabDoYouMeme</title>
</svelte:head>

<div class="p-6 flex flex-col gap-6">
  <h1 class="text-2xl font-bold">Dashboard</h1>

  {#if data.stats}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      {#each [
        { label: 'Active Rooms', value: data.stats.active_rooms ?? 0 },
        { label: 'Total Users', value: data.stats.total_users ?? 0 },
        { label: 'Total Packs', value: data.stats.total_packs ?? 0 },
        { label: 'Pending Invites', value: data.stats.pending_invites ?? 0 },
      ] as card}
        <div class="rounded-xl border border-brand-border bg-brand-white p-4">
          <p class="text-sm text-brand-text-muted">{card.label}</p>
          <p class="text-3xl font-bold mt-1">{card.value}</p>
        </div>
      {/each}
    </div>
  {/if}

  <section class="flex flex-col gap-3">
    <h2 class="text-base font-semibold">Recent Activity</h2>
    <p class="text-sm text-brand-text-muted">No recent activity.</p>
  </section>
</div>
