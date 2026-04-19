package game

import (
	"testing"

	"github.com/google/uuid"
)

func TestHandState_InitialDeal_UniqueAcrossPlayers(t *testing.T) {
	deck := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	hs := newHandState(deck, 3) // hand_size=3
	hs.DealInitial([]string{"p1", "p2"})
	if got := len(hs.HandFor("p1")); got != 3 {
		t.Fatalf("p1 hand len = %d, want 3", got)
	}
	if got := len(hs.HandFor("p2")); got != 3 {
		t.Fatalf("p2 hand len = %d, want 3", got)
	}
	seen := map[uuid.UUID]bool{}
	for _, c := range hs.HandFor("p1") {
		seen[c.CardID] = true
	}
	for _, c := range hs.HandFor("p2") {
		if seen[c.CardID] {
			t.Fatalf("card %s appears in both hands", c.CardID)
		}
	}
}

func TestHandState_Refill(t *testing.T) {
	deck := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	hs := newHandState(deck, 2)
	hs.DealInitial([]string{"p1"})
	played := hs.HandFor("p1")[0].CardID
	if err := hs.Play("p1", played); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if err := hs.Refill([]string{"p1"}); err != nil {
		t.Fatalf("Refill: %v", err)
	}
	if got := len(hs.HandFor("p1")); got != 2 {
		t.Fatalf("after refill len = %d, want 2", got)
	}
}

func TestHandState_Play_RejectsCardNotInHand(t *testing.T) {
	deck := []uuid.UUID{uuid.New(), uuid.New()}
	hs := newHandState(deck, 2)
	hs.DealInitial([]string{"p1"})
	err := hs.Play("p1", uuid.New())
	if err == nil || err.Error() != "invalid_card" {
		t.Fatalf("want invalid_card, got %v", err)
	}
}

func TestHandState_Refill_Exhausted(t *testing.T) {
	deck := []uuid.UUID{uuid.New()}
	hs := newHandState(deck, 2)
	// Not enough to deal the initial hand (need 2 for 1 player).
	hs.DealInitial([]string{"p1"})
	// DealInitial is tolerant; Refill surfaces the shortage.
	err := hs.Refill([]string{"p1"})
	if err == nil || err.Error() != "pack_exhausted" {
		t.Fatalf("want pack_exhausted, got %v", err)
	}
}
