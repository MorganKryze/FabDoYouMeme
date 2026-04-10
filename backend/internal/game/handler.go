// backend/internal/game/handler.go
package game

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Round is the data the hub provides to handler methods per round.
type Round struct {
	ID                 uuid.UUID
	RoomID             uuid.UUID
	ItemID             uuid.UUID
	RoundNumber        int
	StartedAt          *time.Time
	EndedAt            *time.Time
	ItemPayload        json.RawMessage
	ItemPayloadVersion int
}

// Submission is a player's answer for a round.
type Submission struct {
	ID      uuid.UUID
	UserID  uuid.UUID
	Payload json.RawMessage
}

// Vote is a player's vote for a submission.
type Vote struct {
	SubmissionID uuid.UUID
	VoterID      uuid.UUID
	Value        json.RawMessage
}

// GameTypeHandler is the interface every game type must implement.
// The hub calls these methods during gameplay; implementations must be safe for
// concurrent calls from the hub goroutine only (no additional locking needed
// since the hub is single-threaded per room).
type GameTypeHandler interface {
	// Slug returns the game type slug matching game_types.slug (e.g. "meme-caption").
	Slug() string

	// SupportedPayloadVersions returns which item payload versions this handler can process.
	SupportedPayloadVersions() []int

	// SupportsSolo returns true if solo mode (single player) is supported.
	SupportsSolo() bool

	// MaxPlayers is the per-room upper bound on simultaneous players. Return
	// 0 for "no explicit cap" (the hub will allow unlimited joins). Implemented
	// per finding 3.D in the 2026-04-10 review — handlers must express their
	// own caps so the hub can reject joins in handleRegister before we touch
	// h.players. For game types where the cap varies per room config, pick the
	// absolute maximum; runtime config should tighten it further if needed.
	MaxPlayers() int

	// ValidateSubmission checks that the submission payload is valid for this game type and round.
	ValidateSubmission(round Round, payload json.RawMessage) error

	// ValidateVote checks that the vote payload is valid.
	// Hub has already verified: (a) voting phase open, (b) no duplicate vote.
	// Handler must additionally verify: voterID != submission.UserID (self-vote).
	ValidateVote(round Round, submission Submission, voterID uuid.UUID, payload json.RawMessage) error

	// CalculateRoundScores aggregates votes into per-user point awards.
	CalculateRoundScores(submissions []Submission, votes []Vote) map[uuid.UUID]int

	// BuildSubmissionsShownPayload returns data for the {slug}:submissions_shown event.
	BuildSubmissionsShownPayload(submissions []Submission) (json.RawMessage, error)

	// BuildVoteResultsPayload returns data for the {slug}:vote_results event.
	BuildVoteResultsPayload(submissions []Submission, votes []Vote, scores map[uuid.UUID]int) (json.RawMessage, error)
}
