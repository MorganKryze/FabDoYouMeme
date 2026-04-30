// backend/internal/game/types/prompt_freestyle/handler_test.go
package promptfreestyle

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

func TestPromptFreestyle_Slug(t *testing.T) {
	if got := New().Slug(); got != "prompt-freestyle" {
		t.Fatalf("Slug = %q, want prompt-freestyle", got)
	}
}

func TestPromptFreestyle_RequiredPacks_PromptOnly(t *testing.T) {
	reqs := New().RequiredPacks()
	if len(reqs) != 1 {
		t.Fatalf("RequiredPacks len = %d, want 1", len(reqs))
	}
	if reqs[0].Role != game.PackRolePrompt {
		t.Fatalf("Role = %q, want prompt", reqs[0].Role)
	}
	if got := reqs[0].MinItemsFn(game.RoomConfig{RoundCount: 8}, 12); got != 8 {
		t.Fatalf("MinItemsFn(round=8) = %d, want 8", got)
	}
	if len(reqs[0].PayloadVersions) != 1 || reqs[0].PayloadVersions[0] != 4 {
		t.Fatalf("PayloadVersions = %v, want [4]", reqs[0].PayloadVersions)
	}
}

func TestPromptFreestyle_PersonalisesRoundStart_False(t *testing.T) {
	if New().PersonalisesRoundStart() {
		t.Fatalf("prompt-freestyle should not personalise round_started")
	}
}

func TestPromptFreestyle_ValidateSubmission_OK(t *testing.T) {
	raw, _ := json.Marshal(submitPayload{Filler: "a slightly cursed answer"})
	if err := New().ValidateSubmission(game.Round{}, raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromptFreestyle_ValidateSubmission_Empty(t *testing.T) {
	raw, _ := json.Marshal(submitPayload{Filler: "   "})
	if err := New().ValidateSubmission(game.Round{}, raw); err == nil {
		t.Fatal("expected error for empty filler")
	}
}

func TestPromptFreestyle_ValidateSubmission_TooLong(t *testing.T) {
	raw, _ := json.Marshal(submitPayload{Filler: strings.Repeat("a", 201)})
	if err := New().ValidateSubmission(game.Round{}, raw); err == nil {
		t.Fatal("expected error for filler over 200 chars")
	}
}

func TestPromptFreestyle_ValidateSubmission_Malformed(t *testing.T) {
	if err := New().ValidateSubmission(game.Round{}, json.RawMessage(`{not json}`)); err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestPromptFreestyle_ValidateVote_SelfVoteRejected(t *testing.T) {
	voterID := uuid.New()
	sub := game.Submission{UserID: voterID}
	if err := New().ValidateVote(game.Round{}, sub, voterID, nil); !errors.Is(err, ErrSelfVote) {
		t.Fatalf("want ErrSelfVote, got %v", err)
	}
}

func TestPromptFreestyle_CalculateRoundScores(t *testing.T) {
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

func TestPromptFreestyle_BuildSubmissionsShownPayload_HidesAuthors(t *testing.T) {
	body, _ := json.Marshal(submitPayload{Filler: "ma belle-mère"})
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
	if len(out.Submissions) != 1 {
		t.Fatalf("want 1 submission, got %d", len(out.Submissions))
	}
	if _, leaked := out.Submissions[0]["username"]; leaked {
		t.Fatalf("username leaked in submissions_shown")
	}
	if out.Submissions[0]["filler"] != "ma belle-mère" {
		t.Fatalf("filler missing or wrong: %v", out.Submissions[0])
	}
}

func TestPromptFreestyle_BuildVoteResultsPayload_RevealsAuthors(t *testing.T) {
	authorID := uuid.New()
	body, _ := json.Marshal(submitPayload{Filler: "the filler"})
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
