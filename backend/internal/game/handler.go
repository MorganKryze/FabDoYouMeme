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
// AuthorUsername is populated by the hub before BuildVoteResultsPayload is
// called so the handler can include the display name in the reveal payload
// without a separate lookup. It is not used during submission or vote validation.
type Submission struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Payload        json.RawMessage
	AuthorUsername string
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
	// Slug returns the game type slug matching game_types.slug (e.g. "meme-freestyle").
	Slug() string

	// RequiredPacks describes every pack the game type needs to run a room.
	// The API layer (POST /api/rooms) iterates this list to enforce pack
	// compatibility and per-role minimum item counts at creation time.
	RequiredPacks() []PackRequirement

	// SupportsSolo returns true if solo mode (single player) is supported.
	SupportsSolo() bool

	// MaxPlayers is the per-room upper bound on simultaneous players. Return
	// 0 for "no explicit cap" (the hub will allow unlimited joins). Implemented
	// per finding 3.D in the 2026-04-10 review — handlers must express their
	// own caps so the hub can reject joins in handleRegister before we touch
	// h.players. For game types where the cap varies per room config, pick the
	// absolute maximum; runtime config should tighten it further if needed.
	MaxPlayers() int

	// Manifest returns the handler's parsed manifest.yaml. The API layer
	// uses it to validate room config writes (POST /api/rooms and every
	// PATCH /api/rooms/{code}/config), and the startup sync uses it to
	// upsert the game_types DB row. Handlers must return a non-nil value
	// validated at init time — see game.LoadManifest.
	Manifest() *Manifest

	// ValidateSubmission checks that the submission payload is valid for this game type and round.
	ValidateSubmission(round Round, payload json.RawMessage) error

	// ValidateVote checks that the vote payload is valid.
	// Hub has already verified: (a) voting phase open, (b) no duplicate vote.
	// Handler must additionally verify: voterID != submission.UserID (self-vote).
	ValidateVote(round Round, submission Submission, voterID uuid.UUID, payload json.RawMessage) error

	// CalculateRoundScores aggregates votes into per-user point awards.
	CalculateRoundScores(submissions []Submission, votes []Vote) map[uuid.UUID]int

	// BuildSubmissionsShownPayload returns the anonymous display blob embedded in
	// submissions_closed.data.submissions_shown. Must omit all author identity.
	BuildSubmissionsShownPayload(submissions []Submission) (json.RawMessage, error)

	// BuildVoteResultsPayload returns the author-reveal blob embedded in
	// vote_results.data.results. The hub populates Submission.AuthorUsername
	// before this call so display names are available to the handler.
	BuildVoteResultsPayload(submissions []Submission, votes []Vote, scores map[uuid.UUID]int) (json.RawMessage, error)

	// PersonalisesRoundStart signals whether the hub must emit round_started
	// with per-player data (e.g. to include a hand of cards). Handlers that
	// broadcast a single payload (meme-freestyle) return false and keep the
	// hub on the broadcast fast path. The actual per-player data lives on
	// the hub — see Hub.personalRoundStartData.
	PersonalisesRoundStart() bool
}
