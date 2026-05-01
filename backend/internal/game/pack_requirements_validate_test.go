package game

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// stubCounter satisfies PackItemCounter with a fixed lookup. Pool counting
// sums the per-pack counts across the requested ids.
type stubCounter struct {
	counts map[[16]byte]map[int]int64
}

func (s stubCounter) CountItemsForPack(_ context.Context, packID [16]byte, versions []int) (int64, error) {
	total := int64(0)
	for _, v := range versions {
		total += s.counts[packID][v]
	}
	return total, nil
}

func (s stubCounter) CountItemsForPacksPool(_ context.Context, packIDs [][16]byte, versions []int) (int64, error) {
	total := int64(0)
	for _, id := range packIDs {
		for _, v := range versions {
			total += s.counts[id][v]
		}
	}
	return total, nil
}

// stubGameTypeHandler satisfies GameTypeHandler with fixed RequiredPacks.
// Other methods return zero values; ValidatePackRequirements does not call them.
type stubGameTypeHandler struct {
	reqs []PackRequirement
}

func (s stubGameTypeHandler) Slug() string                     { return "stub" }
func (s stubGameTypeHandler) RequiredPacks() []PackRequirement { return s.reqs }
func (s stubGameTypeHandler) SupportsSolo() bool               { return false }
func (s stubGameTypeHandler) MaxPlayers() int                  { return 0 }
func (s stubGameTypeHandler) Manifest() *Manifest              { return &Manifest{} }
func (s stubGameTypeHandler) ValidateSubmission(Round, json.RawMessage) error {
	return nil
}
func (s stubGameTypeHandler) ValidateVote(Round, Submission, uuid.UUID, json.RawMessage) error {
	return nil
}
func (s stubGameTypeHandler) CalculateRoundScores([]Submission, []Vote) map[uuid.UUID]int {
	return nil
}
func (s stubGameTypeHandler) BuildSubmissionsShownPayload([]Submission) (json.RawMessage, error) {
	return nil, nil
}
func (s stubGameTypeHandler) BuildVoteResultsPayload([]Submission, []Vote, map[uuid.UUID]int) (json.RawMessage, error) {
	return nil, nil
}
func (s stubGameTypeHandler) PersonalisesRoundStart() bool { return false }

func stubHandlerWithReqs(reqs ...PackRequirement) GameTypeHandler {
	return stubGameTypeHandler{reqs: reqs}
}

func single(id [16]byte) []WeightedPackRef {
	return []WeightedPackRef{{PackID: id, Weight: 1}}
}

func TestValidatePackRequirements_SingleRole_OK(t *testing.T) {
	handler := stubHandlerWithReqs(
		PackRequirement{Role: PackRoleImage, PayloadVersions: []int{1},
			MinItemsFn: func(cfg RoomConfig, _ int) int { return cfg.RoundCount }},
	)
	imgID := [16]byte{1}
	counter := stubCounter{counts: map[[16]byte]map[int]int64{
		imgID: {1: 10},
	}}
	err := ValidatePackRequirements(
		context.Background(),
		counter,
		handler,
		RoomConfig{RoundCount: 5},
		map[PackRole][]WeightedPackRef{PackRoleImage: single(imgID)},
		8,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePackRequirements_MissingRole(t *testing.T) {
	handler := stubHandlerWithReqs(
		PackRequirement{Role: PackRoleImage, PayloadVersions: []int{1},
			MinItemsFn: func(cfg RoomConfig, _ int) int { return cfg.RoundCount }},
		PackRequirement{Role: PackRoleText, PayloadVersions: []int{2},
			MinItemsFn: func(cfg RoomConfig, p int) int { return cfg.RoundCount * p }},
	)
	imgID := [16]byte{1}
	counter := stubCounter{counts: map[[16]byte]map[int]int64{imgID: {1: 100}}}
	err := ValidatePackRequirements(
		context.Background(),
		counter,
		handler,
		RoomConfig{RoundCount: 5},
		map[PackRole][]WeightedPackRef{PackRoleImage: single(imgID)}, // text omitted
		8,
	)
	if err == nil || err.Code != "text_pack_required" {
		t.Fatalf("want text_pack_required, got %v", err)
	}
}

func TestValidatePackRequirements_InsufficientText(t *testing.T) {
	handler := stubHandlerWithReqs(
		PackRequirement{Role: PackRoleImage, PayloadVersions: []int{1},
			MinItemsFn: func(cfg RoomConfig, _ int) int { return cfg.RoundCount }},
		PackRequirement{Role: PackRoleText, PayloadVersions: []int{2},
			MinItemsFn: func(cfg RoomConfig, p int) int { return cfg.RoundCount * p }},
	)
	imgID := [16]byte{1}
	textID := [16]byte{2}
	counter := stubCounter{counts: map[[16]byte]map[int]int64{
		imgID:  {1: 100},
		textID: {2: 5}, // need 5*8 = 40
	}}
	err := ValidatePackRequirements(
		context.Background(),
		counter,
		handler,
		RoomConfig{RoundCount: 5},
		map[PackRole][]WeightedPackRef{
			PackRoleImage: single(imgID),
			PackRoleText:  single(textID),
		},
		8,
	)
	if err == nil || err.Code != "text_pack_insufficient" {
		t.Fatalf("want text_pack_insufficient, got %v", err)
	}
}

func TestValidatePackRequirements_UnsolicitedRole(t *testing.T) {
	handler := stubHandlerWithReqs(
		PackRequirement{Role: PackRoleImage, PayloadVersions: []int{1},
			MinItemsFn: func(cfg RoomConfig, _ int) int { return cfg.RoundCount }},
	)
	imgID := [16]byte{1}
	textID := [16]byte{2}
	counter := stubCounter{counts: map[[16]byte]map[int]int64{imgID: {1: 100}}}
	err := ValidatePackRequirements(
		context.Background(),
		counter,
		handler,
		RoomConfig{RoundCount: 5},
		map[PackRole][]WeightedPackRef{
			PackRoleImage: single(imgID),
			PackRoleText:  single(textID),
		},
		8,
	)
	if err == nil || err.Code != "text_pack_not_applicable" {
		t.Fatalf("want text_pack_not_applicable, got %v", err)
	}
}

func TestValidatePackRequirements_NoSupportedItems(t *testing.T) {
	handler := stubHandlerWithReqs(
		PackRequirement{Role: PackRoleImage, PayloadVersions: []int{1},
			MinItemsFn: func(cfg RoomConfig, _ int) int { return cfg.RoundCount }},
	)
	imgID := [16]byte{1}
	counter := stubCounter{counts: map[[16]byte]map[int]int64{imgID: {}}}
	err := ValidatePackRequirements(
		context.Background(),
		counter,
		handler,
		RoomConfig{RoundCount: 5},
		map[PackRole][]WeightedPackRef{PackRoleImage: single(imgID)},
		8,
	)
	if err == nil || err.Code != "image_pack_no_supported_items" {
		t.Fatalf("want image_pack_no_supported_items, got %v", err)
	}
}

// TestValidatePackRequirements_PoolModelOK exercises the ADR-016 pool-model
// path: a small accent pack rides alongside a large primary pack, sum
// satisfies MinItemsFn even though the small pack alone wouldn't.
func TestValidatePackRequirements_PoolModelOK(t *testing.T) {
	handler := stubHandlerWithReqs(
		PackRequirement{Role: PackRoleImage, PayloadVersions: []int{1},
			MinItemsFn: func(cfg RoomConfig, _ int) int { return cfg.RoundCount }},
	)
	bigID := [16]byte{1}
	smallID := [16]byte{2}
	counter := stubCounter{counts: map[[16]byte]map[int]int64{
		bigID:   {1: 9},
		smallID: {1: 2}, // pool sum = 11, need 10
	}}
	err := ValidatePackRequirements(
		context.Background(),
		counter,
		handler,
		RoomConfig{RoundCount: 10},
		map[PackRole][]WeightedPackRef{PackRoleImage: {
			{PackID: bigID, Weight: 9},
			{PackID: smallID, Weight: 1},
		}},
		8,
	)
	if err != nil {
		t.Fatalf("expected pool-model accept, got %v", err)
	}
}

// TestValidatePackRequirements_DuplicatePackInRole rejects "same pack twice".
func TestValidatePackRequirements_DuplicatePackInRole(t *testing.T) {
	handler := stubHandlerWithReqs(
		PackRequirement{Role: PackRoleImage, PayloadVersions: []int{1},
			MinItemsFn: func(cfg RoomConfig, _ int) int { return cfg.RoundCount }},
	)
	id := [16]byte{1}
	counter := stubCounter{counts: map[[16]byte]map[int]int64{id: {1: 100}}}
	err := ValidatePackRequirements(
		context.Background(),
		counter,
		handler,
		RoomConfig{RoundCount: 5},
		map[PackRole][]WeightedPackRef{PackRoleImage: {
			{PackID: id, Weight: 1},
			{PackID: id, Weight: 1},
		}},
		8,
	)
	if err == nil || err.Code != "image_pack_invalid" {
		t.Fatalf("want image_pack_invalid, got %v", err)
	}
}

// TestValidatePackRequirements_ZeroWeight rejects weight ≤ 0.
func TestValidatePackRequirements_ZeroWeight(t *testing.T) {
	handler := stubHandlerWithReqs(
		PackRequirement{Role: PackRoleImage, PayloadVersions: []int{1},
			MinItemsFn: func(cfg RoomConfig, _ int) int { return cfg.RoundCount }},
	)
	id := [16]byte{1}
	counter := stubCounter{counts: map[[16]byte]map[int]int64{id: {1: 100}}}
	err := ValidatePackRequirements(
		context.Background(),
		counter,
		handler,
		RoomConfig{RoundCount: 5},
		map[PackRole][]WeightedPackRef{PackRoleImage: {
			{PackID: id, Weight: 0},
		}},
		8,
	)
	if err == nil || err.Code != "image_pack_invalid" {
		t.Fatalf("want image_pack_invalid, got %v", err)
	}
}
