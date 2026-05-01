<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { ArrowLeft, Plus, Package, Check, User, Users } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';
  import { getLocale } from '$lib/paraglide/runtime';
  import { localizeGameType } from '$lib/i18n/gameType';
  import type { ActionData, PageData } from './$types';
  import type { Pack, PaginatedResponse, GameType, RequiredPack } from '$lib/api/types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  type Step = 'game' | 'pack';

  let selectedGameTypeId = $state(untrack(() => data.preselectedId));
  // ADR-016: each role accepts a list of (pack_id, weight) tuples. A single-
  // pack pick is the common case (length 1, weight 1) and the form degrades
  // to today's behaviour when the host doesn't add extras. Clicking a pack
  // card toggles its presence in this list; the weight number input on the
  // selected-row pill drives the relative mix.
  type PackPick = { pack_id: string; weight: number };
  let selectedPacks = $state<Record<string, PackPick[]>>({});

  function packsForRoleSelected(role: string): PackPick[] {
    return selectedPacks[role] ?? [];
  }
  function isPackPicked(role: string, packId: string): boolean {
    return packsForRoleSelected(role).some((p) => p.pack_id === packId);
  }
  function togglePack(role: string, packId: string) {
    const current = packsForRoleSelected(role);
    const idx = current.findIndex((p) => p.pack_id === packId);
    if (idx >= 0) {
      selectedPacks[role] = current.filter((_, i) => i !== idx);
    } else {
      selectedPacks[role] = [...current, { pack_id: packId, weight: 1 }];
    }
  }
  function setWeight(role: string, packId: string, weight: number) {
    const w = Math.max(1, Math.floor(weight || 1));
    selectedPacks[role] = packsForRoleSelected(role).map((p) =>
      p.pack_id === packId ? { ...p, weight: w } : p
    );
  }
  function removePackFromRole(role: string, packId: string) {
    selectedPacks[role] = packsForRoleSelected(role).filter((p) => p.pack_id !== packId);
  }
  /** Renormalised percentages e.g. "60% / 20% / 20%" for a role's mix. */
  function weightsHint(role: string): string {
    const list = packsForRoleSelected(role);
    if (list.length <= 1) return '';
    const total = list.reduce((s, p) => s + p.weight, 0);
    if (total <= 0) return '';
    return list.map((p) => `${Math.round((p.weight / total) * 100)}%`).join(' / ');
  }
  let isSolo = $state(false);
  // Phase 4 — group-scoped toggle. Empty string = Personal mode (today's
  // behaviour). Any non-empty value is a group id from data.groups. The
  // pack picker is intentionally NOT filtered client-side yet — the
  // backend rejects cross-context picks with pack_not_in_group so the
  // UX surfaces the right error when the user picks a personal pack in
  // group mode. Filter UX is a phase-5 polish.
  let selectedGroupID = $state(untrack(() => data.preselectedGroupID ?? ''));
  // Per-room player cap chosen by the host. Defaults to the manifest cap so
  // hosts who don't touch the slider get today's behaviour. The backend
  // validates pack-size requirements against this number, so lowering it
  // makes smaller packs viable for friend-sized lobbies.
  let maxPlayers = $state<number | null>(null);
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
    requiredPacks.length > 0 &&
      requiredPacks.every((r) => packsForRoleSelected(r.role).length > 0)
  );

  /** JSON payload sent to the server action — flattened across roles. */
  const packsPayload = $derived(
    requiredPacks.flatMap((req) =>
      packsForRoleSelected(req.role).map((p) => ({
        role: req.role,
        pack_id: p.pack_id,
        weight: p.weight
      }))
    )
  );

  const packAborts: Record<string, AbortController | null> = {};

  function roleLabel(role: string): string {
    if (role === 'image') return m.host_role_image();
    if (role === 'text') return m.host_role_text();
    if (role === 'prompt') return m.host_role_prompt();
    if (role === 'filler') return m.host_role_filler();
    return m.host_role_generic({ role: role.charAt(0).toUpperCase() + role.slice(1) });
  }

  function roleShort(role: string): string {
    if (role === 'image') return m.host_role_image_short();
    if (role === 'text') return m.host_role_text_short();
    if (role === 'prompt') return m.host_role_prompt_short();
    if (role === 'filler') return m.host_role_filler_short();
    return m.host_role_generic_short({ role: role.charAt(0).toUpperCase() + role.slice(1) });
  }

  function roleHint(role: string): string {
    if (role === 'image') return m.host_role_image_hint();
    if (role === 'text') return m.host_role_text_hint();
    if (role === 'prompt') return m.host_role_prompt_hint();
    if (role === 'filler') return m.host_role_filler_hint();
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
    const gameName = localizeGameType(selectedGameType).name;
    return n === 1
      ? m.host_explainer_single({ game: gameName, count: n, list })
      : m.host_explainer_plural({ game: gameName, count: n, list });
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
      // Drop any stale picks no longer in the filtered list.
      const valid = new Set((body.data ?? []).map((p) => p.id));
      if (selectedPacks[role]) {
        selectedPacks[role] = selectedPacks[role].filter((p) => valid.has(p.pack_id));
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

  // When the game type changes, reset maxPlayers to the manifest cap so the
  // input shows the right starting point. Hosts who actively want a smaller
  // lobby drag the slider after; hosts who don't touch it get the prior
  // (manifest-wide) behaviour.
  $effect(() => {
    if (!selectedGameType) {
      maxPlayers = null;
      return;
    }
    maxPlayers = selectedGameType.config.max_players ?? null;
  });

  // Clear any selected pack that's no longer valid under the current scope
  // filter when the user toggles between personal and group modes. Without
  // this, flipping the scope selector could leave a ghost selection the
  // picker UI isn't even showing.
  $effect(() => {
    // Track selectedGroupID so the effect reruns when scope changes. Drop
    // any picked packs that don't belong to the now-current scope.
    const scope = selectedGroupID;
    for (const role of Object.keys(selectedPacks)) {
      const list = selectedPacks[role] ?? [];
      const filtered = list.filter((p) => {
        const pack = packsByRole[role]?.find((cand) => cand.id === p.pack_id);
        if (!pack) return false;
        return scope
          ? pack.is_system || pack.group_id === scope
          : !pack.group_id;
      });
      if (filtered.length !== list.length) selectedPacks[role] = filtered;
    }
  });

  function pickGame(id: string) {
    selectedGameTypeId = id;
    step = 'pack';
  }

  // Room-scope filter: the backend rejects cross-scope picks with
  // pack_not_in_group / personal-mode equivalent, so pre-filtering here
  // prevents the user from picking something they'll only learn is wrong
  // on submit. Since /api/packs now returns the caller's group packs (via
  // the extended ListPacksForUser), we run both modes against the same
  // source data.
  //
  // Personal mode (empty selectedGroupID): hide group packs.
  // Group mode: keep system packs + that group's packs only.
  function packsForRole(role: string): Pack[] {
    const all = packsByRole[role] ?? [];
    if (!selectedGroupID) {
      return all.filter((p) => !p.group_id);
    }
    return all.filter((p) => p.is_system || p.group_id === selectedGroupID);
  }

  // Partition a role's packs into same-language (matching the UI locale) and
  // other-language buckets. A user hosting in FR rarely wants EN captions, so
  // the other bucket is hidden behind a disclosure but still reachable. Packs
  // tagged 'multi' (language-agnostic content, e.g. image packs) always count
  // as same-language so hosts see them regardless of UI locale.
  function sameLangOf(role: string): Pack[] {
    const loc = getLocale();
    return packsForRole(role).filter((p) => p.language === loc || p.language === 'multi');
  }
  function otherLangOf(role: string): Pack[] {
    const loc = getLocale();
    return packsForRole(role).filter((p) => p.language !== loc && p.language !== 'multi');
  }
  function officialOf(packs: Pack[]): Pack[] {
    return packs.filter((p) => p.is_official || p.is_system);
  }
  function personalOf(packs: Pack[]): Pack[] {
    return packs.filter((p) => !(p.is_official || p.is_system));
  }

  function packLangLabel(lang: Pack['language']): string {
    if (lang === 'multi') return m.host_pack_lang_multi();
    if (lang === 'fr') return m.host_pack_lang_fr();
    return m.host_pack_lang_en();
  }
</script>

<svelte:head>
  <title>{m.host_page_title()}</title>
</svelte:head>

<div class="flex-1 flex items-start justify-center p-4 sm:p-6 pt-6 sm:pt-8 pb-24 md:pb-10">
  <div class="w-full max-w-3xl flex flex-col gap-4 sm:gap-6">

    <header use:reveal class="flex items-center gap-3">
      <a
        href="/"
        use:pressPhysics={'ghost'}
        class="h-10 w-10 inline-flex items-center justify-center rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
        aria-label={m.host_back_aria()}
      >
        <ArrowLeft size={18} strokeWidth={2.5} />
      </a>
      <h1 class="text-2xl font-bold">{m.host_title()}</h1>
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
        <p use:reveal={{ delay: 1 }} class="text-sm font-semibold text-brand-text-muted uppercase tracking-[0.2em]">{m.host_step_game()}</p>
        <div class="grid grid-cols-2 sm:grid-cols-2 lg:grid-cols-3 gap-3 sm:gap-5">
          {#each data.gameTypes as gt, i}
            {@const variant = `a${(i % 6) + 1}`}
            {@const localized = localizeGameType(gt)}
            <button
              type="button"
              use:reveal={{ delay: i + 2 }}
              use:physCard
              use:hoverEffect={'gradient'}
              onclick={() => pickGame(gt.id)}
              class="group text-left rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-3 sm:p-4 flex flex-col gap-2 sm:gap-3 cursor-pointer focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60 sm:min-h-[16rem]"
              style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
            >
              <div class="deck-art {variant}" aria-hidden="true">
                {m.host_game_card_aria()}
              </div>
              <h3 class="text-base sm:text-lg font-bold m-0 leading-tight">
                {localized.name}
              </h3>
              {#if localized.description}
                <p class="hidden sm:block text-sm font-semibold text-brand-text-muted line-clamp-3 m-0">{localized.description}</p>
              {/if}
              <div class="mt-auto flex items-center gap-2">
                {#if gt.supports_solo}
                  <span class="text-[10px] font-bold uppercase tracking-[0.18em] text-brand-text-muted inline-flex items-center gap-1.5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-2.5 py-0.5">
                    <User size={12} strokeWidth={2.5} />
                    {m.host_badge_solo()}
                  </span>
                {:else}
                  <span class="text-[10px] font-bold uppercase tracking-[0.18em] text-brand-text-muted inline-flex items-center gap-1.5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-2.5 py-0.5">
                    <Users size={12} strokeWidth={2.5} />
                    {m.host_badge_multi()}
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
        <!-- ADR-016: weighted multi-pack rooms. The form serialises every
             selected pack across every role into one JSON blob; the server
             action forwards it verbatim to POST /api/rooms. -->
        <input type="hidden" name="packs" value={JSON.stringify(packsPayload)} />
        <input type="hidden" name="is_solo" value={String(isSolo)} />
        <input type="hidden" name="group_id" value={selectedGroupID} />
        <input
          type="hidden"
          name="max_players"
          value={maxPlayers !== null ? String(maxPlayers) : ''}
        />

        {#if data.groups.length > 0}
          <div
            class="rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-3"
            style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
          >
            <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted m-0">
              {m.host_group_scope_label()}
            </p>
            <select
              bind:value={selectedGroupID}
              class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
            >
              <option value="">{m.host_group_scope_personal()}</option>
              {#each data.groups as g (g.id)}
                <option value={g.id}>{g.name}</option>
              {/each}
            </select>
            {#if selectedGroupID}
              <p class="text-xs text-brand-text-muted m-0">{m.host_group_scope_hint()}</p>
            {/if}
          </div>
        {/if}

        <div class="flex flex-col gap-3">
          <div class="flex items-center justify-between gap-3">
            <p class="text-sm font-semibold text-brand-text-muted uppercase tracking-[0.2em]">
              {requiredPacks.length > 1 ? m.host_step_pack_many() : m.host_step_pack_one()}
            </p>
            <button
              type="button"
              use:pressPhysics={'ghost'}
              onclick={() => (step = 'game')}
              class="inline-flex items-center gap-1.5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-3 py-1 text-[10px] font-bold uppercase tracking-[0.18em] text-brand-text cursor-pointer"
              style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
            >
              <ArrowLeft size={12} strokeWidth={2.5} />
              {m.host_change_game()}
            </button>
          </div>

          {#if explainer}
            <p class="text-sm font-semibold text-brand-text-muted m-0">{explainer}</p>
          {/if}

          {#if requiredPacks.length > 0}
            <div class="flex flex-wrap gap-2 mt-1">
              {#each requiredPacks as req, idx (req.role)}
                {@const count = packsForRoleSelected(req.role).length}
                {@const picked = count > 0}
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
                    {#if count > 1}
                      <span class="text-[10px] tabular-nums opacity-80">×{count}</span>
                    {/if}
                  {:else}
                    <span class="h-2.5 w-2.5 rounded-full border-[2px] border-current" aria-hidden="true"></span>
                  {/if}
                </span>
              {/each}
            </div>
          {/if}
        </div>

        {#snippet packCard(p: Pack, role: string, i: number)}
          {@const selected = isPackPicked(role, p.id)}
          <button
            type="button"
            use:reveal={{ delay: i + 1 }}
            use:physCard
            onclick={() => togglePack(role, p.id)}
            aria-pressed={selected}
            class="relative text-left rounded-[22px] border-[2.5px] p-3 sm:p-5 flex flex-col gap-2 cursor-pointer transition-all
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
              <span class="font-bold text-sm sm:text-base">{p.name}</span>
            </div>
            {#if p.description}
              <p class="hidden sm:block text-xs font-semibold text-brand-text-muted line-clamp-2">{p.description}</p>
            {/if}
            <div class="mt-auto flex items-center justify-between gap-2">
              {#if typeof p.item_count === 'number'}
                <span class="text-[0.6rem] sm:text-[0.65rem] font-bold uppercase tracking-[0.18em] sm:tracking-[0.2em] text-brand-text-muted">
                  {m.host_pack_items({ count: p.item_count })}
                </span>
              {:else}
                <span></span>
              {/if}
              <span
                class="inline-flex items-center justify-center rounded-full border-[2px] border-brand-border-heavy bg-brand-white px-2 py-0.5 text-[0.6rem] font-bold tracking-[0.14em]"
              >
                {packLangLabel(p.language)}
              </span>
            </div>
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

            <!-- Weights bar — only visible when more than one pack is picked
                 for this role. Each row exposes a weight number input and a
                 remove button; the renormalised percentages render to the
                 right so the host sees the effective mix at a glance. -->
            {#if packsForRoleSelected(req.role).length > 1}
              <div
                class="rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-3 flex flex-col gap-2"
                style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
              >
                <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted m-0">
                  {m.host_pack_weights_label()}
                  <span class="ml-2 normal-case tracking-normal font-semibold text-brand-text">{weightsHint(req.role)}</span>
                </p>
                {#each packsForRoleSelected(req.role) as pick (pick.pack_id)}
                  {@const pack = (packsByRole[req.role] ?? []).find((cand) => cand.id === pick.pack_id)}
                  <div class="flex items-center gap-2">
                    <span class="flex-1 truncate text-sm font-semibold">{pack?.name ?? pick.pack_id}</span>
                    <input
                      type="number"
                      min="1"
                      step="1"
                      value={pick.weight}
                      oninput={(e) => setWeight(req.role, pick.pack_id, Number((e.target as HTMLInputElement).value))}
                      class="h-8 w-16 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-2 text-sm font-semibold text-center"
                      aria-label={m.host_pack_weight_aria()}
                    />
                    <button
                      type="button"
                      onclick={() => removePackFromRole(req.role, pick.pack_id)}
                      class="h-8 w-8 inline-flex items-center justify-center rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text-muted hover:text-red-600"
                      aria-label={m.host_pack_remove_aria()}
                    >
                      ×
                    </button>
                  </div>
                {/each}
              </div>
            {/if}

            {#if loadingRoles[req.role]}
              <p class="text-sm font-semibold text-brand-text-muted">{m.host_packs_loading()}</p>
            {:else if packsForRole(req.role).length === 0}
              <p class="text-sm font-semibold text-brand-text-muted">
                {selectedGroupID
                  ? m.host_packs_empty_group({ game: selectedGameType ? localizeGameType(selectedGameType).name : m.host_packs_empty_fallback_game() })
                  : m.host_packs_empty({ role: req.role, game: selectedGameType ? localizeGameType(selectedGameType).name : m.host_packs_empty_fallback_game() })}
              </p>
            {:else}
              {@const same = sameLangOf(req.role)}
              {@const other = otherLangOf(req.role)}
              {@const sameOfficial = officialOf(same)}
              {@const samePersonal = personalOf(same)}
              {@const otherOfficial = officialOf(other)}
              {@const otherPersonal = personalOf(other)}
              <div class="flex flex-col gap-6">
                {#if sameOfficial.length > 0}
                  <div class="flex flex-col gap-3">
                    <p class="text-[0.7rem] font-bold text-brand-text-muted uppercase tracking-[0.2em]">{m.host_section_official()}</p>
                    <div class="grid grid-cols-2 lg:grid-cols-3 gap-3 sm:gap-4">
                      {#each sameOfficial as p, i (p.id)}
                        {@render packCard(p, req.role, i)}
                      {/each}
                    </div>
                  </div>
                {/if}
                {#if samePersonal.length > 0}
                  <div class="flex flex-col gap-3">
                    <p class="text-[0.7rem] font-bold text-brand-text-muted uppercase tracking-[0.2em]">{m.host_section_personal()}</p>
                    <div class="grid grid-cols-2 lg:grid-cols-3 gap-3 sm:gap-4">
                      {#each samePersonal as p, i (p.id)}
                        {@render packCard(p, req.role, i + sameOfficial.length)}
                      {/each}
                    </div>
                  </div>
                {/if}
                {#if other.length > 0}
                  <details class="group flex flex-col gap-3">
                    <summary
                      class="cursor-pointer inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-2 text-xs font-bold uppercase tracking-[0.18em] text-brand-text-muted w-fit select-none"
                      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
                    >
                      {m.host_packs_other_languages({ count: other.length })}
                    </summary>
                    <p class="text-xs font-semibold text-brand-text-muted mt-3">
                      {m.host_packs_other_languages_hint()}
                    </p>
                    <div class="flex flex-col gap-6 mt-3">
                      {#if otherOfficial.length > 0}
                        <div class="flex flex-col gap-3">
                          <p class="text-[0.7rem] font-bold text-brand-text-muted uppercase tracking-[0.2em]">{m.host_section_official()}</p>
                          <div class="grid grid-cols-2 lg:grid-cols-3 gap-3 sm:gap-4">
                            {#each otherOfficial as p, i (p.id)}
                              {@render packCard(p, req.role, i)}
                            {/each}
                          </div>
                        </div>
                      {/if}
                      {#if otherPersonal.length > 0}
                        <div class="flex flex-col gap-3">
                          <p class="text-[0.7rem] font-bold text-brand-text-muted uppercase tracking-[0.2em]">{m.host_section_personal()}</p>
                          <div class="grid grid-cols-2 lg:grid-cols-3 gap-3 sm:gap-4">
                            {#each otherPersonal as p, i (p.id)}
                              {@render packCard(p, req.role, i + otherOfficial.length)}
                            {/each}
                          </div>
                        </div>
                      {/if}
                    </div>
                  </details>
                {/if}
              </div>
            {/if}
          </div>
        {/each}

        {#if selectedGameType && selectedGameType.config.max_players}
          {@const minP = selectedGameType.config.min_players}
          {@const capP = selectedGameType.config.max_players}
          <div
            class="rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-2"
            style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
          >
            <label class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted m-0" for="host-max-players">
              {m.host_max_players_label()}
            </label>
            <div class="flex items-center gap-3">
              <input
                id="host-max-players"
                type="range"
                min={minP}
                max={capP}
                step="1"
                bind:value={maxPlayers}
                class="flex-1"
              />
              <input
                type="number"
                min={minP}
                max={capP}
                step="1"
                bind:value={maxPlayers}
                class="h-9 w-20 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-3 text-sm font-semibold text-center"
              />
              <span class="text-xs font-semibold text-brand-text-muted whitespace-nowrap">
                {(maxPlayers ?? capP) === 1
                  ? m.host_max_players_unit_one({ count: maxPlayers ?? capP })
                  : m.host_max_players_unit_many({ count: maxPlayers ?? capP })}
              </span>
            </div>
            <p class="text-xs text-brand-text-muted m-0">{m.host_max_players_hint()}</p>
          </div>
        {/if}

        {#if selectedGameType?.supports_solo}
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" bind:checked={isSolo} class="h-4 w-4 rounded border-brand-border-heavy" />
            <span class="text-sm font-semibold">{m.host_solo_mode()}</span>
          </label>
        {/if}

        <!-- Desktop in-flow submit -->
        <button
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          type="submit"
          disabled={!allPacksPicked}
          class="hidden md:inline-flex h-14 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-50 cursor-pointer items-center justify-center gap-2"
        >
          <Plus size={18} strokeWidth={2.5} />
          {m.host_submit()}
        </button>

        <!-- Mobile sticky CTA bar — keeps "Create room" reachable without
             scrolling past every pack tile. Sits inside the form so the
             button submits the same payload as the desktop counterpart. -->
        <div
          class="md:hidden fixed inset-x-0 bottom-0 z-40 px-4 py-3 bg-brand-white border-t-[2.5px] border-brand-border-heavy"
          style="padding-bottom: max(0.75rem, env(safe-area-inset-bottom));"
        >
          <button
            use:pressPhysics={'dark'}
            use:hoverEffect={'gradient'}
            type="submit"
            disabled={!allPacksPicked}
            class="w-full h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-50 cursor-pointer inline-flex items-center justify-center gap-2"
          >
            <Plus size={18} strokeWidth={2.5} />
            {m.host_submit()}
          </button>
        </div>
      </form>
    {/if}
  </div>
</div>

<footer class="border-t border-brand-border px-6 py-6 flex items-center justify-between text-xs font-semibold text-brand-text-muted">
  <p>© {new Date().getFullYear()} FabDoYouMeme</p>
  <a href="/privacy" class="hover:text-brand-text transition-colors">{m.common_privacy_policy()}</a>
</footer>
