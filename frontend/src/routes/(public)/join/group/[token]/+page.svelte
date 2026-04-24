<script lang="ts">
  import { goto } from '$app/navigation';
  import { groupsApi } from '$lib/api/groups';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { CheckCircle, AlertTriangle } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();
  const token = $derived(data.token);
  const preview = $derived(data.preview);

  let nsfwAcked = $state(false);
  let busy = $state(false);
  let error = $state<string | null>(null);

  const isNSFW = $derived(preview.group.classification === 'nsfw');
  const dead = $derived(preview.revoked || preview.expired || preview.exhausted);
  const canRedeem = $derived(!dead && (!isNSFW || nsfwAcked));

  function deadReason(): string {
    if (preview.revoked) return m.groups_invite_state_revoked();
    if (preview.expired) return m.groups_invite_state_expired();
    if (preview.exhausted) return m.groups_invite_state_exhausted();
    return '';
  }

  async function redeem() {
    if (!canRedeem || busy) return;
    busy = true;
    error = null;
    try {
      const { group } = await groupsApi.redeemInvite(token, isNSFW && nsfwAcked);
      goto(`/groups/${group.id}`);
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<svelte:head>
  <title>{m.groups_join_page_title({ name: preview.group.name })}</title>
</svelte:head>

<header class="flex flex-col gap-2">
  <p class="text-[0.6rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
    {m.groups_join_kicker()}
  </p>
  <h1 class="text-2xl font-bold m-0">{preview.group.name}</h1>
  <p class="text-sm text-brand-text-muted m-0">{preview.group.description}</p>
  <div class="flex items-center gap-2 mt-1">
    <span
      class="text-[0.6rem] font-bold uppercase tracking-[0.15em] rounded-full px-2 py-1 border-[2px] border-brand-border-heavy"
    >
      {preview.group.classification}
    </span>
    <span
      class="text-[0.6rem] font-bold uppercase tracking-[0.15em] rounded-full px-2 py-1 border-[2px] border-brand-border-heavy"
    >
      {preview.group.language}
    </span>
  </div>
</header>

{#if dead}
  <div
    class="rounded-[18px] border-[2.5px] border-red-300 bg-red-50 p-4 flex items-start gap-3"
  >
    <AlertTriangle size={20} strokeWidth={2.5} class="text-red-600 shrink-0 mt-0.5" />
    <div class="text-sm font-semibold text-red-700">
      {deadReason()}
    </div>
  </div>
{:else}
  {#if isNSFW}
    <label class="flex items-start gap-3 rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-white p-4 cursor-pointer">
      <input
        type="checkbox"
        bind:checked={nsfwAcked}
        class="mt-0.5 h-5 w-5"
      />
      <span class="text-sm font-semibold leading-snug">{m.groups_join_nsfw_age_affirmation()}</span>
    </label>
  {/if}

  {#if error}
    <p class="text-sm font-bold text-red-600">{error}</p>
  {/if}

  <div class="flex flex-col gap-2">
    <button
      type="button"
      use:pressPhysics={'dark'}
      use:hoverEffect={'swap'}
      disabled={!canRedeem || busy}
      onclick={redeem}
      class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer inline-flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
    >
      <CheckCircle size={16} strokeWidth={2.5} />
      {m.groups_join_cta()}
    </button>
    <a
      href="/home"
      class="text-xs text-brand-text-muted text-center underline hover:text-brand-text transition-colors"
    >
      {m.groups_join_decline()}
    </a>
  </div>
{/if}
