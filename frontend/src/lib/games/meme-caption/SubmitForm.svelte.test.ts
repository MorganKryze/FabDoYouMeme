import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import { flushSync } from 'svelte';
import SubmitForm from './SubmitForm.svelte';
import { room } from '$lib/state/room.svelte';
import type { Round } from '$lib/api/types';

function makeRound(): Round {
  return {
    round_number: 1,
    // 120s in the future — keeps the component in the submitting state.
    ends_at: new Date(Date.now() + 120_000).toISOString(),
    duration_seconds: 120,
    item: { payload: {} }
  };
}

describe('SubmitForm.svelte', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'setInterval', 'Date'] });
    room.reset();
    room.players = [{ user_id: 'u1', username: 'alice', connected: true }];
  });

  afterEach(() => {
    vi.useRealTimers();
    room.reset();
  });

  it('disables the submit button when caption is empty', () => {
    const { getByRole } = render(SubmitForm, { props: { round: makeRound() } });

    const button = getByRole('button', { name: /submit/i });
    expect(button).toBeDisabled();
  });

  it('enables the submit button once caption has non-whitespace text', async () => {
    const { getByRole, getByPlaceholderText } = render(SubmitForm, {
      props: { round: makeRound() }
    });

    const textarea = getByPlaceholderText(/write your caption/i);
    await fireEvent.input(textarea, { target: { value: 'funny' } });
    flushSync();

    const button = getByRole('button', { name: /submit/i });
    expect(button).not.toBeDisabled();
  });
});
