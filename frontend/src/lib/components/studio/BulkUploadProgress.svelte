<!-- frontend/src/lib/components/studio/BulkUploadProgress.svelte
     Live per-item status panel for bulk image / text imports.

     Why a dedicated panel instead of toast spam: a 83-file import that
     produces 30 failures used to surface as a single "Import failed (30):
     <first reason>" toast — every other failure reason was lost. This panel
     keeps every row visible with its individual outcome so the user can
     scroll, copy, and act on the precise per-file errors. The panel stays
     mounted after completion until the user dismisses it.

     Position: bottom-LEFT, not bottom-right. The studio's right column
     hosts ItemEditor / SingleItemAdd which is the user's primary work
     surface during a paste-and-rename loop; covering it with a status
     panel hid both the panel content (overlap) and the form they were
     trying to fill. Bottom-left rests over the navigator's empty space. -->
<script lang="ts">
  import { Loader2, CheckCircle, XCircle, X, Ban } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';

  export interface BulkUploadEntry {
    filename: string;
    status: 'pending' | 'success' | 'failed' | 'cancelled';
    reason?: string;
  }

  let {
    entries,
    running,
    onClose,
    onAbort
  }: {
    entries: BulkUploadEntry[];
    running: boolean;
    onClose: () => void;
    onAbort?: () => void;
  } = $props();

  const total = $derived(entries.length);
  const ok = $derived(entries.filter((e) => e.status === 'success').length);
  const ko = $derived(entries.filter((e) => e.status === 'failed').length);
  const cancelled = $derived(entries.filter((e) => e.status === 'cancelled').length);
  const done = $derived(ok + ko + cancelled);
  const percent = $derived(total === 0 ? 0 : Math.round((done / total) * 100));
</script>

<!-- bg-brand-white (opaque #FEFEFE) is the studio's solid-surface token —
     same one used by LabHelpDrawer for its right-side panel. The previous
     bg-brand-surface was rgba(...,0.82) and let the animated gradient
     bleed through; bg-brand-bg doesn't exist as a Tailwind token at all,
     so the panel rendered fully transparent.
     text-brand-text is also explicit because position:fixed pulls this
     subtree out of the (app) layout's color cascade. -->
<div
  role="status"
  aria-live="polite"
  aria-label={m.studio_bulk_panel_aria()}
  class="fixed bottom-4 left-4 z-50 w-[380px] max-h-[70vh] flex flex-col rounded-lg border-2 border-brand-border-heavy bg-brand-white text-brand-text shadow-2xl"
>
  <header class="flex items-center gap-2 px-4 py-2.5 border-b border-brand-border shrink-0 rounded-t-lg bg-brand-surface">
    <div class="flex-1 min-w-0">
      <p class="text-sm font-semibold truncate">
        {running
          ? m.studio_bulk_panel_title_running({ done, total })
          : m.studio_bulk_panel_title_done({ ok, ko: ko + cancelled })}
      </p>
      {#if running}
        <p class="text-[11px] text-brand-text-muted mt-0.5">{percent}%</p>
      {/if}
    </div>
    {#if running && onAbort}
      <button
        type="button"
        onclick={onAbort}
        class="px-2 py-1 rounded text-[11px] font-semibold uppercase tracking-wider text-red-600 dark:text-red-400 border border-red-300 dark:border-red-700 hover:bg-red-50 dark:hover:bg-red-950/30 transition-colors inline-flex items-center gap-1"
        title={m.studio_bulk_panel_abort_title()}
      >
        <Ban size={11} strokeWidth={2.5} />
        {m.studio_bulk_panel_abort()}
      </button>
    {/if}
    <button
      type="button"
      onclick={onClose}
      disabled={running}
      class="p-1 rounded text-brand-text-muted hover:text-brand-text disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
      aria-label={m.studio_bulk_panel_close_aria()}
      title={running ? m.studio_bulk_panel_close_blocked_title() : m.studio_bulk_panel_close_aria()}
    >
      <X size={14} strokeWidth={2.5} />
    </button>
  </header>

  {#if running}
    <div class="h-1 bg-muted overflow-hidden shrink-0">
      <div class="h-full bg-primary transition-all duration-200" style:width="{percent}%"></div>
    </div>
  {/if}

  <ul class="overflow-y-auto flex-1 divide-y divide-brand-border/40">
    {#each entries as entry, i (i)}
      <li class="flex items-start gap-2 px-3 py-2 text-xs">
        <span class="shrink-0 mt-0.5">
          {#if entry.status === 'pending'}
            <Loader2 size={14} strokeWidth={2.5} class="animate-spin text-brand-text-muted" />
          {:else if entry.status === 'success'}
            <CheckCircle size={14} strokeWidth={2.5} class="text-emerald-600" />
          {:else if entry.status === 'cancelled'}
            <Ban size={14} strokeWidth={2.5} class="text-brand-text-muted" />
          {:else}
            <XCircle size={14} strokeWidth={2.5} class="text-red-600" />
          {/if}
        </span>
        <div class="flex-1 min-w-0">
          <p class="truncate font-medium" title={entry.filename}>{entry.filename}</p>
          {#if entry.status === 'cancelled'}
            <p class="text-[11px] text-brand-text-muted mt-0.5">{m.studio_bulk_panel_status_cancelled()}</p>
          {:else if entry.reason}
            <p class="text-[11px] text-red-600 dark:text-red-400 mt-0.5 break-words" title={entry.reason}>
              {entry.reason}
            </p>
          {/if}
        </div>
      </li>
    {/each}
  </ul>
</div>
