import type { WsMessage } from '$lib/api/types';
import { guest } from './guest.svelte';

type WsStatus = 'connected' | 'reconnecting' | 'error' | 'closed';

class WsState {
  status = $state<WsStatus>('closed');
  retryCount = $state(0);

  #ws: WebSocket | null = null;
  #handlers = new Map<string, ((data: unknown) => void)[]>();
  #roomCode: string | null = null;
  #retryTimer: ReturnType<typeof setTimeout> | null = null;
  #pingTimer: ReturnType<typeof setInterval> | null = null;
  #pongTimeout: ReturnType<typeof setTimeout> | null = null;
  #pongUnsub: (() => void) | null = null;
  #deliberateClose = false;
  // Timestamp (ms) of the last visibilitychange→hidden. Used to decide whether
  // a wake-up was a quick desktop tab switch (leave the socket alone) or a
  // long mobile background where iOS may have frozen JS past the server's
  // reconnect grace window (force a fresh socket on resume).
  #hiddenAt: number | null = null;
  #lifecycleBound = false;

  /** Connect to a room's WebSocket. */
  connect(roomCode: string) {
    this.#roomCode = roomCode;
    this.retryCount = 0;
    this.#bindLifecycle();
    this.#connect();
  }

  #connect() {
    // Capture the new socket in a local; every listener checks `this.#ws ===
    // next` before mutating state. Without that guard, the close event
    // queued on a superseded socket (typical when wake-on-resume races a
    // dying conn) would schedule a redundant retry on top of the fresh one.
    const old = this.#ws;
    this.#ws = null;
    if (this.#retryTimer) {
      clearTimeout(this.#retryTimer);
      this.#retryTimer = null;
    }
    this.#stopPing();
    if (old) {
      old.close();
    }

    this.#deliberateClose = false;
    // Same-origin WebSocket — the custom Node server in `frontend/server.js`
    // tunnels `/api/ws/*` upgrades to the backend container. Deriving the
    // URL from `window.location` means production (behind a reverse proxy)
    // and dev (behind our custom server) both "just work" with no env var.
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // If we hold a guest token for this room, append it as a query param so
    // the backend handshake in api/ws.go can resolve identity without a
    // session cookie. Registered users hit the same endpoint cookie-only.
    const token = this.#roomCode ? guest.token(this.#roomCode) : null;
    const qs = token ? `?guest_token=${encodeURIComponent(token)}` : '';
    const next = new WebSocket(
      `${proto}//${window.location.host}/api/ws/rooms/${this.#roomCode}${qs}`
    );
    this.#ws = next;

    next.addEventListener('open', () => {
      if (this.#ws !== next) return;
      this.status = 'connected';
      this.retryCount = 0;
      this.#startPing();
    });

    next.addEventListener('message', ev => {
      if (this.#ws !== next) return;
      try {
        const msg: WsMessage = JSON.parse(ev.data as string);
        this.#dispatch(msg);
      } catch {}
    });

    next.addEventListener('close', () => {
      if (this.#ws !== next) return;
      this.#stopPing();
      // Deliberate close (ws.disconnect, or after a room_closed frame):
      // don't flip to 'reconnecting' — that would falsely toast a
      // "Connection lost" message for a disconnect we initiated ourselves.
      if (this.#deliberateClose) {
        this.status = 'closed';
        return;
      }
      if (this.retryCount < 10) {
        this.status = 'reconnecting';
        const delay =
          Math.min(30, Math.pow(2, this.retryCount)) + Math.random();
        this.#retryTimer = setTimeout(() => {
          this.retryCount++;
          this.#connect();
        }, delay * 1000);
      } else {
        this.status = 'error';
      }
    });

    next.addEventListener('error', () => {
      // close event follows; handled there
    });
  }

  /** Send a typed message to the server. */
  send(type: string, data?: unknown) {
    if (this.#ws?.readyState === WebSocket.OPEN) {
      this.#ws.send(JSON.stringify({ type, data }));
    }
  }

  /** Register a handler for a specific message type. Returns unsubscribe fn. */
  onMessage(type: string, handler: (data: unknown) => void): () => void {
    if (!this.#handlers.has(type)) this.#handlers.set(type, []);
    this.#handlers.get(type)!.push(handler);
    return () => {
      const list = this.#handlers.get(type);
      if (list) {
        const i = list.indexOf(handler);
        if (i >= 0) list.splice(i, 1);
      }
    };
  }

  /** Reconnect using the previously set room code (e.g. after a manual retry). */
  reconnect() {
    if (this.#roomCode) {
      this.retryCount = 0;
      this.#connect();
    }
  }

  disconnect() {
    this.#unbindLifecycle();
    this.#roomCode = null;
    if (this.#retryTimer) clearTimeout(this.#retryTimer);
    this.#stopPing();
    this.#deliberateClose = true;
    this.#ws?.close();
    this.#ws = null;
    this.status = 'closed';
    this.retryCount = 0;
    this.#hiddenAt = null;
  }

  #dispatch(msg: WsMessage) {
    const handlers = this.#handlers.get(msg.type) ?? [];
    for (const h of handlers) h(msg.data);
    // Also dispatch to '*' catch-all handlers
    const all = this.#handlers.get('*') ?? [];
    for (const h of all) h(msg);
  }

  #startPing() {
    this.#pingTimer = setInterval(() => {
      this.send('ping');
      this.#pongTimeout = setTimeout(() => {
        // No pong within 10s — server dead; force reconnect
        this.#ws?.close();
      }, 10_000);
    }, 25_000);

    this.#pongUnsub = this.onMessage('pong', () => {
      if (this.#pongTimeout) clearTimeout(this.#pongTimeout);
    });
  }

  #stopPing() {
    if (this.#pingTimer) clearInterval(this.#pingTimer);
    if (this.#pongTimeout) clearTimeout(this.#pongTimeout);
    this.#pongUnsub?.();
    this.#pongUnsub = null;
    this.#pingTimer = null;
    this.#pongTimeout = null;
  }

  #bindLifecycle() {
    if (this.#lifecycleBound || typeof document === 'undefined') return;
    document.addEventListener('visibilitychange', this.#onVisibilityChange);
    window.addEventListener('pageshow', this.#onPageShow);
    this.#lifecycleBound = true;
  }

  #unbindLifecycle() {
    if (!this.#lifecycleBound || typeof document === 'undefined') return;
    document.removeEventListener('visibilitychange', this.#onVisibilityChange);
    window.removeEventListener('pageshow', this.#onPageShow);
    this.#lifecycleBound = false;
  }

  #onVisibilityChange = () => {
    if (typeof document === 'undefined') return;
    if (document.visibilityState === 'hidden') {
      this.#hiddenAt = Date.now();
      return;
    }
    if (document.visibilityState !== 'visible') return;
    const hiddenFor = this.#hiddenAt ? Date.now() - this.#hiddenAt : 0;
    this.#hiddenAt = null;
    // Quick tab/app switches (<2s) on desktop usually keep the WS alive —
    // skip the churn. Anything longer risks the server's reconnect grace
    // window (default 30s) closing while iOS held the JS context frozen,
    // so force a fresh socket on resume rather than waiting up to 30s for
    // the existing exponential-backoff timer to fire.
    if (hiddenFor >= 2000) this.#wakeReconnect();
  };

  #onPageShow = (ev: PageTransitionEvent) => {
    // bfcache restore (Safari/Firefox) silently severs WebSockets without
    // firing a close event the JS context can observe. Always reconnect on
    // a persisted pageshow.
    if (ev.persisted) this.#wakeReconnect();
  };

  /** Cancel any pending backoff and open a fresh socket immediately. Safe
   * to call when the existing socket reports OPEN — iOS can keep zombie
   * WebSockets after suspending the app, and the server's reconnect path
   * (handleRegister, hub.go) handles the rapid-replace cleanly via the
   * `existing.reconnecting` branch. */
  #wakeReconnect() {
    if (!this.#roomCode || this.#deliberateClose) return;
    this.retryCount = 0;
    this.#connect();
  }
}

export const ws = new WsState();
