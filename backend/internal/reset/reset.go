// backend/internal/reset/reset.go
package reset

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

// WipeGameHistory deletes every vote, submission, round, room_player,
// and room. Preserves packs, items, versions, users, and invites.
//
// Each DELETE is issued explicitly (even though CASCADE would handle
// the children) so the Report surfaces exact per-table counts. The
// order walks the FK graph top-down:
//
//	votes → submissions → rounds → room_players → rooms
func WipeGameHistory(ctx context.Context, tx pgx.Tx) (Report, error) {
	q := db.New(tx)
	var r Report

	voteIDs, err := q.DangerDeleteAllVotes(ctx)
	if err != nil {
		return r, err
	}
	r.VotesDeleted = int64(len(voteIDs))

	subIDs, err := q.DangerDeleteAllSubmissions(ctx)
	if err != nil {
		return r, err
	}
	r.SubmissionsDeleted = int64(len(subIDs))

	roundIDs, err := q.DangerDeleteAllRounds(ctx)
	if err != nil {
		return r, err
	}
	r.RoundsDeleted = int64(len(roundIDs))

	rpIDs, err := q.DangerDeleteAllRoomPlayers(ctx)
	if err != nil {
		return r, err
	}
	r.RoomPlayersDeleted = int64(len(rpIDs))

	roomIDs, err := q.DangerDeleteAllRooms(ctx)
	if err != nil {
		return r, err
	}
	r.RoomsDeleted = int64(len(roomIDs))

	return r, nil
}

// Stubs — implemented in subsequent tasks. Keeping them here so the
// HTTP handler can reference the final signatures right away.

// WipeInvites deletes every row in the invites table. Does NOT touch
// users — existing users stay logged in and keep their account.
func WipeInvites(ctx context.Context, tx pgx.Tx) (Report, error) {
	q := db.New(tx)
	ids, err := q.DangerDeleteAllInvites(ctx)
	if err != nil {
		return Report{}, err
	}
	return Report{InvitesDeleted: int64(len(ids))}, nil
}

// WipeSessions deletes every session and every magic_link_token,
// excluding any session belonging to excludeUserID. Pass uuid.Nil to
// wipe everything (used by tests). HTTP callers always pass the acting
// admin's UUID so they stay logged in.
//
// Magic link tokens are wiped unconditionally — any outstanding
// magic-link email becomes dead, which is the whole point of this
// action ("force logout everyone").
func WipeSessions(ctx context.Context, tx pgx.Tx, excludeUserID uuid.UUID) (Report, error) {
	q := db.New(tx)
	var r Report
	sessionIDs, err := q.DangerDeleteSessionsExcept(ctx, excludeUserID)
	if err != nil {
		return r, err
	}
	r.SessionsDeleted = int64(len(sessionIDs))
	mlIDs, err := q.DangerDeleteAllMagicLinkTokens(ctx)
	if err != nil {
		return r, err
	}
	r.MagicTokensDeleted = int64(len(mlIDs))
	if excludeUserID != uuid.Nil {
		r.ExcludedSelf = true
	}
	return r, nil
}

// WipePacksAndMedia deletes every game_pack (which cascades to items
// and versions), every admin_notification, AND all game history that
// depends on those packs. Finally it empties the entire S3 bucket.
//
// Game history is wiped first because rooms.pack_id references
// game_packs.id with NO ACTION (restrict) — you cannot delete a pack
// referenced by a live room. This composition is a DB constraint, not
// a UX choice.
//
// The S3 purge uses empty prefix "" to sweep every object, including
// orphans from previous failed uploads or interrupted wipes. It runs
// outside the DB transaction (S3 cannot participate in Postgres tx);
// if it fails partway, the DB rows are already gone and the error is
// reported in Report.S3Error so the caller can surface it. A retry
// will mop up any remaining S3 objects.
func WipePacksAndMedia(ctx context.Context, tx pgx.Tx, st storage.Storage) (Report, error) {
	var r Report

	// 1. FK prerequisite — history must go first.
	histReport, err := WipeGameHistory(ctx, tx)
	if err != nil {
		return r, err
	}
	r.merge(histReport)

	q := db.New(tx)

	// 2. Admin notifications (belt-and-braces — cascaded from packs but
	//    explicit guards against a future migration dropping the cascade).
	notifIDs, err := q.DangerDeleteAllAdminNotifications(ctx)
	if err != nil {
		return r, err
	}
	r.NotificationsDeleted = int64(len(notifIDs))

	// 3. Versions → items → packs (each cascades from its parent, but
	//    we delete each explicitly so Report counts are exact).
	verIDs, err := q.DangerDeleteAllGameItemVersions(ctx)
	if err != nil {
		return r, err
	}
	r.VersionsDeleted = int64(len(verIDs))

	itemIDs, err := q.DangerDeleteAllGameItems(ctx)
	if err != nil {
		return r, err
	}
	r.ItemsDeleted = int64(len(itemIDs))

	packIDs, err := q.DangerDeleteAllGamePacks(ctx)
	if err != nil {
		return r, err
	}
	r.PacksDeleted = int64(len(packIDs))

	// 4. S3 purge — outside the tx, best-effort. Run last so a DB
	//    rollback doesn't leave an empty bucket and intact DB rows
	//    pointing at missing blobs.
	count, purgeErr := st.Purge(ctx, "")
	r.S3ObjectsDeleted = count
	if purgeErr != nil {
		r.S3Error = purgeErr.Error()
	}
	return r, nil
}

// FullReset composes the four scoped wipes plus a non-protected-user
// delete in a single caller-owned transaction. Preserves: schema,
// game_types seed, sentinel user, every user with is_protected=true,
// and the acting admin (both their user row and their session).
//
// Order of operations:
//  1. WipePacksAndMedia — internally runs WipeGameHistory + deletes
//     packs/items/versions/notifications + Purges S3. Must run before
//     user deletion because pack.owner_id → users.id is ON DELETE SET
//     NULL: wiping users first would leave orphans.
//  2. WipeInvites
//  3. WipeSessions(excludeUserID=actingAdmin)
//  4. DELETE non-protected, non-sentinel, non-acting users.
//
// The audit log entry is written by the HTTP handler AFTER this
// function returns and the tx is committed — never by the service.
func FullReset(ctx context.Context, tx pgx.Tx, st storage.Storage, excludeUserID uuid.UUID) (Report, error) {
	var r Report

	packsReport, err := WipePacksAndMedia(ctx, tx, st)
	if err != nil {
		return r, err
	}
	r.merge(packsReport)

	invReport, err := WipeInvites(ctx, tx)
	if err != nil {
		return r, err
	}
	r.merge(invReport)

	sessReport, err := WipeSessions(ctx, tx, excludeUserID)
	if err != nil {
		return r, err
	}
	r.merge(sessReport)

	q := db.New(tx)
	userIDs, err := q.DangerDeleteNonProtectedUsersExcept(ctx, excludeUserID)
	if err != nil {
		return r, err
	}
	r.UsersDeleted = int64(len(userIDs))
	r.ExcludedSelf = true

	return r, nil
}
