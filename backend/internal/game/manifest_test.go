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
// deal hands (meme-caption) leave MinHandSize/MaxHandSize/DefaultHandSize
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
