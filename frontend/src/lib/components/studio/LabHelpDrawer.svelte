<!-- frontend/src/lib/components/studio/LabHelpDrawer.svelte -->
<script lang="ts">
  import { XCircle } from '$lib/icons';
  import { hoverEffect } from '$lib/actions/hoverEffect';

  let { open = $bindable(false) }: { open: boolean } = $props();

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') open = false;
  }
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
  <!-- Scrim -->
  <button
    type="button"
    aria-label="Close help"
    class="fixed inset-0 bg-black/30 z-40"
    onclick={() => (open = false)}
  ></button>

  <!-- Drawer -->
  <div
    class="fixed right-0 top-0 bottom-0 w-[min(28rem,100vw)] bg-brand-white z-50 shadow-xl border-l border-brand-border-heavy overflow-y-auto"
    role="dialog"
    aria-modal="true"
    aria-labelledby="lab-help-title"
    tabindex="-1"
  >
    <div
      class="flex items-center justify-between px-5 py-4 border-b border-brand-border"
    >
      <h2 id="lab-help-title" class="text-lg font-bold">How packs work</h2>
      <button
        type="button"
        onclick={() => (open = false)}
        use:hoverEffect={'swap'}
        aria-label="Close help"
        class="p-1 rounded-full"
      >
        <XCircle size={18} strokeWidth={2.5} />
      </button>
    </div>
    <div class="px-5 py-4 flex flex-col gap-4 text-sm leading-relaxed">
      <section>
        <h3 class="font-semibold">What a pack is</h3>
        <p class="text-brand-text-muted">
          A pack is a named collection of items — images or text prompts that a
          game draws from.
        </p>
      </section>
      <section>
        <h3 class="font-semibold">Items &amp; versions</h3>
        <p class="text-brand-text-muted">
          Each item keeps its version history. Upload a new version anytime;
          revert to a previous one if a change didn't land well.
        </p>
      </section>
      <section>
        <h3 class="font-semibold">Visibility</h3>
        <p class="text-brand-text-muted">
          Packs start <b>private</b> — only you can pick them. Publish to
          <b>public</b> and any Maker can use your pack in their rooms. Publishing
          notifies admins for moderation.
        </p>
      </section>
      <section>
        <h3 class="font-semibold">Pack → Game</h3>
        <p class="text-brand-text-muted">
          When you host a room, you pick one of your packs. The pack must match
          the game type's item kind (image packs go with image games, text with
          text).
        </p>
      </section>
      <section>
        <h3 class="font-semibold">Moderation</h3>
        <p class="text-brand-text-muted">
          Admins can flag or ban public packs. Flagged packs stop appearing in
          public picks until reviewed.
        </p>
      </section>
    </div>
  </div>
{/if}
