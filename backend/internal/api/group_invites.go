// backend/internal/api/group_invites.go
//
// Phase 2 of the groups paradigm — see
// docs/superpowers/specs/2026-04-22-groups-paradigm-design.md.
//
// Two mint paths share most of the validation surface:
//   - group-join codes: invite an existing platform user to join the group
//   - platform+group codes: invite a new user, register them AND enrol them
//     in the group in one redemption transaction (consumed via the existing
//     /api/auth/register handler when it sees a group_invite_token field).
//
// Per-(admin, group) rate limits guard against runaway minting; the
// constants live here per the phase-1 spec ("promoted to env vars if a real
// tuning need appears").
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

const (
	// Per spec: per-(admin, group) caps. Move to env vars when tuning need
	// surfaces; until then a code constant keeps the handler self-contained.
	groupInviteMaxActivePerAdminGroup    = 50
	groupInviteMaxMintsPerHourPerAdminGr = 20

	// TTL bounds. Default mirrors the spec's "7 days default, 30 days max".
	groupInviteDefaultTTL = 7 * 24 * time.Hour
	groupInviteMaxTTL     = 30 * 24 * time.Hour
)

type GroupInviteHandler struct {
	pool *pgxpool.Pool
	db   *db.Queries
	cfg  *config.Config
}

func NewGroupInviteHandler(pool *pgxpool.Pool, cfg *config.Config) *GroupInviteHandler {
	return &GroupInviteHandler{pool: pool, db: db.New(pool), cfg: cfg}
}

func (h *GroupInviteHandler) requireAuthGroupAdmin(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	uid, ok := requireSessionUUID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	gid, ok := parseGroupID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	if _, err := h.db.GetGroupByID(r.Context(), gid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "group_not_found", "Group not found")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load group")
		}
		return uuid.Nil, uuid.Nil, false
	}
	mem, err := h.db.GetMembership(r.Context(), db.GetMembershipParams{GroupID: gid, UserID: uid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusForbidden, "not_group_member", "You are not a member of this group.")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load membership")
		}
		return uuid.Nil, uuid.Nil, false
	}
	if mem.Role != "admin" {
		writeError(w, r, http.StatusForbidden, "not_group_admin", "Group admin role required")
		return uuid.Nil, uuid.Nil, false
	}
	return uid, gid, true
}

// checkRateLimits returns true when the actor is within the active-code and
// per-hour caps. Writes the appropriate 429 and returns false otherwise.
func (h *GroupInviteHandler) checkRateLimits(w http.ResponseWriter, r *http.Request, actorID, gid uuid.UUID) bool {
	creator := pgtype.UUID{Bytes: actorID, Valid: true}
	active, err := h.db.CountActiveGroupInvitesForCreator(r.Context(), db.CountActiveGroupInvitesForCreatorParams{
		CreatedBy: creator, GroupID: gid,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check active code count")
		return false
	}
	if active >= groupInviteMaxActivePerAdminGroup {
		writeError(w, r, http.StatusTooManyRequests, "invite_rate_limit_active_codes",
			"You have too many active invite codes for this group. Revoke or wait for some to expire.")
		return false
	}
	since := time.Now().Add(-time.Hour)
	hourly, err := h.db.CountGroupInvitesMintedSince(r.Context(), db.CountGroupInvitesMintedSinceParams{
		CreatedBy: creator, GroupID: gid, CreatedAt: since,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check mint rate")
		return false
	}
	if hourly >= groupInviteMaxMintsPerHourPerAdminGr {
		writeError(w, r, http.StatusTooManyRequests, "invite_rate_limit_per_hour",
			"You have minted too many invite codes in the last hour for this group. Try again later.")
		return false
	}
	return true
}

// mintRequest is shared between the two mint endpoints.
type mintRequest struct {
	MaxUses         int32   `json:"max_uses,omitempty"`
	TTLSeconds      int64   `json:"ttl_seconds,omitempty"`
	RestrictedEmail *string `json:"restricted_email,omitempty"`
}

// resolveMintParams normalises max_uses + expires_at + restricted_email and
// writes a 400 on out-of-bounds input. Shared between the group_join and
// platform+group mint paths.
func (h *GroupInviteHandler) resolveMintParams(w http.ResponseWriter, r *http.Request, req mintRequest) (int32, pgtype.Timestamptz, *string, bool) {
	maxUses := req.MaxUses
	if maxUses <= 0 {
		maxUses = 1
	}
	if maxUses > 1000 {
		writeError(w, r, http.StatusBadRequest, "bad_request", "max_uses must be 1..1000")
		return 0, pgtype.Timestamptz{}, nil, false
	}

	ttl := groupInviteDefaultTTL
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}
	if ttl > groupInviteMaxTTL {
		writeError(w, r, http.StatusBadRequest, "bad_request", "ttl_seconds exceeds the platform maximum (30 days)")
		return 0, pgtype.Timestamptz{}, nil, false
	}
	expires := pgtype.Timestamptz{Time: time.Now().Add(ttl), Valid: true}

	var restricted *string
	if req.RestrictedEmail != nil {
		trimmed := strings.TrimSpace(*req.RestrictedEmail)
		if trimmed != "" {
			restricted = &trimmed
		}
	}
	return maxUses, expires, restricted, true
}

// mintBaseRow inserts the row and returns it. Pulled out to keep the two
// mint handlers symmetric.
func (h *GroupInviteHandler) mintBaseRow(r *http.Request, kind string, gid, actorID uuid.UUID, maxUses int32, expires pgtype.Timestamptz, restricted *string) (db.GroupInvite, string, error) {
	token, err := auth.GenerateRawToken()
	if err != nil {
		return db.GroupInvite{}, "", err
	}
	row, err := h.db.CreateGroupInvite(r.Context(), db.CreateGroupInviteParams{
		Token:           token,
		GroupID:         gid,
		CreatedBy:       pgtype.UUID{Bytes: actorID, Valid: true},
		Kind:            kind,
		RestrictedEmail: restricted,
		MaxUses:         maxUses,
		ExpiresAt:       expires,
	})
	return row, token, err
}

// MintGroupJoin handles POST /api/groups/{id}/invites — mints a code that
// only enrols an existing platform user into the group.
func (h *GroupInviteHandler) MintGroupJoin(w http.ResponseWriter, r *http.Request) {
	actorID, gid, ok := h.requireAuthGroupAdmin(w, r)
	if !ok {
		return
	}
	if !h.checkRateLimits(w, r, actorID, gid) {
		return
	}
	var req mintRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	maxUses, expires, restricted, ok := h.resolveMintParams(w, r, req)
	if !ok {
		return
	}

	row, _, err := h.mintBaseRow(r, "group_join", gid, actorID, maxUses, expires, restricted)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create invite")
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

// MintPlatformPlus handles POST /api/groups/{id}/invites/platform_plus —
// mints a code that registers a new platform user AND enrols them into the
// group in one /auth/register call. For non-admin makers the actor's
// user_invite_quotas.used is debited atomically by max_uses; insufficient
// quota → 409. Platform admins bypass the quota check (they already have
// unlimited platform-invite minting via /admin/invites).
func (h *GroupInviteHandler) MintPlatformPlus(w http.ResponseWriter, r *http.Request) {
	actorID, gid, ok := h.requireAuthGroupAdmin(w, r)
	if !ok {
		return
	}
	if !h.checkRateLimits(w, r, actorID, gid) {
		return
	}
	var req mintRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	maxUses, expires, restricted, ok := h.resolveMintParams(w, r, req)
	if !ok {
		return
	}
	// Restricted-email codes only ever produce one account — the email is
	// unique on users.email, so subsequent redemptions of the same code by
	// the same address would be silently no-op'd at registration. Reject
	// the combination at mint time so the admin doesn't quietly burn the
	// extra quota slots they thought they were allocating.
	if restricted != nil && maxUses > 1 {
		writeError(w, r, http.StatusBadRequest, "bad_request",
			"restricted_email codes are inherently single-use; omit max_uses or set it to 1")
		return
	}

	// Platform admins bypass the per-user invite quota entirely — they
	// already mint unlimited /admin/invites codes and there is no
	// operational reason to rate-limit them on this slower path. The cap
	// exists to throttle non-admin makers' ability to bring new accounts
	// onto the platform; staff don't have that constraint. Group-admin
	// role on the target group is still required (already checked in
	// requireAuthGroupAdmin above).
	sessionUser, _ := middleware.GetSessionUser(r)
	isPlatformAdmin := sessionUser.Role == "admin"

	// Quota debit + invite insert in one transaction so a failed insert does
	// not consume slots the admin couldn't actually use. Multi-use codes
	// debit max_uses slots upfront — we cannot defer per-redemption because
	// the redemption flow lives in /auth/register and must be cheap on the
	// hot path.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck
	qtx := h.db.WithTx(tx)

	if !isPlatformAdmin {
		// Lazy first-touch: makers who never had a row explicitly set by a
		// platform admin get one created at DEFAULT_PLATFORM_PLUS_QUOTA on
		// their first attempt. ON CONFLICT DO NOTHING preserves any
		// admin-set allocation, so this never overwrites a row the operator
		// has already tuned. Operators who want explicit allocation only
		// can set DEFAULT_PLATFORM_PLUS_QUOTA=0 (the consume below will then
		// reject with 409 unless the admin has UPSERT'd the row).
		if h.cfg.DefaultPlatformPlusQuota > 0 {
			if err := qtx.EnsureUserInviteQuotaRow(r.Context(), db.EnsureUserInviteQuotaRowParams{
				UserID:    actorID,
				Allocated: int32(h.cfg.DefaultPlatformPlusQuota),
			}); err != nil {
				writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to ensure invite quota row")
				return
			}
		}
		if _, err := qtx.ConsumeUserInviteQuotaN(r.Context(), db.ConsumeUserInviteQuotaNParams{
			UserID: actorID,
			Amount: maxUses,
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, r, http.StatusConflict, "platform_plus_quota_exhausted",
					"You do not have enough remaining platform-registration invite slots for this many uses. Ask the platform admin to allocate more.")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to consume invite quota")
			return
		}
	}

	token, err := auth.GenerateRawToken()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate token")
		return
	}
	row, err := qtx.CreateGroupInvite(r.Context(), db.CreateGroupInviteParams{
		Token:           token,
		GroupID:         gid,
		CreatedBy:       pgtype.UUID{Bytes: actorID, Valid: true},
		Kind:            "platform_plus_group",
		RestrictedEmail: restricted,
		MaxUses:         maxUses,
		ExpiresAt:       expires,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create invite")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to commit transaction")
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

// List handles GET /api/groups/{id}/invites — admin-only audit view of every
// invite ever minted for the group, newest first.
func (h *GroupInviteHandler) List(w http.ResponseWriter, r *http.Request) {
	_, gid, ok := h.requireAuthGroupAdmin(w, r)
	if !ok {
		return
	}
	rows, err := h.db.ListGroupInvites(r.Context(), gid)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list invites")
		return
	}
	if rows == nil {
		rows = []db.GroupInvite{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// Revoke handles DELETE /api/groups/{id}/invites/{inviteID} — admin-only,
// idempotent. Returns 404 only if the (gid, inviteID) pair does not exist.
func (h *GroupInviteHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	_, gid, ok := h.requireAuthGroupAdmin(w, r)
	if !ok {
		return
	}
	inviteID, err := uuid.Parse(chi.URLParam(r, "inviteID"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid invite id")
		return
	}
	if _, err := h.db.RevokeGroupInvite(r.Context(), db.RevokeGroupInviteParams{
		ID: inviteID, GroupID: gid,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "not_found", "Invite not found")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to revoke invite")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Redeem (group-join only) ────────────────────────────────────────────────
//
// Platform+group codes redeem via the extended /api/auth/register handler.
// This endpoint is for an existing platform user enrolling themselves into
// a group through a group-join code.

type redeemRequest struct {
	Token string `json:"token"`
	// Required when the target group is classified nsfw. Captured for the
	// audit trail; not persisted on the user row (one-time consent for
	// this join action, mirroring the spec's platform+group flow).
	NSFWAgeAffirmation bool `json:"nsfw_age_affirmation,omitempty"`
}

type redeemResponse struct {
	Group db.Group `json:"group"`
}

// Preview handles GET /api/groups/invites/preview?token=X — returns minimal
// group info (name, description, classification, language) so the redemption
// UI can render the target identity + the NSFW gate when applicable, BEFORE
// the user commits to redemption. Does not validate the token's redemption
// preconditions; those are enforced by Redeem.
type previewResponse struct {
	Group      db.Group `json:"group"`
	InviteKind string   `json:"invite_kind"`
	Revoked    bool     `json:"revoked"`
	Expired    bool     `json:"expired"`
	Exhausted  bool     `json:"exhausted"`
}

func (h *GroupInviteHandler) Preview(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Missing token")
		return
	}
	invite, err := h.db.GetGroupInviteByToken(r.Context(), token)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Invite not found")
		return
	}
	group, err := h.db.GetGroupByID(r.Context(), invite.GroupID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "group_not_found", "Target group no longer exists")
		return
	}
	resp := previewResponse{
		Group:      group,
		InviteKind: invite.Kind,
		Revoked:    invite.RevokedAt.Valid,
		Expired:    invite.ExpiresAt.Valid && invite.ExpiresAt.Time.Before(time.Now()),
		Exhausted:  invite.UsesCount >= invite.MaxUses,
	}
	writeJSON(w, http.StatusOK, resp)
}

// Redeem handles POST /api/groups/invites/redeem.
func (h *GroupInviteHandler) Redeem(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSessionUUID(w, r)
	if !ok {
		return
	}
	u, _ := middleware.GetSessionUser(r)

	var req redeemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Token) == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid token")
		return
	}

	invite, err := h.db.GetGroupInviteByToken(r.Context(), strings.TrimSpace(req.Token))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Invite not found")
		return
	}
	if invite.Kind != "group_join" {
		// Platform+group codes redeem via /auth/register; surfacing this as
		// a distinct error helps the frontend show the right CTA.
		writeError(w, r, http.StatusBadRequest, "wrong_invite_kind",
			"This code creates a new account; redeem it via the registration page.")
		return
	}
	if invite.RevokedAt.Valid {
		writeError(w, r, http.StatusGone, "invite_revoked", "Invite has been revoked")
		return
	}
	if invite.ExpiresAt.Valid && invite.ExpiresAt.Time.Before(time.Now()) {
		writeError(w, r, http.StatusGone, "invite_expired", "Invite has expired")
		return
	}
	if invite.UsesCount >= invite.MaxUses {
		writeError(w, r, http.StatusGone, "invite_exhausted", "Invite has no remaining uses")
		return
	}

	group, err := h.db.GetGroupByID(r.Context(), invite.GroupID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "group_not_found", "Target group no longer exists")
		return
	}

	if invite.RestrictedEmail != nil && *invite.RestrictedEmail != "" &&
		!strings.EqualFold(*invite.RestrictedEmail, u.Email) {
		writeError(w, r, http.StatusForbidden, "email_mismatch", "Invite is restricted to a different email")
		return
	}

	banned, err := h.db.IsUserBannedFromGroup(r.Context(), db.IsUserBannedFromGroupParams{
		GroupID: group.ID, UserID: uid,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check ban status")
		return
	}
	if banned {
		writeError(w, r, http.StatusForbidden, "user_banned_from_group", "You are banned from this group")
		return
	}

	// Already a member? Surface as 200 with the group so the UI can route
	// to it. Crucially this short-circuits BEFORE the NSFW age gate so a
	// returning member is not asked to re-affirm.
	if existing, err := h.db.GetMembership(r.Context(), db.GetMembershipParams{
		GroupID: group.ID, UserID: uid,
	}); err == nil {
		_ = existing
		writeJSON(w, http.StatusOK, redeemResponse{Group: group})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check membership")
		return
	}

	// NSFW age affirmation per spec — applies to every NSFW group join,
	// not just the platform+group registration path. Placed after the
	// already-member check so re-fetches by existing members don't fail.
	if group.Classification == "nsfw" && !req.NSFWAgeAffirmation {
		writeError(w, r, http.StatusBadRequest, "nsfw_age_affirmation_required",
			"You must confirm you are of legal age for adult content to join this group.")
		return
	}

	count, err := h.db.CountGroupMembershipsForUser(r.Context(), uid)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check membership count")
		return
	}
	if int(count) >= h.cfg.MaxGroupMembershipsPerUser {
		writeError(w, r, http.StatusConflict, "membership_cap_reached",
			"You have reached the maximum number of group memberships.")
		return
	}

	memberCount, err := h.db.CountGroupMembers(r.Context(), group.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check group capacity")
		return
	}
	if int(memberCount) >= int(group.MemberCap) {
		writeError(w, r, http.StatusConflict, "member_cap_reached", "Group has reached its member cap.")
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck
	qtx := h.db.WithTx(tx)

	if _, err := qtx.ConsumeGroupInvite(r.Context(), invite.ID); err != nil {
		// Atomic check failed — most likely a race where another redemption
		// just exhausted or expired the code. Re-surface as exhausted; the
		// concrete error code is not important enough to branch on.
		writeError(w, r, http.StatusGone, "invite_exhausted", "Invite is no longer redeemable")
		return
	}
	if _, err := qtx.CreateMembership(r.Context(), db.CreateMembershipParams{
		GroupID: group.ID, UserID: uid, Role: "member",
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create membership")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to commit transaction")
		return
	}
	writeJSON(w, http.StatusOK, redeemResponse{Group: group})
}
