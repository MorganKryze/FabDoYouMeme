// backend/internal/api/room_replay.go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

type replayRoomHeader struct {
	Code         string          `json:"code"`
	GameTypeSlug string          `json:"game_type_slug"`
	PackName     string          `json:"pack_name"`
	TextPackName string          `json:"text_pack_name,omitempty"`
	StartedAt    time.Time       `json:"started_at"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty"`
	PlayerCount  int64           `json:"player_count"`
	Config       json.RawMessage `json:"config"`
}

type replayAuthor struct {
	DisplayName string `json:"display_name"`
	Kind        string `json:"kind"` // user | guest | deleted
}

type replaySubmission struct {
	ID            string          `json:"id"`
	Author        replayAuthor    `json:"author"`
	Payload       json.RawMessage `json:"payload"`
	VotesReceived int32           `json:"votes_received"`
	PointsAwarded int32           `json:"points_awarded"`
}

type replayRound struct {
	RoundNumber int32              `json:"round_number"`
	Prompt      json.RawMessage    `json:"prompt"`
	Submissions []replaySubmission `json:"submissions"`
}

type replayLeaderboardRow struct {
	Rank        int    `json:"rank"`
	DisplayName string `json:"display_name"`
	Score       int32  `json:"score"`
	Kind        string `json:"kind"`
}

type replayPayload struct {
	Room        replayRoomHeader       `json:"room"`
	Rounds      []replayRound          `json:"rounds"`
	Leaderboard []replayLeaderboardRow `json:"leaderboard"`
}

// GetReplay handles GET /api/rooms/{code}/replay.
func (h *RoomHandler) GetReplay(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	code := chi.URLParam(r, "code")

	room, err := h.db.GetFinishedRoomByCode(r.Context(), code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "not_found", "Room not found or not finished")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load room")
		return
	}

	if u.Role != "admin" {
		callerID, err := uuid.Parse(u.UserID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Invalid session")
			return
		}
		isMember, err := h.db.IsUserRoomMember(r.Context(), db.IsUserRoomMemberParams{
			RoomID: room.ID,
			UserID: pgtype.UUID{Bytes: callerID, Valid: true},
		})
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check membership")
			return
		}
		if !isMember {
			writeError(w, r, http.StatusForbidden, "not_a_player", "You weren't in this room")
			return
		}
	}

	rounds, err := h.db.GetReplayRounds(r.Context(), room.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load rounds")
		return
	}
	subs, err := h.db.GetReplaySubmissions(r.Context(), room.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load submissions")
		return
	}
	if room.GameTypeSlug == "meme-showdown" {
		if err := enrichShowdownSubmissions(r.Context(), h.db, subs); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to resolve showdown cards")
			return
		}
	}
	lbRows, err := h.db.GetReplayLeaderboard(r.Context(), room.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load leaderboard")
		return
	}

	subsByRound := make(map[uuid.UUID][]replaySubmission, len(rounds))
	for _, s := range subs {
		entry := replaySubmission{
			ID:            s.SubmissionID.String(),
			Payload:       s.Payload,
			Author:        replayAuthor{DisplayName: s.AuthorName, Kind: s.AuthorKind},
			VotesReceived: s.VotesReceived,
			PointsAwarded: s.VotesReceived, // 1 point per vote for both shipped game types
		}
		subsByRound[s.RoundID] = append(subsByRound[s.RoundID], entry)
	}

	outRounds := make([]replayRound, 0, len(rounds))
	for _, rnd := range rounds {
		subs := subsByRound[rnd.RoundID]
		if subs == nil {
			// Marshal as [] rather than null so the frontend can always
			// call .map on round.submissions without a guard. Rounds with
			// no submissions are normal (aborted round, all players skipped).
			subs = []replaySubmission{}
		}
		outRounds = append(outRounds, replayRound{
			RoundNumber: rnd.RoundNumber,
			Prompt:      buildPromptPayload(rnd),
			Submissions: subs,
		})
	}

	// Dense rank: equal scores share a rank; the next distinct score jumps
	// by the number of tied entries above it.
	outLB := make([]replayLeaderboardRow, 0, len(lbRows))
	prevScore := int32(-1)
	rank := 0
	for i, row := range lbRows {
		if i == 0 || row.Score != prevScore {
			rank = i + 1
			prevScore = row.Score
		}
		outLB = append(outLB, replayLeaderboardRow{
			Rank:        rank,
			DisplayName: row.DisplayName,
			Score:       row.Score,
			Kind:        row.Kind,
		})
	}

	header := replayRoomHeader{
		Code:         room.Code,
		GameTypeSlug: room.GameTypeSlug,
		PackName:     room.PackName,
		TextPackName: room.TextPackName,
		StartedAt:    room.StartedAt,
		PlayerCount:  room.PlayerCount,
		Config:       room.Config,
	}
	if room.FinishedAt.Valid {
		t := room.FinishedAt.Time
		header.FinishedAt = &t
	}

	writeJSON(w, http.StatusOK, replayPayload{
		Room:        header,
		Rounds:      outRounds,
		Leaderboard: outLB,
	})
}

// enrichShowdownSubmissions splices the resolved card text into each
// submission payload so the frontend can render meme-showdown replays without
// a second round-trip. Submissions store {"card_id":"<uuid>"}; this walks the
// slice, batch-fetches the current version of every distinct item, and adds a
// "text" field alongside the original card_id. Unparseable payloads and
// missing items are skipped silently — a past submission with a corrupted
// payload should still render (as {card_id}), not 500 the whole replay.
func enrichShowdownSubmissions(ctx context.Context, q *db.Queries, subs []db.GetReplaySubmissionsRow) error {
	parsed := make([]map[string]any, len(subs))
	ids := make([]uuid.UUID, 0, len(subs))
	seen := map[uuid.UUID]struct{}{}

	for i, s := range subs {
		var p map[string]any
		if err := json.Unmarshal(s.Payload, &p); err != nil || p == nil {
			continue
		}
		parsed[i] = p
		cardRaw, _ := p["card_id"].(string)
		id, err := uuid.Parse(cardRaw)
		if err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}

	rows, err := q.GetCurrentVersionsForItems(ctx, ids)
	if err != nil {
		return err
	}
	textByItem := make(map[uuid.UUID]string, len(rows))
	for _, row := range rows {
		var p struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(row.Payload, &p)
		textByItem[row.ItemID] = p.Text
	}

	for i := range subs {
		p := parsed[i]
		if p == nil {
			continue
		}
		cardRaw, _ := p["card_id"].(string)
		id, err := uuid.Parse(cardRaw)
		if err != nil {
			continue
		}
		if text, ok := textByItem[id]; ok {
			p["text"] = text
			merged, err := json.Marshal(p)
			if err != nil {
				continue
			}
			subs[i].Payload = merged
		}
	}
	return nil
}

// buildPromptPayload merges the prompt item version's payload with its
// media_key reference. Frontend needs both to render the image card.
func buildPromptPayload(rnd db.GetReplayRoundsRow) json.RawMessage {
	var raw map[string]any
	_ = json.Unmarshal(rnd.PromptPayload, &raw)
	if raw == nil {
		raw = map[string]any{}
	}
	raw["payload_version"] = rnd.PromptPayloadVersion
	if rnd.PromptMediaKey != nil {
		raw["media_key"] = *rnd.PromptMediaKey
	}
	out, _ := json.Marshal(raw)
	return out
}
