// Ephemeral text-card hand for the meme-vote game type.
//
// The server owns the authoritative hand; the hub personalises `round_started`
// with a `hand` field for every live player and rehydrates via `room_state`
// `my_hand` on reconnect. This store mirrors the server-side hand locally so
// components can render it reactively without observing the raw WS stream.

export interface TextCard {
  card_id: string;
  text: string;
}

export class HandStore {
  cards = $state<TextCard[]>([]);

  /** Seeded when the hub broadcasts a personalised round_started. */
  onRoundStarted(data: { hand?: TextCard[] }): void {
    if (data.hand) this.cards = [...data.hand];
  }

  /** Rehydrate from the mid-game snapshot after a reconnect. */
  onRoomState(data: { my_hand?: TextCard[] }): void {
    if (data.my_hand) this.cards = [...data.my_hand];
  }

  /** Remove the played card so the UI shows the remaining cards while we
   *  wait for the next round's deal. */
  onSubmit(cardId: string): void {
    this.cards = this.cards.filter((c) => c.card_id !== cardId);
  }

  reset(): void {
    this.cards = [];
  }
}

// Singleton instance shared by the meme-vote components. `RoomState` invokes
// the mutators from its WS message dispatch; everyone else just reads `.cards`.
export const handStore = new HandStore();
