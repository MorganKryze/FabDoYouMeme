<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { ArrowLeft, Plus, Package, Check, User, Users } from '$lib/icons';
  import type { ActionData, PageData } from './$types';
  import type { Pack, PaginatedResponse, GameType, RequiredPack } from '$lib/api/types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  type Step = 'game' | 'pack';

  let selectedGameTypeId = $state(untrack(() => data.preselectedId));
  // One selection per declared pack role. Role names mirror RequiredPack.role
  // ('image', 'text', …). Empty string means "not yet picked" so the submit
  // button can check with `selectedPacks[role]` without undefined juggling.
  let selectedPacks = $state<Record<string, string>>({});
  let isSolo = $state(false);
  // One catalogue per role so pickers can display side by side without fighting
  // over a shared array.
  let packsByRole = $state<Record<string, Pack[]>>({});
  let loadingRoles = $state<Record<string, boolean>>({});
  let step = $state<Step>(untrack(() => (data.preselectedId ? 'pack' : 'game')));

  const selectedGameType = $derived<GameType | null>(
    data.gameTypes.find((gt) => gt.id === selectedGameTypeId) ?? null
  );

  const requiredPacks = $derived<RequiredPack[]>(
    selectedGameType?.required_packs ?? []
  );

  const allPacksPicked = $derived(
    requiredPacks.length > 0 && requiredPacks.every((r) => !!selectedPacks[r.role])
  );

  const packAborts: Record<string, AbortController | null> = {};

  function roleLabel(role: string): string {
    if (role === 'image') return 'Image pack';
    if (role === 'text') return 'Caption pack';
    return `${role.charAt(0).toUpperCase()}${role.slice(1)} pack`;
  }

  function roleShort(role: string): string {
    if (role === 'image') return 'Image';
    if (role === 'text') return 'Caption';
    return role.charAt(0).toUpperCase() + role.slice(1);
  }

  function roleHint(role: string): string {
    if (role === 'image') return 'the meme canvas';
    if (role === 'text') return 'the text options';
    return '';
  }

  const explainer = $derived.by<string | null>(() => {
    if (!selectedGameType || requiredPacks.length === 0) return null;
    const parts = requiredPacks.map((r) => {
      const hint = roleHint(r.role);
      const label = roleLabel(r.role).toLowerCase();
      const article = /^[aeiou]/.test(label) ? 'an' : 'a';
      return hint ? `${article} ${label} (${hint})` : `${article} ${label}`;
    });
    let list: string;
    if (parts.length === 1) list = parts[0];
    else if (parts.length === 2) list = `${parts[0]} and ${parts[1]}`;
    else list = `${parts.slice(0, -1).join(', ')}, and ${parts[parts.length - 1]}`;
    const n = requiredPacks.length;
    return `${selectedGameType.name} needs ${n} pack${n > 1 ? 's' : ''}: ${list}.`;
  });

  async function loadPacksForRole(role: string) {
    if (!selectedGameType) return;
    packAborts[role]?.abort();
    const ctrl = new AbortController();
    packAborts[role] = ctrl;
    loadingRoles[role] = true;
    try {
      const url = `/api/packs?game_type=${encodeURIComponent(selectedGameType.slug)}&role=${encodeURIComponent(role)}`;
      const res = await fetch(url, { signal: ctrl.signal });
      if (!res.ok) {
        packsByRole[role] = [];
        return;
      }
      const body: PaginatedResponse<Pack> = await res.json();
      packsByRole[role] = body.data ?? [];
      // Clear any stale selection that is no longer in the filtered list.
      if (selectedPacks[role] && !(body.data ?? []).some((p) => p.id === selectedPacks[role])) {
        selectedPacks[role] = '';
      }
    } catch (err) {
      if ((err as Error).name !== 'AbortError') packsByRole[role] = [];
    } finally {
      loadingRoles[role] = false;
    }
  }

  $effect(() => {
    if (step !== 'pack' || !selectedGameType) return;
    // Reset when the game type switches so previous role caches do not leak.
    selectedPacks = {};
    for (const req of requiredPacks) {
      void loadPacksForRole(req.role);
    }
  });

  function pickGame(id: string) {
    selectedGameTypeId = id;
    step = 'pack';
  }

  function officialOf(role: string): Pack[] {
    return (packsByRole[role] ?? []).filter((p) => p.is_official || p.is_system);
  }
  function personalOf(role: string): Pack[] {
    return (packsByRole[role] ?? []).filter((p) => !(p.is_official || p.is_system));
  }
</script>

<svelte:head>
  <title>Host a game — FabDoYouMeme</title>
</svelte:head>

<div class="flex-1 flex items-start justify-center p-6 pt-8">
  <div class="w-full max-w-3xl flex flex-col gap-6">

    <header use:reveal class="flex items-center gap-3">
      <a
        href="/"
        use:pressPhysics={'ghost'}
        class="h-10 w-10 inline-flex items-center justify-center rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
        aria-label="Back to home"
      >
        <ArrowLeft size={18} strokeWidth={2.5} />
      </a>
      <h1 class="text-2xl font-bold">Host a game</h1>
    </header>

    {#if form?.error}
      <div
        use:reveal={{ delay: 1 }}
        class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold"
        style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
      >
        {form.error}
      </div>
    {/if}

    {#if step === 'game'}
      <section class="flex flex-col gap-4">
        <p use:reveal={{ delay: 1 }} class="text-sm font-semibold text-brand-text-muted uppercase tracking-[0.2em]">Pick a game type</p>
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
          {#each data.gameTypes as gt, i}
            {@const variant = `a${(i % 6) + 1}`}
            <button
              type="button"
              use:reveal={{ delay: i + 2 }}
              use:physCard
              use:hoverEffect={'gradient'}
              onclick={() => pickGame(gt.id)}
              class="group text-left rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-3 cursor-pointer focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
              style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
            >
              <div class="deck-art {variant}" aria-hidden="true">
                Game card
              </div>
              <h3 class="text-lg font-bold m-0 leading-tight">
                {gt.name}
              </h3>
              {#if gt.description}
                <p class="text-sm font-semibold text-brand-text-muted line-clamp-3 m-0">{gt.description}</p>
              {/if}
              <div class="mt-auto flex items-center gap-2">
                {#if gt.supports_solo}
                  <span class="text-[10px] font-bold uppercase tracking-[0.18em] text-brand-text-muted inline-flex items-center gap-1.5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-2.5 py-0.5">
                    <User size={12} strokeWidth={2.5} />
                    Solo
                  </span>
                {:else}
                  <span class="text-[10px] font-bold uppercase tracking-[0.18em] text-brand-text-muted inline-flex items-center gap-1.5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-2.5 py-0.5">
                    <Users size={12} strokeWidth={2.5} />
                    Multi
                  </span>
                {/if}
              </div>
            </button>
          {/each}
        </div>
      </section>
    {:else}
      <form method="POST" action="?/createRoom" use:enhance class="flex flex-col gap-5">
        <input type="hidden" name="game_type_id" value={selectedGameTypeId} />
        <input type="hidden" name="pack_id" value={selectedPacks.image ?? ''} />
        <input type="hidden" name="text_pack_id" value={selectedPacks.text ?? ''} />
        <input type="hidden" name="is_solo" value={String(isSolo)} />

        <div class="flex flex-col gap-3">
          <div class="flex items-center justify-between gap-3">
            <p class="text-sm font-semibold text-brand-text-muted uppercase tracking-[0.2em]">
              {requiredPacks.length > 1 ? 'Pick your packs' : 'Pick a pack'}
            </p>
            <button
              type="button"
              use:pressPhysics={'ghost'}
              onclick={() => (step = 'game')}
              class="inline-flex items-center gap-1.5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-3 py-1 text-[10px] font-bold uppercase tracking-[0.18em] text-brand-text cursor-pointer"
              style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
            >
              <ArrowLeft size={12} strokeWidth={2.5} />
              Change game
            </button>
          </div>

          {#if explainer}
            <p class="text-sm font-semibold text-brand-text-muted m-0">{explainer}</p>
          {/if}

          {#if requiredPacks.length > 0}
            <div class="flex flex-wrap gap-2 mt-1">
              {#each requiredPacks as req, idx (req.role)}
                {@const picked = !!selectedPacks[req.role]}
                <span
                  class="inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy pl-1.5 pr-3 py-1 text-xs font-bold uppercase tracking-[0.18em] transition-colors {picked
                    ? 'bg-brand-text text-brand-white'
                    : 'bg-brand-white text-brand-text-muted'}"
                  style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
                >
                  <span
                    class="inline-flex items-center justify-center h-5 w-5 rounded-full text-[11px] font-bold tabular-nums leading-none {picked
                      ? 'bg-brand-white text-brand-text'
                      : 'bg-brand-text text-brand-white'}"
                    aria-hidden="true"
                  >
                    {idx + 1}
                  </span>
                  {roleShort(req.role)}
                  {#if picked}
                    <Check size={12} strokeWidth={3} />
                  {:else}
                    <span class="h-2.5 w-2.5 rounded-full border-[2px] border-current" aria-hidden="true"></span>
                  {/if}
                </span>
              {/each}
            </div>
          {/if}
        </div>

        {#snippet packCard(p: Pack, role: string, i: number)}
          {@const selected = selectedPacks[role] === p.id}
          <button
            type="button"
            use:reveal={{ delay: i + 1 }}
            use:physCard
            onclick={() => (selectedPacks[role] = selectedPacks[role] === p.id ? '' : p.id)}
            aria-pressed={selected}
            class="relative text-left rounded-[22px] border-[2.5px] p-5 flex flex-col gap-2 cursor-pointer transition-all
                   {selected
                     ? 'border-brand-text bg-brand-surface ring-4 ring-brand-text/15'
                     : 'border-brand-border-heavy bg-brand-white'}"
            style={selected
              ? 'box-shadow: 0 6px 0 rgba(0,0,0,0.15);'
              : 'box-shadow: 0 5px 0 rgba(0,0,0,0.08);'}
          >
            {#if selected}
              <span
                class="absolute -top-2 -right-2 h-7 w-7 inline-flex items-center justify-center rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white"
                style="box-shadow: 0 2px 0 rgba(0,0,0,0.12);"
                aria-hidden="true"
              >
                <Check size={14} strokeWidth={3} />
              </span>
            {/if}
            <div class="inline-flex items-center gap-2 pr-8">
              <Package size={16} strokeWidth={2.5} />
              <span class="font-bold">{p.name}</span>
            </div>
            {#if p.description}
              <p class="text-xs font-semibold text-brand-text-muted line-clamp-2">{p.description}</p>
            {/if}
            {#if typeof p.item_count === 'number'}
              <div class="mt-auto text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
                {p.item_count} items
              </div>
            {/if}
          </button>
        {/snippet}

        {#each requiredPacks as req, idx (req.role)}
          {#if idx > 0}
            <hr
              class="border-0 h-0 my-3"
              style="border-top: 2.5px dashed var(--brand-border-heavy); opacity: 0.35;"
              aria-hidden="true"
            />
          {/if}
          <div class="flex flex-col gap-4">
            <div class="flex items-center gap-3">
              <span
                class="inline-flex items-center justify-center h-10 w-10 rounded-full bg-brand-text text-brand-white text-base font-bold tabular-nums border-[2.5px] border-brand-border-heavy"
                style="box-shadow: 0 3px 0 rgba(0,0,0,0.15);"
                aria-hidden="true"
              >
                {idx + 1}
              </span>
              <div class="flex flex-col">
                <h2 class="text-lg font-bold leading-tight m-0">{roleLabel(req.role)}</h2>
                {#if roleHint(req.role)}
                  <p class="text-xs font-semibold text-brand-text-muted m-0 leading-tight mt-0.5">
                    {roleHint(req.role)}
                  </p>
                {/if}
              </div>
            </div>
            {#if loadingRoles[req.role]}
              <p class="text-sm font-semibold text-brand-text-muted">Loading packs…</p>
            {:else if (packsByRole[req.role] ?? []).length === 0}
              <p class="text-sm font-semibold text-brand-text-muted">
                No compatible {req.role} packs for {selectedGameType?.name ?? 'this game'}.
              </p>
            {:else}
              <div class="flex flex-col gap-6">
                {#if officialOf(req.role).length > 0}
                  <div class="flex flex-col gap-3">
                    <p class="text-[0.7rem] font-bold text-brand-text-muted uppercase tracking-[0.2em]">Official</p>
                    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                      {#each officialOf(req.role) as p, i (p.id)}
                        {@render packCard(p, req.role, i)}
                      {/each}
                    </div>
                  </div>
                {/if}
                {#if personalOf(req.role).length > 0}
                  <div class="flex flex-col gap-3">
                    <p class="text-[0.7rem] font-bold text-brand-text-muted uppercase tracking-[0.2em]">Personal</p>
                    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                      {#each personalOf(req.role) as p, i (p.id)}
                        {@render packCard(p, req.role, i + officialOf(req.role).length)}
                      {/each}
                    </div>
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        {/each}

        {#if selectedGameType?.supports_solo}
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" bind:checked={isSolo} class="h-4 w-4 rounded border-brand-border-heavy" />
            <span class="text-sm font-semibold">Solo mode</span>
          </label>
        {/if}

        <button
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          type="submit"
          disabled={!allPacksPicked}
          class="h-14 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-50 cursor-pointer inline-flex items-center justify-center gap-2"
        >
          <Plus size={18} strokeWidth={2.5} />
          Spin up the room
        </button>
      </form>
    {/if}
  </div>
</div>

<footer class="border-t border-brand-border px-6 py-6 flex items-center justify-between text-xs font-semibold text-brand-text-muted">
  <p>© {new Date().getFullYear()} FabDoYouMeme</p>
  <a href="/privacy" class="hover:text-brand-text transition-colors">Privacy Policy</a>
</footer>
