package game_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

func validBounds() game.Bounds {
	return game.Bounds{
		MinRoundDurationSeconds:      15,
		MaxRoundDurationSeconds:      300,
		DefaultRoundDurationSeconds:  60,
		MinVotingDurationSeconds:     10,
		MaxVotingDurationSeconds:     120,
		DefaultVotingDurationSeconds: 30,
		MinRoundCount:                1,
		MaxRoundCount:                50,
		DefaultRoundCount:            10,
		MinPlayers:                   2,
	}
}

type filledConfig struct {
	RoundDurationSeconds  int  `json:"round_duration_seconds"`
	VotingDurationSeconds int  `json:"voting_duration_seconds"`
	RoundCount            int  `json:"round_count"`
	HostPaced             bool `json:"host_paced"`
	JokerCount            int  `json:"joker_count"`
	AllowSkipVote         bool `json:"allow_skip_vote"`
}

func mustFill(t *testing.T, b game.Bounds, raw string) filledConfig {
	t.Helper()
	out, err := b.ValidateAndFill(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("ValidateAndFill(%s): unexpected error %v", raw, err)
	}
	var cfg filledConfig
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal filled: %v", err)
	}
	return cfg
}

func TestValidateAndFill_DefaultsJokerCountFromRoundCount(t *testing.T) {
	cfg := mustFill(t, validBounds(), `{"round_count":10}`)
	if cfg.JokerCount != 2 { // ceil(10/5) == 2
		t.Fatalf("joker_count: want 2, got %d", cfg.JokerCount)
	}
	if !cfg.AllowSkipVote {
		t.Fatalf("allow_skip_vote: want true, got false")
	}
}

func TestValidateAndFill_DefaultsJokerCountRoundsUp(t *testing.T) {
	cfg := mustFill(t, validBounds(), `{"round_count":7}`)
	if cfg.JokerCount != 2 { // ceil(7/5) == 2
		t.Fatalf("joker_count: want 2, got %d", cfg.JokerCount)
	}
}

func TestValidateAndFill_HonoursExplicitJokerCountZero(t *testing.T) {
	cfg := mustFill(t, validBounds(), `{"round_count":10,"joker_count":0}`)
	if cfg.JokerCount != 0 {
		t.Fatalf("joker_count: want 0 (explicit disable), got %d", cfg.JokerCount)
	}
}

func TestValidateAndFill_HonoursExplicitAllowSkipVoteFalse(t *testing.T) {
	cfg := mustFill(t, validBounds(), `{"round_count":10,"allow_skip_vote":false}`)
	if cfg.AllowSkipVote {
		t.Fatalf("allow_skip_vote: want false (explicit disable), got true")
	}
}

func TestValidateAndFill_RejectsNegativeJokerCount(t *testing.T) {
	_, err := validBounds().ValidateAndFill(json.RawMessage(`{"round_count":10,"joker_count":-1}`))
	var verr *game.ValidationError
	if !errors.As(err, &verr) || verr.Field != "joker_count" {
		t.Fatalf("want ValidationError on joker_count, got %v", err)
	}
}

func TestValidateAndFill_RejectsJokerCountAboveRoundCount(t *testing.T) {
	_, err := validBounds().ValidateAndFill(json.RawMessage(`{"round_count":5,"joker_count":6}`))
	var verr *game.ValidationError
	if !errors.As(err, &verr) || verr.Field != "joker_count" {
		t.Fatalf("want ValidationError on joker_count, got %v", err)
	}
}

func TestValidateAndFill_JokerCountEqualToRoundCountOK(t *testing.T) {
	cfg := mustFill(t, validBounds(), `{"round_count":5,"joker_count":5}`)
	if cfg.JokerCount != 5 {
		t.Fatalf("joker_count: want 5 (upper bound inclusive), got %d", cfg.JokerCount)
	}
}

// handBounds returns a Bounds with hand-size opted in. Game types that don't
// deal hands (meme-freestyle) leave MinHandSize/MaxHandSize/DefaultHandSize
// at zero; hand-size tests below assert only those paths.
func handBounds() game.Bounds {
	b := validBounds()
	b.MinHandSize = 3
	b.MaxHandSize = 7
	b.DefaultHandSize = 5
	return b
}

func TestValidateAndFill_HandSize_DefaultWhenMissing(t *testing.T) {
	out, err := handBounds().ValidateAndFill(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var cfg game.RoomConfig
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.HandSize != 5 {
		t.Fatalf("HandSize = %d, want 5", cfg.HandSize)
	}
}

func TestValidateAndFill_HandSize_BelowMin(t *testing.T) {
	_, err := handBounds().ValidateAndFill(json.RawMessage(`{"hand_size":2}`))
	var verr *game.ValidationError
	if !errors.As(err, &verr) || verr.Field != "hand_size" {
		t.Fatalf("expected hand_size ValidationError, got %v", err)
	}
}

func TestValidateAndFill_HandSize_AboveMax(t *testing.T) {
	_, err := handBounds().ValidateAndFill(json.RawMessage(`{"hand_size":99}`))
	var verr *game.ValidationError
	if !errors.As(err, &verr) || verr.Field != "hand_size" {
		t.Fatalf("expected hand_size ValidationError, got %v", err)
	}
}

func TestValidateAndFill_HandSize_OmittedWhenNotOptedIn(t *testing.T) {
	// Bounds without hand_size opt-in should not populate HandSize even if
	// the client sent one — the manifest signals "no hand" to the handler.
	out, err := validBounds().ValidateAndFill(json.RawMessage(`{"hand_size":5}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var cfg game.RoomConfig
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.HandSize != 0 {
		t.Fatalf("HandSize = %d, want 0 when bounds opt-out", cfg.HandSize)
	}
}

// cappedBounds returns a Bounds that declares a player cap. Used by the
// max_players tests below — most other tests use validBounds() which leaves
// MaxPlayers nil ("unbounded"), and we want to assert both shapes.
func cappedBounds() game.Bounds {
	b := validBounds()
	cap := 12
	b.MaxPlayers = &cap
	return b
}

func TestValidateAndFill_MaxPlayers_DefaultsToManifestCap(t *testing.T) {
	out, err := cappedBounds().ValidateAndFill(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var cfg game.RoomConfig
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.MaxPlayers != 12 {
		t.Fatalf("MaxPlayers = %d, want 12 (manifest default)", cfg.MaxPlayers)
	}
}

func TestValidateAndFill_MaxPlayers_HonoursExplicit(t *testing.T) {
	out, err := cappedBounds().ValidateAndFill(json.RawMessage(`{"max_players":4}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var cfg game.RoomConfig
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.MaxPlayers != 4 {
		t.Fatalf("MaxPlayers = %d, want 4", cfg.MaxPlayers)
	}
}

func TestValidateAndFill_MaxPlayers_RejectsBelowMinPlayers(t *testing.T) {
	_, err := cappedBounds().ValidateAndFill(json.RawMessage(`{"max_players":1}`))
	var verr *game.ValidationError
	if !errors.As(err, &verr) || verr.Field != "max_players" {
		t.Fatalf("expected max_players ValidationError, got %v", err)
	}
}

func TestValidateAndFill_MaxPlayers_RejectsAboveManifestCap(t *testing.T) {
	_, err := cappedBounds().ValidateAndFill(json.RawMessage(`{"max_players":99}`))
	var verr *game.ValidationError
	if !errors.As(err, &verr) || verr.Field != "max_players" {
		t.Fatalf("expected max_players ValidationError, got %v", err)
	}
}

func TestValidateAndFill_MaxPlayers_UnboundedManifestLeavesZero(t *testing.T) {
	// validBounds() has MaxPlayers nil — manifest opted out of a cap. Without
	// explicit input the canonical config should leave MaxPlayers at 0 so
	// the hub keeps its existing "no cap" behaviour.
	out, err := validBounds().ValidateAndFill(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var cfg game.RoomConfig
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.MaxPlayers != 0 {
		t.Fatalf("MaxPlayers = %d, want 0 when manifest unbounded and no input", cfg.MaxPlayers)
	}
}

func TestValidateAndFill_MaxPlayers_UnboundedManifestAcceptsInput(t *testing.T) {
	// When the manifest is unbounded, an explicit value is accepted as long
	// as it satisfies MinPlayers. There is no upper bound to enforce.
	out, err := validBounds().ValidateAndFill(json.RawMessage(`{"max_players":50}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var cfg game.RoomConfig
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.MaxPlayers != 50 {
		t.Fatalf("MaxPlayers = %d, want 50", cfg.MaxPlayers)
	}
}
