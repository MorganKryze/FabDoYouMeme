// backend/internal/auth/register.go
package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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
	Locale         string `json:"locale"`

	// Phase 2 (groups): when set, the registration is being driven by a
	// platform+group invite. The token replaces InviteToken (the platform
	// invite slot was already debited from user_invite_quotas at mint time)
	// and triggers a group_memberships insert in the same transaction.
	// NSFWAgeAffirmation is required when the target group is classified
	// nsfw — captured for the audit log, not persisted on the user row.
	GroupInviteToken   string `json:"group_invite_token,omitempty"`
	NSFWAgeAffirmation bool   `json:"nsfw_age_affirmation,omitempty"`
}

// Register handles POST /api/auth/register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	if !req.Consent {
		writeError(w, r, http.StatusBadRequest, "consent_required", "You must accept the terms to register")
		return
	}
	if !req.AgeAffirmation {
		writeError(w, r, http.StatusBadRequest, "age_affirmation_required", "You must confirm you are at least 16 years old")
		return
	}
	// Shape-validate before touching the DB or consuming the invite. A
	// malformed username/email must not cost an invite slot.
	if err := ValidateUsername(req.Username); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_username", err.Error())
		return
	}
	if err := ValidateEmail(req.Email); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_email", err.Error())
		return
	}

	// Two registration paths: the historical platform-invite path consumes a
	// row from `invites`; the phase-2 platform+group path consumes a
	// platform_plus_group code from `group_invites` AND inserts a group
	// membership in the same transaction. Exactly one of the two tokens
	// must be present — sending both is a client bug, not a fallback.
	usingGroupInvite := strings.TrimSpace(req.GroupInviteToken) != ""
	if usingGroupInvite && req.InviteToken != "" {
		writeError(w, r, http.StatusBadRequest, "bad_request",
			"send either invite_token OR group_invite_token, not both")
		return
	}

	var invite db.Invite
	var groupInvite db.GroupInvite
	var targetGroup db.Group
	if usingGroupInvite {
		gi, gerr := h.db.GetGroupInviteByToken(r.Context(), strings.TrimSpace(req.GroupInviteToken))
		if gerr != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_invite", "Invalid or expired invite token")
			return
		}
		if gi.Kind != "platform_plus_group" {
			// group_join codes are for existing users; redirect them to the
			// in-app /api/groups/invites/redeem endpoint.
			writeError(w, r, http.StatusBadRequest, "wrong_invite_kind",
				"This code is for existing users; sign in and redeem it from the app.")
			return
		}
		if gi.RevokedAt.Valid {
			writeError(w, r, http.StatusGone, "invite_revoked", "Invite has been revoked")
			return
		}
		if gi.ExpiresAt.Valid && gi.ExpiresAt.Time.Before(h.clock.Now()) {
			writeError(w, r, http.StatusGone, "invite_expired", "Invite has expired")
			return
		}
		if gi.UsesCount >= gi.MaxUses {
			writeError(w, r, http.StatusGone, "invite_exhausted", "Invite has no remaining uses")
			return
		}
		// Banned-issuer guard per spec: if the issuer's account is no longer
		// active (platform-banned or hard-deleted), the code is treated as
		// revoked even if revoked_at is still null.
		if gi.CreatedBy.Valid {
			issuer, ierr := h.db.GetUserByID(r.Context(), gi.CreatedBy.Bytes)
			if ierr != nil || !issuer.IsActive {
				writeError(w, r, http.StatusGone, "invite_revoked",
					"Invite issuer is no longer active")
				return
			}
		}
		if gi.RestrictedEmail != nil && *gi.RestrictedEmail != "" &&
			!strings.EqualFold(*gi.RestrictedEmail, req.Email) {
			writeError(w, r, http.StatusBadRequest, "invalid_invite", "Invalid or expired invite token")
			return
		}
		grp, gerr := h.db.GetGroupByID(r.Context(), gi.GroupID)
		if gerr != nil {
			writeError(w, r, http.StatusGone, "group_not_found", "Target group no longer exists")
			return
		}
		if grp.Classification == "nsfw" && !req.NSFWAgeAffirmation {
			writeError(w, r, http.StatusBadRequest, "nsfw_age_affirmation_required",
				"You must confirm you are of legal age for adult content to join this group.")
			return
		}
		groupInvite = gi
		targetGroup = grp
	} else {
		inv, err := h.db.GetInviteByToken(r.Context(), req.InviteToken)
		if err != nil || req.InviteToken == "" {
			writeError(w, r, http.StatusBadRequest, "invalid_invite", "Invalid or expired invite token")
			return
		}
		if inv.RestrictedEmail != nil && *inv.RestrictedEmail != "" &&
			*inv.RestrictedEmail != req.Email {
			writeError(w, r, http.StatusBadRequest, "invalid_invite", "Invalid or expired invite token")
			return
		}
		invite = inv
	}

	if _, err := h.db.GetUserByEmail(r.Context(), req.Email); err == nil {
		// Return 201 with empty user_id — do not leak whether the email is registered
		writeJSON(w, http.StatusCreated, map[string]string{"user_id": ""})
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Registration failed")
		return
	}
	defer tx.Rollback(r.Context())

	q := h.db.WithTx(tx)

	var invitedBy pgtype.UUID
	if usingGroupInvite {
		if _, err := q.ConsumeGroupInvite(r.Context(), groupInvite.ID); err != nil {
			writeError(w, r, http.StatusGone, "invite_exhausted", "Invite is no longer redeemable")
			return
		}
		if groupInvite.CreatedBy.Valid {
			invitedBy = groupInvite.CreatedBy
		}
	} else {
		if _, err := q.ConsumeInvite(r.Context(), invite.ID); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_invite", "Invalid or expired invite token")
			return
		}
		if invite.CreatedBy.Valid {
			invitedBy = invite.CreatedBy
		}
	}
	// Locale comes from the registration form. The frontend posts the active UI
	// locale (resolved from cookie/Accept-Language at form-render time); on a
	// missing or unknown value we fall back to "en" silently rather than
	// rejecting — matches the frontend's own default-locale helper.
	locale := req.Locale
	if locale != "en" && locale != "fr" {
		locale = "en"
	}
	newUser, err := q.CreateUser(r.Context(), db.CreateUserParams{
		Username:  req.Username,
		Email:     req.Email,
		Role:      "player",
		IsActive:  true,
		InvitedBy: invitedBy,
		ConsentAt: h.clock.Now().UTC(),
		Locale:    locale,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "users_username_key" {
				writeError(w, r, http.StatusConflict, "username_taken", "That username is already taken")
				return
			}
			// Email uniqueness race — return 201 silently
			writeJSON(w, http.StatusCreated, map[string]string{"user_id": ""})
			return
		}
		if h.log != nil {
			h.log.Error("register: create user", "error", err)
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Registration failed")
		return
	}

	// Phase 2 (groups): when this is a platform+group registration, the same
	// transaction creates the group_memberships row so we cannot end up with
	// a registered-but-unenrolled user (a state the group admin would have
	// no UI to fix).
	if usingGroupInvite {
		if _, err := q.CreateMembership(r.Context(), db.CreateMembershipParams{
			GroupID: targetGroup.ID,
			UserID:  newUser.ID,
			Role:    "member",
		}); err != nil {
			if h.log != nil {
				h.log.Error("register: create group membership", "error", err)
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Registration failed")
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Registration failed")
		return
	}

	resp := map[string]string{"user_id": newUser.ID.String()}
	// Auto-magic-link send: triggered for restricted-email platform invites
	// (today's behaviour) and for any platform+group code (the operator
	// expects the user to land in the group by email-clicking, not by
	// asking for a magic link as a separate step).
	autoMagicLink := false
	if usingGroupInvite {
		autoMagicLink = true
	} else if invite.RestrictedEmail != nil && *invite.RestrictedEmail != "" {
		autoMagicLink = true
	}
	if autoMagicLink {
		if sendErr := h.sendMagicLinkToUser(r.Context(), newUser, "login"); sendErr != nil {
			if h.log != nil {
				h.log.Error("register: auto magic link", "error", sendErr)
			}
			resp["warning"] = "smtp_failure"
		}
	}

	writeJSON(w, http.StatusCreated, resp)
}
