import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { StageChoreographer } from './stage.svelte';

describe('StageChoreographer', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('starts visible with the initial phase', () => {
    const s = new StageChoreographer('idle');
    expect(s.displayPhase).toBe('idle');
    expect(s.visible).toBe(true);
  });

  it('is a no-op when synced with the same phase', () => {
    const s = new StageChoreographer('idle');
    s.sync('idle');
    expect(s.visible).toBe(true);
    expect(s.displayPhase).toBe('idle');
  });

  it('hides immediately on phase change and swaps after 450ms', () => {
    const s = new StageChoreographer('idle');
    s.sync('submitting');
    expect(s.visible).toBe(false);
    expect(s.displayPhase).toBe('idle');
    vi.advanceTimersByTime(449);
    expect(s.displayPhase).toBe('idle');
    vi.advanceTimersByTime(1);
    expect(s.displayPhase).toBe('submitting');
    expect(s.visible).toBe(true);
  });

  it('coalesces rapid phase changes to the latest value', () => {
    const s = new StageChoreographer('idle');
    s.sync('countdown');
    vi.advanceTimersByTime(100);
    s.sync('submitting');
    vi.advanceTimersByTime(450);
    expect(s.displayPhase).toBe('submitting');
    expect(s.visible).toBe(true);
  });

  it('exposes a shared singleton for the app', async () => {
    const mod = await import('./stage.svelte');
    expect(mod.stage).toBeInstanceOf(StageChoreographer);
  });
});
