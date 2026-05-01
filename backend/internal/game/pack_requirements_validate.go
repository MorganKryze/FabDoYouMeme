// backend/internal/game/pack_requirements_validate.go
package game

import (
	"context"
	"fmt"
)

// PackItemCounter is the minimal DB surface ValidatePackRequirements needs.
// The single-pack count is used for the per-pack misconfig check (zero
// compatible items in a chosen pack); the pool count handles the role-wide
// "is the mix large enough" assertion (ADR-016 pool model).
type PackItemCounter interface {
	CountItemsForPack(ctx context.Context, packID [16]byte, versions []int) (int64, error)
	CountItemsForPacksPool(ctx context.Context, packIDs [][16]byte, versions []int) (int64, error)
}

// PackError is the structured outcome of ValidatePackRequirements. Code is a
// stable, machine-readable string used as the REST error code; Message is
// human-readable. Role names the requirement that failed.
type PackError struct {
	Code    string
	Message string
	Role    PackRole
}

func (e *PackError) Error() string { return e.Message }

// ValidatePackRequirements checks every role declared by the handler against
// the weighted pack lists supplied by the caller.
//
// Per role:
//   - Role must be present in packRefs with at least one entry.
//   - Every entry's weight must be > 0; pack ids must be distinct.
//   - Every individual pack in the list must contain at least one item with a
//     payload_version the role accepts (otherwise it's a misconfig — host
//     picked a text pack for the image role).
//   - The POOL of items across all listed packs must satisfy MinItemsFn.
//
// Roles supplied that the handler did not declare are rejected with
// <role>_pack_not_applicable for parity with the single-pack predecessor.
func ValidatePackRequirements(
	ctx context.Context,
	counter PackItemCounter,
	handler GameTypeHandler,
	cfg RoomConfig,
	packRefs map[PackRole][]WeightedPackRef,
	maxPlayers int,
) *PackError {
	required := make(map[PackRole]PackRequirement, len(handler.RequiredPacks()))
	for _, pr := range handler.RequiredPacks() {
		required[pr.Role] = pr
	}

	// Reject any role the caller supplied that the handler did not declare.
	for role := range packRefs {
		if _, ok := required[role]; !ok {
			return &PackError{
				Code:    string(role) + "_pack_not_applicable",
				Message: fmt.Sprintf("%s_pack is not applicable to this game type", role),
				Role:    role,
			}
		}
	}

	for role, req := range required {
		entries, present := packRefs[role]
		if !present || len(entries) == 0 {
			return &PackError{
				Code:    string(role) + "_pack_required",
				Message: fmt.Sprintf("%s pack is required for this game type", role),
				Role:    role,
			}
		}
		seen := make(map[[16]byte]struct{}, len(entries))
		for _, e := range entries {
			if e.Weight <= 0 {
				return &PackError{
					Code:    string(role) + "_pack_invalid",
					Message: fmt.Sprintf("%s pack weight must be a positive integer", role),
					Role:    role,
				}
			}
			if _, dup := seen[e.PackID]; dup {
				return &PackError{
					Code:    string(role) + "_pack_invalid",
					Message: fmt.Sprintf("%s pack list cannot include the same pack twice", role),
					Role:    role,
				}
			}
			seen[e.PackID] = struct{}{}
		}
		// Per-pack misconfig check: every pack in the list must hold at
		// least one compatible item, otherwise the host picked a wrong-kind
		// pack and the round-time picker would only ever return zero items.
		for _, e := range entries {
			count, err := counter.CountItemsForPack(ctx, e.PackID, req.PayloadVersions)
			if err != nil {
				return &PackError{
					Code:    "internal_error",
					Message: "Failed to count pack items: " + err.Error(),
					Role:    role,
				}
			}
			if count == 0 {
				return &PackError{
					Code:    string(role) + "_pack_no_supported_items",
					Message: fmt.Sprintf("%s pack has no items compatible with this game type", role),
					Role:    role,
				}
			}
		}
		// Pool capacity: SUM across the role's packs must satisfy MinItemsFn.
		ids := make([][16]byte, len(entries))
		for i, e := range entries {
			ids[i] = e.PackID
		}
		total, err := counter.CountItemsForPacksPool(ctx, ids, req.PayloadVersions)
		if err != nil {
			return &PackError{
				Code:    "internal_error",
				Message: "Failed to count pack items: " + err.Error(),
				Role:    role,
			}
		}
		needed := int64(req.MinItemsFn(cfg, maxPlayers))
		if total < needed {
			return &PackError{
				Code:    string(role) + "_pack_insufficient",
				Message: fmt.Sprintf("%s packs hold %d compatible items combined; %d required", role, total, needed),
				Role:    role,
			}
		}
	}
	return nil
}
