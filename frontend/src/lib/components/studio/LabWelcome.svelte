<!-- frontend/src/lib/components/studio/LabWelcome.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { createPack } from '$lib/api/studio';
  import { toast } from '$lib/state/toast.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Plus } from '$lib/icons';

  let creating = $state(false);

  async function createFirst() {
    creating = true;
    try {
      const pack = await createPack({ name: 'My first pack' });
      studio.packs = [...studio.packs, pack];
      studio.selectPack(pack.id);
      toast.show('Pack created.', 'success');
    } catch {
      toast.show('Could not create pack.', 'error');
    } finally {
      creating = false;
    }
  }
</script>

<div class="flex flex-col items-center justify-center h-full gap-4 px-6 text-center">
  <div class="text-4xl" aria-hidden="true">🎨</div>
  <h2 class="text-xl font-bold">Welcome to the Lab</h2>
  <p class="text-sm text-brand-text-muted max-w-sm">
    Packs are your personal collections of game content — images or prompts your games
    draw from.
  </p>
  <div class="flex items-center gap-2 text-xs font-medium">
    <span
      class="px-3 py-1.5 rounded-full bg-brand-white border-2 border-brand-border-heavy"
      >Pack</span
    >
    <span aria-hidden="true">→</span>
    <span
      class="px-3 py-1.5 rounded-full bg-brand-white border-2 border-brand-border-heavy"
      >Items</span
    >
    <span aria-hidden="true">→</span>
    <span
      class="px-3 py-1.5 rounded-full bg-brand-white border-2 border-brand-border-heavy"
      >Rooms</span
    >
  </div>
  <button
    type="button"
    onclick={createFirst}
    disabled={creating}
    use:pressPhysics={'dark'}
    class="h-10 px-5 rounded-lg bg-primary text-primary-foreground text-sm font-semibold inline-flex items-center gap-1.5 disabled:opacity-50 mt-2"
  >
    <Plus size={14} strokeWidth={2.5} />
    {creating ? 'Creating…' : 'Create your first pack'}
  </button>
</div>
