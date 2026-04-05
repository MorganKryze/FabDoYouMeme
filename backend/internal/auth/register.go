// backend/internal/auth/register.go
package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type registerRequest struct {
	InviteToken    string `json:"invite_token"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	Consent        bool   `json:"consent"`
	AgeAffirmation bool   `json:"age_affirmation"`
}

// Register handles POST /api/auth/register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	if !req.Consent {
		writeError(w, http.StatusBadRequest, "consent_required", "You must accept the terms to register")
		return
	}
	if !req.AgeAffirmation {
		writeError(w, http.StatusBadRequest, "age_affirmation_required", "You must confirm you are at least 16 years old")
		return
	}

	invite, err := h.db.GetInviteByToken(r.Context(), req.InviteToken)
	if err != nil || req.InviteToken == "" {
		writeError(w, http.StatusBadRequest, "invalid_invite", "Invalid or expired invite token")
		return
	}

	if invite.RestrictedEmail != nil && *invite.RestrictedEmail != "" &&
		*invite.RestrictedEmail != req.Email {
		writeError(w, http.StatusBadRequest, "invalid_invite", "Invalid or expired invite token")
		return
	}

	if existing, err := h.db.GetUserByEmail(r.Context(), req.Email); err == nil {
		writeJSON(w, http.StatusCreated, map[string]string{"user_id": existing.ID.String()})
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Registration failed")
		return
	}
	defer tx.Rollback(r.Context())

	q := h.db.WithTx(tx)

	if _, err := q.ConsumeInvite(r.Context(), invite.ID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_invite", "Invalid or expired invite token")
		return
	}

	var invitedBy pgtype.UUID
	if invite.CreatedBy.Valid {
		invitedBy = invite.CreatedBy
	}
	newUser, err := q.CreateUser(r.Context(), db.CreateUserParams{
		Username:  req.Username,
		Email:     req.Email,
		Role:      "player",
		IsActive:  true,
		InvitedBy: invitedBy,
		ConsentAt: time.Now().UTC(),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "users_username_key" {
				writeError(w, http.StatusConflict, "username_taken", "That username is already taken")
				return
			}
			// Email uniqueness race — return 201 silently
			writeJSON(w, http.StatusCreated, map[string]string{"user_id": ""})
			return
		}
		if h.log != nil {
			h.log.Error("register: create user", "error", err)
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Registration failed")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Registration failed")
		return
	}

	resp := map[string]string{"user_id": newUser.ID.String()}
	if invite.RestrictedEmail != nil && *invite.RestrictedEmail != "" {
		if sendErr := h.sendMagicLinkToUser(r.Context(), newUser, "login"); sendErr != nil {
			if h.log != nil {
				h.log.Error("register: auto magic link", "error", sendErr)
			}
			resp["warning"] = "smtp_failure"
		}
	}

	writeJSON(w, http.StatusCreated, resp)
}
