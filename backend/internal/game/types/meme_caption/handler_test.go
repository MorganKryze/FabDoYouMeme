// backend/internal/game/types/meme_caption/handler_test.go
package memecaption_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	memecaption "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_caption"
)

func newHandler() *memecaption.Handler {
	return memecaption.New()
}

func TestSlug(t *testing.T) {
	if newHandler().Slug() != "meme-caption" {
		t.Error("expected slug 'meme-caption'")
	}
}

func TestSupportedPayloadVersions(t *testing.T) {
	versions := newHandler().SupportedPayloadVersions()
	if len(versions) == 0 {
		t.Error("expected at least one supported payload version")
	}
	found := false
	for _, v := range versions {
		if v == 1 {
			found = true
		}
	}
	if !found {
		t.Error("expected version 1 to be supported")
	}
}

func TestValidateSubmission_TooLong(t *testing.T) {
	h := newHandler()
	long := strings.Repeat("a", 201)
	payload, _ := json.Marshal(map[string]string{"caption": long})
	err := h.ValidateSubmission(game.Round{}, payload)
	if err == nil {
		t.Error("expected error for caption > 200 chars")
	}
}

func TestValidateSubmission_Empty(t *testing.T) {
	h := newHandler()
	payload, _ := json.Marshal(map[string]string{"caption": ""})
	err := h.ValidateSubmission(game.Round{}, payload)
	if err == nil {
		t.Error("expected error for empty caption")
	}
}

func TestValidateSubmission_OK(t *testing.T) {
	h := newHandler()
	payload, _ := json.Marshal(map[string]string{"caption": "This is funny"})
	if err := h.ValidateSubmission(game.Round{}, payload); err != nil {
		t.Errorf("expected no error: %v", err)
	}
}

func TestValidateVote_SelfVote(t *testing.T) {
	h := newHandler()
	voterID := uuid.New()
	submission := game.Submission{UserID: voterID}
	err := h.ValidateVote(game.Round{}, submission, voterID, json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error for self-vote")
	}
	if !errors.Is(err, memecaption.ErrSelfVote) {
		t.Errorf("expected ErrSelfVote, got %v", err)
	}
}

func TestValidateVote_OK(t *testing.T) {
	h := newHandler()
	voterID := uuid.New()
	authorID := uuid.New()
	submission := game.Submission{UserID: authorID}
	if err := h.ValidateVote(game.Round{}, submission, voterID, json.RawMessage(`{}`)); err != nil {
		t.Errorf("expected no error: %v", err)
	}
}

func TestCalculateRoundScores_OneVotePerSubmission(t *testing.T) {
	h := newHandler()
	authorA := uuid.New()
	authorB := uuid.New()
	voter1 := uuid.New()
	voter2 := uuid.New()
	subA := game.Submission{ID: uuid.New(), UserID: authorA}
	subB := game.Submission{ID: uuid.New(), UserID: authorB}
	votes := []game.Vote{
		{SubmissionID: subA.ID, VoterID: voter1},
		{SubmissionID: subA.ID, VoterID: voter2},
		{SubmissionID: subB.ID, VoterID: authorA}, // authorA votes for authorB
	}
	scores := h.CalculateRoundScores([]game.Submission{subA, subB}, votes)
	if scores[authorA] != 2 {
		t.Errorf("authorA should have 2 votes, got %d", scores[authorA])
	}
	if scores[authorB] != 1 {
		t.Errorf("authorB should have 1 vote, got %d", scores[authorB])
	}
}

func TestBuildSubmissionsShownPayload_HidesAuthors(t *testing.T) {
	h := newHandler()
	subs := []game.Submission{
		{ID: uuid.New(), UserID: uuid.New(), Payload: json.RawMessage(`{"caption":"funny"}`)},
	}
	payload, err := h.BuildSubmissionsShownPayload(subs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(payload), "author") {
		t.Error("submissions_shown payload should not reveal author information")
	}
}
