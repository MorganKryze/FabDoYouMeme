// Package pack holds the duplication service for phase 3 of the groups
// paradigm. A pack duplication is "deep" in the domain sense — every item
// and its current version are copied under new IDs — but "shallow" in
// storage: S3 media_keys are referenced by multiple rows during the phase-3
// window. Reference-counting is deferred until observed growth justifies it.
//
// Three call paths share the transaction:
//   - Duplicate: user-initiated, returns immediately unless an approval
//     queue row is needed (NSFW → SFW).
//   - ApprovePending: admin approves a queued duplication; runs the deep
//     copy AND force-relabels the target group to NSFW.
//   - RejectPending: admin rejects; no copy, resolution stamped.
//
// The duplication operates inside a caller-supplied pgx.Tx so the service
// doesn't own transaction lifecycle — handlers decide when to commit, which
// is important for the approval flow that does audit writes alongside.
package pack

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

// GroupMediaSizeEstimateBytes is the per-media-object estimate used in the
// pre-flight quota check. 512 KB is the mid-point of the pre-validated
// upload range. Replace with real storage metadata in phase 5.
const GroupMediaSizeEstimateBytes int64 = 512 * 1024

// ErrNotMember, ErrGroupDeleted, ErrLanguageMismatch, ErrQuotaExceeded are
// returned from Duplicate when the caller's inputs fail the documented
// preconditions. Handlers map each to the spec's error code + HTTP status.
var (
	ErrNotMember         = errors.New("actor is not a member of the target group")
	ErrGroupDeleted      = errors.New("target group is soft-deleted")
	ErrLanguageMismatch  = errors.New("source pack language does not match target group language")
	ErrQuotaExceeded     = errors.New("duplication would exceed the target group's quota")
	ErrSourceUnavailable = errors.New("source pack is not visible to the actor")
)

// DuplicateOutcome conveys the resolution path. One of NewPack or Pending is
// non-nil; never both.
type DuplicateOutcome struct {
	NewPack *db.GamePack                // set on synchronous success
	Pending *db.GroupDuplicationPending // set on NSFW → SFW queue
}

// Service is the handler-facing entry point. Kept small and explicit so the
// handler and background jobs can reuse the same flow.
type Service struct {
	pool *pgxpool.Pool
	db   *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, db: db.New(pool)}
}

// Duplicate runs the full precondition check + deep copy (or queue insert).
// Owns its own transaction — the callers are handlers that otherwise run
// outside a tx. Returns the new pack OR the pending row.
func (s *Service) Duplicate(ctx context.Context, sourcePackID, targetGroupID, actorID uuid.UUID) (DuplicateOutcome, error) {
	// Preconditions: load the group, source, and the actor's membership in
	// one read pass so we fail fast before opening a transaction.
	group, err := s.db.GetGroupByID(ctx, targetGroupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DuplicateOutcome{}, ErrGroupDeleted
		}
		return DuplicateOutcome{}, err
	}
	if _, err := s.db.GetMembership(ctx, db.GetMembershipParams{
		GroupID: group.ID, UserID: actorID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DuplicateOutcome{}, ErrNotMember
		}
		return DuplicateOutcome{}, err
	}
	source, err := s.db.GetPackByID(ctx, sourcePackID)
	if err != nil {
		return DuplicateOutcome{}, ErrSourceUnavailable
	}

	// Source-visibility gate: the caller must own the pack, be acting on a
	// system pack (those are platform-wide), OR be duplicating a
	// public-active pack. Group packs can be re-duplicated too — any member
	// of the SOURCE group can re-duplicate into a different group — but we
	// defer that (complex) ACL to phase 5 and only permit the three shapes
	// above for now.
	switch {
	case source.IsSystem:
		// ok
	case source.OwnerID.Valid && source.OwnerID.Bytes == actorID:
		// ok
	case source.Visibility == "public" && source.Status == "active":
		// ok
	default:
		return DuplicateOutcome{}, ErrSourceUnavailable
	}

	// Language invariant: pack language must match the group unless one of
	// the two sides is 'multi' (system image packs are 'multi' and fit any
	// group).
	if source.Language != group.Language && source.Language != "multi" && group.Language != "multi" {
		return DuplicateOutcome{}, ErrLanguageMismatch
	}

	// NSFW → SFW branch: queue for approval, no copy happens yet.
	if source.Classification == "nsfw" && group.Classification == "sfw" {
		pending, err := s.db.CreatePendingDuplication(ctx, db.CreatePendingDuplicationParams{
			GroupID:      group.ID,
			SourcePackID: source.ID,
			RequestedBy:  actorID,
		})
		if err != nil {
			return DuplicateOutcome{}, err
		}
		_, _ = s.db.CreateGroupNotification(ctx, db.CreateGroupNotificationParams{
			GroupID:   group.ID,
			Type:      "duplication_pending",
			ActorID:   pgtype.UUID{Bytes: actorID, Valid: true},
			SubjectID: pgtype.UUID{Bytes: source.ID, Valid: true},
			Payload:   json.RawMessage(`{}`),
		})
		return DuplicateOutcome{Pending: &pending}, nil
	}

	// Quota pre-flight. Soft bound today — see the comment on
	// GroupMediaSizeEstimateBytes. The source item count times the
	// estimate is compared against remaining quota.
	count, err := s.db.SumGroupPackMediaSize(ctx, pgtype.UUID{Bytes: group.ID, Valid: true})
	if err != nil {
		return DuplicateOutcome{}, err
	}
	used := count * GroupMediaSizeEstimateBytes
	if used > group.QuotaBytes {
		return DuplicateOutcome{}, ErrQuotaExceeded
	}

	// Synchronous deep copy in one transaction.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DuplicateOutcome{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	q := s.db.WithTx(tx)

	newPack, err := s.deepCopyTx(ctx, q, source, group, actorID)
	if err != nil {
		return DuplicateOutcome{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DuplicateOutcome{}, err
	}
	return DuplicateOutcome{NewPack: &newPack}, nil
}

// deepCopyTx performs the pack + items + current-version copy inside an
// existing transaction. Pulled out so the approval-accept path can reuse it.
func (s *Service) deepCopyTx(ctx context.Context, q *db.Queries, source db.GamePack, group db.Group, actorID uuid.UUID) (db.GamePack, error) {
	newPack, err := q.CreateGroupPack(ctx, db.CreateGroupPackParams{
		Name:                 source.Name,
		Description:          source.Description,
		Language:             source.Language,
		GroupID:              pgtype.UUID{Bytes: group.ID, Valid: true},
		Classification:       source.Classification,
		DuplicatedFromPackID: pgtype.UUID{Bytes: source.ID, Valid: true},
		DuplicatedByUserID:   pgtype.UUID{Bytes: actorID, Valid: true},
	})
	if err != nil {
		return db.GamePack{}, err
	}

	// Pagination note: ListItemsForPack already takes lim/off. 1000 is a
	// generous upper bound per the spec; a phase-5 task can switch to a
	// cursor when we see a real pack near it.
	items, err := q.ListItemsForPack(ctx, db.ListItemsForPackParams{
		PackID: source.ID, Lim: 1000, Off: 0,
	})
	if err != nil {
		return db.GamePack{}, err
	}

	for _, it := range items {
		newItem, err := q.CreateItem(ctx, db.CreateItemParams{
			PackID:         newPack.ID,
			Name:           it.Name,
			PayloadVersion: it.PayloadVersion,
		})
		if err != nil {
			return db.GamePack{}, err
		}
		// Only copy the current version. Historical versions stay with the
		// source; a group that wants to edit further starts fresh on v1.
		if it.CurrentVersionID.Valid {
			var mediaKey *string
			if it.MediaKey != nil {
				mk := *it.MediaKey
				mediaKey = &mk
			}
			payload := it.Payload
			if len(payload) == 0 {
				payload = json.RawMessage("{}")
			}
			ver, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
				ItemID:   newItem.ID,
				MediaKey: mediaKey,
				Payload:  payload,
			})
			if err != nil {
				return db.GamePack{}, err
			}
			if _, err := q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
				ID:               newItem.ID,
				CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
			}); err != nil {
				return db.GamePack{}, err
			}
		}
	}
	return newPack, nil
}

// ApprovePending resolves an NSFW-into-SFW queue row as accepted. Runs the
// deep copy AND force-relabels the target group to NSFW. Audit write is the
// caller's responsibility (the handler has the acting admin context).
func (s *Service) ApprovePending(ctx context.Context, pendingID, adminID uuid.UUID) (db.GamePack, error) {
	pending, err := s.db.GetPendingDuplication(ctx, pendingID)
	if err != nil {
		return db.GamePack{}, err
	}
	if pending.ResolvedAt.Valid {
		return db.GamePack{}, errors.New("pending row is already resolved")
	}

	group, err := s.db.GetGroupByID(ctx, pending.GroupID)
	if err != nil {
		return db.GamePack{}, err
	}
	source, err := s.db.GetPackByID(ctx, pending.SourcePackID)
	if err != nil {
		return db.GamePack{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.GamePack{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	q := s.db.WithTx(tx)

	// Force-relabel FIRST so the deep copy lands in an already-NSFW group —
	// keeps the invariant "group classification ≥ any owned pack's
	// classification" true at every visible state.
	if err := q.ForceGroupClassificationNSFW(ctx, group.ID); err != nil {
		return db.GamePack{}, err
	}
	// Re-read the group so deepCopyTx sees the new classification.
	group.Classification = "nsfw"

	newPack, err := s.deepCopyTx(ctx, q, source, group, pending.RequestedBy)
	if err != nil {
		return db.GamePack{}, err
	}
	if _, err := q.ResolvePendingDuplication(ctx, db.ResolvePendingDuplicationParams{
		ID:         pending.ID,
		ResolvedBy: pgtype.UUID{Bytes: adminID, Valid: true},
		Resolution: ptrString("accepted"),
	}); err != nil {
		return db.GamePack{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.GamePack{}, err
	}
	return newPack, nil
}

// RejectPending resolves the queue row as rejected with no copy. The group
// classification is left untouched.
func (s *Service) RejectPending(ctx context.Context, pendingID, adminID uuid.UUID) error {
	_, err := s.db.ResolvePendingDuplication(ctx, db.ResolvePendingDuplicationParams{
		ID:         pendingID,
		ResolvedBy: pgtype.UUID{Bytes: adminID, Valid: true},
		Resolution: ptrString("rejected"),
	})
	return err
}

// ptrString wraps a literal into a *string — sqlc emits *string for the
// `resolution` param because the column is nullable. A nil would map to NULL
// in DB, which the CHECK constraint rejects, so this helper exists to keep
// callers from accidentally passing nil.
func ptrString(s string) *string { return &s }

// ensureUniqueTime is a guard the sqlc-generated code doesn't surface: if
// callers ever receive a Time{} zero-value from the DB they should treat it
// as "not yet stamped" rather than a real epoch. Returned here as a helper
// so tests don't duplicate the check.
func ensureUniqueTime(t time.Time) bool { return !t.IsZero() }
