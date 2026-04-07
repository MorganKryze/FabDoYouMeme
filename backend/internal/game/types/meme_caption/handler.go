// backend/internal/game/types/meme_caption/handler.go
package memecaption

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

// ErrSelfVote is returned when a player tries to vote for their own submission.
var ErrSelfVote = errors.New("cannot_vote_for_self")

// Handler implements game.GameTypeHandler for the meme-caption game type.
type Handler struct{}

func New() *Handler { return &Handler{} }

func (h *Handler) Slug() string                    { return "meme-caption" }
func (h *Handler) SupportedPayloadVersions() []int { return []int{1} }
func (h *Handler) SupportsSolo() bool              { return false }

type submitPayload struct {
	Caption string `json:"caption"`
}

// ValidateSubmission checks caption is non-empty and ≤300 chars.
func (h *Handler) ValidateSubmission(_ game.Round, raw json.RawMessage) error {
	var p submitPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("invalid submission payload: %w", err)
	}
	p.Caption = strings.TrimSpace(p.Caption)
	if p.Caption == "" {
		return fmt.Errorf("caption cannot be empty")
	}
	if len([]rune(p.Caption)) > 300 {
		return fmt.Errorf("caption exceeds 300 characters")
	}
	return nil
}

// ValidateVote prevents self-vote. The vote payload for meme-caption is { submission_id }.
func (h *Handler) ValidateVote(_ game.Round, submission game.Submission, voterID uuid.UUID, _ json.RawMessage) error {
	if submission.UserID == voterID {
		return ErrSelfVote
	}
	return nil
}

// CalculateRoundScores awards one point per vote received.
// Tied submissions each receive full points — no tiebreaker.
func (h *Handler) CalculateRoundScores(submissions []game.Submission, votes []game.Vote) map[uuid.UUID]int {
	// Map submission ID → author user ID
	authorBySubmission := make(map[uuid.UUID]uuid.UUID, len(submissions))
	for _, s := range submissions {
		authorBySubmission[s.ID] = s.UserID
	}

	scores := make(map[uuid.UUID]int)
	for _, v := range votes {
		if authorID, ok := authorBySubmission[v.SubmissionID]; ok {
			scores[authorID]++
		}
	}
	return scores
}

type submissionShown struct {
	ID      string `json:"id"`
	Caption string `json:"caption"`
	// Note: no author fields — authors are hidden during voting phase
}

// BuildSubmissionsShownPayload returns captions without author information.
func (h *Handler) BuildSubmissionsShownPayload(submissions []game.Submission) (json.RawMessage, error) {
	shown := make([]submissionShown, 0, len(submissions))
	for _, s := range submissions {
		var p submitPayload
		if err := json.Unmarshal(s.Payload, &p); err != nil {
			continue
		}
		shown = append(shown, submissionShown{
			ID:      s.ID.String(),
			Caption: strings.TrimSpace(p.Caption),
		})
	}
	payload, err := json.Marshal(map[string]any{"submissions": shown})
	return json.RawMessage(payload), err
}

type submissionResult struct {
	ID             string `json:"id"`
	Caption        string `json:"caption"`
	AuthorUsername string `json:"author_username,omitempty"` // revealed after voting
	VotesReceived  int    `json:"votes_received"`
	PointsAwarded  int    `json:"points_awarded"`
}

// BuildVoteResultsPayload reveals authors and scores after voting closes.
// Note: author username is not available in the Submission struct (only UserID).
// The hub must pass username-enriched submissions; for now we use user IDs.
// Phase 7 hub wiring enriches this with usernames from the DB.
func (h *Handler) BuildVoteResultsPayload(
	submissions []game.Submission,
	votes []game.Vote,
	scores map[uuid.UUID]int,
) (json.RawMessage, error) {
	// Count votes per submission
	votesPerSub := make(map[uuid.UUID]int)
	for _, v := range votes {
		votesPerSub[v.SubmissionID]++
	}

	results := make([]submissionResult, 0, len(submissions))
	for _, s := range submissions {
		var p submitPayload
		json.Unmarshal(s.Payload, &p) //nolint:errcheck
		results = append(results, submissionResult{
			ID:            s.ID.String(),
			Caption:       strings.TrimSpace(p.Caption),
			VotesReceived: votesPerSub[s.ID],
			PointsAwarded: scores[s.UserID],
		})
	}

	// Round scores list
	roundScores := make([]map[string]any, 0, len(scores))
	for userID, pts := range scores {
		roundScores = append(roundScores, map[string]any{
			"user_id": userID.String(),
			"points":  pts,
		})
	}

	payload, err := json.Marshal(map[string]any{
		"submissions":  results,
		"round_scores": roundScores,
	})
	return json.RawMessage(payload), err
}

// Compile-time interface check
var _ game.GameTypeHandler = (*Handler)(nil)
