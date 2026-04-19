# Game Engine

## How a game session works

A game session flows through a fixed set of states. Understanding these states explains almost everything about how the backend, WebSocket hub, and frontend interact.

```plain
Room states:   lobby  →  playing  →  finished

Round phases (repeating N times):
  round_started → [submissions] → submissions_closed
                → [voting]      → vote_results
  (repeat for each round)
  → game_ended
```

---

## Rooms

A room is created by an authenticated user who becomes the host. At creation, the host chooses:

- **Game type** (e.g. `meme-freestyle`)
- **Pack** of content items to play with
- **Mode** — multiplayer or solo (only if the game type supports it)
- **Config** — round count, round duration, voting duration (constrained by the game type's min/max)

A room has a short code (4 uppercase letters) used by players to join. Codes are generated with `crypto/rand` and retried on collision.

### Lobby phase

Players join the lobby via WebSocket. The host can configure the room, kick players, or leave. If the host leaves during the lobby, the room closes. Once the host sends `start`, the room transitions to `playing` and no new players can join.

Lobby kick (`POST /api/rooms/{code}/kick`) removes the player and writes a `room_bans` row. The WS handshake gates against this table on every connect, so the ban survives reconnects and server restarts. Bans are room-scoped and die with the room (`ON DELETE CASCADE`).

### Playing phase

The backend hub drives the round lifecycle autonomously once `start` is received:

1. Selects the next item from the pack (items are shuffled at round start, not reused)
2. Broadcasts `round_started` with the item, timer info (`duration_seconds` + `ends_at`), and round number
3. Accepts submissions until the timer expires or all players have submitted
4. If at least one submission exists: broadcasts `submissions_closed` and starts the voting timer. If zero submissions: skips voting entirely.
5. Accepts votes until the timer expires or all players have voted (skipped when step 4 had zero submissions)
6. Calculates scores, broadcasts `vote_results` (submissions and scores are empty when voting was skipped)
7. **Server-paced (default, `host_paced: false`)**: waits 10 seconds then automatically advances. The deadline is echoed to clients in `vote_results.data.next_round_at` (RFC 3339) and `results_duration_seconds` so the results view can render a countdown. **Host-paced (`host_paced: true`)**: waits for the host to send `next_round` — no deadline is emitted; the client shows "Waiting for host…". An absent host is recovered by a 5-minute safety ceiling.
8. Repeats until all rounds are complete

### Skip-turn joker and skip-vote abstention

Both features are **platform-level** — they live in the hub, not inside any game handler, so they work for every game type without code duplication.

- **Skip-turn joker.** Each player gets `config.joker_count` jokers for the game (default: `ceil(round_count / 5)`). A player spends one by sending the platform message `skip_submit` during the submit phase. The hub counts the skip toward "all players finished" and broadcasts `player_skipped_submit { user_id, player_id, jokers_remaining }`. Once exhausted, further `skip_submit` messages return `{ code: "jokers_exhausted" }`. Setting `joker_count = 0` disables the feature for the room.
- **Skip-vote abstention.** During the voting phase any player may send `skip_vote` to abstain. Abstention is unlimited and counts toward early-close. The hub broadcasts `player_skipped_vote { user_id, player_id }`. If every player abstains, `vote_results` still fires with empty scores so the round ends cleanly. Setting `allow_skip_vote = false` disables the feature.

Both messages are rejected when sent outside their valid phase (`submission_closed` / `voting_closed`) or after the same player already acted (`already_submitted` / `already_voted`). The `room_state` snapshot surfaces `my_jokers_remaining`, `skipped_submit`, and `skipped_vote` so a reconnecting player's UI lands in the correct state without waiting for the next broadcast.

### Finished phase

The room moves to `finished` when:

- All rounds complete (`reason: "game_complete"`)
- The host disconnects and the grace window expires (`reason: "host_disconnected"`)
- All players disconnect simultaneously and none return within the grace ceiling (`reason: "all_players_disconnected"`)
- The pack runs out of unused items mid-game (`reason: "pack_exhausted"`)

Finished rooms are permanent — they cannot be resumed. The leaderboard is accessible via `GET /api/rooms/:code/leaderboard` after the game ends.

### Out-of-band termination

`Hub.EndRoom(ctx, reason)` is the out-of-band termination path, invoked by the `POST /api/rooms/:code/end` handler. It broadcasts `room_closed` to every player, closes every send channel (so `writePump` drains remaining messages, writes a close frame, and exits), and leaves no rematch opportunity. Reasons are `ended_by_host` (host clicked End Room) or `ended_by_admin` (a platform admin ended a room they do not host).

The handler's DB action is state-dependent:

- **lobby** — the row (and all FK-cascaded data: room_players, guest_players, rounds, submissions, votes) is hard-deleted so the room disappears from history as if it was never created.
- **playing** — the row is marked `state='finished'` so gameplay data (rounds, submissions, votes) is preserved for post-game history and leaderboard queries. The hub is still notified first so clients receive `room_closed` before their sockets drop.
- **finished** — rejected with 409; the room is already done.

---

## WebSocket hub

Each room has a dedicated goroutine — the hub — that owns all mutable room state. No shared state is touched outside the hub goroutine, which eliminates race conditions. The hub communicates with `runRounds` via a channel (`roundCtrl`), keeping the round state machine decoupled from player management.

### Reconnection & grace window

When a player disconnects mid-game, they are not immediately removed. Instead:

- The hub broadcasts `reconnecting` to other players (they see a "reconnecting…" indicator)
- A `RECONNECT_GRACE_WINDOW` timer starts (default 30 seconds)
- If the player reconnects within the window, the hub sends a full `room_state` snapshot and the player continues
- If the timer expires, `player_left` is broadcast and their pending turn is skipped

If all players are simultaneously in the grace window, the round timer pauses and `round_paused` is broadcast. When at least one reconnects, the timer resumes from the remaining duration and `round_resumed { ends_at }` is broadcast with the adjusted deadline. If no player reconnects before the grace ceiling (`RECONNECT_GRACE_WINDOW`) elapses, the room ends with `reason: "all_players_disconnected"`.

### Heartbeat

The client sends a `ping` every `WS_PING_INTERVAL` (default 25 seconds). The server replies with `pong` and resets its read deadline (`WS_READ_DEADLINE`, default 60 seconds). A connection that does not pong within the deadline is treated as dead and cleaned up.

---

## Game type system

Game types are registered handler units. The entire backend game logic is contained behind a single Go interface: `game.GameTypeHandler` (defined in `backend/internal/game/handler.go`). Each game type implements this interface and is added to the process-wide registry at startup.

### Per-handler manifest — single source of truth

Every game type ships a `manifest.yaml` embedded next to its Go handler (`backend/internal/game/types/{slug}/manifest.yaml`). The manifest is the **single source of truth** for the game type's identity, capabilities, and tunable bounds:

```yaml
# backend/internal/game/types/meme_freestyle/manifest.yaml
slug: meme-freestyle
name: Meme Freestyle
description: Write the funniest caption for an image. Others vote for their favourite.
version: "1.0.0"
supports_solo: false
payload_versions: [1]

config:
  min_round_duration_seconds: 15
  max_round_duration_seconds: 300
  default_round_duration_seconds: 60
  min_voting_duration_seconds: 10
  max_voting_duration_seconds: 120
  default_voting_duration_seconds: 30
  min_round_count: 1
  max_round_count: 50
  default_round_count: 10
  min_players: 2
  max_players: 12        # null = no cap
```

The file is embedded into the Go binary at build time via `//go:embed manifest.yaml` in the handler package. Parsing and validation happen at package init; a malformed manifest (missing slug, default outside min/max, min > max, etc.) **panics at startup** so a broken handler cannot silently corrupt running rooms.

```go
// backend/internal/game/types/meme_freestyle/handler.go
//go:embed manifest.yaml
var manifestYAML []byte

var manifest = func() *game.Manifest {
    m, err := game.LoadManifest(manifestYAML)
    if err != nil { panic(fmt.Sprintf("meme_freestyle: load manifest: %v", err)) }
    return m
}()

type Handler struct{}
func New() *Handler                          { return &Handler{} }
func (h *Handler) Slug() string              { return manifest.Slug }
func (h *Handler) SupportsSolo() bool        { return manifest.SupportsSolo }
func (h *Handler) MaxPlayers() int           { return manifest.MaxPlayersOrDefault() }
func (h *Handler) Manifest() *game.Manifest  { return manifest }
// ... game-specific methods below ...
```

### Registration and DB sync at startup

`main.go` builds the registry and calls `game.SyncGameTypes(ctx, queries, registry, logger)`. That helper iterates every registered handler, reads its manifest, and upserts the `game_types` row with `slug` as the natural key (the row's UUID is preserved across upserts so `rooms.game_type_id` FKs stay valid across restarts).

```go
// backend/cmd/server/main.go
registry := game.NewRegistry()
registry.Register(memefreestyle.New())
// registry.Register(trivia.New())   // future game types added here

if err := game.SyncGameTypes(ctx, queries, registry, logger); err != nil {
    logger.Error("game type sync failed", "error", err)
    os.Exit(1)
}
```

Effect: editing `manifest.yaml` and restarting the backend is all it takes to lower `max_round_duration_seconds` from 300 to 180 or bump `max_players` from 12 to 16. No migration, no manual DB edit, no frontend change — the frontend reads the same bounds via `GET /api/game-types`.

Migration `002_seed_game_types.up.sql` still seeds the initial `meme-freestyle` row on a fresh DB so the UUID is stable from first boot. Every subsequent field (name, description, version, `supports_solo`, `config`) is reconciled from the in-binary manifest on every startup — the migration is a first-boot seed, not an ongoing source of truth.

### Authoritative bounds enforcement

`Bounds.ValidateAndFill(raw json.RawMessage)` (in `backend/internal/game/manifest.go`) is the single server-side choke point for room config:

- Missing fields are populated from the manifest's `default_*` values
- Out-of-range fields return a typed `*game.ValidationError{Field, Reason}` so the API layer can emit a stable `invalid_config` error code per field
- Output is always a fully-populated canonical `RoomConfig` JSON, safe to store verbatim in `rooms.config`

Both write paths funnel through it:

- `POST /api/rooms` (create) — validates the initial config
- `PATCH /api/rooms/{code}/config` (edit in lobby) — accepts a **partial patch**, calls `game.MergeJSON(room.Config, req.Config)`, then `ValidateAndFill` on the merged result. Clients only send the field they changed; the server refuses to overwrite the others with zeros.

The frontend never enforces bounds as a contract — it reads them from `gameType.config` and reflects them in `min`/`max` attributes for UX only. The server is always the source of truth.

`RoomConfig` (the canonical runtime shape stored in `rooms.config`) is:

| Field                    | Type | Default                     | Notes                                                                  |
| ------------------------ | ---- | --------------------------- | ---------------------------------------------------------------------- |
| `round_duration_seconds` | int  | manifest `default_*`        | Manifest min/max per game type                                         |
| `voting_duration_seconds`| int  | manifest `default_*`        | Manifest min/max per game type                                         |
| `round_count`            | int  | manifest `default_*`        | Manifest min/max per game type                                         |
| `host_paced`             | bool | `false`                     | When true, host advances rounds manually                               |
| `joker_count`            | int  | `ceil(round_count/5)`       | Platform-wide. `0` disables the skip-turn joker for this room          |
| `allow_skip_vote`        | bool | `true`                      | Platform-wide. `false` disables the "none of these" vote for this room |

`joker_count` has a cross-field rule: `0 ≤ joker_count ≤ round_count`. Violations on either `POST /api/rooms` or `PATCH /api/rooms/{code}/config` produce `422 invalid_config` with `joker_count` named in the error message. Setting both `joker_count = 0` and `allow_skip_vote = false` puts the room in "fully disabled" mode — the two buttons are hidden and any platform skip message is refused.

### The `GameTypeHandler` interface

The interface defines ten methods. The hub calls them during gameplay and never knows which game type it is running.

**Event naming convention:** phase-transition events (`round_started`, `submissions_closed`, `vote_results`, `game_ended`) are **universal** — every game type uses the same event names. Incoming game-specific messages are slug-prefixed (`meme-freestyle:submit`, `meme-freestyle:vote`). The handler-produced blobs are embedded inside the universal events: `submissions_closed.data.submissions_shown` and `vote_results.data.results`.

| Method                           | Purpose                                                                                        |
| -------------------------------- | ---------------------------------------------------------------------------------------------- |
| `Slug()`                         | Matches `game_types.slug` in the DB (e.g. `"meme-freestyle"`); also the registry key. Read from `manifest.yaml`. |
| `SupportedPayloadVersions()`     | Item payload versions this handler can process. Read from `manifest.yaml`.                     |
| `SupportsSolo()`                 | Whether the handler permits single-player rooms. Read from `manifest.yaml`.                    |
| `MaxPlayers()`                   | Per-room cap from `manifest.config.max_players`; `0` means no cap (manifest value `null`). Checked in `handleRegister` before allocating state. |
| `Manifest()`                     | Returns the embedded `*game.Manifest` so the registry can sync bounds to `game_types.config` at startup and the API layer can call `Manifest().Config.ValidateAndFill` on every config write. |
| `ValidateSubmission()`           | Validates a player's submission payload                                                        |
| `ValidateVote()`                 | Validates a vote (self-vote check; hub has already verified phase + duplicate)                 |
| `CalculateRoundScores()`         | Computes per-player points from votes                                                          |
| `BuildSubmissionsShownPayload()` | Transforms raw submissions into the anonymous display blob embedded in `submissions_closed.data.submissions_shown` |
| `BuildVoteResultsPayload()`      | Builds the author-reveal blob embedded in `vote_results.data.results`                                              |

Implementations must be safe to call from a single hub goroutine — the hub is single-threaded per room, so no additional locking is needed.

### Adding a new game type

Adding a game type is a **three-directory edit**. You do not touch the schema, the WebSocket protocol, existing game types, or any other handler's code. Most of the work is the game's own logic (scoring, payload shape) — the platform scaffolding is minimal by design.

#### 1. Create the handler package — `backend/internal/game/types/{slug}/`

Drop three files in a new directory:

**`manifest.yaml`** — the single source of truth for identity and bounds. Copy `meme_freestyle/manifest.yaml` as a starting point and tune the numbers:

```yaml
slug: trivia
name: Trivia
description: Answer questions fastest for the most points.
version: "1.0.0"
supports_solo: true

required_packs:
  - role: image
    payload_versions: [1]

config:
  min_round_duration_seconds: 10
  max_round_duration_seconds: 60
  default_round_duration_seconds: 20
  min_voting_duration_seconds: 0       # trivia has no voting phase
  max_voting_duration_seconds: 0
  default_voting_duration_seconds: 0
  min_round_count: 5
  max_round_count: 30
  default_round_count: 10
  min_players: 1
  max_players: 20
```

Self-checks in `game.LoadManifest` refuse to boot on malformed values (default outside min/max, min > max, `max_players < min_players`, missing required fields). You get a clean startup error at the first boot instead of a silent corruption later.

**`handler.go`** — embed the manifest, implement the `GameTypeHandler` interface. The identity methods are four one-liners delegating to the manifest; the rest is your game's logic (validation, scoring, reveal payload):

```go
package trivia

import (
    _ "embed"
    "fmt"

    "github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

//go:embed manifest.yaml
var manifestYAML []byte

var manifest = func() *game.Manifest {
    m, err := game.LoadManifest(manifestYAML)
    if err != nil { panic(fmt.Sprintf("trivia: load manifest: %v", err)) }
    return m
}()

type Handler struct{}
func New() *Handler                          { return &Handler{} }
func (h *Handler) Slug() string              { return manifest.Slug }
func (h *Handler) SupportsSolo() bool        { return manifest.SupportsSolo }
func (h *Handler) MaxPlayers() int           { return manifest.MaxPlayersOrDefault() }
func (h *Handler) Manifest() *game.Manifest  { return manifest }

// RequiredPacks declares every pack role the game type consumes. `MinItemsFn`
// computes the minimum item count the pack must contain (e.g. one per round)
// and is evaluated at POST /api/rooms time against the chosen RoomConfig.
func (h *Handler) RequiredPacks() []game.PackRequirement {
    return []game.PackRequirement{
        {
            Role:            game.PackRoleImage,
            PayloadVersions: []int{1},
            MinItemsFn: func(cfg game.RoomConfig, _ int) int {
                return cfg.RoundCount
            },
        },
    }
}

// ... game-specific methods: ValidateSubmission, ValidateVote,
// CalculateRoundScores, BuildSubmissionsShownPayload, BuildVoteResultsPayload ...

var _ game.GameTypeHandler = (*Handler)(nil) // compile-time interface check
```

**`handler_test.go`** (recommended) — unit-test the scoring and payload builders. The hub is single-threaded per room, so no concurrency scaffolding is needed.

If your game type needs more than one content stream (e.g. images **and** captions), extend the `required_packs:` block in the manifest with an extra role entry and return a matching `PackRequirement` from `RequiredPacks()`. The API layer iterates the list and enforces compatibility per role; `rooms.text_pack_id` stores the second pack. `MinItemsFn` is Go-only — it reads `RoomConfig` + `max_players` to return the minimum item count the pack must contain for the room to be creatable.

#### 2. Register at startup — one line in `backend/cmd/server/main.go`

```go
registry := game.NewRegistry()
registry.Register(memefreestyle.New())
registry.Register(trivia.New())         // ← new line

if err := game.SyncGameTypes(ctx, queries, registry, logger); err != nil { ... }
```

That's it for the backend. `SyncGameTypes` will upsert the `game_types` row on the next boot using the manifest's slug as the natural key. No migration is required for the new game type — the row appears automatically. (You *may* add a `NNN_seed_{slug}.up.sql` migration if you want the UUID to be deterministic from a fresh DB, but it is optional; the startup upsert creates the row either way.)

#### 3. Create the frontend plugin — `frontend/src/lib/games/{slug}/`

Four Svelte components plus a barrel:

- `SubmitForm.svelte` — what the player sees during the submit phase
- `VoteForm.svelte` — what the player sees during the voting phase (can be a no-op for games without voting)
- `ResultsView.svelte` — the post-round reveal
- `GameRules.svelte` — rules shown in the lobby
- `index.ts` — re-exports the four components

The room page resolves the bundle by slug at runtime; no router config, no registry entry on the frontend. The Room Settings inputs automatically pick up the manifest's bounds via `GET /api/game-types` — you don't wire the new game's bounds into the UI; they flow in from the YAML.

#### What you do *not* touch

- No database migrations for the game type itself (startup upsert handles it)
- No changes to `rooms`, `submissions`, `votes`, or any other schema
- No WebSocket protocol changes — phase events are universal (`round_started`, `submissions_closed`, `vote_results`, `game_ended`); your game-specific messages are namespaced (`{slug}:submit`, `{slug}:vote`)
- No edits to existing handlers, the hub, the API layer, or the frontend room shell
- No hardcoded bounds anywhere — frontend reads them from the manifest via the API

### Tuning an existing game type

Lowering a bound, raising a cap, bumping the default round count — all one-file edits:

1. Edit `backend/internal/game/types/{slug}/manifest.yaml`
2. Rebuild the backend binary and restart

On the next boot `SyncGameTypes` upserts the new values into `game_types.config`, the frontend picks them up on its next `/api/game-types` fetch, and every subsequent `POST /api/rooms` or `PATCH /api/rooms/{code}/config` validates against them. Existing rooms in the lobby continue to use whatever config they were created with; the new bounds apply to future writes.

If you ever need a bound that isn't already in `Bounds` (e.g. a new per-round setting), the extension pattern is three files:

1. Add the fields to `Bounds` (manifest shape) and `RoomConfig` (canonical runtime shape) in `backend/internal/game/manifest.go`; extend `ValidateAndFill` to enforce the new bound
2. Add the `min_* / max_* / default_*` entries to each handler's `manifest.yaml`
3. Bind the new control in `frontend/src/lib/components/room/WaitingStage.svelte`, reading its bounds from `gameType.config`

Everything else — DB sync, PATCH merging, validation error codes — is already wired.

---

## Meme-caption game type

The launch game type. Gameplay per round:

1. All players see the same image with a prompt
2. Each writes a caption (max 200 characters, Enter submits) — sent as `meme-freestyle:submit`
3. Submissions close when time runs out or all players submit
4. All captions are shown anonymously — players vote for the funniest with `meme-freestyle:vote`
5. Voting closes when time runs out or all players vote
6. Authors are revealed; each player receives 1 point per vote they received
7. Tied captions both receive full points (no tiebreaker)

Author identity is hidden during voting: `submissions_closed.data.submissions_shown.submissions` contains no author fields. Authors are revealed in `vote_results.data.results.submissions[].username` after voting closes.

A player cannot vote for their own submission. The hub pre-validates this before calling the handler.

---

## Meme-vote game type

A twist on the caption game: players pick a caption from a dealt hand instead of typing one. Gameplay per round:

1. At game start, every player is dealt a hand of captions (`default_hand_size: 5`, bounded by `min_hand_size` and `max_hand_size`). Each caption is a text-pack item with `payload_version: 2`.
2. All players see the same image with a prompt. Each player plays one caption from their hand — sent as `meme-showdown:submit` with `{ "card_id": "<uuid>" }`.
3. The played card is consumed; the hand is refilled from the text pack at the start of the next round.
4. Submissions close when time runs out or all players submit. Captions are shown anonymously; players vote for the funniest with `meme-showdown:vote`.
5. Authors are revealed after voting closes; scoring is identical to `meme-freestyle` (1 point per vote received, ties both get full points).

Two pack roles are required per room:

- `pack_id` — the image pack (`payload_version: 1`, same format as `meme-freestyle`)
- `text_pack_id` — the caption pack (`payload_version: 2`, `{ "text": "..." }`)

The hub personalises `round_started` for this game type (`PersonalisesRoundStart() returns true`): each player receives their own `hand` array inside `round_started.data` so the frontend can render the dealt cards. The hub maintains per-player hand state across reconnects and replays it in `room_state.data.my_hand` when a player rejoins mid-game.

`supports_solo` is `false` — voting requires at least two distinct players.

---

## Pack compatibility

Packs are game-type-agnostic at the item-payload level — item payloads are versioned JSONB and compatibility is decided per item by `payload_version`. A game type declares one or more **pack roles** it consumes via `GameTypeHandler.RequiredPacks()`. Each requirement names a role (`image`, `text`, …) and the `payload_version` set it accepts. A room references one pack per declared role (`rooms.pack_id` for image; `rooms.text_pack_id` for text). At creation, the API counts compatible items per role and rejects the room if any pack is missing, empty of compatible items, or smaller than the role's `MinItemsFn` sizing.

Example item payloads:

- `payload_version: 1` (image) — `{ "image_url": "...", "prompt": "..." }`
- `payload_version: 2` (text)  — `{ "text": "..." }`

`meme-freestyle` declares `[{image, [1]}]`. `meme-showdown` declares `[{image, [1]}, {text, [2]}]`. Existing image packs are compatible with both game types as-is; a text pack is additionally required to host a `meme-showdown` room.

---

## Startup recovery

On every backend restart, two recovery operations run:

- Any room with `state = 'playing'` is moved to `finished` — in-progress games cannot be resumed after a crash
- Any room with `state = 'lobby'` older than 24 hours is closed

Both are idempotent.
