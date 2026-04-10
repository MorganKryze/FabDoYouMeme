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

Game types are registered handler units. The entire backend game logic is contained behind a single Go interface: `GameTypeHandler`. Each game type implements this interface and registers itself at startup with `game.Register(slug, handler)`.

The interface defines five responsibilities:

| Method                           | Purpose                                                   |
| -------------------------------- | --------------------------------------------------------- |
| `SupportedPayloadVersions()`     | Declares which item payload versions this handler can use |
| `ValidateSubmission()`           | Validates a player's submission payload                   |
| `BuildSubmissionsShownPayload()` | Transforms raw submissions into anonymous display data    |
| `ValidateVote()`                 | Validates a vote (prevents self-vote, duplicate vote)     |
| `CalculateRoundScores()`         | Computes per-player points from votes                     |

The hub calls these methods but never knows which game type it is running. Game-specific WebSocket messages use a prefixed type (`meme-caption:submit`, `trivia:answer`) so the protocol can carry any game type without changes.

### Adding a new game type

1. Add a migration seeding a new row in `game_types` with the slug, name, and config ranges
2. Create `backend/internal/game/types/{slug}/handler.go` implementing `GameTypeHandler`
3. Call `game.Register("{slug}", handler)` in `backend/cmd/server/main.go`
4. Create `frontend/src/lib/games/{slug}/` with `SubmitForm`, `VoteForm`, `ResultsView`, and `GameRules` components, plus an `index.ts` re-export
5. Optionally add new frontend routes if the game needs unique views

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
