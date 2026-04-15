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

- **Game type** (e.g. `meme-caption`)
- **Pack** of content items to play with
- **Mode** — multiplayer or solo (only if the game type supports it)
- **Config** — round count, round duration, voting duration (constrained by the game type's min/max)

A room has a short code (4 uppercase letters) used by players to join. Codes are generated with `crypto/rand` and retried on collision.

### Lobby phase

Players join the lobby via WebSocket. The host can configure the room, kick players, or leave. If the host leaves during the lobby, the room closes. Once the host sends `start`, the room transitions to `playing` and no new players can join.

### Playing phase

The backend hub drives the round lifecycle autonomously once `start` is received:

1. Selects the next item from the pack (items are shuffled at round start, not reused)
2. Broadcasts `round_started` with the item, timer info (`duration_seconds` + `ends_at`), and round number
3. Accepts submissions until the timer expires or all players have submitted
4. Broadcasts `submissions_closed`, starts the voting timer
5. Accepts votes until the timer expires or all players have voted
6. Calculates scores, broadcasts `vote_results`
7. Waits for the host to send `next_round` before advancing
8. Repeats until all rounds are complete

### Finished phase

The room moves to `finished` when:

- All rounds complete (`reason: "completed"`)
- The host disconnects and the grace window expires (`reason: "host_disconnected"`)
- All players disconnect simultaneously and none return (`reason: "all_players_disconnected"`)
- The pack runs out of unused items mid-game (`reason: "pack_exhausted"`)

Finished rooms are permanent — they cannot be resumed. The leaderboard is accessible via `GET /api/rooms/:code/leaderboard` after the game ends.

### Out-of-band termination

`Hub.EndRoom(ctx, reason)` is the out-of-band termination path, invoked by the `POST /api/rooms/:code/end` handler after the DB row has already been flipped to `finished`. Unlike `finishRoom` — which leaves the hub alive for a rematch window — `EndRoom` broadcasts `room_closed` to every player, closes every send channel (so `writePump` drains remaining messages, writes a close frame, and exits), and leaves no rematch opportunity. Reasons are `ended_by_host` (host clicked End Room) or `ended_by_admin` (a platform admin ended a room they do not host).

---

## WebSocket hub

Each room has a dedicated goroutine — the hub — that owns all mutable room state. No shared state is touched outside the hub goroutine, which eliminates race conditions. The hub communicates with `runRounds` via a channel (`roundCtrl`), keeping the round state machine decoupled from player management.

### Reconnection & grace window

When a player disconnects mid-game, they are not immediately removed. Instead:

- The hub broadcasts `reconnecting` to other players (they see a "reconnecting…" indicator)
- A `RECONNECT_GRACE_WINDOW` timer starts (default 30 seconds)
- If the player reconnects within the window, the hub sends a full `room_state` snapshot and the player continues
- If the timer expires, `player_left` is broadcast and their pending turn is skipped

If all players are simultaneously in the grace window, the round timer pauses. If at least one reconnects, the game resumes.

### Heartbeat

The client sends a `ping` every `WS_PING_INTERVAL` (default 25 seconds). The server replies with `pong` and resets its read deadline (`WS_READ_DEADLINE`, default 60 seconds). A connection that does not pong within the deadline is treated as dead and cleaned up.

---

## Game type system

Game types are registered handler units. The entire backend game logic is contained behind a single Go interface: `game.GameTypeHandler` (defined in `backend/internal/game/handler.go`). Each game type implements this interface and is added to the process-wide registry at startup.

### Registration at startup

A `*game.Registry` is constructed in `main.go`, and each handler is added via `registry.Register(handler)`. The slug is taken from the handler itself (`h.Slug()`) — there is no separate slug argument. Duplicate slugs panic at startup rather than failing silently at runtime, so a misconfiguration is caught on the first boot.

```go
// backend/cmd/server/main.go
registry := game.NewRegistry()
registry.Register(memecaption.New())
// registry.Register(trivia.New())   // future game types added here
```

The registry is then passed to the game manager and the `GameTypeHandler` HTTP handler, which are the only components that resolve slugs back to handlers (`registry.Get(slug)`).

### The `GameTypeHandler` interface

The interface defines nine methods. The hub calls them during gameplay and never knows which game type it is running — game-specific WebSocket messages use a prefixed type (`meme-caption:submit`, `trivia:answer`) so the protocol can carry any game type without changes.

| Method                           | Purpose                                                                                        |
| -------------------------------- | ---------------------------------------------------------------------------------------------- |
| `Slug()`                         | Matches `game_types.slug` in the DB (e.g. `"meme-caption"`); also the registry key             |
| `SupportedPayloadVersions()`     | Declares which item payload versions this handler can process                                  |
| `SupportsSolo()`                 | Whether the handler permits single-player rooms                                                |
| `MaxPlayers()`                   | Absolute per-room cap; `0` means no cap. Checked in `handleRegister` before allocating state   |
| `ValidateSubmission()`           | Validates a player's submission payload                                                        |
| `ValidateVote()`                 | Validates a vote (self-vote check; hub has already verified phase + duplicate)                 |
| `CalculateRoundScores()`         | Computes per-player points from votes                                                          |
| `BuildSubmissionsShownPayload()` | Transforms raw submissions into the anonymous display payload for `{slug}:submissions_shown`   |
| `BuildVoteResultsPayload()`      | Builds the reveal payload for `{slug}:vote_results`                                            |

Implementations must be safe to call from a single hub goroutine — the hub is single-threaded per room, so no additional locking is needed.

### Two places the slug must match

A game type has two halves that must agree on the slug:

1. **DB row** in `game_types` — holds the operator-tunable metadata: name, description, `supports_solo`, and the `config` JSONB with round/voting duration bounds, player bounds, and round count bounds. Seeded via a migration in `backend/db/migrations/`.
2. **Go handler** in `backend/internal/game/types/{slug}/` — holds the behaviour (validation, scoring, payload builders) and hard caps (`MaxPlayers`).

If either half is missing, the game type is unusable: a missing handler means `registry.Get(slug)` returns `false` and room creation is rejected; a missing DB row means there is nothing for the UI to list under `/api/game-types` even though the handler is registered.

### Adding a new game type

1. **Migration** — add `backend/db/migrations/NNN_seed_{slug}.up.sql` (and a matching `.down.sql`) inserting a row into `game_types` with the slug, name, description, `supports_solo`, and the config JSONB (min/max/default for round duration, voting duration, player count, round count). Use `002_seed_game_types.up.sql` as the template.
2. **Handler package** — create `backend/internal/game/types/{slug}/handler.go` implementing every method on `game.GameTypeHandler`. Add a compile-time assertion at the bottom of the file: `var _ game.GameTypeHandler = (*Handler)(nil)`. This turns a missing method into a build error instead of a runtime panic.
3. **Register at startup** — in `backend/cmd/server/main.go`, add `registry.Register(yourpkg.New())` alongside the existing `memecaption` line.
4. **Frontend plugin** — create `frontend/src/lib/games/{slug}/` with `SubmitForm.svelte`, `VoteForm.svelte`, `ResultsView.svelte`, and `GameRules.svelte`, plus an `index.ts` that re-exports them. The room page selects the bundle by slug at runtime.
5. **Optional frontend routes** — only if the game needs views that do not fit the default room layout.

No database schema changes. No WebSocket protocol changes. No changes to existing game types.

---

## Meme-caption game type

The launch game type. Gameplay per round:

1. All players see the same image with a prompt
2. Each writes a caption (max 300 characters) — sent as `meme-caption:submit`
3. Submissions close when time runs out or all players submit
4. All captions are shown anonymously — players vote for the funniest with `meme-caption:vote`
5. Voting closes when time runs out or all players vote
6. Authors are revealed; each player receives 1 point per vote they received
7. Tied captions both receive full points (no tiebreaker)

Author identity is hidden during voting: the payload broadcast at `submissions_closed` contains no `author_id` or `author_username`. They are revealed only in `vote_results`.

A player cannot vote for their own submission. The hub pre-validates this before calling the handler.

---

## Pack compatibility

Packs are game-type-agnostic — the same pack of meme images can be used across `meme-caption`, `meme-vote`, or any future type. Compatibility is checked at room creation by counting items with a `payload_version` supported by the chosen game type's handler. If the pack has zero compatible items or fewer than the configured round count, room creation is rejected with a specific error.

Item payloads are JSONB; the structure is defined per game type. `meme-caption` uses `{ "image_url": "...", "prompt": "..." }`.

---

## Startup recovery

On every backend restart, two recovery operations run:

- Any room with `state = 'playing'` is moved to `finished` — in-progress games cannot be resumed after a crash
- Any room with `state = 'lobby'` older than 24 hours is closed

Both are idempotent.
