<script lang="ts">
  import { untrack, onMount } from 'svelte';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { adminApi } from '$lib/api/admin';
  import { toast } from '$lib/state/toast.svelte';
  import {
    RotateCw,
    Copy,
    Info,
    Package,
    ImageIcon,
    Server
  } from '$lib/icons';
  import type {
    DeepHealthResponse,
    DeepHealthCheck,
    AdminStats,
    AdminStorageStats,
    AuditEntry
  } from '$lib/api/types';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  // ── Reactive state ───────────────────────────────────────────────────────
  let health = $state<DeepHealthResponse | null>(
    untrack(() => data.health ?? null)
  );
  let stats = $state<AdminStats | null>(untrack(() => data.stats ?? null));
  let storageStats = $state<AdminStorageStats | null>(
    untrack(() => data.storage ?? null)
  );
  let audit = $state<AuditEntry[]>(untrack(() => data.audit ?? []));
  let refreshing = $state(false);
  let lastRefreshAt = $state<number>(Date.now());
  let now = $state<number>(Date.now());
  let pollDelayMs = $state(30_000);
  let networkDegraded = $state(false);

  // Snapshot of stats from the previous page visit, loaded once on mount.
  // Stays stable across polls on the same visit — the delta signal should
  // answer "what changed since I last looked", not "what changed in the
  // last 30 seconds".
  let statsBaseline = $state<AdminStats | null>(null);

  // Per-check ring buffer of recent samples — powers the uptime rail.
  // 30 slots × 30s poll = ~15 min of visible history.
  type Sample = { status: DeepHealthCheck['status']; latency: number };
  const HISTORY_CAP = 30;
  let history = $state<Record<string, Sample[]>>({
    postgres: [],
    rustfs: [],
    smtp: []
  });

  // ── Static config ────────────────────────────────────────────────────────
  const checks = [
    { key: 'postgres', label: 'Postgres' },
    { key: 'rustfs', label: 'RustFS (S3)' },
    { key: 'smtp', label: 'SMTP' }
  ] as const;

  // Runbook hints shown inline when a check flips to degraded. Kept terse —
  // this is the "don't alt-tab during an incident" shortcut.
  const runbook: Record<string, string> = {
    postgres:
      'Likely causes: connection pool exhausted, slow query holding locks, or the DB container restarting. Check `docker compose logs postgres` and `pg_stat_activity`.',
    rustfs:
      'Likely causes: RustFS container crashed, network between backend ↔ pangolin broken, or credentials rotated. Verify RUSTFS_ENDPOINT reachable and the bucket exists.',
    smtp:
      'Likely causes: SMTP provider rate-limiting, credentials rotated, or relay DNS/TLS misconfig. Dev stacks should hit Mailpit; check SMTP_HOST and outbox toast state.'
  };

  // Versioned key: bump the suffix whenever the AdminStats shape changes
  // so stale baselines from a previous schema don't produce NaN deltas.
  // v2: swapped total_packs → games_played.
  const STATS_STORAGE_KEY = 'admin:stats:baseline:v2';

  // ── Helpers ──────────────────────────────────────────────────────────────
  function formatAge(ms: number): string {
    const seconds = Math.floor(ms / 1000);
    if (seconds < 5) return 'just now';
    if (seconds < 60) return `${seconds}s ago`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ago`;
  }

  function recordSample(snap: DeepHealthResponse) {
    const next: Record<string, Sample[]> = { ...history };
    for (const c of checks) {
      const info = snap.checks[c.key];
      const sample: Sample = {
        status: info.status,
        latency: info.latency_ms ?? 0
      };
      const prev = next[c.key] ?? [];
      const rolled = [...prev, sample];
      if (rolled.length > HISTORY_CAP) rolled.shift();
      next[c.key] = rolled;
    }
    history = next;
  }

  // Classic uptime rail: color-only status, uniform bar height. Latency is
  // shown as text above the rail and inside each bar's <title> tooltip, so
  // the rail itself only needs to answer "was it up?" at a glance.
  function barColor(status: Sample['status']): string {
    if (status === 'ok') return '#22c55e'; // green-500 — readable on light & dark
    if (status === 'degraded') return '#ef4444'; // red-500
    return '#94a3b8'; // slate-400 for skipped
  }

  // Humanize byte counts for the storage widget. Under 1 MB shows KB to one
  // decimal so empty/seed installs don't read as a flat "0 MB".
  function formatBytes(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes <= 0) return '0 MB';
    const kb = bytes / 1024;
    if (kb < 1024) return `${kb.toFixed(1)} KB`;
    const mb = kb / 1024;
    if (mb < 1024) return `${mb.toFixed(mb < 10 ? 1 : 0)} MB`;
    const gb = mb / 1024;
    return `${gb.toFixed(gb < 10 ? 2 : 1)} GB`;
  }

  // Sub-ms latencies get one decimal ("0.3 ms"); anything >= 1 is rounded
  // to int. Guards against the old "postgres has no response time" bug
  // where float 0.0 was rendered as nothing.
  function formatLatency(ms: number | undefined): string {
    if (ms === undefined || ms === null) return '';
    if (ms < 1) return `${ms.toFixed(1)} ms`;
    return `${Math.round(ms)} ms`;
  }

  // HH:MM:SS in the user's locale — the audit feed needs precise
  // timestamps so operators can correlate with upstream logs.
  function formatTime(iso: string): string {
    const d = new Date(iso);
    return d.toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    });
  }

  // Flatten the `changes` JSON into "k: v, k2: v2". Primitive values only;
  // nested objects are rendered compactly via JSON.stringify. Returns an
  // empty string for null/empty — the UI then hides the arrow segment.
  function formatChanges(changes: unknown): string {
    if (!changes || typeof changes !== 'object') return '';
    const obj = changes as Record<string, unknown>;
    const pairs = Object.entries(obj).map(([k, v]) => {
      if (v === null || typeof v !== 'object') return `${k}: ${v}`;
      return `${k}: ${JSON.stringify(v)}`;
    });
    return pairs.join(', ');
  }

  // ── Stats delta (item 3) ─────────────────────────────────────────────────
  function loadBaseline() {
    try {
      const raw = localStorage.getItem(STATS_STORAGE_KEY);
      if (raw) statsBaseline = JSON.parse(raw) as AdminStats;
    } catch {
      /* ignore malformed storage */
    }
    if (stats) {
      try {
        localStorage.setItem(STATS_STORAGE_KEY, JSON.stringify(stats));
      } catch {
        /* storage full / disabled — non-fatal */
      }
    }
  }

  function delta(field: keyof AdminStats): number | null {
    if (!stats || !statsBaseline) return null;
    const d = stats[field] - statsBaseline[field];
    return d === 0 ? null : d;
  }

  function deltaClass(d: number | null): string {
    if (d === null) return '';
    return d > 0 ? 'text-green-700' : 'text-red-700';
  }

  // ── Audit log formatting (item 4) ────────────────────────────────────────
  function formatAction(a: string): string {
    return a.replace(/_/g, ' ');
  }

  // ── Refresh + SWR-style polling (item 8) ─────────────────────────────────
  async function fetchAll(): Promise<boolean> {
    let ok = true;
    try {
      const [h, s] = await Promise.all([
        adminApi.getHealth(),
        adminApi.getStats()
      ]);
      health = h;
      stats = s;
      lastRefreshAt = Date.now();
      recordSample(h);
    } catch {
      ok = false;
    }
    // Storage stats walk the RustFS bucket — keep them off the hot path so
    // a slow/unreachable RustFS never stalls the health+stats refresh or
    // trips the network-degraded backoff. Failure silently retains the
    // previous snapshot, matching the audit feed behaviour below.
    try {
      storageStats = await adminApi.getStorageStats();
    } catch {
      /* keep previous snapshot on failure */
    }
    // Audit refresh is best-effort and never blocks: operators care most
    // about health + stats freshness, the audit feed is low-frequency.
    try {
      const a = await adminApi.listAudit(10);
      audit = a.data;
    } catch {
      /* keep previous feed on failure */
    }
    return ok;
  }

  async function refreshHealth() {
    if (refreshing) return;
    refreshing = true;
    const ok = await fetchAll();
    refreshing = false;
    if (!ok) {
      toast.show('Health check failed.', 'error');
    } else {
      // Manual refresh always resets the backoff — the operator is clearly
      // expecting things to work now.
      networkDegraded = false;
      pollDelayMs = 30_000;
    }
  }

  // ── Lifecycle ────────────────────────────────────────────────────────────
  onMount(() => {
    loadBaseline();

    // Seed the sparkline with the SSR snapshot so the first card has one bar
    // instead of an empty rail.
    if (health) recordSample(health);

    let timer: ReturnType<typeof setTimeout>;
    const schedule = () => {
      timer = setTimeout(async () => {
        const ok = await fetchAll();
        if (ok) {
          networkDegraded = false;
          pollDelayMs = 30_000;
        } else {
          networkDegraded = true;
          // Exponential backoff, capped at 5 minutes. Protects the backend
          // from a thundering dashboard during a real incident.
          pollDelayMs = Math.min(pollDelayMs * 2, 300_000);
        }
        schedule();
      }, pollDelayMs);
    };
    schedule();

    const tick = setInterval(() => {
      now = Date.now();
    }, 1_000);

    return () => {
      clearTimeout(timer);
      clearInterval(tick);
    };
  });

  async function copyError(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      toast.show('Copied error to clipboard.', 'success');
    } catch {
      toast.show('Copy failed — select manually.', 'error');
    }
  }

  // ── Derived ──────────────────────────────────────────────────────────────
  const refreshAge = $derived(formatAge(now - lastRefreshAt));
</script>

<svelte:head>
  <title>Admin Dashboard — FabDoYouMeme</title>
</svelte:head>

<div class="p-6 flex flex-col gap-6" use:reveal>
  <h1 class="text-2xl font-bold">Dashboard</h1>

  {#if networkDegraded}
    <div
      class="rounded-xl border border-amber-300 bg-amber-50 text-amber-900 text-sm px-4 py-2"
      role="status"
    >
      Network degraded — backing off to every {Math.round(pollDelayMs / 1000)}s.
    </div>
  {/if}

  {#if stats}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      {#each [
        { label: 'Active Rooms', value: stats.active_rooms, field: 'active_rooms' as const },
        { label: 'Total Users', value: stats.total_users, field: 'total_users' as const },
        { label: 'Games Played', value: stats.games_played, field: 'games_played' as const },
        { label: 'Pending Invites', value: stats.pending_invites, field: 'pending_invites' as const },
      ] as card, i}
        {@const d = delta(card.field)}
        <div
          use:reveal={{ delay: i }}
          use:physCard
          class="rounded-xl border border-brand-border bg-brand-white p-4"
        >
          <p class="text-sm text-brand-text-muted">{card.label}</p>
          <div class="flex items-baseline gap-2 mt-1">
            <p class="text-3xl font-bold">{card.value}</p>
            {#if d !== null}
              <span class={'text-xs font-semibold ' + deltaClass(d)}>
                {d > 0 ? '+' : ''}{d}
              </span>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}

  {#if storageStats}
    <section class="flex flex-col gap-3">
      <h2 class="text-base font-semibold">Storage</h2>
      <div
        use:reveal
        use:physCard
        class="rounded-xl border border-brand-border bg-brand-white p-4 grid grid-cols-1 sm:grid-cols-3 gap-4 sm:divide-x sm:divide-brand-border"
      >
        {#each [
          {
            label: 'Packs',
            value: storageStats.packs_count.toLocaleString(),
            Icon: Package,
          },
          {
            label: 'Assets',
            value: storageStats.assets_count.toLocaleString(),
            Icon: ImageIcon,
          },
          {
            label: 'Used on RustFS',
            value: formatBytes(storageStats.total_bytes),
            Icon: Server,
          },
        ] as item}
          {@const IconCmp = item.Icon}
          <div class="flex items-center gap-3 sm:px-4 first:sm:pl-0 last:sm:pr-0">
            <div
              class="shrink-0 inline-flex items-center justify-center h-10 w-10 rounded-lg bg-muted/40 text-brand-text-muted"
              aria-hidden="true"
            >
              <IconCmp size={20} strokeWidth={2} />
            </div>
            <div class="flex flex-col">
              <p class="text-xs uppercase tracking-wider text-brand-text-muted">
                {item.label}
              </p>
              <p class="text-xl font-bold tabular-nums leading-tight">
                {item.value}
              </p>
            </div>
          </div>
        {/each}
      </div>
    </section>
  {/if}

  {#if health}
    <section class="flex flex-col gap-3">
      <div class="flex items-center gap-3 flex-wrap">
        <h2 class="text-base font-semibold">System health</h2>
        <span class="text-xs text-brand-text-muted" aria-live="polite">
          Refreshed {refreshAge}
        </span>
        <button
          type="button"
          onclick={refreshHealth}
          disabled={refreshing}
          use:pressPhysics={'ghost'}
          use:hoverEffect={'swap'}
          class="inline-flex items-center gap-1.5 h-8 px-3 rounded-full border border-brand-border text-xs font-medium disabled:opacity-50 disabled:cursor-not-allowed ml-auto"
        >
          <RotateCw
            size={12}
            strokeWidth={2.5}
            class={refreshing ? 'animate-spin' : ''}
          />
          {refreshing ? 'Refreshing…' : 'Refresh'}
        </button>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
        {#each checks as c}
          {@const info = health.checks[c.key]}
          {@const samples = history[c.key] ?? []}
          {@const slotOffset = HISTORY_CAP - samples.length}
          <div
            class="rounded-xl border border-brand-border bg-brand-white p-4 flex flex-col gap-2"
          >
            <p
              class="text-xs uppercase tracking-wider text-brand-text-muted"
            >
              {c.label}
            </p>
            <p
              class={'text-xl font-bold ' +
                (info.status === 'ok'
                  ? 'text-green-700'
                  : info.status === 'degraded'
                    ? 'text-red-700'
                    : 'text-brand-text-muted')}
            >
              {info.status}
            </p>
            {#if info.status !== 'skipped'}
              <p class="text-xs text-brand-text-muted">
                {formatLatency(info.latency_ms)}
              </p>
            {/if}

            <!-- Classic uptime rail: HISTORY_CAP fixed slots, uniform
                 height, color-only status. Unfilled slots render as
                 low-opacity ghost bars so the rail is the same width from
                 first paint. Native <title> gives free hover tooltips.
                 Oldest is left, newest right. -->
            <div class="mt-1 flex flex-col gap-1">
              <svg
                viewBox={`0 0 ${HISTORY_CAP * 7 - 2} 28`}
                preserveAspectRatio="none"
                class="w-full h-7 text-brand-text"
                role="img"
                aria-label={`${c.label} recent health samples`}
              >
                {#each Array(HISTORY_CAP) as _, idx}
                  {@const s = idx >= slotOffset ? samples[idx - slotOffset] : null}
                  <rect
                    x={idx * 7}
                    y={0}
                    width={5}
                    height={28}
                    rx={1.5}
                    ry={1.5}
                    fill={s ? barColor(s.status) : 'currentColor'}
                    fill-opacity={s ? 0.92 : 0.1}
                  >
                    {#if s}
                      <title>{s.status} · {formatLatency(s.latency)}</title>
                    {/if}
                  </rect>
                {/each}
              </svg>
              <div class="flex justify-between text-[10px] uppercase tracking-wider text-brand-text-muted">
                <span>~15 min ago</span>
                <span>now</span>
              </div>
            </div>

            {#if info.error}
              <div class="relative mt-1">
                <button
                  type="button"
                  onclick={() => copyError(info.error ?? '')}
                  title="Copy error"
                  aria-label="Copy error to clipboard"
                  class="absolute top-1 right-1 z-10 inline-flex items-center justify-center h-6 w-6 rounded border border-red-200 bg-white/80 hover:bg-white text-red-700"
                >
                  <Copy size={12} strokeWidth={2.5} />
                </button>
                <!-- break-all so long tokens (URLs, request IDs) wrap instead
                     of overflowing the card. Monospace for scannability; the
                     same class is used by the terminal widgets elsewhere. -->
                <pre
                  class="text-[11px] leading-snug text-red-700 bg-red-50 border border-red-200 rounded p-2 pr-8 whitespace-pre-wrap break-all font-mono max-h-40 overflow-y-auto"
                >{info.error}</pre>
              </div>
            {/if}

            {#if info.status === 'degraded'}
              <div
                class="flex items-start gap-1.5 text-[11px] leading-snug text-amber-900 bg-amber-50 border border-amber-200 rounded p-2"
              >
                <Info size={12} strokeWidth={2.5} class="mt-0.5 shrink-0" />
                <span>{runbook[c.key]}</span>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    </section>
  {/if}

  <section class="flex flex-col gap-3">
    <h2 class="text-base font-semibold">Recent Activity</h2>
    {#if audit.length === 0}
      <p class="text-sm text-brand-text-muted">No recent activity.</p>
    {:else}
      <ul
        class="flex flex-col divide-y divide-brand-border rounded-xl border border-brand-border bg-brand-white"
      >
        {#each audit as entry}
          {@const changesText = formatChanges(entry.changes)}
          {@const target =
            entry.resource_label || entry.resource_id.slice(0, 8) || '—'}
          <li
            class="flex items-center gap-3 px-4 py-2 text-sm"
            title={`${new Date(entry.created_at).toLocaleString()} — ${formatAge(
              now - new Date(entry.created_at).getTime()
            )}`}
          >
            <span
              class="font-mono text-xs text-brand-text-muted shrink-0 w-20"
            >
              {formatTime(entry.created_at)}
            </span>
            <span
              class="font-medium px-1.5 py-0.5 rounded bg-muted/50 text-xs shrink-0"
            >
              {formatAction(entry.action)}
            </span>
            {#if entry.resource_type}
              <span class="text-xs text-brand-text-muted shrink-0">
                {entry.resource_type}
              </span>
            {/if}
            <span class="font-medium truncate">{target}</span>
            {#if changesText}
              <span class="text-brand-text-muted shrink-0">→</span>
              <span class="text-xs text-brand-text-muted truncate">
                {changesText}
              </span>
            {/if}
            <span
              class="ml-auto text-xs text-brand-text-muted shrink-0"
            >
              {entry.admin_username || '—'}
            </span>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>
