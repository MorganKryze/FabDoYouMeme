<!-- frontend/src/lib/components/studio/LabWelcome.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { createPack } from '$lib/api/studio';
  import { toast } from '$lib/state/toast.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Plus } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';

  let creating = $state(false);

  async function createFirst() {
    creating = true;
    try {
      const pack = await createPack({ name: m.studio_welcome_first_pack_name() });
      studio.packs = [...studio.packs, pack];
      studio.selectPack(pack.id);
      toast.show(m.studio_toast_pack_created(), 'success');
    } catch {
      toast.show(m.studio_toast_pack_create_failed(), 'error');
    } finally {
      creating = false;
    }
  }
</script>

<div class="flex flex-col items-center justify-center h-full gap-4 px-6 text-center">
  <div class="text-4xl" aria-hidden="true">🎨</div>
  <h2 class="text-xl font-bold">{m.studio_welcome_heading()}</h2>
  <p class="text-sm text-brand-text-muted max-w-sm">
    {m.studio_welcome_body()}
  </p>
  <div class="flex items-center gap-2 text-xs font-medium">
    <span
      class="px-3 py-1.5 rounded-full bg-brand-white border-2 border-brand-border-heavy"
      >{m.studio_welcome_step_pack()}</span
    >
    <span aria-hidden="true">→</span>
    <span
      class="px-3 py-1.5 rounded-full bg-brand-white border-2 border-brand-border-heavy"
      >{m.studio_welcome_step_items()}</span
    >
    <span aria-hidden="true">→</span>
    <span
      class="px-3 py-1.5 rounded-full bg-brand-white border-2 border-brand-border-heavy"
      >{m.studio_welcome_step_rooms()}</span
    >
  </div>
  <button
    type="button"
    onclick={createFirst}
    disabled={creating}
    use:pressPhysics={'dark'}
    class="h-10 px-5 rounded-lg border border-brand-border bg-primary text-primary-foreground text-sm font-semibold inline-flex items-center gap-1.5 disabled:opacity-50 mt-2"
  >
    <Plus size={14} strokeWidth={2.5} />
    {creating ? m.studio_welcome_creating() : m.studio_welcome_create_first()}
  </button>
</div>
