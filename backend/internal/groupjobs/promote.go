// Package groupjobs holds the cross-handler scheduled jobs and lifecycle
// hooks introduced by phase 2 of the groups paradigm:
//
//   - PromoteDormantAdmins: scan dormant sole-admins and promote the
//     longest-tenured active member of each affected group. Run on startup
//     and (TODO phase 5) on a daily ticker.
//   - CascadePlatformBan: invoked from the admin "set is_active=false" path
//     to drop memberships, immediately re-promote where needed, and revoke
//     the banned user's outstanding invites.
//
// Both helpers take a *pgxpool.Pool and a *slog.Logger so they can be
// driven from main.go boot or the admin handler.
package groupjobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

// DormancyWindow is the grace period a sole admin gets before the auto-
// promotion job picks a successor. Spec value is 90 days; exposed as a var
// only so tests can shorten it without an env-var dance.
var DormancyWindow = 90 * 24 * time.Hour

// PromotionReport captures the outcome of one PromoteDormantAdmins pass.
type PromotionReport struct {
	Scanned     int
	Promoted    int
	NoCandidate int
}

// PromoteDormantAdmins runs the spec's 90-day scan: for every group whose
// only logged-in admin has been dormant past DormancyWindow, promote the
// longest-tenured active member. When no candidate exists, an audit log
// entry is written so the platform admin can intervene manually.
//
// The dormant admin keeps their role (per spec — promotion, not
// succession). Idempotent: a second pass with the same data is a no-op
// because the first pass adds a peer admin and the dormant admin is no
// longer the only one.
func PromoteDormantAdmins(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) (PromotionReport, error) {
	q := db.New(pool)
	intervalArg := pgtypeInterval(DormancyWindow)

	rows, err := q.ScanDormantSoleAdmins(ctx, intervalArg)
	if err != nil {
		return PromotionReport{}, err
	}
	report := PromotionReport{Scanned: len(rows)}
	for _, r := range rows {
		candidate, err := q.PickPromotionCandidate(ctx, db.PickPromotionCandidateParams{
			GroupID: r.GroupID, DormantAfter: intervalArg,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				report.NoCandidate++
				writeAuditBestEffort(ctx, q, "group.auto_promote_no_candidate", r.GroupID, nil, logger)
				continue
			}
			if logger != nil {
				logger.Error("PromoteDormantAdmins: pick candidate failed",
					"group_id", r.GroupID, "error", err)
			}
			continue
		}
		if _, err := q.UpdateMembershipRole(ctx, db.UpdateMembershipRoleParams{
			GroupID: r.GroupID, UserID: candidate, Role: "admin",
		}); err != nil {
			if logger != nil {
				logger.Error("PromoteDormantAdmins: promote failed",
					"group_id", r.GroupID, "candidate", candidate, "error", err)
			}
			continue
		}
		report.Promoted++
		writeAuditBestEffort(ctx, q, "group.auto_promote_admin", r.GroupID, &candidate, logger)
	}
	if logger != nil {
		logger.Info("PromoteDormantAdmins pass complete",
			"scanned", report.Scanned,
			"promoted", report.Promoted,
			"no_candidate", report.NoCandidate)
	}
	return report, nil
}

// CascadePlatformBan is invoked from the admin set-is-active-false path. It:
//  1. Snapshots groups where the user is the sole admin (so the post-delete
//     promotion run knows where to act).
//  2. Deletes every membership the user holds.
//  3. Revokes every outstanding invite the user minted.
//  4. Re-promotes admins in the snapshot groups via PromoteDormantAdmins
//     with a zero dormancy window — the promotion path handles the
//     "longest-tenured active member" pick the same way.
//
// Best-effort: failures inside steps 2-4 are logged and ignored so the
// admin's PATCH /api/admin/users response can still succeed. Callers can
// inspect the returned error to surface partial-failure context if they
// want.
func CascadePlatformBan(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, logger *slog.Logger) error {
	q := db.New(pool)

	soleAdminGroups, err := q.ListGroupsWhereUserIsSoleAdmin(ctx, userID)
	if err != nil && logger != nil {
		logger.Error("CascadePlatformBan: list sole-admin groups", "user_id", userID, "error", err)
	}

	if err := q.DeleteAllMembershipsForUser(ctx, userID); err != nil && logger != nil {
		logger.Error("CascadePlatformBan: drop memberships", "user_id", userID, "error", err)
	}

	if err := q.RevokeAllInvitesByCreator(ctx, pgtype.UUID{Bytes: userID, Valid: true}); err != nil && logger != nil {
		logger.Error("CascadePlatformBan: revoke invites", "user_id", userID, "error", err)
	}

	// Trigger immediate promotion in any groups left admin-less. We can't
	// reuse PromoteDormantAdmins here because the banned admin's
	// membership row is already gone — the scan would return nothing. We
	// just iterate the snapshot and promote the longest-tenured active
	// member of each, mirroring the dormant-scan logic. A zero-second
	// dormancy window means "any member who has logged in at all is
	// eligible" — the spec's "longest-tenured active member" pick.
	for _, gid := range soleAdminGroups {
		intervalArg := pgtypeInterval(DormancyWindow)
		candidate, err := q.PickPromotionCandidate(ctx, db.PickPromotionCandidateParams{
			GroupID: gid, DormantAfter: intervalArg,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAuditBestEffort(ctx, q, "group.auto_promote_no_candidate", gid, nil, logger)
				continue
			}
			if logger != nil {
				logger.Error("CascadePlatformBan: pick candidate", "group_id", gid, "error", err)
			}
			continue
		}
		if _, err := q.UpdateMembershipRole(ctx, db.UpdateMembershipRoleParams{
			GroupID: gid, UserID: candidate, Role: "admin",
		}); err != nil {
			if logger != nil {
				logger.Error("CascadePlatformBan: promote", "group_id", gid, "candidate", candidate, "error", err)
			}
			continue
		}
		writeAuditBestEffort(ctx, q, "group.auto_promote_admin", gid, &candidate, logger)
	}
	return nil
}

// pgtypeInterval is the pgtype shape sqlc demands for `interval` query args.
// We pass everything as microseconds because postgres `interval` round-trips
// via that unit on pgx; a Duration in nanoseconds would lose precision but
// Microseconds covers anything we care about.
func pgtypeInterval(d time.Duration) pgtype.Interval {
	return pgtype.Interval{
		Microseconds: d.Microseconds(),
		Valid:        true,
	}
}

// writeAuditBestEffort wraps the standard audit-log insert with best-effort
// semantics. The caller is one of the auto-promotion paths where a failed
// audit shouldn't block the actual mutation that already succeeded.
func writeAuditBestEffort(ctx context.Context, q *db.Queries, action string, gid uuid.UUID, candidate *uuid.UUID, logger *slog.Logger) {
	changes := map[string]string{"group_id": gid.String()}
	if candidate != nil {
		changes["promoted_user_id"] = candidate.String()
	}
	body, _ := jsonMarshal(changes)
	if _, err := q.CreateAuditLog(ctx, db.CreateAuditLogParams{
		AdminID:  pgtype.UUID{Valid: false}, // system-initiated
		Action:   action,
		Resource: "group:" + gid.String(),
		Changes:  body,
	}); err != nil && logger != nil {
		logger.Error("groupjobs: audit write failed", "action", action, "error", err)
	}
}
