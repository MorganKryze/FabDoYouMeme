// backend/internal/game/hub_handdeck.go
//
// Hand-deck mechanics shared by every showdown-style game type. A "showdown"
// here means: each player is dealt a private hand of cards from a pack at
// game start; one card is played per round; the hand refills between rounds.
// The hub owns the deck and per-player hands; handlers stay stateless.
//
// Two game types use this file: meme-showdown (text captions, payload v2) and
// prompt-showdown (text fillers, payload v3). The role and payload version of
// the deck come from the handler's RequiredPacks()[1] — that is, the
// secondary pack declared after the primary content pack.
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

// textCard is one card in a player's hand. Text is snapshotted at deal time
// so the round history stays stable against later pack edits.
type textCard struct {
	CardID uuid.UUID
	Text   string
}

// handState owns the deck and per-player hands for a showdown room.
// It is accessed only from the hub goroutine; no locking.
type handState struct {
	deck     []uuid.UUID
	textFor  map[uuid.UUID]string
	handSize int
	hands    map[string][]textCard
}

// newHandStateWithText takes a shuffled deck of item IDs, a lookup of their
// text payloads, and the hand size for the room. Drawing pops the tail of
// the deck.
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

// loadHandDeck fetches every item in the secondary pack at the requested
// payload version and returns the id deck (unshuffled) plus a lookup of
// each item's text. Returns an error if the pack id is NULL or the DB
// read fails; an empty deck is not an error here — ValidatePackRequirements
// at room creation already guarantees the deck is large enough.
func loadHandDeck(ctx context.Context, q *db.Queries, packID pgtype.UUID, payloadVersion int) ([]uuid.UUID, map[uuid.UUID]string, error) {
	if !packID.Valid {
		return nil, nil, fmt.Errorf("no secondary pack on room")
	}
	rows, err := q.ListPackItemsByPayloadVersion(ctx, db.ListPackItemsByPayloadVersionParams{
		PackID:         packID.Bytes,
		PayloadVersion: int32(payloadVersion),
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

// shuffleDeck randomises the order of a deck in place. Card ordering is not
// a cryptographic concern — math/rand/v2 is fine.
func shuffleDeck(deck []uuid.UUID) {
	rand.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
}

// initHandDeck loads the secondary pack's items (using the payload version
// declared by the handler), initialises h.handDeck, and deals the initial
// hand to every seated player. Returns false (and ends the room with reason
// "pack_exhausted") if anything goes wrong — the caller in startGame must
// abort. Called for any handler whose RequiredPacks() declares a secondary
// entry AND whose PersonalisesRoundStart() is true.
func (h *Hub) initHandDeck(ctx context.Context) bool {
	handler, ok := h.registry.Get(h.gameTypeSlug)
	if !ok {
		h.finishRoom(ctx, "pack_exhausted", nil)
		return false
	}
	reqs := handler.RequiredPacks()
	if len(reqs) < 2 || len(reqs[1].PayloadVersions) == 0 {
		h.finishRoom(ctx, "pack_exhausted", nil)
		return false
	}
	// Every handler that uses hand-deck mechanics declares a single payload
	// version for the secondary pack. Pick the first; if a future handler
	// supports multiple versions in one deck, generalise here.
	payloadVersion := reqs[1].PayloadVersions[0]

	room, err := h.db.GetRoomByID(ctx, h.roomID)
	if err != nil {
		if h.log != nil {
			h.log.Error("hub: handdeck get room", "error", err, "room", h.roomCode)
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
	deck, textFor, err := loadHandDeck(ctx, h.db, room.TextPackID, payloadVersion)
	if err != nil {
		if h.log != nil {
			h.log.Error("hub: handdeck load deck", "error", err, "room", h.roomCode)
		}
		h.finishRoom(ctx, "pack_exhausted", nil)
		return false
	}
	shuffleDeck(deck)
	h.handDeck = newHandStateWithText(deck, textFor, cfg.HandSize)
	h.handDeck.DealInitial(h.seatedPlayerIDs())
	return true
}

// seatedPlayerIDs returns the playerIDs of every currently-known player. Used
// to deal and refill hands; a reconnecting player is still considered
// seated so their hand survives the grace window.
func (h *Hub) seatedPlayerIDs() []string {
	ids := make([]string, 0, len(h.players))
	for id := range h.players {
		ids = append(ids, id)
	}
	return ids
}

// handPayloadFor is the per-player hand projection used in both round_started
// and room_state.my_hand. Returns nil when there is no hand-deck state (i.e.
// any non-showdown game type).
func (h *Hub) handPayloadFor(playerID string) []map[string]string {
	if h.handDeck == nil {
		return nil
	}
	hand := h.handDeck.HandFor(playerID)
	out := make([]map[string]string, 0, len(hand))
	for _, c := range hand {
		out = append(out, map[string]string{
			"card_id": c.CardID.String(),
			"text":    c.Text,
		})
	}
	return out
}
