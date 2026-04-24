<script lang="ts">
  // Phase 3 of the groups paradigm. Any-member tab that lists group-owned
  // packs and offers the duplicate-into-this-group flow. The picker lists
  // the caller's own personal packs plus public packs they can see — the
  // same shape the existing host picker uses, so there's no dedicated
  // group-visibility endpoint yet.
  import { onMount } from 'svelte';
  import { fade, scale } from 'svelte/transition';
  import { backOut } from 'svelte/easing';
  import { goto } from '$app/navigation';
  import {
    groupsApi,
    type GroupPack,
    type DuplicatePendingResponse
  } from '$lib/api/groups';
  import { packsApi } from '$lib/api/packs';
  import { ApiError } from '$lib/api/client';
  import type { Pack } from '$lib/api/types';
  import { user } from '$lib/state/user.svelte';
  import { groupDetailState } from '$lib/state/groups.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Plus, Trash2, Ban, ListChecks, Edit2 } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';

  let { gid }: { gid: string } = $props();

  let packs = $state<GroupPack[]>([]);
  let loading = $state(false);
  let busy = $state<string | null>(null);

  let pickerOpen = $state(false);
  let sources = $state<Pack[]>([]);
  let sourcesLoading = $state(false);

  const selfIsAdmin = $derived(
    groupDetailState.members.some((mem) => mem.user_id === user.id && mem.role === 'admin')
  );

  // Compatible sources for duplication — the backend's language invariant
  // rejects EN→FR (and vice versa) unless one side is 'multi', and banned or
  // flagged packs never pass the visibility gate. Pre-filter here so users
  // can't pick a pack that will 409 on submit.
  const compatibleSources = $derived.by(() => {
    const groupLang = groupDetailState.group?.language;
    if (!groupLang) return [];
    return sources.filter((src) => {
      if (src.status !== 'active') return false;
      return src.language === groupLang || src.language === 'multi' || groupLang === 'multi';
    });
  });

  function packLangLabel(lang: Pack['language']): string {
    if (lang === 'multi') return m.host_pack_lang_multi();
    if (lang === 'fr') return m.host_pack_lang_fr();
    return m.host_pack_lang_en();
  }

  async function load() {
    loading = true;
    try {
      packs = await groupsApi.listPacks(gid);
    } catch (e) {
      toast.show(apiErrorMessage(e), 'error');
    } finally {
      loading = false;
    }
  }

  async function openPicker() {
    pickerOpen = true;
    if (sources.length > 0) return;
    sourcesLoading = true;
    try {
      const res = await packsApi.list({ limit: 100 });
      sources = res.data ?? [];
    } catch (e) {
      toast.show(apiErrorMessage(e), 'error');
    } finally {
      sourcesLoading = false;
    }
  }

  // Backend error codes translated on the client so FR users don't get
  // English leaks. Codes that aren't in this map fall back to the backend's
  // plaintext message.
  function apiErrorMessage(e: unknown): string {
    if (e instanceof ApiError) {
      const groupLang = groupDetailState.group?.language ?? 'en';
      switch (e.code) {
        case 'language_mismatch':
          return m.groups_packs_error_language_mismatch({ lang: packLangLabel(groupLang) });
        case 'group_quota_exceeded':
          return m.groups_packs_error_quota_exceeded();
        case 'source_pack_unavailable':
          return m.groups_packs_error_source_unavailable();
        case 'not_group_member':
          return m.groups_packs_error_not_member();
        case 'group_not_found':
          return m.groups_packs_error_group_gone();
      }
    }
    return (e as Error).message;
  }

  async function duplicate(source: Pack) {
    busy = `dup:${source.id}`;
    try {
      const res = await groupsApi.duplicatePack(gid, source.id);
      if ((res as DuplicatePendingResponse).status === 'pending_admin_approval') {
        toast.show(m.groups_packs_duplicate_pending(), 'warning');
      } else {
        toast.show(m.groups_packs_duplicated(), 'success');
      }
      pickerOpen = false;
      await load();
    } catch (e) {
      toast.show(apiErrorMessage(e), 'error');
    } finally {
      busy = null;
    }
  }

  async function removePack(pack: GroupPack) {
    if (!confirm(m.groups_packs_delete_confirm())) return;
    busy = `del:${pack.id}`;
    try {
      await groupsApi.deletePack(gid, pack.id);
      await load();
    } catch (e) {
      toast.show(apiErrorMessage(e), 'error');
    } finally {
      busy = null;
    }
  }

  async function evictPack(pack: GroupPack) {
    if (!confirm(m.groups_packs_evict_confirm())) return;
    busy = `evict:${pack.id}`;
    try {
      await groupsApi.evictPack(gid, pack.id);
      await load();
    } catch (e) {
      toast.show(apiErrorMessage(e), 'error');
    } finally {
      busy = null;
    }
  }

  // The parent page uses `use:reveal`, which leaves a `transform: translateY(0)`
  // on the container. That transform creates a containing block, so a child
  // `position: fixed` scrim ends up sized to the container, not the viewport —
  // hence the "small overlay block" effect. Re-parenting the modal to
  // document.body escapes the trap. Same pattern used in EndRoomButton.
  function portal(node: HTMLElement) {
    document.body.appendChild(node);
    return {
      destroy() {
        node.remove();
      }
    };
  }

  onMount(load);
</script>

<div class="flex flex-col gap-6">
  <section class="flex items-center justify-between gap-3">
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
      {m.groups_packs_heading()}
    </h2>
    <div class="flex items-center gap-2">
      {#if selfIsAdmin}
        <button
          type="button"
          onclick={() => goto(`/groups/${gid}/pending`)}
          use:hoverEffect={'swap'}
          class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2"
        >
          <ListChecks size={16} strokeWidth={2.5} />
          {m.groups_packs_queue_cta()}
        </button>
      {/if}
      <button
        type="button"
        use:pressPhysics={'dark'}
        use:hoverEffect={'swap'}
        onclick={openPicker}
        class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2"
      >
        <Plus size={16} strokeWidth={2.5} />
        {m.groups_packs_duplicate_cta()}
      </button>
    </div>
  </section>

  {#if loading}
    <p class="text-sm font-semibold text-brand-text-muted">{m.groups_loading()}</p>
  {:else if packs.length === 0}
    <p class="text-sm font-semibold text-brand-text-muted">{m.groups_packs_empty()}</p>
  {:else}
    <ul class="grid grid-cols-1 sm:grid-cols-2 gap-3 list-none p-0 m-0">
      {#each packs as p (p.id)}
        <li
          class="rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-2"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
        >
          <button
            type="button"
            onclick={() => goto(`/studio?pack=${p.id}`)}
            use:hoverEffect={'glow'}
            class="flex flex-col gap-1 text-left cursor-pointer rounded-[10px] -m-1 p-1"
            aria-label={m.groups_packs_open_in_lab({ name: p.name })}
          >
            <div class="flex items-center justify-between gap-2">
              <h3 class="text-sm font-bold m-0 truncate">{p.name}</h3>
              <span class="shrink-0 text-[0.6rem] font-bold uppercase tracking-[0.15em] rounded-full px-2 py-1 border-[2px] border-brand-border-heavy">
                {p.classification}
              </span>
            </div>
            {#if p.description}
              <p class="text-xs text-brand-text-muted line-clamp-2 m-0">{p.description}</p>
            {/if}
            <span class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted inline-flex items-center gap-1.5 mt-1">
              <Edit2 size={10} strokeWidth={2.5} />
              {m.groups_packs_open_in_lab_hint()}
            </span>
          </button>
          {#if selfIsAdmin}
            <div class="flex items-center gap-2 mt-2">
              <button
                type="button"
                disabled={busy !== null}
                onclick={() => evictPack(p)}
                class="h-9 px-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-xs font-bold cursor-pointer inline-flex items-center gap-1 disabled:opacity-50"
              >
                <Ban size={14} strokeWidth={2.5} />
                {m.groups_packs_evict()}
              </button>
              <button
                type="button"
                disabled={busy !== null}
                onclick={() => removePack(p)}
                class="h-9 px-3 rounded-full border-[2.5px] border-brand-border-heavy bg-red-50 text-red-700 text-xs font-bold cursor-pointer inline-flex items-center gap-1 disabled:opacity-50"
              >
                <Trash2 size={14} strokeWidth={2.5} />
                {m.groups_packs_delete()}
              </button>
            </div>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

<svelte:window onkeydown={(e) => pickerOpen && e.key === 'Escape' && (pickerOpen = false)} />

{#if pickerOpen}
  <!-- Modal: source-pack picker. Light fade-in scrim + click-outside close;
       modal card scales in to match the WaitingStage kick-dialog pattern.
       The scrim is intentionally soft (bg-black/20) because duplication is
       non-destructive — it doesn't deserve the dark modal treatment.
       `use:portal` re-parents to <body> — see the portal() helper above. -->
  <div
    use:portal
    class="fixed inset-0 z-50 bg-black/20"
    aria-hidden="true"
    transition:fade={{ duration: 120 }}
  >
    <button
      type="button"
      aria-label={m.common_cancel()}
      class="absolute inset-0 w-full h-full cursor-default"
      onclick={() => (pickerOpen = false)}
    ></button>
    <div class="absolute inset-0 flex items-center justify-center p-4 pointer-events-none">
      <div
        class="w-full max-w-lg rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-4 max-h-[80vh] overflow-hidden pointer-events-auto"
        style="box-shadow: 0 10px 0 rgba(0,0,0,0.12);"
        role="dialog"
        aria-modal="true"
        aria-labelledby="packs-picker-title"
        tabindex="-1"
        transition:scale={{ duration: 180, start: 0.85, easing: backOut }}
      >
      <h3 id="packs-picker-title" class="text-sm font-bold m-0">{m.groups_packs_picker_heading()}</h3>
      {#if groupDetailState.group}
        <p class="text-xs font-semibold text-brand-text-muted m-0">
          {m.groups_packs_picker_hint({
            lang: packLangLabel(groupDetailState.group.language)
          })}
        </p>
      {/if}
      {#if sourcesLoading}
        <p class="text-sm font-semibold text-brand-text-muted">{m.groups_loading()}</p>
      {:else if sources.length === 0}
        <p class="text-sm font-semibold text-brand-text-muted">{m.groups_packs_picker_empty()}</p>
      {:else if compatibleSources.length === 0}
        <p class="text-sm font-semibold text-brand-text-muted">
          {m.groups_packs_picker_no_compatible({
            lang: groupDetailState.group ? packLangLabel(groupDetailState.group.language) : ''
          })}
        </p>
      {:else}
        <ul class="flex flex-col gap-2 overflow-y-auto list-none p-0 m-0">
          {#each compatibleSources as src (src.id)}
            <li>
              <button
                type="button"
                disabled={busy !== null}
                onclick={() => duplicate(src)}
                class="w-full text-left rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3 cursor-pointer hover:bg-brand-surface transition-colors disabled:opacity-50 flex items-start justify-between gap-3"
              >
                <div class="min-w-0">
                  <p class="text-sm font-bold m-0 truncate">{src.name}</p>
                  {#if src.description}
                    <p class="text-xs text-brand-text-muted mt-0.5 m-0 line-clamp-1">{src.description}</p>
                  {/if}
                </div>
                <span class="shrink-0 inline-flex items-center justify-center rounded-full border-[2px] border-brand-border-heavy bg-brand-white px-2 py-0.5 text-[0.6rem] font-bold tracking-[0.14em]">
                  {packLangLabel(src.language)}
                </span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
      <div class="flex justify-end">
        <button
          type="button"
          onclick={() => (pickerOpen = false)}
          class="h-9 px-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-xs font-bold cursor-pointer"
        >
          {m.common_cancel()}
        </button>
      </div>
      </div>
    </div>
  </div>
{/if}
