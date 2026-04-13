<script lang="ts">
  import { goto } from '$app/navigation';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import RoomCodeInput from '$lib/components/RoomCodeInput.svelte';
  import { guest } from '$lib/state/guest.svelte';
  import { Play } from '$lib/icons';

  let code = $state('');
  let displayName = $state('');
  let error = $state<string | null>(null);
  let submitting = $state(false);

  async function onSubmit(e: Event) {
    e.preventDefault();
    error = null;
    if (code.length !== 4) { error = 'Enter a 4-character room code.'; return; }
    if (displayName.trim().length < 1) { error = 'Enter a display name.'; return; }

    submitting = true;
    try {
      const res = await fetch(`/api/rooms/${code}/guest-join`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ display_name: displayName.trim() })
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        error = body.message ?? 'Could not join. Check the code and try again.';
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
      error = 'Network error. Try again.';
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head>
  <title>Join a room — FabDoYouMeme</title>
</svelte:head>

<h1 class="text-2xl font-bold text-center">Join a room</h1>
<p class="text-sm font-semibold text-brand-text-muted text-center -mt-4">
  Drop your code and a name — no account needed.
</p>

<form onsubmit={onSubmit} class="flex flex-col gap-4">
  {#if error}
    <div
      class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
    >
      {error}
    </div>
  {/if}

  <div class="flex flex-col gap-1">
    <label for="code" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Room code</label>
    <RoomCodeInput bind:value={code} autofocus />
  </div>

  <div class="flex flex-col gap-1">
    <label for="display_name" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Display name</label>
    <input
      id="display_name"
      bind:value={displayName}
      type="text"
      maxlength={32}
      placeholder="Pick a nickname"
      class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
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
    {submitting ? 'Joining…' : 'Play'}
  </button>
</form>

<p class="text-center text-xs text-brand-text-muted">
  Already have an account? <a href="/auth/magic-link" class="underline font-bold">Sign in</a>
</p>
