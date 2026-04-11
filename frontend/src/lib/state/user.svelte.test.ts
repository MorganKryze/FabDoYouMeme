import { describe, it, expect, beforeEach } from 'vitest';
import { UserState } from './user.svelte';

describe('UserState', () => {
  let u: UserState;

  beforeEach(() => {
    u = new UserState();
  });

  it('is unauthenticated on a fresh instance', () => {
    expect(u.id).toBeNull();
    expect(u.isAuthenticated).toBe(false);
    expect(u.isAdmin).toBe(false);
  });

  it('becomes authenticated after setFrom', () => {
    u.setFrom({
      id: 'user-1',
      username: 'alice',
      email: 'alice@example.com',
      role: 'player'
    });

    expect(u.isAuthenticated).toBe(true);
    expect(u.id).toBe('user-1');
    expect(u.username).toBe('alice');
    expect(u.email).toBe('alice@example.com');
    expect(u.role).toBe('player');
    expect(u.isAdmin).toBe(false);
  });

  it('isAdmin is true when role is admin', () => {
    u.setFrom({
      id: 'admin-1',
      username: 'root',
      email: 'root@example.com',
      role: 'admin'
    });

    expect(u.isAdmin).toBe(true);
    expect(u.isAuthenticated).toBe(true);
  });

  it('clear() resets all fields', () => {
    u.setFrom({
      id: 'user-1',
      username: 'alice',
      email: 'alice@example.com',
      role: 'admin'
    });

    u.clear();

    expect(u.id).toBeNull();
    expect(u.username).toBeNull();
    expect(u.email).toBeNull();
    expect(u.role).toBeNull();
    expect(u.isAuthenticated).toBe(false);
    expect(u.isAdmin).toBe(false);
  });
});
