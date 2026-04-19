import { describe, expect, it } from 'vitest';
import { HandStore } from './handStore.svelte';

describe('HandStore', () => {
  it('hydrates from round_started', () => {
    const store = new HandStore();
    store.onRoundStarted({
      hand: [
        { card_id: 'a', text: 'one' },
        { card_id: 'b', text: 'two' }
      ]
    });
    expect(store.cards).toEqual([
      { card_id: 'a', text: 'one' },
      { card_id: 'b', text: 'two' }
    ]);
  });

  it('trims on submit', () => {
    const store = new HandStore();
    store.onRoundStarted({
      hand: [
        { card_id: 'a', text: 'one' },
        { card_id: 'b', text: 'two' }
      ]
    });
    store.onSubmit('a');
    expect(store.cards).toEqual([{ card_id: 'b', text: 'two' }]);
  });

  it('rehydrates from room_state.my_hand on reconnect', () => {
    const store = new HandStore();
    store.onRoundStarted({ hand: [{ card_id: 'a', text: 'one' }] });
    store.onRoomState({ my_hand: [{ card_id: 'z', text: 'zzz' }] });
    expect(store.cards).toEqual([{ card_id: 'z', text: 'zzz' }]);
  });

  it('ignores empty round_started and room_state payloads', () => {
    const store = new HandStore();
    store.onRoundStarted({ hand: [{ card_id: 'a', text: 'one' }] });
    store.onRoundStarted({});
    store.onRoomState({});
    expect(store.cards).toEqual([{ card_id: 'a', text: 'one' }]);
  });

  it('reset clears the hand', () => {
    const store = new HandStore();
    store.onRoundStarted({ hand: [{ card_id: 'a', text: 'one' }] });
    store.reset();
    expect(store.cards).toEqual([]);
  });
});
