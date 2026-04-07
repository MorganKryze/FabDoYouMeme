import type { WsMessage } from '$lib/api/types';
import { env } from '$env/dynamic/public';

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

  /** Connect to a room's WebSocket. */
  connect(roomCode: string) {
    this.#roomCode = roomCode;
    this.retryCount = 0;
    this.#connect();
  }

  #connect() {
    if (this.#ws) {
      this.#ws.close();
    }
    const base = (env.PUBLIC_API_URL || 'http://localhost:8080').replace(
      /^http/,
      'ws'
    );
    this.#ws = new WebSocket(`${base}/api/ws/rooms/${this.#roomCode}`);

    this.#ws.addEventListener('open', () => {
      this.status = 'connected';
      this.retryCount = 0;
      this.#startPing();
    });

    this.#ws.addEventListener('message', ev => {
      try {
        const msg: WsMessage = JSON.parse(ev.data as string);
        this.#dispatch(msg);
      } catch {}
    });

    this.#ws.addEventListener('close', () => {
      this.#stopPing();
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

    this.#ws.addEventListener('error', () => {
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

  disconnect() {
    this.#roomCode = null;
    if (this.#retryTimer) clearTimeout(this.#retryTimer);
    this.#stopPing();
    this.#ws?.close();
    this.#ws = null;
    this.status = 'closed';
    this.retryCount = 0;
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
  }
}

export const ws = new WsState();
