// backend/internal/auth/profile.go
package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

type patchMeRequest struct {
	Username *string `json:"username,omitempty"`
	Email    *string `json:"email,omitempty"`
}

// PatchMe handles PATCH /api/users/me.
func (h *Handler) PatchMe(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	userID, err := uuid.Parse(u.UserID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Invalid session")
		return
	}

	var req patchMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	if req.Username != nil {
		if err := ValidateUsername(*req.Username); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_username", err.Error())
			return
		}
		updated, err := h.db.UpdateUserUsername(r.Context(), db.UpdateUserUsernameParams{
			ID:       userID,
			Username: *req.Username,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeError(w, r, http.StatusConflict, "username_taken", "That username is already taken")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Update failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"id":       updated.ID.String(),
			"username": updated.Username,
			"email":    updated.Email,
		})
		return
	}

	if req.Email != nil {
		if err := ValidateEmail(*req.Email); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_email", err.Error())
			return
		}
		updated, err := h.db.SetPendingEmail(r.Context(), db.SetPendingEmailParams{
			ID:           userID,
			PendingEmail: req.Email,
		})
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Update failed")
			return
		}
		if sendErr := h.sendMagicLinkToUser(r.Context(), updated, "email_change"); sendErr != nil {
			if h.log != nil {
				h.log.Error("patch me: email change magic link", "error", sendErr)
			}
			writeError(w, r, http.StatusInternalServerError, "smtp_failure", "Failed to send verification email")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "Verification link sent to new address",
		})
		return
	}

	writeError(w, r, http.StatusBadRequest, "bad_request", "Provide username or email to update")
}

type historyRoom struct {
	Code         string     `json:"code"`
	GameTypeSlug string     `json:"game_type_slug"`
	PackName     string     `json:"pack_name"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	Score        int32      `json:"score"`
	Rank         int64      `json:"rank"`
	PlayerCount  int64      `json:"player_count"`
}

// GetHistory handles GET /api/users/me/history.
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	userID, err := uuid.Parse(u.UserID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Invalid session")
		return
	}

	limit := int32(50)
	offset := int32(0)
	if l := r.URL.Query().Get("limit"); l != "" {
		if lv, err := strconv.ParseInt(l, 10, 32); err == nil && lv > 0 && lv <= 100 {
			limit = int32(lv)
		}
	}
	if cursor := r.URL.Query().Get("after"); cursor != "" {
		if ov, err := strconv.ParseInt(cursor, 10, 32); err == nil && ov > 0 {
			offset = int32(ov)
		}
	}

	rows, err := h.db.GetUserGameHistory(r.Context(), db.GetUserGameHistoryParams{
		UserID: userID,
		Lim:    limit,
		Off:    offset,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load history")
		return
	}

	rooms := make([]historyRoom, 0, len(rows))
	for _, row := range rows {
		var finishedAt *time.Time
		if row.FinishedAt.Valid {
			t := row.FinishedAt.Time
			finishedAt = &t
		}
		rooms = append(rooms, historyRoom{
			Code:         row.Code,
			GameTypeSlug: row.GameTypeSlug,
			PackName:     row.PackName,
			StartedAt:    row.StartedAt,
			FinishedAt:   finishedAt,
			Score:        row.Score,
			Rank:         row.Rank,
			PlayerCount:  row.PlayerCount,
		})
	}

	var nextCursor *string
	if int32(len(rows)) == limit {
		next := strconv.Itoa(int(offset + limit))
		nextCursor = &next
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"rooms":       rooms,
		"next_cursor": nextCursor,
	})
}

type exportUser struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ConsentAt time.Time `json:"consent_at"`
	CreatedAt time.Time `json:"created_at"`
}

type exportSubmission struct {
	RoundID      string          `json:"round_id"`
	RoomCode     string          `json:"room_code"`
	GameTypeSlug string          `json:"game_type_slug"`
	Payload      json.RawMessage `json:"payload"`
	CreatedAt    time.Time       `json:"created_at"`
}

type exportGameHistory struct {
	RoomCode     string     `json:"room_code"`
	GameTypeSlug string     `json:"game_type_slug"`
	PackName     string     `json:"pack_name"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	Score        int32      `json:"score"`
	Rank         int64      `json:"rank"`
	PlayerCount  int64      `json:"player_count"`
}

// GetExport handles GET /api/users/me/export (GDPR Art. 20).
func (h *Handler) GetExport(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	userID, err := uuid.Parse(u.UserID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Invalid session")
		return
	}

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load user")
		return
	}

	historyRows, _ := h.db.GetUserGameHistory(r.Context(), db.GetUserGameHistoryParams{
		UserID: userID,
		Lim:    1000,
		Off:    0,
	})
	gameHistory := make([]exportGameHistory, 0, len(historyRows))
	for _, row := range historyRows {
		var finishedAt *time.Time
		if row.FinishedAt.Valid {
			t := row.FinishedAt.Time
			finishedAt = &t
		}
		gameHistory = append(gameHistory, exportGameHistory{
			RoomCode:     row.Code,
			GameTypeSlug: row.GameTypeSlug,
			PackName:     row.PackName,
			StartedAt:    row.StartedAt,
			FinishedAt:   finishedAt,
			Score:        row.Score,
			Rank:         row.Rank,
			PlayerCount:  row.PlayerCount,
		})
	}

	submissionRows, _ := h.db.GetUserSubmissions(r.Context(), userID)
	submissions := make([]exportSubmission, 0, len(submissionRows))
	for _, s := range submissionRows {
		submissions = append(submissions, exportSubmission{
			RoundID:      s.RoundID.String(),
			RoomCode:     s.RoomCode,
			GameTypeSlug: s.GameTypeSlug,
			Payload:      s.Payload,
			CreatedAt:    s.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"exported_at": h.clock.Now().UTC(),
		"user": exportUser{
			ID:        user.ID.String(),
			Username:  user.Username,
			Email:     user.Email,
			Role:      user.Role,
			ConsentAt: user.ConsentAt,
			CreatedAt: user.CreatedAt,
		},
		"game_history": gameHistory,
		"submissions":  submissions,
	})
}
