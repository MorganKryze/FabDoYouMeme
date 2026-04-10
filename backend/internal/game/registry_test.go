// backend/internal/game/registry_test.go
package game_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

// stubHandler is a minimal GameTypeHandler for registry tests.
type stubHandler struct{ slug string }

func (s *stubHandler) Slug() string                    { return s.slug }
func (s *stubHandler) SupportedPayloadVersions() []int { return []int{1} }
func (s *stubHandler) SupportsSolo() bool              { return false }
func (s *stubHandler) MaxPlayers() int                 { return 0 }
func (s *stubHandler) ValidateSubmission(_ game.Round, _ json.RawMessage) error { return nil }
func (s *stubHandler) ValidateVote(_ game.Round, _ game.Submission, _ uuid.UUID, _ json.RawMessage) error {
	return nil
}
func (s *stubHandler) CalculateRoundScores(_ []game.Submission, _ []game.Vote) map[uuid.UUID]int {
	return nil
}
func (s *stubHandler) BuildSubmissionsShownPayload(_ []game.Submission) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (s *stubHandler) BuildVoteResultsPayload(_ []game.Submission, _ []game.Vote, _ map[uuid.UUID]int) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := game.NewRegistry()
	r.Register(&stubHandler{slug: "test-game"})
	h, ok := r.Get("test-game")
	if !ok {
		t.Fatal("expected handler to be registered")
	}
	if h.Slug() != "test-game" {
		t.Errorf("wrong slug: %s", h.Slug())
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate slug registration")
		}
	}()
	r := game.NewRegistry()
	r.Register(&stubHandler{slug: "dup"})
	r.Register(&stubHandler{slug: "dup"}) // must panic
}

func TestRegistry_GetMissing(t *testing.T) {
	r := game.NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected ok=false for unregistered slug")
	}
}
