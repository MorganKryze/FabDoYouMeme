// backend/internal/game/hub_memevote.go
package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

// textCard is one caption in a player's hand. Text is snapshotted at deal
// time so the round history stays stable against later pack edits.
type textCard struct {
	CardID uuid.UUID
	Text   string
}

// handState owns the text-deck and per-player hands for a meme-vote room.
// It is accessed only from the hub goroutine; no locking.
type handState struct {
	deck     []uuid.UUID
	textFor  map[uuid.UUID]string
	handSize int
	hands    map[string][]textCard
}

// newHandStateWithText takes a shuffled deck of text-item IDs, a lookup of
// their text payloads, and the hand size for the room. Drawing pops the tail
// of the deck.
func newHandStateWithText(deck []uuid.UUID, textFor map[uuid.UUID]string, handSize int) *handState {
	return &handState{
		deck:     append([]uuid.UUID(nil), deck...),
		textFor:  textFor,
		handSize: handSize,
		hands:    make(map[string][]textCard),
	}
}

// newHandState is the test-friendly constructor — text is the empty string.
func newHandState(deck []uuid.UUID, handSize int) *handState {
	textFor := make(map[uuid.UUID]string, len(deck))
	for _, id := range deck {
		textFor[id] = ""
	}
	return newHandStateWithText(deck, textFor, handSize)
}

// DealInitial assigns up to handSize cards to every player. Best-effort —
// if the deck runs out mid-deal, the remaining hands are partial and the
// first Refill will surface the shortage.
func (h *handState) DealInitial(playerIDs []string) {
	for _, pid := range playerIDs {
		for len(h.hands[pid]) < h.handSize && len(h.deck) > 0 {
			h.hands[pid] = append(h.hands[pid], h.drawOne())
		}
	}
}

// Refill tops every player's hand back to handSize. Returns errPackExhausted
// if any player cannot be topped off — the room must end.
func (h *handState) Refill(playerIDs []string) error {
	for _, pid := range playerIDs {
		for len(h.hands[pid]) < h.handSize {
			if len(h.deck) == 0 {
				return errPackExhausted
			}
			h.hands[pid] = append(h.hands[pid], h.drawOne())
		}
	}
	return nil
}

// Play validates that cardID is in the player's hand and removes it.
// Returns errInvalidCard if not present.
func (h *handState) Play(playerID string, cardID uuid.UUID) error {
	hand := h.hands[playerID]
	for i, c := range hand {
		if c.CardID == cardID {
			h.hands[playerID] = append(hand[:i], hand[i+1:]...)
			return nil
		}
	}
	return errInvalidCard
}

// HandFor returns a caller-owned copy of the current hand for a player.
func (h *handState) HandFor(playerID string) []textCard {
	return append([]textCard(nil), h.hands[playerID]...)
}

func (h *handState) drawOne() textCard {
	n := len(h.deck)
	id := h.deck[n-1]
	h.deck = h.deck[:n-1]
	return textCard{CardID: id, Text: h.textFor[id]}
}

var (
	errInvalidCard   = errors.New("invalid_card")
	errPackExhausted = errors.New("pack_exhausted")
)

// loadTextDeck fetches every payload_version==2 item in the room's text pack
// and returns the id deck (unshuffled) plus a lookup of each item's caption
// text. Returns an error if the pack id is NULL (no text pack on the room)
// or the DB read fails; an empty deck is not an error here — the
// ValidatePackRequirements check at room creation already guarantees the
// deck is large enough.
func loadTextDeck(ctx context.Context, q *db.Queries, textPackID pgtype.UUID) ([]uuid.UUID, map[uuid.UUID]string, error) {
	if !textPackID.Valid {
		return nil, nil, fmt.Errorf("no text pack on room")
	}
	rows, err := q.ListPackItemsByPayloadVersion(ctx, db.ListPackItemsByPayloadVersionParams{
		PackID:         textPackID.Bytes,
		PayloadVersion: 2,
	})
	if err != nil {
		return nil, nil, err
	}
	deck := make([]uuid.UUID, 0, len(rows))
	text := make(map[uuid.UUID]string, len(rows))
	for _, row := range rows {
		deck = append(deck, row.ID)
		var p struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(row.Payload, &p)
		text[row.ID] = p.Text
	}
	return deck, text, nil
}

// shuffleDeck randomises the order of a deck in place. Caption ordering is
// not a cryptographic concern — math/rand/v2 is fine.
func shuffleDeck(deck []uuid.UUID) {
	rand.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
}

// initMemeVoteHands loads the text deck, initialises h.memeVote, and deals
// the initial hand to every seated player. Returns false (and ends the room
// with reason "pack_exhausted") if anything goes wrong — the caller in
// startGame must abort. Called only for meme-vote rooms.
func (h *Hub) initMemeVoteHands(ctx context.Context) bool {
	room, err := h.db.GetRoomByID(ctx, h.roomID)
	if err != nil {
		if h.log != nil {
			h.log.Error("hub: meme-vote get room", "error", err, "room", h.roomCode)
		}
		h.finishRoom(ctx, "pack_exhausted", nil)
		return false
	}
	var cfg RoomConfig
	if room.Config != nil {
		_ = json.Unmarshal(room.Config, &cfg)
	}
	if cfg.HandSize <= 0 {
		cfg.HandSize = 5
	}
	deck, textFor, err := loadTextDeck(ctx, h.db, room.TextPackID)
	if err != nil {
		if h.log != nil {
			h.log.Error("hub: meme-vote load text deck", "error", err, "room", h.roomCode)
		}
		h.finishRoom(ctx, "pack_exhausted", nil)
		return false
	}
	shuffleDeck(deck)
	h.memeVote = newHandStateWithText(deck, textFor, cfg.HandSize)
	h.memeVote.DealInitial(h.seatedPlayerIDs())
	return true
}

// seatedPlayerIDs returns the playerIDs of every currently-known player. Used
// to deal and refill meme-vote hands; a reconnecting player is still
// considered seated so their hand survives the grace window.
func (h *Hub) seatedPlayerIDs() []string {
	ids := make([]string, 0, len(h.players))
	for id := range h.players {
		ids = append(ids, id)
	}
	return ids
}

// memeVoteHandPayload is the per-player hand projection used in both
// round_started and room_state.my_hand. Returns nil when there is no
// meme-vote state (i.e. any other game type).
func (h *Hub) memeVoteHandPayload(playerID string) []map[string]string {
	if h.memeVote == nil {
		return nil
	}
	hand := h.memeVote.HandFor(playerID)
	out := make([]map[string]string, 0, len(hand))
	for _, c := range hand {
		out = append(out, map[string]string{
			"card_id": c.CardID.String(),
			"text":    c.Text,
		})
	}
	return out
}
