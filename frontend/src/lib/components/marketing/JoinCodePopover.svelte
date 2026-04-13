<script lang="ts">
  import { goto } from '$app/navigation';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import RoomCodeInput from '$lib/components/RoomCodeInput.svelte';
  import { Play, ChevronDown } from '$lib/icons';

  let open = $state(false);
  let code = $state('');
  let rootEl = $state<HTMLDivElement | null>(null);

  function toggle() {
    open = !open;
  }

  function submit(next: string) {
    code = next;
    if (next.length !== 4) return;
    // Guest-join flow collects a display name on /join/[code] before
    // opening a guest session against the room.
    goto(`/join/${next}`);
    open = false;
  }

  // Close on click outside.
  $effect(() => {
    if (!open) return;
    function onDocClick(e: MouseEvent) {
      if (rootEl && !rootEl.contains(e.target as Node)) open = false;
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') open = false;
    }
    document.addEventListener('click', onDocClick);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('click', onDocClick);
      document.removeEventListener('keydown', onKey);
    };
  });
</script>

<div bind:this={rootEl} class="relative">
  <button
    type="button"
    onclick={toggle}
    use:pressPhysics={'ghost'}
    use:hoverEffect={'swap'}
    aria-expanded={open}
    class="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-white border-[2.5px] border-brand-border-heavy transition-colors"
    style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
  >
    Got a code?
    <ChevronDown
      size={14}
      strokeWidth={2.5}
      class="transition-transform duration-200 {open ? 'rotate-180' : ''}"
    />
  </button>

  {#if open}
    <div
      class="absolute right-0 mt-3 w-[min(22rem,calc(100vw-2rem))] rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-3 z-50"
      style="box-shadow: 0 8px 0 rgba(0,0,0,0.08);"
      role="dialog"
      aria-label="Join a room by code"
    >
      <div class="text-center">
        <p class="text-sm font-bold">Jump into a room</p>
        <p class="text-xs font-semibold text-brand-text-muted mt-0.5">
          No account needed — just the code.
        </p>
      </div>

      <RoomCodeInput bind:value={code} autofocus onenter={submit} />

      <button
        type="button"
        use:pressPhysics={'dark'}
        use:hoverEffect={'gradient'}
        disabled={code.length !== 4}
        onclick={() => submit(code)}
        class="h-12 w-full rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-40 cursor-pointer inline-flex items-center justify-center gap-2"
      >
        <Play size={16} strokeWidth={2.5} />
        Join
      </button>
    </div>
  {/if}
</div>
