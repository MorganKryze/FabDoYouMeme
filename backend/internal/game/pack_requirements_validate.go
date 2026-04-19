// backend/internal/game/pack_requirements_validate.go
package game

import (
	"context"
	"fmt"
)

// PackItemCounter is the minimal DB surface ValidatePackRequirements needs.
// The concrete implementation lives in api/rooms.go and adapts the sqlc
// CountCompatibleItems query; an in-memory stub backs the tests in this
// package so internal/game/ has no import on the sqlc layer.
type PackItemCounter interface {
	CountItemsForPack(ctx context.Context, packID [16]byte, versions []int) (int64, error)
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
// the packs supplied by the caller. Missing-but-required, present-but-unsolicited,
// zero-compatible, and insufficient-count are all reported as a PackError with
// a stable code. On success returns nil.
func ValidatePackRequirements(
	ctx context.Context,
	counter PackItemCounter,
	handler GameTypeHandler,
	cfg RoomConfig,
	packRefs map[PackRole][16]byte,
	maxPlayers int,
) *PackError {
	required := make(map[PackRole]PackRequirement, len(handler.RequiredPacks()))
	for _, pr := range handler.RequiredPacks() {
		required[pr.Role] = pr
	}

	// Any role provided that the handler did not declare is a client error.
	for role := range packRefs {
		if _, ok := required[role]; !ok {
			return &PackError{
				Code:    string(role) + "_pack_not_applicable",
				Message: fmt.Sprintf("%s_pack is not applicable to this game type", role),
				Role:    role,
			}
		}
	}

	// Every declared role must be supplied and must meet count requirements.
	for role, req := range required {
		packID, present := packRefs[role]
		if !present {
			return &PackError{
				Code:    string(role) + "_pack_required",
				Message: fmt.Sprintf("%s pack is required for this game type", role),
				Role:    role,
			}
		}
		count, err := counter.CountItemsForPack(ctx, packID, req.PayloadVersions)
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
		needed := int64(req.MinItemsFn(cfg, maxPlayers))
		if count < needed {
			return &PackError{
				Code:    string(role) + "_pack_insufficient",
				Message: fmt.Sprintf("%s pack has %d compatible items; %d required", role, count, needed),
				Role:    role,
			}
		}
	}
	return nil
}
