<script lang="ts">
  // Phase 3. Admin-only approval queue for NSFW→SFW duplications. The
  // detail-page tab bar deep-links here via "Pending" button; accepting
  // force-relabels the group to NSFW per the spec.
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { groupsApi, type PendingDuplication } from '$lib/api/groups';
  import { toast } from '$lib/state/toast.svelte';
  import { reveal } from '$lib/actions/reveal';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { CheckCircle, XCircle, ArrowLeft, AlertTriangle } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();
  const gid = $derived(data.gid);

  let rows = $state<PendingDuplication[]>([]);
  let loading = $state(false);
  let busy = $state<string | null>(null);

  async function load() {
    loading = true;
    try {
      rows = await groupsApi.listPending(gid);
    } catch (e) {
      toast.show((e as Error).message, 'error');
    } finally {
      loading = false;
    }
  }

  async function accept(row: PendingDuplication) {
    if (!confirm(m.groups_queue_accept_confirm())) return;
    busy = `accept:${row.id}`;
    try {
      await groupsApi.acceptPending(gid, row.id);
      toast.show(m.groups_queue_accepted(), 'success');
      await load();
    } catch (e) {
      toast.show((e as Error).message, 'error');
    } finally {
      busy = null;
    }
  }

  async function reject(row: PendingDuplication) {
    if (!confirm(m.groups_queue_reject_confirm())) return;
    busy = `reject:${row.id}`;
    try {
      await groupsApi.rejectPending(gid, row.id);
      toast.show(m.groups_queue_rejected(), 'success');
      await load();
    } catch (e) {
      toast.show((e as Error).message, 'error');
    } finally {
      busy = null;
    }
  }

  onMount(load);
</script>

<svelte:head>
  <title>{m.groups_queue_page_title()}</title>
</svelte:head>

<div class="w-full max-w-3xl mx-auto p-6 flex flex-col gap-6" use:reveal>
  <header class="flex items-center gap-3">
    <button
      type="button"
      onclick={() => goto(`/groups/${gid}`)}
      use:hoverEffect={'swap'}
      class="h-10 px-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-xs font-bold cursor-pointer inline-flex items-center gap-2"
    >
      <ArrowLeft size={14} strokeWidth={2.5} />
      {m.common_back()}
    </button>
    <h1 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
      {m.groups_queue_heading()}
    </h1>
  </header>

  <section
    class="rounded-[18px] border-[2.5px] border-orange-300 bg-orange-50 p-4 flex items-start gap-3"
  >
    <AlertTriangle size={20} strokeWidth={2.5} class="text-orange-600 shrink-0 mt-0.5" />
    <p class="text-sm font-semibold text-orange-700 m-0">{m.groups_queue_banner()}</p>
  </section>

  {#if loading}
    <p class="text-sm font-semibold text-brand-text-muted">{m.groups_loading()}</p>
  {:else if rows.length === 0}
    <p class="text-sm font-semibold text-brand-text-muted">{m.groups_queue_empty()}</p>
  {:else}
    <ul class="flex flex-col gap-3 list-none p-0 m-0">
      {#each rows as row (row.id)}
        <li
          class="rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-3"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
        >
          <div class="flex items-center justify-between gap-2">
            <div class="min-w-0">
              <p class="text-sm font-bold m-0 truncate">{row.source_pack_name}</p>
              <p class="text-xs text-brand-text-muted m-0">
                {m.groups_queue_requested_by({ username: row.requested_by_username })}
              </p>
            </div>
            <span
              class="shrink-0 text-[0.6rem] font-bold uppercase tracking-[0.15em] rounded-full px-2 py-1 border-[2px] border-brand-border-heavy"
            >
              {row.source_classification}
            </span>
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              use:pressPhysics={'dark'}
              use:hoverEffect={'swap'}
              disabled={busy !== null}
              onclick={() => accept(row)}
              class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <CheckCircle size={16} strokeWidth={2.5} />
              {m.groups_queue_accept()}
            </button>
            <button
              type="button"
              use:hoverEffect={'swap'}
              disabled={busy !== null}
              onclick={() => reject(row)}
              class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <XCircle size={16} strokeWidth={2.5} />
              {m.groups_queue_reject()}
            </button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</div>
