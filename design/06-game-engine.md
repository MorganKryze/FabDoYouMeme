# 06 — Game Engine & Extensibility

## Overview

Game types are self-contained handler units. Adding a new game type requires:

1. A new migration seeding `game_types` with the slug, name, and config
2. A new backend handler package under `internal/game/types/{slug}/`
3. A new frontend plugin folder under `src/lib/games/{slug}/`
4. New frontend route(s) if the game type needs unique views (otherwise the generic game view handles dispatch)

**No schema or WebSocket protocol changes are needed.** The DB schema and WS message envelope are intentionally game-type-agnostic. Game-specific content lives in JSONB `payload` fields and prefixed WS message types.

---

## Backend: GameTypeHandler Interface

Every game type must implement the following Go interface:

```go
// internal/game/handler.go

type Round struct {
    ID          uuid.UUID
    RoomID      uuid.UUID
    ItemID      uuid.UUID
    RoundNumber int
    StartedAt   *time.Time
    EndedAt     *time.Time
    // Item payload loaded alongside the round
    ItemPayload        json.RawMessage
    ItemPayloadVersion int
}

type Submission struct {
    ID      uuid.UUID
    UserID  uuid.UUID
    Payload json.RawMessage
}

type Vote struct {
    SubmissionID uuid.UUID
    VoterID      uuid.UUID
    Value        json.RawMessage
}

type GameTypeHandler interface {
    // Slug returns the game type slug matching game_types.slug (e.g. "meme-caption").
    Slug() string

    // SupportedPayloadVersions returns which item payload versions this handler can process.
    // If an item's payload_version is not in this list, the round is skipped with an error log.
    SupportedPayloadVersions() []int

    // SupportsSolo returns whether this game type supports solo mode.
    SupportsSolo() bool

    // ValidateSubmission checks that the submission payload is valid for this game type and round.
    // Returns a descriptive error (sent back to the client as {"code":"invalid_submission",...}).
    ValidateSubmission(round Round, payload json.RawMessage) error

    // ValidateVote checks that the vote payload is valid.
    // Must also verify voterID != submission.UserID (self-vote prevention).
    ValidateVote(round Round, submission Submission, voterID uuid.UUID, payload json.RawMessage) error

    // CalculateRoundScores aggregates votes into per-user point awards.
    // Returns a map of userID → points earned in this round.
    // Called after voting closes; results are broadcast in vote_results.
    CalculateRoundScores(submissions []Submission, votes []Vote) map[uuid.UUID]int

    // BuildSubmissionsShownPayload returns the data for the {slug}:submissions_shown event.
    // Implementations control what is revealed during voting (e.g. captions shown, authors hidden).
    BuildSubmissionsShownPayload(submissions []Submission) (json.RawMessage, error)

    // BuildVoteResultsPayload returns the data for the {slug}:vote_results event.
    // Implementations control what is revealed after voting (e.g. authors revealed, scores shown).
    BuildVoteResultsPayload(submissions []Submission, votes []Vote, scores map[uuid.UUID]int) (json.RawMessage, error)
}
```

### Handler Registration

Handlers are registered in `main.go` before the server starts:

```go
// cmd/server/main.go
game.Register(memecaption.New())
// game.Register(trivia.New())
// game.Register(drawing.New())
```

`game.Register` panics on duplicate slugs — caught at startup, never at runtime.

### Dispatch

The WebSocket hub dispatches game-specific messages via the registry:

```go
// internal/game/registry.go
func Dispatch(slug string, msgType string, ...) error {
    h, ok := registry[slug]
    if !ok {
        return errors.New("unknown_game_type")
    }
    // route to handler methods based on msgType suffix
}
```

---

## Payload Version Handling

When loading items for a round:

1. Check `item.payload_version`
2. If not in `handler.SupportedPayloadVersions()`, log an error and skip the item — select the next available item in the pack instead
3. Use version-appropriate parsing logic inside the handler

When a new payload version is released:

- Deploy updated handler declaring `SupportedPayloadVersions: []int{1, 2}` (both old and new)
- New items uploaded by admins use version 2; existing version-1 items continue to work
- No DB migrations needed — old data is immutable

---

## Frontend Plugin Contract

Each game type must provide a plugin folder at `src/lib/games/{slug}/` exporting four Svelte 5 components and a metadata object:

```plain
src/lib/games/
  meme-caption/
    SubmitForm.svelte    ← renders during submission phase
    VoteForm.svelte      ← renders during voting phase
    ResultsView.svelte   ← renders after vote_results event
    GameRules.svelte     ← modal content explaining how to play
    index.ts             ← re-exports all four + metadata
```

```ts
// src/lib/games/meme-caption/index.ts
export { default as SubmitForm } from './SubmitForm.svelte';
export { default as VoteForm } from './VoteForm.svelte';
export { default as ResultsView } from './ResultsView.svelte';
export { default as GameRules } from './GameRules.svelte';

export const meta = {
  slug: 'meme-caption',
  name: 'Meme Caption',
  description: 'Write the funniest caption for an image. Others vote.'
};
```

The generic game view (`src/routes/(app)/rooms/[code]/+page.svelte`) dynamically imports the correct plugin based on `room.game_type.slug`:

```ts
// Dynamic import pattern (SvelteKit)
const plugin = await import(`../../lib/games/${room.game_type.slug}/index.ts`);
```

Each component receives game state via props or Svelte 5 context. The room state store (`src/lib/state/room.svelte.ts`) is the source of truth.

**Required component props**:

| Component     | Key props                             |
| ------------- | ------------------------------------- |
| `SubmitForm`  | `round`, `onSubmit(payload)`          |
| `VoteForm`    | `submissions`, `onVote(submissionId)` |
| `ResultsView` | `results`, `ownUserId`                |
| `GameRules`   | _(no required props)_                 |

---

## Game Type Lifecycle & Versioning

Game types are **immutable once seeded** in production. If mechanics change significantly, seed a new slug (e.g., `meme-caption-v2`) rather than mutating the existing entry. This preserves historical room data references.

The `game_types.version` column is for informational display only and does not affect dispatch logic.

---

## Pack Validation at Room Creation

Before a room is created, the backend verifies the selected pack has enough usable items for the game type and the configured round count. The `GameTypeHandler` interface exposes this check:

```go
// Returns the payload versions this handler considers "usable" for a round.
// Used during room creation to count compatible items in the pack.
SupportedPayloadVersions() []int
```

The room creation handler (`POST /api/rooms`) queries:

```sql
SELECT COUNT(*) FROM game_items
WHERE pack_id = $pack_id
  AND payload_version = ANY($supported_versions::int[])
  AND pack_id IN (SELECT id FROM game_packs WHERE deleted_at IS NULL);
```

If the count is less than `config.round_count`, the request is rejected with `422 pack_insufficient_items`. This prevents creating a room that would silently run out of items mid-game.

---

## In-Memory State and Restart Behaviour

The WebSocket hub holds all active room state (connected players, current round, timer goroutines) **in memory only**. Persistent state (room row, round rows, submissions, votes) is written to PostgreSQL as events occur, but the hub's runtime state is not recoverable from the DB.

**Consequence**: if the backend process restarts (crash or deploy), all active WebSocket connections are dropped and in-progress games are lost. Players reconnecting after a restart will find their room in the last persisted state — typically `playing` with no active hub. The frontend must handle this: if a `reconnect` is rejected because the hub no longer exists for a `playing` room, show:

> "The server restarted and this game could not be recovered. The room has been marked as finished."

The backend moves orphaned `playing` rooms to `finished` on startup (startup migration: `UPDATE rooms SET state = 'finished', finished_at = now() WHERE state = 'playing'`).

This is a deliberate simplicity trade-off. At self-hosted scale with planned maintenance windows, full game-state persistence adds significant complexity for little benefit.

---

## meme-caption Handler (Reference Implementation)

**Scoring model**: one point per vote received. The submission with the most votes wins the round; ties share points equally (both get full points — there is no tie-break).

**Self-vote**: `ValidateVote` returns an error if `voterID == submission.UserID`. Error code: `cannot_vote_for_self`.

**Submission window**: submissions are accepted from `round_started` until `submissions_closed` is broadcast. The hub closes submissions after `round_duration_seconds` or when all players have submitted — whichever comes first.

**Voting window**: voting is open after `submissions_shown` until `voting_duration_seconds` elapse or all players have voted — whichever comes first.

**Author anonymity**: `BuildSubmissionsShownPayload` omits `author_id` and `author_username`. `BuildVoteResultsPayload` includes both.
