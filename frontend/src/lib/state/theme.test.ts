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
    t.timeOfDay = 'light';
    expect(t.active).toBe('light');
    t.timeOfDay = 'dark';
    expect(t.active).toBe('dark');
  });

  it('derives active band from preference when overridden', () => {
    const t = new ThemeState();
    t.timeOfDay = 'light';
    t.setPreference('dark');
    expect(t.active).toBe('dark');
  });

  it('persists preference to localStorage on set', () => {
    const t = new ThemeState();
    t.setPreference('light');
    expect(localStorage.getItem('fdym:theme')).toBe('light');
  });

  it('hydrates preference from localStorage on construction', () => {
    localStorage.setItem('fdym:theme', 'dark');
    const t = new ThemeState();
    expect(t.preference).toBe('dark');
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
    expect(t.timeOfDay).toBe('light');

    spy.mockReturnValue(14);
    t.tickTimeOfDay();
    expect(t.timeOfDay).toBe('light');

    spy.mockReturnValue(19);
    t.tickTimeOfDay();
    expect(t.timeOfDay).toBe('dark');

    spy.mockReturnValue(23);
    t.tickTimeOfDay();
    expect(t.timeOfDay).toBe('dark');

    spy.mockReturnValue(3);
    t.tickTimeOfDay();
    expect(t.timeOfDay).toBe('dark');

    spy.mockRestore();
  });
});
