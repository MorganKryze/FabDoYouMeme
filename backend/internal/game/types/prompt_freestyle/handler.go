// backend/internal/game/types/prompt_freestyle/handler.go
package promptfreestyle

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

var manifest = func() *game.Manifest {
	m, err := game.LoadManifest(manifestYAML)
	if err != nil {
		panic(fmt.Sprintf("prompt_freestyle: load manifest: %v", err))
	}
	return m
}()

// Handler implements game.GameTypeHandler for prompt-freestyle. Players see
// a sentence with a blank and type their own filler. Scoring mirrors
// meme-freestyle: one point per vote received, ties both score.
type Handler struct{}

func New() *Handler { return &Handler{} }

func (h *Handler) Slug() string             { return manifest.Slug }
func (h *Handler) SupportsSolo() bool       { return manifest.SupportsSolo }
func (h *Handler) MaxPlayers() int          { return manifest.MaxPlayersOrDefault() }
func (h *Handler) Manifest() *game.Manifest { return manifest }

// RequiredPacks declares the single prompt pack the game type consumes. The
// pack must contain at least RoundCount items compatible with payload_version
// 4 (the prompt-sentence shape: { prefix, suffix }).
func (h *Handler) RequiredPacks() []game.PackRequirement {
	return []game.PackRequirement{
		{
			Role:            game.PackRolePrompt,
			PayloadVersions: []int{4},
			MinItemsFn: func(cfg game.RoomConfig, _ int) int {
				return cfg.RoundCount
			},
		},
	}
}

type submitPayload struct {
	Filler string `json:"filler"`
}

// ValidateSubmission checks the filler is non-empty and ≤200 characters
// (mirrors the meme-freestyle caption cap).
func (h *Handler) ValidateSubmission(_ game.Round, raw json.RawMessage) error {
	var p submitPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("invalid submission payload: %w", err)
	}
	p.Filler = strings.TrimSpace(p.Filler)
	if p.Filler == "" {
		return fmt.Errorf("filler cannot be empty")
	}
	if len([]rune(p.Filler)) > 200 {
		return fmt.Errorf("filler exceeds 200 characters")
	}
	return nil
}

func (h *Handler) ValidateVote(_ game.Round, submission game.Submission, voterID uuid.UUID, _ json.RawMessage) error {
	if submission.UserID == voterID {
		return ErrSelfVote
	}
	return nil
}

// CalculateRoundScores awards one point per vote received. Ties both score.
func (h *Handler) CalculateRoundScores(submissions []game.Submission, votes []game.Vote) map[uuid.UUID]int {
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
	ID     string `json:"id"`
	Filler string `json:"filler"`
}

// BuildSubmissionsShownPayload returns fillers without author information.
// The frontend splices each filler into the round's prompt sentence at render
// time — the prompt itself is already in round_started.
func (h *Handler) BuildSubmissionsShownPayload(submissions []game.Submission) (json.RawMessage, error) {
	shown := make([]submissionShown, 0, len(submissions))
	for _, s := range submissions {
		var p submitPayload
		if err := json.Unmarshal(s.Payload, &p); err != nil {
			continue
		}
		shown = append(shown, submissionShown{
			ID:     s.ID.String(),
			Filler: strings.TrimSpace(p.Filler),
		})
	}
	payload, err := json.Marshal(map[string]any{"submissions": shown})
	return json.RawMessage(payload), err
}

type submissionResult struct {
	ID            string `json:"id"`
	Filler        string `json:"filler"`
	Username      string `json:"username,omitempty"`
	VotesReceived int    `json:"votes_received"`
	PointsAwarded int    `json:"points_awarded"`
}

func (h *Handler) BuildVoteResultsPayload(
	submissions []game.Submission,
	votes []game.Vote,
	scores map[uuid.UUID]int,
) (json.RawMessage, error) {
	votesPerSub := make(map[uuid.UUID]int)
	for _, v := range votes {
		votesPerSub[v.SubmissionID]++
	}
	results := make([]submissionResult, 0, len(submissions))
	for _, s := range submissions {
		var p submitPayload
		_ = json.Unmarshal(s.Payload, &p)
		results = append(results, submissionResult{
			ID:            s.ID.String(),
			Filler:        strings.TrimSpace(p.Filler),
			Username:      s.AuthorUsername,
			VotesReceived: votesPerSub[s.ID],
			PointsAwarded: scores[s.UserID],
		})
	}
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

// PersonalisesRoundStart returns false: prompt-freestyle broadcasts a single
// round_started payload to every player.
func (h *Handler) PersonalisesRoundStart() bool { return false }

var _ game.GameTypeHandler = (*Handler)(nil)
