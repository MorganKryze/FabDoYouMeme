package game_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

// TestHub_KickPlayerRespectsContext is the regression test for finding 4.B.
// Before the fix KickPlayer bare-sent on h.incoming and would block forever
// when the buffer was full — stalling the caller HTTP goroutine. The fix
// threads a context through the send so the caller can bail out.
//
// We exercise the blocked path without spinning up the Run loop: the hub is
// NewHub'd but Run is never called, so nothing drains h.incoming. We fill it
// to capacity (64), then call KickPlayer with a deadline-bounded context and
// assert it returns ctx.Err() inside the deadline window.
func TestHub_KickPlayerRespectsContext(t *testing.T) {
	hub := game.NewHub(game.HubConfig{
		RoomCode:     "KICK",
		RoomID:       uuid.New(),
		GameTypeSlug: "meme-freestyle",
		HostUserID:   uuid.New().String(),
		Registry:     game.NewRegistry(),
		DB:           nil, // not touched — Run is never started
		Cfg:          &config.Config{},
		Log:          slog.Default(),
		Clock:        clock.Real{},
	})

	// Saturate the incoming channel (capacity 64). The first kick fills the
	// last slot; the second kick hits the blocking path.
	fillCtx, cancelFill := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelFill()
	for i := 0; i < 64; i++ {
		if err := hub.KickPlayer(fillCtx, uuid.New().String()); err != nil {
			t.Fatalf("fill kick %d: unexpected error %v", i, err)
		}
	}

	// Now the buffer is full. The blocking send must be cut off by ctx.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := hub.KickPlayer(ctx, uuid.New().String())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("want ctx error, got nil (elapsed=%s)", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("KickPlayer blocked %s — expected ≤200ms bail-out", elapsed)
	}
}

// TestHub_KickPlayerSucceedsWhenDrainable confirms the happy path: when the
// incoming buffer has room, KickPlayer enqueues without touching ctx.
func TestHub_KickPlayerSucceedsWhenDrainable(t *testing.T) {
	hub := game.NewHub(game.HubConfig{
		RoomCode:     "KICK2",
		RoomID:       uuid.New(),
		GameTypeSlug: "meme-freestyle",
		HostUserID:   uuid.New().String(),
		Registry:     game.NewRegistry(),
		Cfg:          &config.Config{},
		Log:          slog.Default(),
		Clock:        clock.Real{},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := hub.KickPlayer(ctx, uuid.New().String()); err != nil {
		t.Fatalf("KickPlayer on empty hub: %v", err)
	}
}
