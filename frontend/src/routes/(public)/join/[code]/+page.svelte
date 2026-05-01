<script lang="ts">
  import { goto } from '$app/navigation';
  import { untrack } from 'svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import RoomCodeInput from '$lib/components/RoomCodeInput.svelte';
  import { guest } from '$lib/state/guest.svelte';
  import { Play, AlertTriangle } from '$lib/icons';
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  let code = $state(untrack(() => data.code));
  let displayName = $state('');
  let error = $state<string | null>(null);
  let submitting = $state(false);
  let nameInput = $state<HTMLInputElement | null>(null);

  $effect(() => {
    if (nameInput) nameInput.focus();
  });

  async function onSubmit(e: Event) {
    e.preventDefault();
    error = null;
    if (code.length !== 4) { error = m.join_error_code_required(); return; }
    if (displayName.trim().length < 1) { error = m.join_error_name_required(); return; }

    submitting = true;
    try {
      const res = await fetch(`/api/rooms/${code}/guest-join`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ display_name: displayName.trim() })
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        if (body.code === 'banned_from_room') {
          error = m.join_error_banned();
        } else {
          error = body.message ?? m.join_error_generic();
        }
        return;
      }
      const body = await res.json();
      guest.set(code, {
        player_id: body.player_id,
        display_name: body.display_name,
        token: body.guest_token
      });
      await goto(`/rooms/${code}?as=guest`);
    } catch {
      error = m.join_error_network();
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head>
  <title>{m.join_page_title_code({ code: data.code })}</title>
</svelte:head>

<h1 class="text-2xl font-bold text-center">{m.join_heading_invited()}</h1>
<p class="text-sm font-semibold text-brand-text-muted text-center -mt-4">
  {m.join_subtitle_code_prefix()} <span class="font-mono font-bold text-brand-text">{data.code}</span> {m.join_subtitle_code_suffix()}
</p>

<form onsubmit={onSubmit} class="flex flex-col gap-4">
  {#if error}
    <div
      role="alert"
      class="rounded-2xl border-[2.5px] border-red-300 bg-red-50 px-4 py-3 flex items-start gap-3"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
    >
      <AlertTriangle size={20} strokeWidth={2.5} class="text-red-600 shrink-0 mt-0.5" />
      <p class="text-sm font-bold text-red-700 leading-snug">{error}</p>
    </div>
  {/if}

  <div class="flex flex-col gap-1">
    <label for="code" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">{m.join_field_code_label()}</label>
    <RoomCodeInput bind:value={code} />
  </div>

  <div class="flex flex-col gap-1">
    <label for="display_name" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">{m.join_field_display_name_label()}</label>
    <input
      id="display_name"
      bind:this={nameInput}
      bind:value={displayName}
      type="text"
      maxlength={32}
      placeholder={m.common_pick_nickname()}
      class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 text-base font-semibold focus:outline-none focus:border-brand-text transition-colors"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
    />
  </div>

  <button
    use:pressPhysics={'dark'}
    use:hoverEffect={'gradient'}
    type="submit"
    disabled={submitting}
    class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-50 cursor-pointer inline-flex items-center justify-center gap-2"
  >
    <Play size={18} strokeWidth={2.5} />
    {submitting ? m.join_submitting() : m.join_submit()}
  </button>
</form>

<p class="text-center text-xs text-brand-text-muted">
  {m.common_already_have_account_prefix()} <a href="/auth/magic-link" class="underline font-bold">{m.common_sign_in()}</a>
</p>
