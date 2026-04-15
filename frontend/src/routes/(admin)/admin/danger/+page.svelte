<!-- frontend/src/routes/(admin)/admin/danger/+page.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { adminApi } from '$lib/api/admin';
  import { toast } from '$lib/state/toast.svelte';
  import { AlertTriangle } from '$lib/icons';
  import type { DangerReport } from '$lib/api/types';

  // ── Action catalog ─────────────────────────────────────────────────────
  type ActionKey =
    | 'wipe-game-history'
    | 'wipe-packs-and-media'
    | 'wipe-invites'
    | 'wipe-sessions'
    | 'full-reset';

  type Action = {
    key: ActionKey;
    title: string;
    description: string;
    phrase: string;
    run: () => Promise<DangerReport>;
    umbrella?: boolean;
  };

  const actions: Action[] = [
    {
      key: 'wipe-game-history',
      title: 'Wipe game history',
      description:
        'Deletes every room, round, submission, and vote. Preserves packs, users, invites, and object storage.',
      phrase: 'wipe game history',
      run: () => adminApi.danger.wipeGameHistory()
    },
    {
      key: 'wipe-packs-and-media',
      title: 'Wipe packs and media',
      description:
        'Deletes every pack, every item, every pack media object, AND all game history that depends on them (rooms, rounds, submissions, votes). Empties the entire object storage bucket.',
      phrase: 'wipe packs and media',
      run: () => adminApi.danger.wipePacksAndMedia()
    },
    {
      key: 'wipe-invites',
      title: 'Wipe invites',
      description:
        'Deletes every invite token. Existing users stay logged in and keep their account — this only affects future sign-ups.',
      phrase: 'wipe invites',
      run: () => adminApi.danger.wipeInvites()
    },
    {
      key: 'wipe-sessions',
      title: 'Force logout everyone',
      description:
        "Deletes every session and every magic link token except your own. All other users are bounced to login on their next request. Your session is preserved.",
      phrase: 'force logout everyone',
      run: () => adminApi.danger.wipeSessions()
    },
    {
      key: 'full-reset',
      title: 'Full reset to first-boot state',
      description:
        'Runs every action above and additionally deletes all non-protected users. Preserves only: you, the bootstrap admin, the sentinel user, and game type seed data. Equivalent to a freshly booted container.',
      phrase: 'RESET TO FIRST BOOT',
      run: () => adminApi.danger.fullReset(),
      umbrella: true
    }
  ];

  // ── Last-run state (localStorage) ──────────────────────────────────────
  type LastRun = { at: number; summary: string };
  let lastRun = $state<Record<ActionKey, LastRun | null>>({
    'wipe-game-history': null,
    'wipe-packs-and-media': null,
    'wipe-invites': null,
    'wipe-sessions': null,
    'full-reset': null
  });

  const STORAGE_KEY = 'admin:danger:lastRun';

  function loadLastRun() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw) lastRun = JSON.parse(raw);
    } catch {
      /* ignore */
    }
  }

  function saveLastRun() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(lastRun));
    } catch {
      /* ignore */
    }
  }

  function summarize(report: DangerReport): string {
    const parts: string[] = [];
    const add = (n: number, label: string) => {
      if (n > 0) parts.push(`${n} ${label}`);
    };
    add(report.rooms_deleted, 'rooms');
    add(report.submissions_deleted, 'submissions');
    add(report.votes_deleted, 'votes');
    add(report.packs_deleted, 'packs');
    add(report.items_deleted, 'items');
    add(report.invites_deleted, 'invites');
    add(report.sessions_deleted, 'sessions');
    add(report.users_deleted, 'users');
    add(report.s3_objects_deleted, 'S3 objects');
    return parts.length === 0 ? 'nothing to delete' : parts.join(', ');
  }

  function formatAge(ms: number): string {
    const seconds = Math.floor(ms / 1000);
    if (seconds < 5) return 'just now';
    if (seconds < 60) return `${seconds}s ago`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ago`;
  }

  // ── Modal state ────────────────────────────────────────────────────────
  let modalOpen = $state(false);
  let modalAction = $state<Action | null>(null);
  let confirmationInput = $state('');
  let busy = $state(false);

  function openModal(action: Action) {
    modalAction = action;
    confirmationInput = '';
    modalOpen = true;
  }

  function closeModal() {
    modalOpen = false;
    modalAction = null;
    confirmationInput = '';
  }

  async function runAction() {
    if (!modalAction) return;
    if (confirmationInput !== modalAction.phrase) return;
    busy = true;
    try {
      const report = await modalAction.run();
      const summary = summarize(report);
      lastRun[modalAction.key] = { at: Date.now(), summary };
      saveLastRun();
      toast.show(`Deleted: ${summary}`, 'success');
      closeModal();
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Unknown error';
      toast.show(`Failed: ${msg}`, 'error');
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    loadLastRun();
  });
</script>

<svelte:head>
  <title>Danger Zone — FabDoYouMeme</title>
</svelte:head>

<div class="p-6 flex flex-col gap-6">
  <div class="rounded-xl border border-red-300 bg-red-50 text-red-900 px-4 py-3 flex items-start gap-3">
    <AlertTriangle size={18} strokeWidth={2.5} class="mt-0.5 shrink-0" />
    <div>
      <h1 class="text-lg font-bold">Danger zone</h1>
      <p class="text-sm mt-1">
        These actions permanently delete data and cannot be undone. They are
        disabled in production. Use them in dev and preprod to reset the system
        to a known state.
      </p>
    </div>
  </div>

  <div class="flex flex-col gap-4">
    {#each actions as action (action.key)}
      {@const last = lastRun[action.key]}
      <div
        class={'rounded-xl bg-brand-white p-5 flex flex-col gap-3 ' +
          (action.umbrella
            ? 'border-2 border-red-500'
            : 'border border-brand-border border-l-4 border-l-red-500')}
      >
        <div class="flex items-start justify-between gap-4">
          <div class="flex-1">
            <h2 class="text-base font-semibold">{action.title}</h2>
            <p class="text-sm text-brand-text-muted mt-1">{action.description}</p>
            {#if last}
              <p class="text-xs text-brand-text-muted mt-2">
                Last run: {formatAge(Date.now() - last.at)} ({last.summary})
              </p>
            {/if}
          </div>
          <button
            type="button"
            onclick={() => openModal(action)}
            class={'shrink-0 inline-flex items-center gap-1.5 h-9 px-4 rounded-full text-sm font-semibold text-white transition-colors ' +
              (action.umbrella
                ? 'bg-red-700 hover:bg-red-800'
                : 'bg-red-600 hover:bg-red-700')}
          >
            {action.umbrella ? 'RESET TO FIRST BOOT' : action.title}
          </button>
        </div>
      </div>
    {/each}
  </div>
</div>

{#if modalOpen && modalAction}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="danger-modal-title"
  >
    <div class="w-full max-w-md rounded-xl bg-brand-white border border-red-300 shadow-xl p-6 flex flex-col gap-4">
      <h3 id="danger-modal-title" class="text-lg font-bold text-red-900">
        Confirm: {modalAction.title}
      </h3>
      <p class="text-sm text-brand-text-muted">{modalAction.description}</p>
      <label class="text-sm font-medium flex flex-col gap-1">
        Type <code class="px-1.5 py-0.5 rounded bg-muted/50 font-mono text-xs">{modalAction.phrase}</code> to confirm:
        <input
          type="text"
          bind:value={confirmationInput}
          class="h-9 px-3 rounded-lg border border-brand-border bg-brand-white text-sm font-mono focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-red-300"
          autocomplete="off"
          spellcheck={false}
          disabled={busy}
        />
      </label>
      <div class="flex justify-end gap-2">
        <button
          type="button"
          onclick={closeModal}
          disabled={busy}
          class="h-9 px-4 rounded-full border border-brand-border text-sm font-medium disabled:opacity-50"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={runAction}
          disabled={busy || confirmationInput !== modalAction.phrase}
          class="h-9 px-4 rounded-full bg-red-600 hover:bg-red-700 text-white text-sm font-semibold disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {busy ? 'Working…' : 'Confirm'}
        </button>
      </div>
    </div>
  </div>
{/if}
