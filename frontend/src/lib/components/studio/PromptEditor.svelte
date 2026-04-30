<!-- frontend/src/lib/components/studio/PromptEditor.svelte -->
<script lang="ts">
  import { untrack } from 'svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Save } from '$lib/icons';
  import SentenceWithBlank from '$lib/games/_shared/SentenceWithBlank.svelte';
  import * as m from '$lib/paraglide/messages';

  const MAX_CHARS = 500;

  let {
    initialPrefix = '',
    initialSuffix = '',
    onSave,
  }: {
    initialPrefix?: string;
    initialSuffix?: string;
    onSave: (payload: { prefix: string; suffix: string }) => void;
  } = $props();

  let prefix = $state(untrack(() => initialPrefix ?? ''));
  let suffix = $state(untrack(() => initialSuffix ?? ''));

  const valid = $derived(
    (prefix.trim().length > 0 || suffix.trim().length > 0) &&
    prefix.length <= MAX_CHARS &&
    suffix.length <= MAX_CHARS
  );
</script>

<div class="flex flex-col gap-3">
  <div class="grid gap-2">
    <label class="flex flex-col gap-1">
      <span class="text-xs font-bold uppercase tracking-[0.18em] text-brand-text-mid">
        {m.studio_prompt_prefix_label()}
      </span>
      <textarea
        bind:value={prefix}
        rows={2}
        maxlength={MAX_CHARS}
        placeholder={m.studio_prompt_prefix_placeholder()}
        class="w-full rounded-lg border border-brand-border-heavy bg-brand-white p-3 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-ring"
      ></textarea>
    </label>
    <label class="flex flex-col gap-1">
      <span class="text-xs font-bold uppercase tracking-[0.18em] text-brand-text-mid">
        {m.studio_prompt_suffix_label()}
      </span>
      <textarea
        bind:value={suffix}
        rows={2}
        maxlength={MAX_CHARS}
        placeholder={m.studio_prompt_suffix_placeholder()}
        class="w-full rounded-lg border border-brand-border-heavy bg-brand-white p-3 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-ring"
      ></textarea>
    </label>
  </div>

  <div class="flex flex-col gap-1.5">
    <span class="text-xs font-bold uppercase tracking-[0.18em] text-brand-text-mid">
      {m.studio_prompt_preview_label()}
    </span>
    <div
      class="rounded-lg border border-brand-border bg-brand-surface px-3 py-3 text-brand-text leading-snug"
    >
      <SentenceWithBlank
        prefix={prefix}
        suffix={suffix}
        placeholder={m.studio_prompt_preview_placeholder()}
        size="md"
      />
    </div>
    {#if !valid && (prefix.length > 0 || suffix.length > 0)}
      <p class="text-xs text-brand-text-muted m-0">
        {m.studio_prompt_validation_required()}
      </p>
    {/if}
  </div>

  <button
    type="button"
    onclick={() => onSave({ prefix, suffix })}
    disabled={!valid}
    use:pressPhysics={'dark'}
    class="h-10 rounded-lg bg-primary text-primary-foreground text-sm font-medium disabled:opacity-50 inline-flex items-center justify-center gap-1.5"
  >
    <Save size={14} strokeWidth={2.5} />
    {m.studio_prompt_save_version()}
  </button>
</div>
