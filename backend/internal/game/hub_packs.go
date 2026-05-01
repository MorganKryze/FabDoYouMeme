// backend/internal/game/hub_packs.go
//
// Weighted multi-pack helpers shared by the runRounds primary-pack pick and
// the hand-deck loader. ADR-016 introduced the `room_packs` join table and
// per-pack `weight`; the math is renormalised at sample time so the host
// never needs to think in fractions or "must sum to 100".
package game

import (
	"context"
	"fmt"
	"math"
	mathrand "math/rand/v2"
	"sort"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

// ensurePacksLoaded hydrates h.packsByRole on first access. Hubs created by
// the production manager already have it populated; the test seam
// GetOrCreate(...) does not, so the lazy load keeps that path working.
func (h *Hub) ensurePacksLoaded(ctx context.Context) error {
	if len(h.packsByRole) > 0 {
		return nil
	}
	rows, err := h.db.ListRoomPacks(ctx, h.roomID)
	if err != nil {
		return err
	}
	out := map[PackRole][]WeightedPackRef{}
	for _, row := range rows {
		out[PackRole(row.Role)] = append(out[PackRole(row.Role)], WeightedPackRef{
			PackID: row.PackID,
			Weight: int(row.Weight),
		})
	}
	h.packsByRole = out
	return nil
}

// primaryRole returns the first role declared by the handler, or empty.
func (h *Hub) primaryRole() PackRole {
	handler, ok := h.registry.Get(h.gameTypeSlug)
	if !ok {
		return ""
	}
	reqs := handler.RequiredPacks()
	if len(reqs) == 0 {
		return ""
	}
	return reqs[0].Role
}

// secondaryRole returns the second role declared by the handler, or empty.
func (h *Hub) secondaryRole() PackRole {
	handler, ok := h.registry.Get(h.gameTypeSlug)
	if !ok {
		return ""
	}
	reqs := handler.RequiredPacks()
	if len(reqs) < 2 {
		return ""
	}
	return reqs[1].Role
}

// pickPrimaryItem chooses the next round's item from the room's primary-role
// pack mix. Pack pick is weighted (renormalised); fallback walks the rest of
// the list in weight order if the chosen pack has no unplayed items left.
// Returns an error only when every pack in the role is exhausted.
func (h *Hub) pickPrimaryItem(ctx context.Context, versions []int32) (db.GetRandomUnplayedItemsRow, error) {
	if err := h.ensurePacksLoaded(ctx); err != nil {
		return db.GetRandomUnplayedItemsRow{}, err
	}
	role := h.primaryRole()
	entries := h.packsByRole[role]
	if len(entries) == 0 {
		return db.GetRandomUnplayedItemsRow{}, fmt.Errorf("no packs configured for role %q", role)
	}
	for _, e := range weightedShuffle(entries) {
		items, err := h.db.GetRandomUnplayedItems(ctx, db.GetRandomUnplayedItemsParams{
			PackID:   e.PackID,
			Versions: versions,
			RoomID:   h.roomID,
		})
		if err != nil {
			return db.GetRandomUnplayedItemsRow{}, err
		}
		if len(items) > 0 {
			return items[0], nil
		}
	}
	return db.GetRandomUnplayedItemsRow{}, fmt.Errorf("all packs exhausted for role %q", role)
}

// weightedShuffle returns the entries in a random order biased by weight.
// Each entry's sort key is `-ln(u) / weight`, where u is uniform in (0,1].
// Lowest key wins; the first element is the "chosen" pack for this draw and
// the remaining elements are the fallback order if the first pack is empty.
//
// Reference: Efraimidis & Spirakis, "Weighted random sampling with a
// reservoir" (2006). For our purpose (n ≤ ~5 packs per role) the cost is
// negligible.
func weightedShuffle(entries []WeightedPackRef) []WeightedPackRef {
	type keyed struct {
		ref WeightedPackRef
		key float64
	}
	scored := make([]keyed, len(entries))
	for i, e := range entries {
		u := mathrand.Float64()
		if u < 1e-12 {
			u = 1e-12
		}
		w := float64(e.Weight)
		if w <= 0 {
			w = 1
		}
		scored[i] = keyed{ref: e, key: -math.Log(u) / w}
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].key < scored[j].key })
	out := make([]WeightedPackRef, len(scored))
	for i, s := range scored {
		out[i] = s.ref
	}
	return out
}

