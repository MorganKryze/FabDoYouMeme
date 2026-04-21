<!-- frontend/src/lib/components/studio/LabHelpDrawer.svelte -->
<script lang="ts">
  import { XCircle } from '$lib/icons';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import * as m from '$lib/paraglide/messages';

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
    aria-label={m.studio_help_close()}
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
      <h2 id="lab-help-title" class="text-lg font-bold">{m.studio_help_title()}</h2>
      <button
        type="button"
        onclick={() => (open = false)}
        use:hoverEffect={'swap'}
        aria-label={m.studio_help_close()}
        class="p-1 rounded-full"
      >
        <XCircle size={18} strokeWidth={2.5} />
      </button>
    </div>
    <div class="px-5 py-4 flex flex-col gap-4 text-sm leading-relaxed">
      <section>
        <h3 class="font-semibold">{m.studio_help_what_heading()}</h3>
        <p class="text-brand-text-muted">
          {m.studio_help_what_body()}
        </p>
      </section>
      <section>
        <h3 class="font-semibold">{m.studio_help_versions_heading()}</h3>
        <p class="text-brand-text-muted">
          {m.studio_help_versions_body()}
        </p>
      </section>
      <section>
        <h3 class="font-semibold">{m.studio_help_visibility_heading()}</h3>
        <p class="text-brand-text-muted">
          {m.studio_help_visibility_prefix()}<b>{m.studio_help_visibility_private()}</b>{m.studio_help_visibility_middle()}<b>{m.studio_help_visibility_public()}</b>{m.studio_help_visibility_suffix()}
        </p>
      </section>
      <section>
        <h3 class="font-semibold">{m.studio_help_pack_game_heading()}</h3>
        <p class="text-brand-text-muted">
          {m.studio_help_pack_game_body()}
        </p>
      </section>
      <section>
        <h3 class="font-semibold">{m.studio_help_moderation_heading()}</h3>
        <p class="text-brand-text-muted">
          {m.studio_help_moderation_body()}
        </p>
      </section>
    </div>
  </div>
{/if}
