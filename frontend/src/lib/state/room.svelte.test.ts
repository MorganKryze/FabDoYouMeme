import { describe, it, expect, beforeEach, vi } from 'vitest';

// Mock `$app/navigation` before importing room.svelte so the mocked `goto`
// is wired in. vi.mock hoists to the top of the file and works for ESM too.
const gotoMock = vi.fn();
vi.mock('$app/navigation', () => ({
  goto: (...args: unknown[]) => gotoMock(...args)
}));

import { RoomState } from './room.svelte';
import { toast } from './toast.svelte';
import { ws } from './ws.svelte';
import type { Player, Round, WsMessage } from '$lib/api/types';

function makePlayer(userId: string, username = userId): Player {
  return { user_id: userId, username, connected: true };
}

function makeRound(n: number): Round {
  return {
    round_number: n,
    ends_at: new Date(Date.now() + 60_000).toISOString(),
    duration_seconds: 60,
    item: { payload: {} }
  };
}

describe('RoomState.handleMessage', () => {
  let r: RoomState;

  beforeEach(() => {
    r = new RoomState();
    r.init({
      code: 'ABCD',
      game_type: {
        id: 'g1',
        slug: 'meme-caption',
        name: 'Meme Caption',
        description: null,
        version: '1',
        supports_solo: true,
        config: {
          min_round_duration_seconds: 10,
          max_round_duration_seconds: 120,
          default_round_duration_seconds: 60,
          min_voting_duration_seconds: 10,
          max_voting_duration_seconds: 60,
          default_voting_duration_seconds: 20,
          min_players: 2,
          max_players: 8,
          min_round_count: 1,
          max_round_count: 10,
          default_round_count: 3
        },
        supported_payload_versions: [1]
      },
      state: 'lobby',
      players: [makePlayer('u1', 'alice')],
      host_id: 'u1'
    });
  });

  it('player_joined appends a new player', () => {
    const msg: WsMessage = {
      type: 'player_joined',
      data: makePlayer('u2', 'bob')
    };

    r.handleMessage(msg);

    expect(r.players).toHaveLength(2);
    expect(r.players[1].user_id).toBe('u2');
    expect(r.players[1].is_host).toBe(false);
  });

  it('player_left removes the player by user_id', () => {
    r.handleMessage({ type: 'player_joined', data: makePlayer('u2') });
    expect(r.players).toHaveLength(2);

    r.handleMessage({ type: 'player_left', data: makePlayer('u2') });

    expect(r.players).toHaveLength(1);
    expect(r.players[0].user_id).toBe('u1');
  });

  it('round_started sets phase to submitting and clears submit/vote flags', () => {
    r.hasSubmitted = true;
    r.hasVoted = true;
    r.phase = 'results';
    r.submissions = [
      { id: 's-old', user_id: 'u1', username: 'alice', caption: 'stale' }
    ];

    r.handleMessage({ type: 'round_started', data: makeRound(1) });

    expect(r.phase).toBe('submitting');
    expect(r.hasSubmitted).toBe(false);
    expect(r.hasVoted).toBe(false);
    expect(r.submissions).toHaveLength(0);
    expect(r.currentRound?.round_number).toBe(1);
  });

  it('game_ended sets state to finished and captures leaderboard', () => {
    r.handleMessage({
      type: 'game_ended',
      data: {
        reason: 'normal',
        leaderboard: [
          { user_id: 'u1', username: 'alice', total_score: 10, rank: 1 }
        ]
      }
    });

    expect(r.state).toBe('finished');
    expect(r.phase).toBe('idle');
    expect(r.endReason).toBe('normal');
    expect(r.leaderboard).toHaveLength(1);
    expect(r.leaderboard[0].rank).toBe(1);
  });

  it('error with code already_submitted marks hasSubmitted', () => {
    expect(r.hasSubmitted).toBe(false);

    r.handleMessage({
      type: 'error',
      data: { code: 'already_submitted', message: 'nope' }
    });

    expect(r.hasSubmitted).toBe(true);
  });

  it('room_closed disconnects ws, toasts host message, resets state, navigates home', () => {
    const toastSpy = vi.spyOn(toast, 'show');
    const wsDisconnectSpy = vi.spyOn(ws, 'disconnect').mockImplementation(() => {});
    gotoMock.mockClear();

    r.state = 'playing';

    r.handleMessage({
      type: 'room_closed',
      data: { reason: 'ended_by_host' }
    } as unknown as WsMessage);

    expect(wsDisconnectSpy).toHaveBeenCalled();
    expect(toastSpy).toHaveBeenCalledWith(
      expect.stringContaining('host'),
      expect.anything()
    );
    expect(gotoMock).toHaveBeenCalledWith('/');
    expect(r.code).toBe(null); // reset() was called
  });

  it('room_closed uses the admin-flavoured toast for ended_by_admin', () => {
    const toastSpy = vi.spyOn(toast, 'show');
    vi.spyOn(ws, 'disconnect').mockImplementation(() => {});
    gotoMock.mockClear();

    r.handleMessage({
      type: 'room_closed',
      data: { reason: 'ended_by_admin' }
    } as unknown as WsMessage);

    expect(toastSpy).toHaveBeenCalledWith(
      expect.stringContaining('admin'),
      expect.anything()
    );
  });
});
