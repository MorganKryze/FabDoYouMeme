import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ToastState } from './toast.svelte';

describe('ToastState', () => {
  let t: ToastState;

  beforeEach(() => {
    vi.useFakeTimers();
    t = new ToastState();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('caps visible items at 3 and drops the oldest', () => {
    t.show('a', 'success');
    t.show('b', 'success');
    t.show('c', 'success');
    t.show('d', 'success');

    expect(t.items).toHaveLength(3);
    expect(t.items.map((i) => i.message)).toEqual(['b', 'c', 'd']);
  });

  it('dismiss removes only the targeted item', () => {
    t.show('a', 'success');
    t.show('b', 'success');
    const firstId = t.items[0].id;

    t.dismiss(firstId);

    expect(t.items).toHaveLength(1);
    expect(t.items[0].message).toBe('b');
  });

  it('error type gets duration 0 (persistent)', () => {
    t.show('boom', 'error');

    expect(t.items).toHaveLength(1);
    expect(t.items[0].type).toBe('error');
    expect(t.items[0].duration).toBe(0);
  });
});
