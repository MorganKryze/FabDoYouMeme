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
	"math"
	"math/rand/v2"
	"sort"

	"github.com/google/uuid"

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

// loadWeightedHandDeck fetches every item across the supplied weighted pack
// list at the requested payload version, then returns a single id deck whose
// order is biased by per-pack weight (each item's exponential key is
// `-ln(rand)/weight`). Drawing from the deck (which the hub does by popping
// the tail in handState.drawOne) consumes the lightest-key items first, so
// the dealt hand's marginal distribution across packs matches the weights.
//
// An empty deck is not an error here — ValidatePackRequirements at room
// creation already guarantees the pool is large enough.
func loadWeightedHandDeck(ctx context.Context, q *db.Queries, packs []WeightedPackRef, payloadVersion int) ([]uuid.UUID, map[uuid.UUID]string, error) {
	if len(packs) == 0 {
		return nil, nil, fmt.Errorf("no secondary pack on room")
	}
	type keyed struct {
		id  uuid.UUID
		key float64
	}
	all := make([]keyed, 0)
	text := make(map[uuid.UUID]string)
	for _, p := range packs {
		rows, err := q.ListPackItemsByPayloadVersion(ctx, db.ListPackItemsByPayloadVersionParams{
			PackID:         p.PackID,
			PayloadVersion: int32(payloadVersion),
		})
		if err != nil {
			return nil, nil, err
		}
		w := float64(p.Weight)
		if w <= 0 {
			w = 1
		}
		for _, row := range rows {
			u := rand.Float64()
			if u < 1e-12 {
				u = 1e-12
			}
			all = append(all, keyed{id: row.ID, key: -math.Log(u) / w})
			var pl struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(row.Payload, &pl)
			text[row.ID] = pl.Text
		}
	}
	// Sort ascending — handState.drawOne pops the tail, so put the highest
	// key (lowest probability of being dealt) at the front. The exponential
	// keying does the rest of the work.
	sort.Slice(all, func(i, j int) bool { return all[i].key > all[j].key })
	deck := make([]uuid.UUID, len(all))
	for i, k := range all {
		deck[i] = k.id
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
	if err := h.ensurePacksLoaded(ctx); err != nil {
		if h.log != nil {
			h.log.Error("hub: handdeck load packs", "error", err, "room", h.roomCode)
		}
		h.finishRoom(ctx, "pack_exhausted", nil)
		return false
	}
	secondaryRefs := h.packsByRole[h.secondaryRole()]
	deck, textFor, err := loadWeightedHandDeck(ctx, h.db, secondaryRefs, payloadVersion)
	if err != nil {
		if h.log != nil {
			h.log.Error("hub: handdeck load deck", "error", err, "room", h.roomCode)
		}
		h.finishRoom(ctx, "pack_exhausted", nil)
		return false
	}
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
