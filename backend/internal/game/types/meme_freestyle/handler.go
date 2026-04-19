// backend/internal/game/types/meme_freestyle/handler.go
package memefreestyle

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

// ErrSelfVote is returned when a player tries to vote for their own submission.
var ErrSelfVote = errors.New("cannot_vote_for_self")

//go:embed manifest.yaml
var manifestYAML []byte

// manifest is loaded once at package init. A malformed manifest fails the
// init — the binary will not start, which is the behavior we want: a
// broken manifest can silently corrupt every room it touches.
var manifest = func() *game.Manifest {
	m, err := game.LoadManifest(manifestYAML)
	if err != nil {
		panic(fmt.Sprintf("meme_freestyle: load manifest: %v", err))
	}
	return m
}()

// Handler implements game.GameTypeHandler for the meme-freestyle game type.
// All identity/config metadata is delegated to the embedded manifest so
// there is a single source of truth for bounds and defaults (see
// manifest.yaml in this package).
type Handler struct{}

// New creates a Handler. Identity and caps are read from the embedded
// manifest; there is no runtime-tunable knob here on purpose — deploying
// a changed bound means rebuilding the binary with the edited manifest,
// which keeps game_types.config and the shipped handler code in lock-step.
func New() *Handler { return &Handler{} }

func (h *Handler) Slug() string             { return manifest.Slug }
func (h *Handler) SupportsSolo() bool       { return manifest.SupportsSolo }
func (h *Handler) MaxPlayers() int          { return manifest.MaxPlayersOrDefault() }
func (h *Handler) Manifest() *game.Manifest { return manifest }

// RequiredPacks declares the one image pack meme-freestyle consumes. The pack
// must contain at least RoundCount items compatible with payload_version 1.
func (h *Handler) RequiredPacks() []game.PackRequirement {
	return []game.PackRequirement{
		{
			Role:            game.PackRoleImage,
			PayloadVersions: []int{1},
			MinItemsFn: func(cfg game.RoomConfig, _ int) int {
				return cfg.RoundCount
			},
		},
	}
}

type submitPayload struct {
	Caption string `json:"caption"`
}

// ValidateSubmission checks caption is non-empty and ≤200 chars.
func (h *Handler) ValidateSubmission(_ game.Round, raw json.RawMessage) error {
	var p submitPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("invalid submission payload: %w", err)
	}
	p.Caption = strings.TrimSpace(p.Caption)
	if p.Caption == "" {
		return fmt.Errorf("caption cannot be empty")
	}
	if len([]rune(p.Caption)) > 200 {
		return fmt.Errorf("caption exceeds 200 characters")
	}
	return nil
}

// ValidateVote prevents self-vote. The vote payload for meme-freestyle is { submission_id }.
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
	ID            string `json:"id"`
	Caption       string `json:"caption"`
	Username      string `json:"username,omitempty"` // revealed after voting
	VotesReceived int    `json:"votes_received"`
	PointsAwarded int    `json:"points_awarded"`
}

// BuildVoteResultsPayload reveals authors and scores after voting closes.
// The hub populates Submission.AuthorUsername before calling this method so
// the reveal payload includes the display name. The result is embedded in
// vote_results.data.results by the hub.
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
			Username:      s.AuthorUsername,
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

// PersonalisesRoundStart returns false: meme-freestyle broadcasts a single
// round_started payload to every player.
func (h *Handler) PersonalisesRoundStart() bool { return false }

// Compile-time interface check
var _ game.GameTypeHandler = (*Handler)(nil)
