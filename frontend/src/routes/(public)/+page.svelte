<!-- frontend/src/routes/(public)/+page.svelte -->
<script lang="ts">
  import { user } from '$lib/state/user.svelte';

  let code = $state('');

  function handleJoin() {
    const trimmed = code.trim().toUpperCase();
    if (trimmed.length !== 4) return;
    if (user.isAuthenticated) {
      window.location.href = `/rooms/${trimmed}`;
    } else {
      window.location.href = `/auth/magic-link?next=/rooms/${trimmed}`;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') handleJoin();
  }
</script>

<svelte:head>
  <title>FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col items-center gap-8 text-center">
  <h1 class="text-4xl font-bold tracking-tight">FabDoYouMeme</h1>

  <div class="w-full flex flex-col gap-3">
    <label for="room-code" class="text-sm font-medium text-muted-foreground">Enter a room code to join</label>
    <input
      id="room-code"
      type="text"
      inputmode="text"
      autocomplete="off"
      autocapitalize="characters"
      maxlength={4}
      placeholder="WXYZ"
      class="h-14 w-full rounded-lg border border-input bg-background px-4 text-center text-2xl font-mono tracking-widest uppercase focus:outline-none focus:ring-2 focus:ring-ring"
      bind:value={code}
      onkeydown={handleKeydown}
      autofocus
    />
    <button
      type="button"
      onclick={handleJoin}
      disabled={code.trim().length !== 4}
      class="h-12 rounded-lg bg-primary text-primary-foreground font-semibold text-base disabled:opacity-50 disabled:cursor-not-allowed hover:bg-primary/90 transition-colors"
    >
      Join Game
    </button>
  </div>

  <a
    href={user.isAuthenticated ? '/' : '/auth/magic-link?next=/'}
    class="text-sm text-muted-foreground hover:text-foreground transition-colors"
  >
    I'm hosting →
  </a>
</div>
