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
    if (role === 'image') return m.host_role_image();
    if (role === 'text') return m.host_role_text();
    return m.host_role_generic({ role: role.charAt(0).toUpperCase() + role.slice(1) });
  }

  function roleShort(role: string): string {
    if (role === 'image') return m.host_role_image_short();
    if (role === 'text') return m.host_role_text_short();
    return m.host_role_generic_short({ role: role.charAt(0).toUpperCase() + role.slice(1) });
  }

  function roleHint(role: string): string {
    if (role === 'image') return m.host_role_image_hint();
    if (role === 'text') return m.host_role_text_hint();
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

  // Partition a role's packs into same-language (matching the UI locale) and
  // other-language buckets. A user hosting in FR rarely wants EN captions, so
  // the other bucket is hidden behind a disclosure but still reachable. Packs
  // tagged 'multi' (language-agnostic content, e.g. image packs) always count
  // as same-language so hosts see them regardless of UI locale.
  function sameLangOf(role: string): Pack[] {
    const loc = getLocale();
    return (packsByRole[role] ?? []).filter((p) => p.language === loc || p.language === 'multi');
  }
  function otherLangOf(role: string): Pack[] {
    const loc = getLocale();
    return (packsByRole[role] ?? []).filter((p) => p.language !== loc && p.language !== 'multi');
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

<div class="flex-1 flex items-start justify-center p-6 pt-8">
  <div class="w-full max-w-3xl flex flex-col gap-6">

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
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
          {#each data.gameTypes as gt, i}
            {@const variant = `a${(i % 6) + 1}`}
            {@const localized = localizeGameType(gt)}
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
                {m.host_game_card_aria()}
              </div>
              <h3 class="text-lg font-bold m-0 leading-tight">
                {localized.name}
              </h3>
              {#if localized.description}
                <p class="text-sm font-semibold text-brand-text-muted line-clamp-3 m-0">{localized.description}</p>
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
        <input type="hidden" name="pack_id" value={selectedPacks.image ?? ''} />
        <input type="hidden" name="text_pack_id" value={selectedPacks.text ?? ''} />
        <input type="hidden" name="is_solo" value={String(isSolo)} />

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
            <div class="mt-auto flex items-center justify-between gap-2">
              {#if typeof p.item_count === 'number'}
                <span class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
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
            {#if loadingRoles[req.role]}
              <p class="text-sm font-semibold text-brand-text-muted">{m.host_packs_loading()}</p>
            {:else if (packsByRole[req.role] ?? []).length === 0}
              <p class="text-sm font-semibold text-brand-text-muted">
                {m.host_packs_empty({ role: req.role, game: selectedGameType ? localizeGameType(selectedGameType).name : m.host_packs_empty_fallback_game() })}
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
                    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                      {#each sameOfficial as p, i (p.id)}
                        {@render packCard(p, req.role, i)}
                      {/each}
                    </div>
                  </div>
                {/if}
                {#if samePersonal.length > 0}
                  <div class="flex flex-col gap-3">
                    <p class="text-[0.7rem] font-bold text-brand-text-muted uppercase tracking-[0.2em]">{m.host_section_personal()}</p>
                    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
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
                          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                            {#each otherOfficial as p, i (p.id)}
                              {@render packCard(p, req.role, i)}
                            {/each}
                          </div>
                        </div>
                      {/if}
                      {#if otherPersonal.length > 0}
                        <div class="flex flex-col gap-3">
                          <p class="text-[0.7rem] font-bold text-brand-text-muted uppercase tracking-[0.2em]">{m.host_section_personal()}</p>
                          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
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

        {#if selectedGameType?.supports_solo}
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" bind:checked={isSolo} class="h-4 w-4 rounded border-brand-border-heavy" />
            <span class="text-sm font-semibold">{m.host_solo_mode()}</span>
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
          {m.host_submit()}
        </button>
      </form>
    {/if}
  </div>
</div>

<footer class="border-t border-brand-border px-6 py-6 flex items-center justify-between text-xs font-semibold text-brand-text-muted">
  <p>© {new Date().getFullYear()} FabDoYouMeme</p>
  <a href="/privacy" class="hover:text-brand-text transition-colors">{m.common_privacy_policy()}</a>
</footer>
