// backend/internal/game/types/prompt_showdown/handler.go
package promptshowdown

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

// ErrSelfVote is returned when a player votes for their own played filler.
var ErrSelfVote = errors.New("cannot_vote_for_self")

//go:embed manifest.yaml
var manifestYAML []byte

var manifest = func() *game.Manifest {
	m, err := game.LoadManifest(manifestYAML)
	if err != nil {
		panic(fmt.Sprintf("prompt_showdown: load manifest: %v", err))
	}
	return m
}()

// Handler implements game.GameTypeHandler for prompt-showdown. Each round
// pairs a sentence-with-blank (from the prompt pack) with a hand of fillers
// (from the filler pack). Every player plays one card; the anonymous reveal
// is voted on; one point per vote received.
type Handler struct{}

func New() *Handler { return &Handler{} }

func (h *Handler) Slug() string             { return manifest.Slug }
func (h *Handler) SupportsSolo() bool       { return manifest.SupportsSolo }
func (h *Handler) MaxPlayers() int          { return manifest.MaxPlayersOrDefault() }
func (h *Handler) Manifest() *game.Manifest { return manifest }

// RequiredPacks declares the two packs prompt-showdown consumes. Filler
// MinItemsFn mirrors meme-showdown's worst-case sizing: initial hand_size ×
// players, plus (round_count − 1) × players refills.
func (h *Handler) RequiredPacks() []game.PackRequirement {
	return []game.PackRequirement{
		{
			Role:            game.PackRolePrompt,
			PayloadVersions: []int{4},
			MinItemsFn: func(cfg game.RoomConfig, _ int) int {
				return cfg.RoundCount
			},
		},
		{
			Role:            game.PackRoleFiller,
			PayloadVersions: []int{3},
			MinItemsFn: func(cfg game.RoomConfig, maxPlayers int) int {
				refills := 0
				if cfg.RoundCount > 1 {
					refills = (cfg.RoundCount - 1) * maxPlayers
				}
				return cfg.HandSize*maxPlayers + refills
			},
		},
	}
}

// submitPayload — body of prompt-showdown:submit. Text is snapshotted by
// the hub at play time before persisting; clients never set it.
type submitPayload struct {
	CardID string `json:"card_id"`
	Text   string `json:"text,omitempty"`
}

// ValidateSubmission checks body shape. The hub enforces hand-membership
// (is card_id in the player's current hand?) before calling this.
func (h *Handler) ValidateSubmission(_ game.Round, raw json.RawMessage) error {
	var p submitPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("invalid submission payload: %w", err)
	}
	if strings.TrimSpace(p.CardID) == "" {
		return fmt.Errorf("card_id is required")
	}
	if _, err := uuid.Parse(p.CardID); err != nil {
		return fmt.Errorf("card_id must be a UUID: %w", err)
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
	ID   string `json:"id"`
	Text string `json:"text"`
}

// BuildSubmissionsShownPayload reveals played fillers without authorship.
func (h *Handler) BuildSubmissionsShownPayload(submissions []game.Submission) (json.RawMessage, error) {
	shown := make([]submissionShown, 0, len(submissions))
	for _, s := range submissions {
		var p submitPayload
		if err := json.Unmarshal(s.Payload, &p); err != nil {
			continue
		}
		shown = append(shown, submissionShown{
			ID:   s.ID.String(),
			Text: strings.TrimSpace(p.Text),
		})
	}
	payload, err := json.Marshal(map[string]any{"submissions": shown})
	return json.RawMessage(payload), err
}

type submissionResult struct {
	ID            string `json:"id"`
	Text          string `json:"text"`
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
			Text:          strings.TrimSpace(p.Text),
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

// PersonalisesRoundStart returns true: prompt-showdown sends each player
// their own hand of filler cards in round_started.
func (h *Handler) PersonalisesRoundStart() bool { return true }

var _ game.GameTypeHandler = (*Handler)(nil)
