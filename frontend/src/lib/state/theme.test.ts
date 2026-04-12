import { describe, it, expect, beforeEach, vi } from 'vitest';
import { ThemeState } from './theme.svelte';

describe('ThemeState', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('defaults to auto preference', () => {
    const t = new ThemeState();
    expect(t.preference).toBe('auto');
  });

  it('derives active band from timeOfDay when preference is auto', () => {
    const t = new ThemeState();
    t.timeOfDay = 'evening';
    expect(t.active).toBe('evening');
    t.timeOfDay = 'night';
    expect(t.active).toBe('night');
  });

  it('derives active band from preference when overridden', () => {
    const t = new ThemeState();
    t.timeOfDay = 'morning';
    t.setPreference('night');
    expect(t.active).toBe('night');
  });

  it('persists preference to localStorage on set', () => {
    const t = new ThemeState();
    t.setPreference('evening');
    expect(localStorage.getItem('fdym:theme')).toBe('evening');
  });

  it('hydrates preference from localStorage on construction', () => {
    localStorage.setItem('fdym:theme', 'night');
    const t = new ThemeState();
    expect(t.preference).toBe('night');
  });

  it('ignores invalid localStorage values on hydration', () => {
    localStorage.setItem('fdym:theme', 'rainbow');
    const t = new ThemeState();
    expect(t.preference).toBe('auto');
  });

  it('tickTimeOfDay computes band from clock hours', () => {
    const t = new ThemeState();
    const spy = vi.spyOn(Date.prototype, 'getHours');

    spy.mockReturnValue(8);
    t.tickTimeOfDay();
    expect(t.timeOfDay).toBe('morning');

    spy.mockReturnValue(19);
    t.tickTimeOfDay();
    expect(t.timeOfDay).toBe('evening');

    spy.mockReturnValue(23);
    t.tickTimeOfDay();
    expect(t.timeOfDay).toBe('night');

    spy.mockReturnValue(3);
    t.tickTimeOfDay();
    expect(t.timeOfDay).toBe('night');

    spy.mockRestore();
  });
});
