<script lang="ts">
  import { untrack, onMount } from 'svelte';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { adminApi } from '$lib/api/admin';
  import { toast } from '$lib/state/toast.svelte';
  import { RotateCw, Copy, Info } from '$lib/icons';
  import type {
    DeepHealthResponse,
    DeepHealthCheck,
    AdminStats,
    AuditEntry
  } from '$lib/api/types';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  // ── Reactive state ───────────────────────────────────────────────────────
  let health = $state<DeepHealthResponse | null>(
    untrack(() => data.health ?? null)
  );
  let stats = $state<AdminStats | null>(untrack(() => data.stats ?? null));
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

  // Per-check ring buffer of recent samples (last 20). Powers the sparkline.
  type Sample = { status: DeepHealthCheck['status']; latency: number };
  const HISTORY_CAP = 20;
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

  const STATS_STORAGE_KEY = 'admin:stats:baseline';

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

  // Linear normalization per-check over the buffer's own min/max — keeps
  // narrow-band variation (Postgres at 0.2–0.5 ms, RustFS at 150–180 ms)
  // visible instead of collapsing them to near-flat bars. When the buffer
  // is all equal (or single sample) we render a uniform mid-height bar.
  function barHeight(
    latency: number,
    minL: number,
    maxL: number
  ): number {
    if (latency < 0 || !Number.isFinite(latency)) return 2;
    if (maxL <= minL) return 12; // flat buffer → stable mid-height
    const scaled = (latency - minL) / (maxL - minL);
    return Math.max(2, Math.round(scaled * 22));
  }

  function barColor(status: Sample['status']): string {
    if (status === 'ok') return '#15803d'; // green-700
    if (status === 'degraded') return '#b91c1c'; // red-700
    return '#9ca3af'; // grey-400 for skipped
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
        { label: 'Total Packs', value: stats.total_packs, field: 'total_packs' as const },
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
          {@const okLatencies = samples
            .filter((s) => s.status !== 'skipped' && s.latency >= 0)
            .map((s) => s.latency)}
          {@const bufMin = okLatencies.length ? Math.min(...okLatencies) : 0}
          {@const bufMax = okLatencies.length ? Math.max(...okLatencies) : 0}
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

            <!-- Sparkline: 20-sample ring buffer. Bar height is linearly
                 normalized against this check's own min/max latency so
                 narrow bands (e.g. Postgres 0.2–0.5 ms) still show
                 variation. Bar color reflects the status at that sample. -->
            {#if samples.length > 0}
              <svg
                viewBox="0 0 120 24"
                class="w-full h-6"
                role="img"
                aria-label={`${c.label} recent health samples`}
              >
                {#each samples as s, idx}
                  {@const h = barHeight(s.latency, bufMin, bufMax)}
                  <rect
                    x={idx * 6}
                    y={24 - h}
                    width="4"
                    height={h}
                    fill={barColor(s.status)}
                    opacity={0.85}
                  />
                {/each}
              </svg>
            {/if}

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
