import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import { flushSync } from 'svelte';
import VoteForm from './VoteForm.svelte';
import { room } from '$lib/state/room.svelte';
import { user } from '$lib/state/user.svelte';
import type { Submission } from '$lib/api/types';

const SUBMISSIONS: Submission[] = [
  { id: 's1', user_id: 'u1', username: 'alice', caption: 'first caption' },
  { id: 's2', user_id: 'u2', username: 'bob', caption: 'second caption' }
];

describe('VoteForm.svelte', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'setInterval', 'Date'] });
    room.reset();
    room.currentRound = {
      round_number: 1,
      ends_at: new Date(Date.now() + 60_000).toISOString(),
      duration_seconds: 60,
      item: { payload: {} }
    };
    user.setFrom({
      id: 'u3',
      username: 'carol',
      email: 'carol@example.com',
      role: 'player'
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    room.reset();
    user.clear();
  });

  it('renders one button per submission', () => {
    const { getAllByText } = render(VoteForm, { props: { submissions: SUBMISSIONS } });

    expect(getAllByText('first caption')).toHaveLength(1);
    expect(getAllByText('second caption')).toHaveLength(1);
  });

  it('disables the vote button until a card is selected', async () => {
    const { getByRole } = render(VoteForm, { props: { submissions: SUBMISSIONS } });

    const voteBtn = getByRole('button', { name: /^vote$/i });
    expect(voteBtn).toBeDisabled();

    // Click the first submission card (alice's — carol is voter, not own).
    const card = getByRole('button', { name: /first caption/i });
    await fireEvent.click(card);
    flushSync();

    expect(voteBtn).not.toBeDisabled();
  });
});
