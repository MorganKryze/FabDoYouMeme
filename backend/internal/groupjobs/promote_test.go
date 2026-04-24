package groupjobs_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/groupjobs"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupSuite(m))
}

// makeUserWithLogin inserts a user and stamps last_login_at to a known time.
// Pass time.Time{} (zero) to leave last_login_at NULL — used for "never
// logged in" scenarios.
func makeUserWithLogin(t *testing.T, q *db.Queries, suffix string, role string, lastLogin time.Time) db.User {
	t.Helper()
	base := testutil.SeedName(t)
	if len(base) > 24 {
		base = base[:24]
	}
	slug := base + "_" + suffix
	u, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username:  slug,
		Email:     slug + "@test.com",
		Role:      role,
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if !lastLogin.IsZero() {
		if _, err := testutil.Pool().Exec(context.Background(),
			"UPDATE users SET last_login_at = $2 WHERE id = $1", u.ID, lastLogin); err != nil {
			t.Fatalf("set last_login_at: %v", err)
		}
	} else {
		if _, err := testutil.Pool().Exec(context.Background(),
			"UPDATE users SET last_login_at = NULL WHERE id = $1", u.ID); err != nil {
			t.Fatalf("null last_login_at: %v", err)
		}
	}
	return u
}

func makeGroup(t *testing.T, q *db.Queries, suffix string, creator uuid.UUID) db.Group {
	t.Helper()
	g, err := q.CreateGroup(context.Background(), db.CreateGroupParams{
		Name:           "GJobs_" + testutil.SeedName(t) + "_" + suffix,
		Description:    "x",
		Language:       "en",
		Classification: "sfw",
		QuotaBytes:     500 * 1024 * 1024,
		MemberCap:      100,
		CreatedBy:      pgtype.UUID{Bytes: creator, Valid: true},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	return g
}

func addMembership(t *testing.T, q *db.Queries, gid, uid uuid.UUID, role string, joinedAt time.Time) {
	t.Helper()
	if _, err := q.CreateMembership(context.Background(), db.CreateMembershipParams{
		GroupID: gid, UserID: uid, Role: role,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}
	if !joinedAt.IsZero() {
		if _, err := testutil.Pool().Exec(context.Background(),
			"UPDATE group_memberships SET joined_at = $3 WHERE group_id = $1 AND user_id = $2",
			gid, uid, joinedAt); err != nil {
			t.Fatalf("set joined_at: %v", err)
		}
	}
}

func TestPromoteDormantAdmins_PromotesLongestTenured(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)

	dormant := makeUserWithLogin(t, q, "da", "player", time.Now().Add(-100*24*time.Hour))
	older := makeUserWithLogin(t, q, "ol", "player", time.Now().Add(-1*24*time.Hour))
	newer := makeUserWithLogin(t, q, "nw", "player", time.Now().Add(-1*24*time.Hour))

	g := makeGroup(t, q, "p", dormant.ID)
	addMembership(t, q, g.ID, dormant.ID, "admin", time.Now().Add(-200*24*time.Hour))
	addMembership(t, q, g.ID, older.ID, "member", time.Now().Add(-150*24*time.Hour))
	addMembership(t, q, g.ID, newer.ID, "member", time.Now().Add(-10*24*time.Hour))

	rep, err := groupjobs.PromoteDormantAdmins(context.Background(), pool, nil)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if rep.Promoted != 1 {
		t.Fatalf("want 1 promotion, got %+v", rep)
	}
	mem, err := q.GetMembership(context.Background(), db.GetMembershipParams{GroupID: g.ID, UserID: older.ID})
	if err != nil || mem.Role != "admin" {
		t.Fatalf("expected longer-tenured 'older' promoted, got role=%q err=%v", mem.Role, err)
	}
	dormantMem, _ := q.GetMembership(context.Background(), db.GetMembershipParams{GroupID: g.ID, UserID: dormant.ID})
	if dormantMem.Role != "admin" {
		t.Fatalf("dormant admin should retain role, got %q", dormantMem.Role)
	}
}

func TestPromoteDormantAdmins_NoCandidate(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)

	dormant := makeUserWithLogin(t, q, "dn", "player", time.Now().Add(-100*24*time.Hour))
	otherDormant := makeUserWithLogin(t, q, "od", "player", time.Now().Add(-100*24*time.Hour))

	g := makeGroup(t, q, "nc", dormant.ID)
	addMembership(t, q, g.ID, dormant.ID, "admin", time.Now().Add(-200*24*time.Hour))
	addMembership(t, q, g.ID, otherDormant.ID, "member", time.Now().Add(-180*24*time.Hour))

	rep, err := groupjobs.PromoteDormantAdmins(context.Background(), pool, nil)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if rep.NoCandidate != 1 {
		t.Fatalf("want 1 no_candidate, got %+v", rep)
	}
}

func TestPromoteDormantAdmins_PeerActiveAdminSkipped(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)

	dormant := makeUserWithLogin(t, q, "dp", "player", time.Now().Add(-100*24*time.Hour))
	activeAdmin := makeUserWithLogin(t, q, "aa", "player", time.Now().Add(-1*24*time.Hour))

	g := makeGroup(t, q, "pa", dormant.ID)
	addMembership(t, q, g.ID, dormant.ID, "admin", time.Time{})
	addMembership(t, q, g.ID, activeAdmin.ID, "admin", time.Time{})

	rep, err := groupjobs.PromoteDormantAdmins(context.Background(), pool, nil)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if rep.Promoted != 0 {
		t.Fatalf("want 0 promotions when peer admin is active, got %+v", rep)
	}
}

func TestCascadePlatformBan_DropsMemberships(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)

	user := makeUserWithLogin(t, q, "cb", "player", time.Now().Add(-1*24*time.Hour))
	other := makeUserWithLogin(t, q, "cb_o", "player", time.Now().Add(-1*24*time.Hour))

	g := makeGroup(t, q, "cb", other.ID)
	addMembership(t, q, g.ID, other.ID, "admin", time.Time{})
	addMembership(t, q, g.ID, user.ID, "member", time.Time{})

	if err := groupjobs.CascadePlatformBan(context.Background(), pool, user.ID, nil); err != nil {
		t.Fatalf("cascade: %v", err)
	}
	if _, err := q.GetMembership(context.Background(), db.GetMembershipParams{GroupID: g.ID, UserID: user.ID}); err == nil {
		t.Fatal("expected membership dropped after cascade")
	}
}

func TestCascadePlatformBan_PromotesWhenSoleAdminBanned(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)

	soleAdmin := makeUserWithLogin(t, q, "sa", "player", time.Now().Add(-1*24*time.Hour))
	member := makeUserWithLogin(t, q, "sa_m", "player", time.Now().Add(-1*24*time.Hour))

	g := makeGroup(t, q, "sa", soleAdmin.ID)
	addMembership(t, q, g.ID, soleAdmin.ID, "admin", time.Time{})
	addMembership(t, q, g.ID, member.ID, "member", time.Now().Add(-30*24*time.Hour))

	if err := groupjobs.CascadePlatformBan(context.Background(), pool, soleAdmin.ID, nil); err != nil {
		t.Fatalf("cascade: %v", err)
	}
	mem, err := q.GetMembership(context.Background(), db.GetMembershipParams{GroupID: g.ID, UserID: member.ID})
	if err != nil {
		t.Fatalf("get promoted: %v", err)
	}
	if mem.Role != "admin" {
		t.Fatalf("want member promoted to admin, got %q", mem.Role)
	}
}
