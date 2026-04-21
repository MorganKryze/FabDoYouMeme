<!-- frontend/src/routes/(admin)/admin/danger/+page.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { adminApi } from '$lib/api/admin';
  import { toast } from '$lib/state/toast.svelte';
  import { AlertTriangle } from '$lib/icons';
  import type { DangerReport } from '$lib/api/types';
  import * as m from '$lib/paraglide/messages';

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
      title: m.admin_danger_action_wipe_game_history_title(),
      description: m.admin_danger_action_wipe_game_history_description(),
      phrase: m.admin_danger_action_wipe_game_history_phrase(),
      run: () => adminApi.danger.wipeGameHistory()
    },
    {
      key: 'wipe-packs-and-media',
      title: m.admin_danger_action_wipe_packs_title(),
      description: m.admin_danger_action_wipe_packs_description(),
      phrase: m.admin_danger_action_wipe_packs_phrase(),
      run: () => adminApi.danger.wipePacksAndMedia()
    },
    {
      key: 'wipe-invites',
      title: m.admin_danger_action_wipe_invites_title(),
      description: m.admin_danger_action_wipe_invites_description(),
      phrase: m.admin_danger_action_wipe_invites_phrase(),
      run: () => adminApi.danger.wipeInvites()
    },
    {
      key: 'wipe-sessions',
      title: m.admin_danger_action_wipe_sessions_title(),
      description: m.admin_danger_action_wipe_sessions_description(),
      phrase: m.admin_danger_action_wipe_sessions_phrase(),
      run: () => adminApi.danger.wipeSessions()
    },
    {
      key: 'full-reset',
      title: m.admin_danger_action_full_reset_title(),
      description: m.admin_danger_action_full_reset_description(),
      phrase: m.admin_danger_action_full_reset_phrase(),
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
    add(report.rooms_deleted, m.admin_danger_unit_rooms());
    add(report.submissions_deleted, m.admin_danger_unit_submissions());
    add(report.votes_deleted, m.admin_danger_unit_votes());
    add(report.packs_deleted, m.admin_danger_unit_packs());
    add(report.items_deleted, m.admin_danger_unit_items());
    add(report.invites_deleted, m.admin_danger_unit_invites());
    add(report.sessions_deleted, m.admin_danger_unit_sessions());
    add(report.users_deleted, m.admin_danger_unit_users());
    add(report.s3_objects_deleted, m.admin_danger_unit_s3_objects());
    return parts.length === 0 ? m.admin_danger_nothing_deleted() : parts.join(', ');
  }

  function formatAge(ms: number): string {
    const seconds = Math.floor(ms / 1000);
    if (seconds < 5) return m.admin_age_just_now();
    if (seconds < 60) return m.admin_age_seconds({ seconds });
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return m.admin_age_minutes({ minutes });
    const hours = Math.floor(minutes / 60);
    return m.admin_age_hours({ hours });
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
      toast.show(m.admin_danger_toast_deleted({ summary }), 'success');
      closeModal();
    } catch (err) {
      const msg = err instanceof Error ? err.message : m.admin_danger_unknown_error();
      toast.show(m.admin_danger_toast_failed({ error: msg }), 'error');
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    loadLastRun();
  });
</script>

<svelte:head>
  <title>{m.admin_danger_page_title()}</title>
</svelte:head>

<div class="p-6 flex flex-col gap-6">
  <div class="rounded-xl border border-red-300 bg-red-50 text-red-900 px-4 py-3 flex items-start gap-3">
    <AlertTriangle size={18} strokeWidth={2.5} class="mt-0.5 shrink-0" />
    <div>
      <h1 class="text-lg font-bold">{m.admin_danger_heading()}</h1>
      <p class="text-sm mt-1">
        {m.admin_danger_intro()}
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
                {m.admin_danger_last_run({ age: formatAge(Date.now() - last.at), summary: last.summary })}
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
            {action.umbrella ? m.admin_danger_reset_button() : action.title}
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
        {m.admin_danger_confirm_title({ title: modalAction.title })}
      </h3>
      <p class="text-sm text-brand-text-muted">{modalAction.description}</p>
      <label class="text-sm font-medium flex flex-col gap-1">
        {m.admin_danger_confirm_prompt_prefix()} <code class="px-1.5 py-0.5 rounded bg-muted/50 font-mono text-xs">{modalAction.phrase}</code> {m.admin_danger_confirm_prompt_suffix()}
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
          {m.admin_danger_cancel()}
        </button>
        <button
          type="button"
          onclick={runAction}
          disabled={busy || confirmationInput !== modalAction.phrase}
          class="h-9 px-4 rounded-full bg-red-600 hover:bg-red-700 text-white text-sm font-semibold disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {busy ? m.admin_danger_working() : m.admin_danger_confirm_button()}
        </button>
      </div>
    </div>
  </div>
{/if}
