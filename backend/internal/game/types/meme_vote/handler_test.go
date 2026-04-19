// backend/internal/game/types/meme_vote/handler_test.go
package memevote

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

func TestMemeVote_Slug(t *testing.T) {
	if New().Slug() != "meme-vote" {
		t.Fatalf("Slug = %q", New().Slug())
	}
}

func TestMemeVote_RequiredPacks_ImageAndText(t *testing.T) {
	reqs := New().RequiredPacks()
	if len(reqs) != 2 {
		t.Fatalf("RequiredPacks len = %d, want 2", len(reqs))
	}
	byRole := map[game.PackRole]game.PackRequirement{}
	for _, pr := range reqs {
		byRole[pr.Role] = pr
	}
	if _, ok := byRole[game.PackRoleImage]; !ok {
		t.Fatalf("missing image role")
	}
	if _, ok := byRole[game.PackRoleText]; !ok {
		t.Fatalf("missing text role")
	}
	// Text requirement scales with round_count, hand_size and max_players.
	textReq := byRole[game.PackRoleText]
	got := textReq.MinItemsFn(game.RoomConfig{RoundCount: 5, HandSize: 5}, 4)
	// initial hand: 5*4 = 20; refills: (5-1)*4 = 16; total 36
	if got != 36 {
		t.Fatalf("text MinItemsFn(round=5, hand=5, players=4) = %d, want 36", got)
	}
	// Single-round game has no refills.
	if got := textReq.MinItemsFn(game.RoomConfig{RoundCount: 1, HandSize: 5}, 4); got != 20 {
		t.Fatalf("text MinItemsFn(round=1) = %d, want 20 (no refills)", got)
	}
	// Image requirement is round_count.
	if got := byRole[game.PackRoleImage].MinItemsFn(game.RoomConfig{RoundCount: 8}, 12); got != 8 {
		t.Fatalf("image MinItemsFn = %d, want 8", got)
	}
}

func TestMemeVote_ValidateSubmission_OK(t *testing.T) {
	raw, _ := json.Marshal(submitPayload{CardID: uuid.NewString()})
	if err := New().ValidateSubmission(game.Round{}, raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemeVote_ValidateSubmission_MissingCardID(t *testing.T) {
	if err := New().ValidateSubmission(game.Round{}, json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestMemeVote_ValidateSubmission_InvalidUUID(t *testing.T) {
	raw := json.RawMessage(`{"card_id": "not-a-uuid"}`)
	if err := New().ValidateSubmission(game.Round{}, raw); err == nil {
		t.Fatal("expected error for non-UUID card_id")
	}
}

func TestMemeVote_ValidateSubmission_Malformed(t *testing.T) {
	if err := New().ValidateSubmission(game.Round{}, json.RawMessage(`{not json}`)); err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestMemeVote_ValidateVote_SelfVoteRejected(t *testing.T) {
	voterID := uuid.New()
	sub := game.Submission{UserID: voterID}
	err := New().ValidateVote(game.Round{}, sub, voterID, nil)
	if !errors.Is(err, ErrSelfVote) {
		t.Fatalf("want ErrSelfVote, got %v", err)
	}
}

func TestMemeVote_CalculateRoundScores_OneVotePerSubmission(t *testing.T) {
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

func TestMemeVote_BuildSubmissionsShownPayload_HidesAuthors(t *testing.T) {
	body, _ := json.Marshal(submitPayload{CardID: uuid.NewString(), Text: "funny caption"})
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
	if out.Submissions[0]["text"] != "funny caption" {
		t.Fatalf("text missing or wrong: %v", out.Submissions[0])
	}
}

func TestMemeVote_BuildVoteResultsPayload_RevealsAuthors(t *testing.T) {
	authorID := uuid.New()
	body, _ := json.Marshal(submitPayload{CardID: uuid.NewString(), Text: "the caption"})
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
