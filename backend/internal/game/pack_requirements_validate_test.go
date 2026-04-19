package game

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// stubCounter satisfies PackItemCounter with a fixed lookup.
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
		map[PackRole][16]byte{PackRoleImage: imgID},
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
		map[PackRole][16]byte{PackRoleImage: imgID}, // text omitted
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
		map[PackRole][16]byte{PackRoleImage: imgID, PackRoleText: textID},
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
		map[PackRole][16]byte{PackRoleImage: imgID, PackRoleText: textID},
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
		map[PackRole][16]byte{PackRoleImage: imgID},
		8,
	)
	if err == nil || err.Code != "image_pack_no_supported_items" {
		t.Fatalf("want image_pack_no_supported_items, got %v", err)
	}
}
