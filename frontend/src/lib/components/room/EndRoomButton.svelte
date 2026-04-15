<script lang="ts">
  import { user } from '$lib/state/user.svelte';
  import { room } from '$lib/state/room.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { roomsApi } from '$lib/api/rooms';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { X } from '$lib/icons';
  import { fade, scale } from 'svelte/transition';
  import { backOut } from 'svelte/easing';

  const canEnd = $derived(
    (user.id !== null && room.hostUserId === user.id) || user.role === 'admin'
  );

  let open = $state(false);
  let pending = $state(false);
  let error = $state<string | null>(null);

  // Anchor coordinates for the confirmation dialog — captured from the
  // trigger button's bounding rect so the dialog appears directly over
  // the cancel/start button row rather than in the viewport centre.
  let triggerEl: HTMLButtonElement | undefined = $state();
  let anchor = $state({ x: 0, y: 0 });

  function openModal() {
    error = null;
    if (triggerEl) {
      const rect = triggerEl.getBoundingClientRect();
      anchor = {
        x: rect.left + rect.width / 2,
        y: rect.top + rect.height / 2
      };
    }
    open = true;
  }

  function closeModal() {
    if (pending) return;
    open = false;
  }

  async function confirm() {
    if (!room.code || pending) return;
    pending = true;
    error = null;
    try {
      await roomsApi.end(room.code);
    } catch (e) {
      pending = false;
      const message =
        e instanceof Error ? e.message : 'Could not end the room. Try again.';
      error = message;
      toast.show(message, 'error');
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (open && e.key === 'Escape') closeModal();
  }

  // Moves the node to document.body so position: fixed is not trapped by
  // a transformed ancestor (e.g. the .reveal wrapper in WaitingStage).
  function portal(node: HTMLElement) {
    document.body.appendChild(node);
    return {
      destroy() {
        node.remove();
      }
    };
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if canEnd}
  <button
    bind:this={triggerEl}
    use:pressPhysics={'ghost'}
    type="button"
    onclick={openModal}
    class="h-14 w-14 shrink-0 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text-mid hover:text-red-600 hover:border-red-600 inline-flex items-center justify-center cursor-pointer transition-colors"
    title="Cancel room"
    aria-label="Cancel room"
  >
    <X size={20} strokeWidth={2.5} />
  </button>
{/if}

{#if open}
  <!-- Full-viewport transparent backdrop — clicking it closes the dialog. -->
  <div
    use:portal
    class="fixed inset-0 z-50"
    role="presentation"
    onclick={closeModal}
    transition:fade={{ duration: 120 }}
  >
    <!-- Positioner: fixed to the anchor and holds the translate so that
         the inner dialog's transition can freely animate its own transform. -->
    <div
      class="absolute w-[min(24rem,calc(100vw-2rem))]"
      style="left: {anchor.x}px; top: {anchor.y}px; transform: translate(-50%, -50%);"
    >
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div
        class="w-full bg-brand-white border-[2.5px] border-brand-border-heavy rounded-3xl p-6 flex flex-col gap-4 origin-center"
        style="box-shadow: 0 10px 0 rgba(0,0,0,0.15);"
        role="dialog"
        aria-modal="true"
        aria-labelledby="end-room-title"
        tabindex="-1"
        onclick={(e) => e.stopPropagation()}
        transition:scale={{ duration: 180, start: 0.85, easing: backOut }}
      >
      <h2 id="end-room-title" class="text-xl font-bold">Cancel this room?</h2>
      <p class="text-sm font-semibold text-brand-text-mid">
        All players will be disconnected. This can't be undone.
      </p>
      {#if error}
        <p class="text-sm font-semibold text-red-600">{error}</p>
      {/if}
      <div class="flex gap-3 justify-end mt-2">
        <button
          use:pressPhysics={'ghost'}
          type="button"
          onclick={closeModal}
          disabled={pending}
          class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-sm font-bold disabled:opacity-50 cursor-pointer"
        >
          Keep playing
        </button>
        <button
          use:pressPhysics={'dark'}
          type="button"
          onclick={confirm}
          disabled={pending}
          class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-red-600 text-white text-sm font-bold disabled:opacity-50 cursor-pointer"
        >
          {pending ? 'Cancelling…' : 'Cancel room'}
        </button>
      </div>
      </div>
    </div>
  </div>
{/if}
