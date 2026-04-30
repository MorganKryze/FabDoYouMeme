// backend/internal/game/types/prompt_showdown/handler_test.go
package promptshowdown

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

func TestPromptShowdown_Slug(t *testing.T) {
	if got := New().Slug(); got != "prompt-showdown" {
		t.Fatalf("Slug = %q, want prompt-showdown", got)
	}
}

func TestPromptShowdown_RequiredPacks_PromptAndFiller(t *testing.T) {
	reqs := New().RequiredPacks()
	if len(reqs) != 2 {
		t.Fatalf("RequiredPacks len = %d, want 2", len(reqs))
	}
	if reqs[0].Role != game.PackRolePrompt {
		t.Fatalf("primary Role = %q, want prompt", reqs[0].Role)
	}
	if reqs[1].Role != game.PackRoleFiller {
		t.Fatalf("secondary Role = %q, want filler", reqs[1].Role)
	}
	// Filler scales: initial hand + refills.
	got := reqs[1].MinItemsFn(game.RoomConfig{RoundCount: 5, HandSize: 5}, 4)
	if got != 36 {
		t.Fatalf("filler MinItemsFn(round=5, hand=5, players=4) = %d, want 36", got)
	}
	// Single round = no refills.
	if got := reqs[1].MinItemsFn(game.RoomConfig{RoundCount: 1, HandSize: 5}, 4); got != 20 {
		t.Fatalf("filler MinItemsFn(round=1) = %d, want 20", got)
	}
	// Prompt = round_count.
	if got := reqs[0].MinItemsFn(game.RoomConfig{RoundCount: 8}, 12); got != 8 {
		t.Fatalf("prompt MinItemsFn = %d, want 8", got)
	}
}

func TestPromptShowdown_PersonalisesRoundStart_True(t *testing.T) {
	if !New().PersonalisesRoundStart() {
		t.Fatalf("prompt-showdown must personalise round_started for hand")
	}
}

func TestPromptShowdown_ValidateSubmission_OK(t *testing.T) {
	raw, _ := json.Marshal(submitPayload{CardID: uuid.NewString()})
	if err := New().ValidateSubmission(game.Round{}, raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromptShowdown_ValidateSubmission_MissingCardID(t *testing.T) {
	if err := New().ValidateSubmission(game.Round{}, json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error for missing card_id")
	}
}

func TestPromptShowdown_ValidateSubmission_InvalidUUID(t *testing.T) {
	raw := json.RawMessage(`{"card_id": "not-a-uuid"}`)
	if err := New().ValidateSubmission(game.Round{}, raw); err == nil {
		t.Fatal("expected error for non-UUID card_id")
	}
}

func TestPromptShowdown_ValidateVote_SelfVoteRejected(t *testing.T) {
	voterID := uuid.New()
	sub := game.Submission{UserID: voterID}
	if err := New().ValidateVote(game.Round{}, sub, voterID, nil); !errors.Is(err, ErrSelfVote) {
		t.Fatalf("want ErrSelfVote, got %v", err)
	}
}

func TestPromptShowdown_CalculateRoundScores(t *testing.T) {
	authorA := uuid.New()
	authorB := uuid.New()
	subA := uuid.New()
	subB := uuid.New()
	subs := []game.Submission{
		{ID: subA, UserID: authorA},
		{ID: subB, UserID: authorB},
	}
	votes := []game.Vote{
		{SubmissionID: subA},
		{SubmissionID: subA},
		{SubmissionID: subB},
	}
	scores := New().CalculateRoundScores(subs, votes)
	if scores[authorA] != 2 || scores[authorB] != 1 {
		t.Fatalf("scores wrong: %v", scores)
	}
}

func TestPromptShowdown_BuildSubmissionsShownPayload_HidesAuthors(t *testing.T) {
	body, _ := json.Marshal(submitPayload{CardID: uuid.NewString(), Text: "ma belle-mère"})
	subs := []game.Submission{
		{ID: uuid.New(), UserID: uuid.New(), AuthorUsername: "Alice", Payload: body},
	}
	raw, err := New().BuildSubmissionsShownPayload(subs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out struct {
		Submissions []map[string]any `json:"submissions"`
	}
	_ = json.Unmarshal(raw, &out)
	if _, leaked := out.Submissions[0]["username"]; leaked {
		t.Fatalf("username leaked in submissions_shown")
	}
	if out.Submissions[0]["text"] != "ma belle-mère" {
		t.Fatalf("text missing or wrong: %v", out.Submissions[0])
	}
}

func TestPromptShowdown_BuildVoteResultsPayload_RevealsAuthors(t *testing.T) {
	authorID := uuid.New()
	body, _ := json.Marshal(submitPayload{CardID: uuid.NewString(), Text: "the filler"})
	subs := []game.Submission{
		{ID: uuid.New(), UserID: authorID, AuthorUsername: "Alice", Payload: body},
	}
	scores := map[uuid.UUID]int{authorID: 2}
	votes := []game.Vote{{SubmissionID: subs[0].ID}, {SubmissionID: subs[0].ID}}
	raw, err := New().BuildVoteResultsPayload(subs, votes, scores)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out struct {
		Submissions []map[string]any `json:"submissions"`
	}
	_ = json.Unmarshal(raw, &out)
	if out.Submissions[0]["username"] != "Alice" {
		t.Fatalf("author not revealed: %v", out.Submissions[0])
	}
	if int(out.Submissions[0]["votes_received"].(float64)) != 2 {
		t.Fatalf("votes_received wrong: %v", out.Submissions[0])
	}
}
