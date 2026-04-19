// backend/internal/game/types/meme_showdown/handler.go
package memeshowdown

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

// ErrSelfVote is returned when a player votes for their own played caption.
var ErrSelfVote = errors.New("cannot_vote_for_self")

//go:embed manifest.yaml
var manifestYAML []byte

var manifest = func() *game.Manifest {
	m, err := game.LoadManifest(manifestYAML)
	if err != nil {
		panic(fmt.Sprintf("meme_showdown: load manifest: %v", err))
	}
	return m
}()

// Handler implements game.GameTypeHandler for the meme-showdown game type.
// Each round pairs an image (from the image pack) with a hand of captions
// (from the text pack). Every player plays one card; the anonymous reveal
// is then voted on; one point per vote received.
type Handler struct{}

func New() *Handler { return &Handler{} }

func (h *Handler) Slug() string             { return manifest.Slug }
func (h *Handler) SupportsSolo() bool       { return manifest.SupportsSolo }
func (h *Handler) MaxPlayers() int          { return manifest.MaxPlayersOrDefault() }
func (h *Handler) Manifest() *game.Manifest { return manifest }

// RequiredPacks declares the two packs this game type consumes. MinItemsFn
// is evaluated against the normalised RoomConfig and the handler's max_players
// cap — so the room-creation check is worst-case accurate regardless of the
// actual lobby headcount.
func (h *Handler) RequiredPacks() []game.PackRequirement {
	return []game.PackRequirement{
		{
			Role:            game.PackRoleImage,
			PayloadVersions: []int{1},
			MinItemsFn: func(cfg game.RoomConfig, _ int) int {
				return cfg.RoundCount
			},
		},
		{
			Role:            game.PackRoleText,
			PayloadVersions: []int{2},
			MinItemsFn: func(cfg game.RoomConfig, maxPlayers int) int {
				// Worst-case deck usage: initial hand_size × players, plus
				// (round_count − 1) × players refills (one card played and
				// replaced each subsequent round). round_count == 1 → no
				// refills.
				refills := 0
				if cfg.RoundCount > 1 {
					refills = (cfg.RoundCount - 1) * maxPlayers
				}
				return cfg.HandSize*maxPlayers + refills
			},
		},
	}
}

// submitPayload is the body the client sends in meme-showdown:submit.
// card_id is the game_items.id of the caption the player played. Text is
// snapshotted at play time in the hub before persisting so the round history
// is immune to later pack edits; clients never set it.
type submitPayload struct {
	CardID string `json:"card_id"`
	Text   string `json:"text,omitempty"`
}

// ValidateSubmission checks body shape. The hub enforces hand-membership
// (is card_id in the player's current hand?) before calling this, alongside
// its existing phase + already-submitted checks.
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

// ValidateVote prevents self-vote. Hub has already verified phase + duplicate.
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
	// No author fields — hidden during voting phase.
}

// BuildSubmissionsShownPayload reveals captions without authorship so the
// voting UI stays anonymous. Authors are revealed in BuildVoteResultsPayload.
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

// PersonalisesRoundStart returns true: meme-showdown sends each player their own
// round_started payload so the hand of caption cards stays private.
func (h *Handler) PersonalisesRoundStart() bool { return true }

// Compile-time interface check.
var _ game.GameTypeHandler = (*Handler)(nil)
