import { describe, it, expect, beforeEach } from 'vitest';
import { ToneState } from './tone.svelte';

describe('ToneState', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('defaults to Playful (level 2)', () => {
    const t = new ToneState();
    expect(t.level).toBe(2);
  });

  it('persists level to localStorage on set', () => {
    const t = new ToneState();
    t.setLevel(4);
    expect(localStorage.getItem('fdym:tone')).toBe('4');
  });

  it('updates reactive level on set', () => {
    const t = new ToneState();
    t.setLevel(0);
    expect(t.level).toBe(0);
    t.setLevel(3);
    expect(t.level).toBe(3);
  });

  it('hydrates level from localStorage on construction', () => {
    localStorage.setItem('fdym:tone', '4');
    const t = new ToneState();
    expect(t.level).toBe(4);
  });

  it('ignores non-numeric localStorage values on hydration', () => {
    localStorage.setItem('fdym:tone', 'chaos');
    const t = new ToneState();
    expect(t.level).toBe(2);
  });

  it('ignores out-of-range numeric localStorage values on hydration', () => {
    localStorage.setItem('fdym:tone', '7');
    const t = new ToneState();
    expect(t.level).toBe(2);
  });

  it('ignores negative numeric localStorage values on hydration', () => {
    localStorage.setItem('fdym:tone', '-1');
    const t = new ToneState();
    expect(t.level).toBe(2);
  });
});
