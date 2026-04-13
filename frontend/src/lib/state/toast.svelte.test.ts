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

describe('ToastState action', () => {
  it('stores no action when none is provided', () => {
    const t = new ToastState();
    t.show('hello', 'success');
    expect(t.items[0].action).toBeUndefined();
  });

  it('stores action label and fn when provided', () => {
    const t = new ToastState();
    const fn = vi.fn();
    t.show('Connection failed.', 'error', { label: 'Retry', fn });
    expect(t.items[0].action?.label).toBe('Retry');
    expect(t.items[0].action?.fn).toBe(fn);
  });

  it('error toasts with action are persistent (duration 0)', () => {
    const t = new ToastState();
    t.show('Connection failed.', 'error', { label: 'Retry', fn: vi.fn() });
    expect(t.items[0].duration).toBe(0);
  });

  it('dismiss removes the toast', () => {
    const t = new ToastState();
    t.show('msg', 'success');
    const id = t.items[0].id;
    t.dismiss(id);
    expect(t.items).toHaveLength(0);
  });
});
