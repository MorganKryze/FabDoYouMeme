<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { user } from '$lib/state/user.svelte';
  import { reveal } from '$lib/actions/reveal';
  import { listItems } from '$lib/api/studio';
  import PackNavigator from '$lib/components/studio/PackNavigator.svelte';
  import ItemTable from '$lib/components/studio/ItemTable.svelte';
  import ItemEditor from '$lib/components/studio/ItemEditor.svelte';
  import SingleItemAdd from '$lib/components/studio/SingleItemAdd.svelte';
  import LabWelcome from '$lib/components/studio/LabWelcome.svelte';
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  // Deep-link preselection fires once per "navigation that carries a new
  // pack id" — re-landing on the same id (e.g. a data-only refetch) must
  // not thrash the selection the user may have since changed.
  let lastHonoredPackId: string | null = null;

  $effect(() => {
    studio.packs = data.packs;
    studio.groups = data.groups;
    const deepLink = data.preselectedPackId;
    if (deepLink && deepLink !== lastHonoredPackId) {
      lastHonoredPackId = deepLink;
      studio.selectPack(deepLink);
      void listItems(deepLink).then((items) => {
        if (studio.selectedPackId === deepLink) studio.items = items;
      });
    }
  });

  const isSelectedSystem = $derived(
    studio.packs.find((p) => p.id === studio.selectedPackId)?.is_system ?? false
  );

  const hasPersonalPack = $derived(
    studio.packs.some((p) => p.owner_id === user.id)
  );
</script>

<svelte:head>
  <title>{m.studio_page_title()}</title>
</svelte:head>

<!-- Below lg the 3-column layout (208 + flex + 320 = 528 px of fixed chrome)
     does not fit. Pack creation is bulk-edit heavy enough that a true mobile
     flow would feel worse than a polite redirect — show a placeholder with
     a back link instead. Telemetry can promote this to a real mobile flow
     if demand emerges. -->
<div class="lg:hidden flex-1 flex flex-col items-center justify-center text-center p-6 gap-4">
  <div
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-6 max-w-md flex flex-col gap-3"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <h1 class="text-xl font-bold m-0">{m.studio_desktop_only_title()}</h1>
    <p class="text-sm font-semibold text-brand-text-muted m-0">{m.studio_desktop_only_body()}</p>
    <a
      href="/home"
      class="self-center mt-2 h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold inline-flex items-center justify-center gap-2 no-underline"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      {m.studio_desktop_only_back()}
    </a>
  </div>
</div>

<div class="hidden lg:flex flex-1 overflow-hidden h-[calc(100dvh-3.5rem)]" use:reveal>
  <!-- Left: Pack Navigator (fixed width) -->
  <div class="w-52 shrink-0 border-r border-brand-border overflow-y-auto">
    <PackNavigator />
  </div>

  <!-- Center: Item Table (flexible) -->
  <div class="flex-1 min-w-0 border-r border-brand-border overflow-y-auto">
    {#if studio.selectedPackId}
      <ItemTable />
    {:else if !hasPersonalPack}
      <LabWelcome />
    {:else}
      <div class="flex items-center justify-center h-full text-brand-text-muted text-sm">
        {m.studio_placeholder_select_pack_to_view()}
      </div>
    {/if}
  </div>

  <!-- Right: Item Editor (fixed width) -->
  <div class="w-80 shrink-0 overflow-y-auto">
    {#if studio.selectedItemId}
      <ItemEditor />
    {:else if studio.selectedPackId}
      {#if isSelectedSystem}
        <div class="flex items-center justify-center h-full text-brand-text-muted text-sm p-4 text-center">
          {m.studio_placeholder_system_pack()}
        </div>
      {:else}
        <SingleItemAdd />
      {/if}
    {:else}
      <div class="flex items-center justify-center h-full text-brand-text-muted text-sm p-4 text-center">
        {m.studio_placeholder_select_pack_to_start()}
      </div>
    {/if}
  </div>
</div>
