// backend/internal/game/hub.go
package game

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// PlayerIdentity is the minimum info the hub needs to register a connection.
// A player is either a registered user (IsGuest=false, ID is users.id) or a
// guest participant (IsGuest=true, ID is guest_players.id). Hosts are always
// users — the B1 plan explicitly forbids guests from hosting.
type PlayerIdentity struct {
	ID          string
	DisplayName string
	IsGuest     bool
}

// pgUUID converts a plain UUID string to a pgtype.UUID value for sqlc params
// that were made nullable in migration 004 (the B1 player identity refactor).
// If the string is not a parseable UUID, Valid is false — the sqlc call will
// then fail cleanly at the DB layer rather than panic here.
func pgUUID(s string) pgtype.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

// HubState tracks in-memory room lifecycle.
type HubState string

const (
	HubLobby    HubState = "lobby"
	HubPlaying  HubState = "playing"
	HubFinished HubState = "finished" // game ended; connections kept alive so clients see the end screen
)

// graceExpiredMsg is sent to the Run goroutine when a reconnect grace window expires.
type graceExpiredMsg struct {
	userID   string
	username string
	isGuest  bool
}

// hubRoundPhase tracks the current phase of a round.
type hubRoundPhase int

const (
	phaseIdle       hubRoundPhase = iota
	phaseSubmitting               // accepting player submissions
	phaseVoting                   // accepting votes
	phaseResults                  // showing round results; in host-paced mode, waiting for next_round
)

// roundCtrlMsg is the interface for all round lifecycle control messages.
type roundCtrlMsg interface{ roundCtrl() }

// roundCtrlAdvance signals runRounds to move to the next round (host-triggered).
type roundCtrlAdvance struct{}

func (roundCtrlAdvance) roundCtrl() {}

// roundCtrlStartRound signals the hub to broadcast round_started.
type roundCtrlStartRound struct {
	roundID        uuid.UUID
	itemID         uuid.UUID
	payload        json.RawMessage
	mediaURL       string
	endsAt         time.Time
	duration       time.Duration
	allSubmittedCh chan struct{} // closed by hub when all players have submitted
}

func (roundCtrlStartRound) roundCtrl() {}

// roundCtrlCloseSubmissions signals the hub to broadcast submissions_closed.
type roundCtrlCloseSubmissions struct {
	votingEndsAt time.Time
	duration     time.Duration
	allVotedCh   chan struct{} // closed by hub when all players have voted
}

func (roundCtrlCloseSubmissions) roundCtrl() {}

// roundCtrlCloseVoting signals the hub to tally votes and broadcast vote_results.
type roundCtrlCloseVoting struct{}

func (roundCtrlCloseVoting) roundCtrl() {}

// roundCtrlEndGame signals the hub to finish the room and broadcast game_ended.
// reason is forwarded directly into finishRoom (e.g. "game_complete",
// "pack_exhausted", "all_players_disconnected").
type roundCtrlEndGame struct{ reason string }

func (roundCtrlEndGame) roundCtrl() {}

// roundCtrlPaused signals the hub to broadcast round_paused (all players in grace).
type roundCtrlPaused struct{}

func (roundCtrlPaused) roundCtrl() {}

// roundCtrlResumed signals the hub to broadcast round_resumed with the adjusted deadline.
type roundCtrlResumed struct{ newEndsAt time.Time }

func (roundCtrlResumed) roundCtrl() {}

// Hub manages one room's WebSocket connections and game lifecycle.
// All state mutations happen in the main Run() goroutine — no locking needed
// for internal state. Incoming connections are registered via a channel.
type Hub struct {
	roomCode     string
	roomID       uuid.UUID
	gameTypeSlug string
	hostUserID   string

	registry *Registry
	db       *db.Queries
	cfg      *config.Config
	log      *slog.Logger
	clock    clock.Clock

	state   HubState
	players map[string]*connectedPlayer // playerID → player (user OR guest UUID)

	// playerTypes remembers whether each playerID is a guest, for the duration
	// of the hub's lifetime. This lets the scoring loop branch between
	// UpdatePlayerScore and UpdateGuestPlayerScore even after a player
	// disconnects and drops out of the players map (e.g. mid-round grace
	// expiration). Populated on first register; never cleared until the hub
	// goroutine itself exits.
	playerTypes map[string]bool // playerID → isGuest

	register     chan *connectedPlayer
	unregister   chan *connectedPlayer
	incoming     chan playerMessage
	graceExpired chan graceExpiredMsg
	roundCtrl    chan roundCtrlMsg // round lifecycle signals from runRounds

	// roundGrace is a buffered(1) channel through which the Run() goroutine
	// notifies waitPhase of all-in-grace state changes. true = every connected
	// player is in the reconnect window; false = at least one is active.
	// The drain-then-send pattern in notifyGraceState ensures waitPhase always
	// sees the latest state rather than a stale queued value.
	roundGrace chan bool

	// roundAdvanceCh is the signal channel for host-paced mode. When the room
	// config has host_paced=true, runRounds blocks on this channel (or a safety
	// timeout) after broadcasting vote_results, instead of auto-advancing after
	// 3 seconds. handleRoundCtrl sends here when it receives roundCtrlAdvance
	// and the current phase is phaseResults.
	roundAdvanceCh chan struct{}

	// Round state — only accessed from Run() goroutine
	roundPhase       hubRoundPhase
	roundNum         int
	currentRound     *db.Round
	roundSubmissions map[string]Submission // userID → submission
	roundVotes       map[string]Vote       // userID → vote

	// Round snapshot for mid-game reconnection rehydration. A client that
	// refreshes or joins late would otherwise receive only the minimal
	// room_state (state/players/host) and have no way to render the
	// in-flight round — buildRoomState reads these fields when the hub is in
	// HubPlaying so the snapshot carries enough to resume the stage. All
	// fields are set by handleRoundCtrl and reset when a new round starts or
	// the room finishes; they are only ever accessed from the Run() goroutine.
	roundItemPayload   json.RawMessage
	roundMediaURL      string
	roundSubmitEndsAt  time.Time
	roundSubmitDur     time.Duration
	roundVoteEndsAt    time.Time
	roundVoteDur       time.Duration
	submissionsShown   json.RawMessage
	resultsShown       json.RawMessage
	resultsLeaderboard []db.GetRoomLeaderboardRow
	roundPausedFlag    bool

	// Early-close channels: created by runRounds, stored here so handleGameMessage
	// can close them when all players have submitted / voted. Set to nil after close.
	allSubmittedCh chan struct{}
	allVotedCh     chan struct{}

	// roundsCancel aborts the runRounds goroutine spawned in startGame.
	// It is set by startGame and cleared by finishRoom. Accessed only from
	// the Run() goroutine, so no mutex is needed despite the function type.
	roundsCancel context.CancelFunc
}

// connectedPlayer is hub-internal player state. userID is really a generic
// "player ID" since the B1 refactor — for registered users it matches
// users.id, for guests it matches guest_players.id. The name is preserved to
// minimise churn in existing tests and broadcast field names.
type connectedPlayer struct {
	userID       string
	username     string
	isGuest      bool
	conn         *websocket.Conn
	send         chan []byte
	reconnecting bool
}

// playerMessage is a message arriving from a player connection.
type playerMessage struct {
	player  *connectedPlayer
	msgType string
	data    json.RawMessage
}

// HubConfig groups the dependencies Hub needs.
type HubConfig struct {
	RoomCode     string
	RoomID       uuid.UUID
	GameTypeSlug string
	HostUserID   string
	Registry     *Registry
	DB           *db.Queries
	Cfg          *config.Config
	Log          *slog.Logger
	Clock        clock.Clock
}

// NewHub creates a Hub but does not start it. Call hub.Run() in a goroutine.
func NewHub(hc HubConfig) *Hub {
	clk := hc.Clock
	if clk == nil {
		clk = clock.Real{}
	}
	return &Hub{
		roomCode:     hc.RoomCode,
		roomID:       hc.RoomID,
		gameTypeSlug: hc.GameTypeSlug,
		hostUserID:   hc.HostUserID,
		registry:     hc.Registry,
		db:           hc.DB,
		cfg:          hc.Cfg,
		log:          hc.Log,
		clock:        clk,
		state:            HubLobby,
		players:          make(map[string]*connectedPlayer),
		playerTypes:      make(map[string]bool),
		register:         make(chan *connectedPlayer, 8),   // burst: ≤8 simultaneous joins before blocking
		unregister:       make(chan *connectedPlayer, 8),   // burst: ≤8 simultaneous disconnects
		incoming:         make(chan playerMessage, 64),     // 64 msgs queued before readPump blocks
		graceExpired:     make(chan graceExpiredMsg, 16),   // 16 concurrent grace expirations
		roundCtrl:        make(chan roundCtrlMsg, 8),       // round lifecycle signals from runRounds
		roundGrace:       make(chan bool, 1),               // all-in-grace state changes for waitPhase
		roundAdvanceCh:   make(chan struct{}, 1),           // host-paced advance signal from handleRoundCtrl
		roundPhase:       phaseIdle,
		roundSubmissions: make(map[string]Submission),
		roundVotes:       make(map[string]Vote),
	}
}

// Join is called from the HTTP handler (outside Run goroutine) to add a new
// WS connection. It blocks until the player is registered or the context is
// cancelled. The identity tells the hub whether this connection represents a
// logged-in user or a guest participant.
func (h *Hub) Join(ctx context.Context, ident PlayerIdentity, conn *websocket.Conn) {
	p := &connectedPlayer{
		userID:   ident.ID,
		username: ident.DisplayName,
		isGuest:  ident.IsGuest,
		conn:     conn,
		send:     make(chan []byte, 64),
	}
	select {
	case h.register <- p:
	case <-ctx.Done():
		conn.Close()
	}
}

// Run is the hub's main event loop. Call in a goroutine; it exits when the room ends.
func (h *Hub) Run(ctx context.Context) {
	h.log.Info("hub started", "room", h.roomCode)
	defer h.log.Info("hub stopped", "room", h.roomCode)

	for {
		select {
		case <-ctx.Done():
			h.broadcast(buildMessage("game_ended", map[string]string{"reason": "server_shutdown"}))
			return

		case p := <-h.register:
			h.handleRegister(ctx, p)

		case p := <-h.unregister:
			h.handleUnregister(p)

		case msg := <-h.incoming:
			h.handleMessage(ctx, msg)

		case msg := <-h.graceExpired:
			h.handleGraceExpired(ctx, msg)

		case ctrl := <-h.roundCtrl:
			h.handleRoundCtrl(ctx, ctrl)
		}
	}
}

func (h *Hub) handleRegister(ctx context.Context, p *connectedPlayer) {
	existing, reconnecting := h.players[p.userID]
	if reconnecting && existing.reconnecting {
		// Player reconnecting within grace window.
		// Close the old send channel to stop the old writePump cleanly,
		// then install the new connection struct — avoiding field mutation
		// that would race with the old writePump goroutine.
		close(existing.send)
		h.players[p.userID] = p
		h.playerTypes[p.userID] = p.isGuest
		p.reconnecting = false
		// Notify waitPhase that at least one player is now active; this
		// resumes the round timer if it was paused due to all-in-grace.
		h.notifyGraceState()
		h.sendTo(p, buildMessage("room_state", h.buildRoomState(p.userID)))
		h.broadcast(buildMessage("player_joined", playerJoinedPayload(p)))
		go h.readPump(p)
		go h.writePump(p)
		return
	}

	if h.state == HubPlaying {
		writeWS(p.conn, buildMessage("error", map[string]string{
			"code": "game_already_started", "message": "Game is already in progress",
		}))
		p.conn.Close()
		return
	}
	// HubFinished: new connections are accepted as read-only viewers; they get a room_state snapshot so the client shows the end screen.

	// Reject the join once the handler's per-room cap is reached. MaxPlayers==0
	// means "no explicit cap" — current behaviour for handlers that haven't
	// wired in a limit yet. Finding 3.D in the 2026-04-10 review.
	if handler, ok := h.registry.Get(h.gameTypeSlug); ok {
		if cap := handler.MaxPlayers(); cap > 0 && len(h.players) >= cap {
			writeWS(p.conn, buildMessage("error", map[string]string{
				"code": "room_full", "message": "Room is full",
			}))
			p.conn.Close()
			return
		}
	}

	h.players[p.userID] = p
	h.playerTypes[p.userID] = p.isGuest
	// Persist registered joiners into room_players so GetActiveRoomForUser,
	// leaderboard aggregation, and post-game history all see them. Guests are
	// already inserted at token mint time in api/guest_join.go. The upsert is
	// idempotent, so reconnecting hosts / returning players are no-ops.
	// Skip for HubFinished: a player connecting to a finished room is joining
	// read-only and must not be re-linked to the room row (B21).
	if !p.isGuest && h.db != nil && (h.state == HubLobby || h.state == HubPlaying) {
		if err := h.db.UpsertRoomPlayer(ctx, db.UpsertRoomPlayerParams{
			RoomID: h.roomID,
			UserID: pgUUID(p.userID),
		}); err != nil && h.log != nil {
			h.log.Warn("hub: upsert room_players failed",
				"room", h.roomCode, "user_id", p.userID, "err", err)
		}
	}
	// Send the full room snapshot to the new arrival so they see all existing
	// players immediately, without waiting for individual player_joined events.
	// (The reconnect path above already does this for returning players.)
	h.sendTo(p, buildMessage("room_state", h.buildRoomState(p.userID)))
	h.broadcast(buildMessage("player_joined", playerJoinedPayload(p)))
	go h.readPump(p)
	go h.writePump(p)
}

// playerJoinedPayload emits both the legacy (user_id / username) and the new
// (player_id / display_name / is_guest) field sets so that existing tests and
// the pre-F3 frontend keep working while the in-room rewrite catches up.
func playerJoinedPayload(p *connectedPlayer) map[string]any {
	return map[string]any{
		"user_id":      p.userID,
		"username":     p.username,
		"player_id":    p.userID,
		"display_name": p.username,
		"is_guest":     p.isGuest,
	}
}

func (h *Hub) handleUnregister(p *connectedPlayer) {
	if h.players[p.userID] != p {
		// Either the player was never registered, or a new connection for the
		// same user has already replaced this one in the map (fast page reload).
		// In the latter case silently drop: the new socket is live and we must
		// not broadcast a stale "reconnecting" that confuses other players.
		return
	}
	p.reconnecting = true
	h.broadcast(buildMessage("reconnecting", map[string]any{
		"user_id":      p.userID,
		"username":     p.username,
		"player_id":    p.userID,
		"display_name": p.username,
		"is_guest":     p.isGuest,
	}))
	// AfterFunc avoids a goroutine leak: the fake/real timer fires once and
	// exits. All state changes still happen inside Run() because the send
	// is into a buffered channel read by Run().
	userID, username, isGuest := p.userID, p.username, p.isGuest
	grace := h.cfg.ReconnectGraceWindow
	h.clock.AfterFunc(grace, func() {
		// Non-blocking send: if graceExpired is full (16 concurrent
		// expirations would be extreme) we drop and log. The player stays
		// flagged reconnecting=true until the next expiration cycle or a
		// real reconnect arrives. Finding 4.G in the 2026-04-10 review.
		select {
		case h.graceExpired <- graceExpiredMsg{userID: userID, username: username, isGuest: isGuest}:
		default:
			if h.log != nil {
				h.log.Warn("hub: graceExpired channel full, dropping expiry",
					"user_id", userID, "room", h.roomCode)
			}
		}
	})
	h.notifyGraceState()
}

func (h *Hub) handleGraceExpired(ctx context.Context, msg graceExpiredMsg) {
	cp, ok := h.players[msg.userID]
	if !ok || !cp.reconnecting {
		return // player reconnected in time — ignore
	}
	delete(h.players, msg.userID)
	h.broadcast(buildMessage("player_left", map[string]any{
		"user_id":      msg.userID,
		"username":     msg.username,
		"player_id":    msg.userID,
		"display_name": msg.username,
		"is_guest":     msg.isGuest,
	}))
	h.notifyGraceState()
	if msg.userID == h.hostUserID && h.state == HubPlaying {
		h.finishRoom(ctx, "host_disconnected", nil)
	}
}

// allPlayersInGrace returns true when there is at least one player in the hub
// and every one of them is currently in the reconnect grace window. A hub with
// no players returns false — we only pause when there was an active game.
// Called only from the Run() goroutine.
func (h *Hub) allPlayersInGrace() bool {
	if len(h.players) == 0 {
		return false
	}
	for _, p := range h.players {
		if !p.reconnecting {
			return false
		}
	}
	return true
}

// notifyGraceState publishes the current all-in-grace state to the waitPhase
// helper running in the runRounds goroutine. The drain-then-send pattern
// ensures waitPhase always reads the latest value rather than a stale one
// queued before the most recent unregister/register.
// Called only from the Run() goroutine.
func (h *Hub) notifyGraceState() {
	state := h.allPlayersInGrace()
	// Drain any unread stale value so we overwrite it with the fresh state.
	select {
	case <-h.roundGrace:
	default:
	}
	select {
	case h.roundGrace <- state:
	default:
	}
}

func (h *Hub) handleMessage(ctx context.Context, msg playerMessage) {
	switch msg.msgType {
	case "start":
		if msg.player.userID != h.hostUserID {
			h.sendTo(msg.player, buildMessage("error", map[string]string{
				"code": "not_host", "message": "Only the host can start the game",
			}))
			return
		}
		h.startGame(ctx)

	case "reconnect":
		if cp, ok := h.players[msg.player.userID]; ok && !cp.reconnecting {
			h.safeSend(msg.player, buildMessage("room_state", h.buildRoomState(msg.player.userID)))
		}

	case "next_round":
		if msg.player.userID != h.hostUserID {
			h.safeSend(msg.player, buildMessage("error", map[string]string{
				"code": "not_host", "message": "Only the host can advance rounds",
			}))
			return
		}
		select {
		case h.roundCtrl <- roundCtrlAdvance{}:
		default:
		}

	case "ping":
		h.sendTo(msg.player, buildMessage("pong", nil))

	case "system:kick":
		var d struct {
			TargetUserID string `json:"target_user_id"`
		}
		if err := json.Unmarshal(msg.data, &d); err != nil {
			if h.log != nil {
				h.log.Warn("hub: system:kick payload unmarshal failed",
					"error", err, "room", h.roomCode)
			}
			return
		}
		if p, ok := h.players[d.TargetUserID]; ok {
			h.safeSend(p, buildMessage("player_kicked", map[string]string{"user_id": d.TargetUserID}))
			delete(h.players, d.TargetUserID)
			h.broadcast(buildMessage("player_kicked", map[string]string{"user_id": d.TargetUserID}))
		}

	case "system:end_room":
		var d struct {
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(msg.data, &d); err != nil {
			if h.log != nil {
				h.log.Warn("hub: system:end_room payload unmarshal failed",
					"error", err, "room", h.roomCode)
			}
			return
		}
		h.endRoomInternal(d.Reason)

	default:
		// Game-type-specific messages are prefixed with the slug
		expected := h.gameTypeSlug + ":"
		if len(msg.msgType) > len(expected) && msg.msgType[:len(expected)] == expected {
			h.handleGameMessage(ctx, msg)
		} else {
			h.sendTo(msg.player, buildMessage("error", map[string]string{
				"code": "unknown_message_type", "message": "Unknown message type",
			}))
		}
	}
}

func (h *Hub) handleRoundCtrl(ctx context.Context, ctrl roundCtrlMsg) {
	switch c := ctrl.(type) {
	case roundCtrlAdvance:
		// In host-paced mode, signal runRounds to move to the next round.
		// Only meaningful while phaseResults is active; otherwise it's a no-op
		// (e.g. a duplicate next_round from a race with the safety timeout).
		if h.roundPhase == phaseResults {
			select {
			case h.roundAdvanceCh <- struct{}{}:
			default:
				// signal already queued; drop duplicate
			}
		}

	case roundCtrlStartRound:
		h.roundPhase = phaseSubmitting
		h.roundNum++
		h.roundSubmissions = make(map[string]Submission)
		h.roundVotes = make(map[string]Vote)
		h.currentRound = &db.Round{ID: c.roundID}
		h.allSubmittedCh = c.allSubmittedCh
		h.allVotedCh = nil
		// Cache for reconnection rehydration. The results/submissions fields
		// from any prior round are wiped so a reconnecting client doesn't see
		// stale data bleed across rounds.
		h.roundItemPayload = c.payload
		h.roundMediaURL = c.mediaURL
		h.roundSubmitEndsAt = c.endsAt
		h.roundSubmitDur = c.duration
		h.roundVoteEndsAt = time.Time{}
		h.roundVoteDur = 0
		h.submissionsShown = nil
		h.resultsShown = nil
		h.resultsLeaderboard = nil
		h.roundPausedFlag = false
		h.broadcast(buildMessage("round_started", map[string]any{
			"round_number":     h.roundNum,
			"ends_at":          c.endsAt.UTC().Format(time.RFC3339),
			"duration_seconds": int(c.duration.Seconds()),
			"item": map[string]any{
				"payload":   c.payload,
				"media_url": nilIfEmpty(c.mediaURL),
			},
		}))

	case roundCtrlCloseSubmissions:
		submissions := submissionsMapToSlice(h.roundSubmissions)
		// If nobody submitted, skip the voting phase entirely. Closing allVotedCh
		// immediately causes waitPhase in runRounds to unblock, which then sends
		// roundCtrlCloseVoting; that broadcasts vote_results with empty data so
		// the frontend transitions from submitting → results without getting stuck
		// on a voting phase that would never receive any votes.
		if len(submissions) == 0 {
			h.roundPhase = phaseIdle
			h.allSubmittedCh = nil
			close(c.allVotedCh)
			return
		}
		h.roundPhase = phaseVoting
		h.allSubmittedCh = nil
		h.allVotedCh = c.allVotedCh
		h.roundVoteEndsAt = c.votingEndsAt
		h.roundVoteDur = c.duration
		handler, ok := h.registry.Get(h.gameTypeSlug)
		if ok {
			shown, err := handler.BuildSubmissionsShownPayload(submissions)
			if err == nil {
				h.submissionsShown = shown
				h.broadcast(buildMessage("submissions_closed", map[string]any{
					"submissions_shown": json.RawMessage(shown),
					"ends_at":          c.votingEndsAt.UTC().Format(time.RFC3339),
					"duration_seconds": int(c.duration.Seconds()),
				}))
				return
			}
		}
		h.broadcast(buildMessage("submissions_closed", map[string]any{
			"ends_at":          c.votingEndsAt.UTC().Format(time.RFC3339),
			"duration_seconds": int(c.duration.Seconds()),
		}))

	case roundCtrlCloseVoting:
		h.roundPhase = phaseResults
		handler, ok := h.registry.Get(h.gameTypeSlug)
		if ok {
			submissions := submissionsMapToSlice(h.roundSubmissions)
			// Enrich each submission with its author's display name so the
			// handler can include it in the vote_results reveal payload.
			for i := range submissions {
				if p, ok := h.players[submissions[i].UserID.String()]; ok {
					submissions[i].AuthorUsername = p.username
				}
			}
			votes := votesMapToSlice(h.roundVotes)
			scores := handler.CalculateRoundScores(submissions, votes)
			for playerID, pts := range scores {
				pgID := pgtype.UUID{Bytes: playerID, Valid: true}
				isGuest := h.playerTypes[playerID.String()]
				if isGuest {
					if _, err := h.db.UpdateGuestPlayerScore(ctx, db.UpdateGuestPlayerScoreParams{
						RoomID:        h.roomID,
						GuestPlayerID: pgID,
						Score:         int32(pts),
					}); err != nil && h.log != nil {
						h.log.Error("hub: update guest player score", "error", err)
					}
				} else {
					if _, err := h.db.UpdatePlayerScore(ctx, db.UpdatePlayerScoreParams{
						RoomID: h.roomID,
						UserID: pgID,
						Score:  int32(pts),
					}); err != nil && h.log != nil {
						h.log.Error("hub: update player score", "error", err)
					}
				}
			}
			resultsPayload, err := handler.BuildVoteResultsPayload(submissions, votes, scores)
			if err == nil {
				leaderboard, _ := h.db.GetRoomLeaderboard(ctx, h.roomID)
				h.resultsShown = resultsPayload
				h.resultsLeaderboard = leaderboard
				h.broadcast(buildMessage("vote_results", map[string]any{
					"results":     json.RawMessage(resultsPayload),
					"leaderboard": leaderboard,
				}))
				return
			}
		}
		h.broadcast(buildMessage("vote_results", nil))

	case roundCtrlPaused:
		h.roundPausedFlag = true
		h.broadcast(buildMessage("round_paused", nil))

	case roundCtrlResumed:
		h.roundPausedFlag = false
		// Update whichever deadline is currently active so the reconnect
		// snapshot reflects the resumed timer rather than the pre-pause one.
		switch h.roundPhase {
		case phaseSubmitting:
			h.roundSubmitEndsAt = c.newEndsAt
		case phaseVoting:
			h.roundVoteEndsAt = c.newEndsAt
		}
		h.broadcast(buildMessage("round_resumed", map[string]any{
			"ends_at": c.newEndsAt.UTC().Format(time.RFC3339),
		}))

	case roundCtrlEndGame:
		leaderboard, _ := h.db.GetRoomLeaderboard(ctx, h.roomID)
		h.finishRoom(ctx, c.reason, map[string]any{
			"leaderboard": leaderboard,
		})
	}
}

func (h *Hub) startGame(ctx context.Context) {
	if h.state != HubLobby {
		return
	}
	h.state = HubPlaying
	if _, err := h.db.SetRoomState(ctx, db.SetRoomStateParams{
		ID: h.roomID, State: "playing",
	}); err != nil {
		h.log.Error("hub: set room state playing", "error", err)
	}
	h.broadcast(buildMessage("game_started", map[string]any{
		"player_count": len(h.players),
	}))
	// Derive a child context scoped to the round loop. finishRoom cancels
	// it so runRounds stops emitting roundCtrl messages into a hub that is
	// already "finished" (host disconnect, early termination, etc.).
	roundsCtx, cancel := context.WithCancel(ctx)
	h.roundsCancel = cancel
	go h.runRounds(roundsCtx)
}

// sendRoundCtrl forwards a round lifecycle message to the Run goroutine while
// respecting the round loop's context. Returning false means the context was
// cancelled (finishRoom was called elsewhere) and the caller must exit
// immediately without touching any more hub state.
func (h *Hub) sendRoundCtrl(ctx context.Context, msg roundCtrlMsg) bool {
	select {
	case h.roundCtrl <- msg:
		return true
	case <-ctx.Done():
		return false
	}
}

// waitPhase waits until the phase duration expires, allDone is closed (all
// players acted early), the context is cancelled, or every player has been in
// the reconnect grace window for longer than ReconnectGraceWindow (in which
// case the room is ended with reason "all_players_disconnected").
//
// While all players are in grace the phase timer is frozen; when at least one
// reconnects the remaining duration resumes and round_resumed is broadcast with
// the adjusted deadline.
//
// Passing a nil allDone is safe — a nil channel in a select case never fires,
// so the inter-round delay (which has no early-exit) can reuse this helper.
func (h *Hub) waitPhase(ctx context.Context, duration time.Duration, allDone chan struct{}) bool {
	deadline := h.clock.Now().Add(duration)
	ticker := h.clock.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var pauseStart time.Time
	paused := false
	remaining := duration

	for {
		select {
		case <-ctx.Done():
			return false
		case <-allDone:
			return true
		case inGrace := <-h.roundGrace:
			switch {
			case inGrace && !paused:
				paused = true
				pauseStart = h.clock.Now()
				remaining = deadline.Sub(h.clock.Now())
				if remaining < 0 {
					remaining = 0
				}
				if !h.sendRoundCtrl(ctx, roundCtrlPaused{}) {
					return false
				}
			case !inGrace && paused:
				paused = false
				deadline = h.clock.Now().Add(remaining)
				if !h.sendRoundCtrl(ctx, roundCtrlResumed{newEndsAt: deadline}) {
					return false
				}
			}
		case <-ticker.C():
			if paused {
				if h.clock.Now().Sub(pauseStart) >= h.cfg.ReconnectGraceWindow {
					if !h.sendRoundCtrl(ctx, roundCtrlEndGame{reason: "all_players_disconnected"}) {
						return false
					}
					return false
				}
			} else if !h.clock.Now().Before(deadline) {
				return true
			}
		}
	}
}

func (h *Hub) runRounds(ctx context.Context) {
	room, err := h.db.GetRoomByID(ctx, h.roomID)
	if err != nil {
		if h.log != nil {
			h.log.Error("runRounds: get room", "error", err)
		}
		h.sendRoundCtrl(ctx, roundCtrlEndGame{reason: "pack_exhausted"})
		return
	}

	var cfg struct {
		RoundCount            int  `json:"round_count"`
		RoundDurationSeconds  int  `json:"round_duration_seconds"`
		VotingDurationSeconds int  `json:"voting_duration_seconds"`
		HostPaced             bool `json:"host_paced"`
	}
	if room.Config != nil {
		if err := json.Unmarshal(room.Config, &cfg); err != nil && h.log != nil {
			// Fall through with zero values; defaults below will kick in.
			// Logging it is enough — a malformed room.Config is an operator
			// issue, not a reason to abort the round loop.
			h.log.Warn("runRounds: room.Config unmarshal failed",
				"error", err, "room", h.roomCode)
		}
	}
	if cfg.RoundCount == 0 {
		cfg.RoundCount = 3
	}
	if cfg.RoundDurationSeconds == 0 {
		cfg.RoundDurationSeconds = 60
	}
	if cfg.VotingDurationSeconds == 0 {
		cfg.VotingDurationSeconds = 30
	}

	handler, hasHandler := h.registry.Get(h.gameTypeSlug)
	var versions []int32
	if hasHandler {
		for _, v := range handler.SupportedPayloadVersions() {
			versions = append(versions, int32(v))
		}
	}

	roundDuration := time.Duration(cfg.RoundDurationSeconds) * time.Second
	votingDuration := time.Duration(cfg.VotingDurationSeconds) * time.Second

	for i := 0; i < cfg.RoundCount; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Fetch an unplayed item from the pack
		items, err := h.db.GetRandomUnplayedItems(ctx, db.GetRandomUnplayedItemsParams{
			PackID:   room.PackID,
			Versions: versions,
			RoomID:   h.roomID,
		})
		if err != nil || len(items) == 0 {
			if h.log != nil {
				h.log.Error("runRounds: get items", "error", err)
			}
			h.sendRoundCtrl(ctx, roundCtrlEndGame{reason: "pack_exhausted"})
			return
		}
		item := items[0]

		dbRound, err := h.db.CreateRound(ctx, db.CreateRoundParams{
			RoomID: h.roomID,
			ItemID: item.ID,
		})
		if err != nil {
			if h.log != nil {
				h.log.Error("runRounds: create round", "error", err)
			}
			h.sendRoundCtrl(ctx, roundCtrlEndGame{reason: "pack_exhausted"})
			return
		}

		mediaURL := ""
		if item.MediaKey != nil && *item.MediaKey != "" {
			// Mirror api.MediaURL: expose the backend-served proxy URL rather
			// than the raw storage key. The frontend loads it via <img src=…>
			// straight from the round_started payload — leaking the media_key
			// unchanged would 404 in the browser.
			mediaURL = "/api/assets/media?key=" + url.QueryEscape(*item.MediaKey)
		}
		allSubmittedCh := make(chan struct{})
		endsAt := h.clock.Now().Add(roundDuration)
		if !h.sendRoundCtrl(ctx, roundCtrlStartRound{
			roundID:        dbRound.ID,
			itemID:         item.ID,
			payload:        item.Payload,
			mediaURL:       mediaURL,
			endsAt:         endsAt,
			duration:       roundDuration,
			allSubmittedCh: allSubmittedCh,
		}) {
			return
		}

		if !h.waitPhase(ctx, roundDuration, allSubmittedCh) {
			return
		}

		allVotedCh := make(chan struct{})
		votingEndsAt := h.clock.Now().Add(votingDuration)
		if !h.sendRoundCtrl(ctx, roundCtrlCloseSubmissions{
			votingEndsAt: votingEndsAt,
			duration:     votingDuration,
			allVotedCh:   allVotedCh,
		}) {
			return
		}

		if !h.waitPhase(ctx, votingDuration, allVotedCh) {
			return
		}

		if !h.sendRoundCtrl(ctx, roundCtrlCloseVoting{}) {
			return
		}

		if cfg.HostPaced {
			// Host-paced: wait for the host to click "Next Round", with a
			// 5-minute safety auto-advance so an absent host doesn't stall
			// the room forever. Both paths continue to the next iteration.
			select {
			case <-ctx.Done():
				return
			case <-h.roundAdvanceCh:
				// host clicked "Next Round"
			case <-h.clock.After(5 * time.Minute):
				// safety timeout: auto-advance
			}
		} else {
			// Server-paced: inter-round pause to let clients read the results
			// (podium + mini leaderboard). 3s was too brief for a scanning
			// player; 10s gives time to process before the next round lands.
			if !h.waitPhase(ctx, 10*time.Second, nil) {
				return
			}
		}
	}

	h.sendRoundCtrl(ctx, roundCtrlEndGame{reason: "game_complete"})
}

// finishRoom transitions the hub and the persisted room row to "finished"
// and broadcasts game_ended. The extra map is merged into the broadcast
// payload — callers use it to attach the final leaderboard when they have
// one handy.
//
// The hub itself is NOT torn down. Its Run() goroutine keeps draining
// channels so connected clients receive the end screen state snapshot.
func (h *Hub) finishRoom(ctx context.Context, reason string, extra map[string]any) {
	// Cancel the round loop before touching anything else. This is called
	// from three paths — host grace expiry, normal game completion, and
	// error branches inside handleRoundCtrl — so the cancel must be the
	// single point of truth. runRounds' next <-ctx.Done() check returns
	// immediately, preventing ghost roundCtrl messages after game_ended.
	if h.roundsCancel != nil {
		h.roundsCancel()
		h.roundsCancel = nil
	}

	h.state = HubFinished

	if _, err := h.db.SetRoomState(ctx, db.SetRoomStateParams{
		ID: h.roomID, State: "finished",
	}); err != nil {
		h.log.Error("hub: set room state finished", "error", err)
	}

	data := map[string]any{
		"reason": reason,
	}
	for k, v := range extra {
		data[k] = v
	}
	h.broadcast(buildMessage("game_ended", data))
}

// endRoomInternal is the Run-goroutine half of EndRoom: broadcast
// room_closed, tear down every player connection, and move the hub to
// HubFinished. A killed room is terminal — no end screen, no reconnect.
//
// Ordering is subtle: we queue the room_closed message on each player's
// send channel, then close the send channel. writePump drains remaining
// messages first, then sees the closed channel, writes a CloseMessage,
// and its defer closes the underlying conn. That guarantees the client
// sees room_closed before the close frame. We then empty h.players so
// the follow-up unregister sent by readPump's read error is a no-op
// (handleUnregister early-returns when the player is gone).
func (h *Hub) endRoomInternal(reason string) {
	if h.roundsCancel != nil {
		h.roundsCancel()
		h.roundsCancel = nil
	}
	h.state = HubFinished

	msg := buildMessage("room_closed", map[string]string{"reason": reason})
	for _, p := range h.players {
		h.safeSend(p, msg)
	}
	for _, p := range h.players {
		close(p.send)
	}
	h.players = map[string]*connectedPlayer{}
}

func (h *Hub) handleGameMessage(ctx context.Context, msg playerMessage) {
	handler, ok := h.registry.Get(h.gameTypeSlug)
	if !ok {
		h.safeSend(msg.player, buildMessage("error", map[string]string{"code": "unknown_game_type"}))
		return
	}
	suffix := msg.msgType[len(h.gameTypeSlug)+1:]
	switch suffix {
	case "submit":
		if h.roundPhase != phaseSubmitting {
			h.safeSend(msg.player, buildMessage("error", map[string]string{
				"code": "submission_closed", "message": "Submission window is closed",
			}))
			return
		}
		if _, already := h.roundSubmissions[msg.player.userID]; already {
			h.safeSend(msg.player, buildMessage("error", map[string]string{
				"code": "already_submitted", "message": "You have already submitted",
			}))
			return
		}
		roundRef := Round{}
		if h.currentRound != nil {
			roundRef.ID = h.currentRound.ID
		}
		if err := handler.ValidateSubmission(roundRef, msg.data); err != nil {
			h.safeSend(msg.player, buildMessage("error", map[string]string{
				"code": "invalid_submission", "message": err.Error(),
			}))
			return
		}
		uid, _ := uuid.Parse(msg.player.userID)
		pgID := pgtype.UUID{Bytes: uid, Valid: true}
		var sub db.Submission
		var subErr error
		if msg.player.isGuest {
			sub, subErr = h.db.CreateGuestSubmission(ctx, db.CreateGuestSubmissionParams{
				RoundID:       h.currentRound.ID,
				GuestPlayerID: pgID,
				Payload:       msg.data,
			})
		} else {
			sub, subErr = h.db.CreateSubmission(ctx, db.CreateSubmissionParams{
				RoundID: h.currentRound.ID,
				UserID:  pgID,
				Payload: msg.data,
			})
		}
		if subErr != nil && h.log != nil {
			h.log.Error("hub: create submission", "error", subErr)
		}
		h.roundSubmissions[msg.player.userID] = Submission{
			ID:      sub.ID,
			UserID:  uid,
			Payload: msg.data,
		}
		h.safeSend(msg.player, buildMessage("submission_accepted", map[string]any{
			"submission_id": sub.ID.String(),
		}))
		if len(h.roundSubmissions) >= len(h.players) && h.allSubmittedCh != nil {
			close(h.allSubmittedCh)
			h.allSubmittedCh = nil
		}
		// Announce progress (no caption content) so every client can flip the
		// submitted-indicator on the player panel without waiting for the
		// round to close.
		h.broadcast(buildMessage("player_submitted", map[string]any{
			"user_id":   msg.player.userID,
			"player_id": msg.player.userID,
		}))

	case "vote":
		if h.roundPhase != phaseVoting {
			h.safeSend(msg.player, buildMessage("error", map[string]string{
				"code": "vote_closed", "message": "Voting window is closed",
			}))
			return
		}
		if _, already := h.roundVotes[msg.player.userID]; already {
			h.safeSend(msg.player, buildMessage("error", map[string]string{
				"code": "already_voted", "message": "You have already voted",
			}))
			return
		}
		var voteData struct {
			SubmissionID string `json:"submission_id"`
		}
		if err := json.Unmarshal(msg.data, &voteData); err != nil {
			h.safeSend(msg.player, buildMessage("error", map[string]string{
				"code": "invalid_vote", "message": "Invalid vote payload",
			}))
			return
		}
		submissionID, _ := uuid.Parse(voteData.SubmissionID)
		var targetSub *Submission
		for _, s := range h.roundSubmissions {
			if s.ID == submissionID {
				cp := s
				targetSub = &cp
				break
			}
		}
		if targetSub == nil {
			h.safeSend(msg.player, buildMessage("error", map[string]string{
				"code": "submission_not_found", "message": "Submission not found",
			}))
			return
		}
		voterID, _ := uuid.Parse(msg.player.userID)
		roundRef := Round{}
		if err := handler.ValidateVote(roundRef, *targetSub, voterID, msg.data); err != nil {
			h.safeSend(msg.player, buildMessage("error", map[string]string{
				"code": err.Error(), "message": "Invalid vote",
			}))
			return
		}
		pgVoterID := pgtype.UUID{Bytes: voterID, Valid: true}
		var voteErr error
		if msg.player.isGuest {
			_, voteErr = h.db.CreateGuestVote(ctx, db.CreateGuestVoteParams{
				SubmissionID: submissionID,
				GuestVoterID: pgVoterID,
				Value:        json.RawMessage(`{"points":1}`),
			})
		} else {
			_, voteErr = h.db.CreateVote(ctx, db.CreateVoteParams{
				SubmissionID: submissionID,
				VoterID:      pgVoterID,
				Value:        json.RawMessage(`{"points":1}`),
			})
		}
		if voteErr != nil && h.log != nil {
			h.log.Error("hub: create vote", "error", voteErr)
		}
		h.roundVotes[msg.player.userID] = Vote{
			SubmissionID: submissionID,
			VoterID:      voterID,
		}
		h.safeSend(msg.player, buildMessage("vote_accepted", nil))
		if len(h.roundVotes) >= len(h.players) && h.allVotedCh != nil {
			close(h.allVotedCh)
			h.allVotedCh = nil
		}

	default:
		h.safeSend(msg.player, buildMessage("error", map[string]string{"code": "unknown_message_type"}))
	}
}

// Helper: submissionsMapToSlice converts the round submissions map to a slice.
func submissionsMapToSlice(m map[string]Submission) []Submission {
	out := make([]Submission, 0, len(m))
	for _, s := range m {
		out = append(out, s)
	}
	return out
}

// Helper: votesMapToSlice converts the round votes map to a slice.
func votesMapToSlice(m map[string]Vote) []Vote {
	out := make([]Vote, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// Helper: nilIfEmpty returns nil if s is empty, otherwise returns s.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// readPump reads messages from a player's connection and forwards them to the hub.
func (h *Hub) readPump(p *connectedPlayer) {
	defer func() { h.unregister <- p }()
	p.conn.SetReadLimit(h.cfg.WSReadLimitBytes)

	for {
		_, raw, err := p.conn.ReadMessage()
		if err != nil {
			return
		}
		var m Message
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		h.incoming <- playerMessage{player: p, msgType: m.Type, data: m.Data}
	}
}

// writePump drains the player's send channel to the WebSocket connection.
func (h *Hub) writePump(p *connectedPlayer) {
	pingTicker := h.clock.NewTicker(h.cfg.WSPingInterval)
	defer func() {
		pingTicker.Stop()
		p.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-p.send:
			if !ok {
				p.conn.WriteMessage(websocket.CloseMessage, nil) //nolint:errcheck
				return
			}
			if err := p.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-pingTicker.C():
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// safeSend sends msg to p.send, recovering from a panic if the channel is closed.
func (h *Hub) safeSend(p *connectedPlayer, msg []byte) {
	defer func() {
		if r := recover(); r != nil {
			if h.log != nil {
				h.log.Warn("hub: send to closed channel", "user_id", p.userID)
			}
		}
	}()
	select {
	case p.send <- msg:
	default:
		// Slow consumer: drop message, increment counter, and force-close
		// the connection. Closing the underlying conn makes readPump's
		// next ReadMessage error out, which routes the player through the
		// unregister path and into the normal reconnect-grace window. A
		// silent drop would leave the server and client with divergent
		// state (finding 4.I in the 2026-04-10 review).
		msgType := extractType(msg)
		middleware.WSMessagesDroppedTotal.WithLabelValues(msgType).Inc()
		if h.log != nil {
			h.log.Warn("hub: slow consumer — closing connection",
				"user_id", p.userID, "room", h.roomCode, "msg_type", msgType)
		}
		// Run the close in a goroutine: safeSend is invoked from the Run
		// loop, and websocket.Conn.Close acquires an internal write lock
		// that may briefly block if writePump is mid-write.
		go p.conn.Close()
	}
}

// extractType reads the "type" field from a JSON message for logging.
func extractType(msg []byte) string {
	var m struct{ Type string `json:"type"` }
	if err := json.Unmarshal(msg, &m); err != nil {
		return "unknown"
	}
	return m.Type
}

func (h *Hub) broadcast(msg []byte) {
	for _, p := range h.players {
		if !p.reconnecting {
			h.safeSend(p, msg)
		}
	}
}

func (h *Hub) sendTo(p *connectedPlayer, msg []byte) {
	h.safeSend(p, msg)
}

// KickPlayer sends a kick message to a target player, removing them from the hub.
// It is safe to call from outside the Run goroutine (sends via the incoming
// channel). The context bounds the send: if h.incoming is saturated because
// the Run loop is stuck on a slow DB write, the caller's ctx deadline (the
// HTTP request ctx in practice) interrupts the send instead of leaking the
// goroutine. Finding 4.B in the 2026-04-10 review. Returns ctx.Err() when
// the context fires; a nil return means the message was enqueued.
func (h *Hub) KickPlayer(ctx context.Context, userID string) error {
	data, _ := json.Marshal(map[string]string{"target_user_id": userID})
	select {
	case h.incoming <- playerMessage{
		player:  &connectedPlayer{userID: "system"},
		msgType: "system:kick",
		data:    data,
	}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// EndRoom instructs the hub to terminate the room: broadcast room_closed to
// every player and close every connection. A killed room is terminal. Safe
// to call from outside the Run goroutine; routes through h.incoming so all
// state mutations happen in Run(). Same context-bounded send pattern as
// KickPlayer (finding 4.B). Returns ctx.Err() when the context fires; a
// nil return means the termination is in flight.
func (h *Hub) EndRoom(ctx context.Context, reason string) error {
	data, _ := json.Marshal(map[string]string{"reason": reason})
	select {
	case h.incoming <- playerMessage{
		player:  &connectedPlayer{userID: "system"},
		msgType: "system:end_room",
		data:    data,
	}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hub) buildRoomState(recipientUserID string) map[string]any {
	players := make([]map[string]any, 0, len(h.players))
	for _, p := range h.players {
		players = append(players, map[string]any{
			"user_id":      p.userID,
			"username":     p.username,
			"player_id":    p.userID,
			"display_name": p.username,
			"is_guest":     p.isGuest,
		})
	}
	out := map[string]any{
		"state":   string(h.state),
		"players": players,
		"host_id": h.hostUserID,
	}
	// Mid-round rehydration: a client that refreshes or joins late needs
	// enough state to render the in-flight phase. Only emitted when a round
	// is actually in progress — lobby and finished rooms skip this entirely.
	if h.state == HubPlaying && h.roundPhase != phaseIdle {
		out["phase"] = roundPhaseName(h.roundPhase)
		out["round_number"] = h.roundNum
		out["round_paused"] = h.roundPausedFlag
		out["item"] = map[string]any{
			"payload":   h.roundItemPayload,
			"media_url": nilIfEmpty(h.roundMediaURL),
		}
		// Re-key the recipient's own submission so the vote UI can tag it
		// with "You" even after a refresh. Captions are not a reliable key —
		// two players can submit identical text.
		if own, ok := h.roundSubmissions[recipientUserID]; ok {
			out["own_submission_id"] = own.ID.String()
		}
		switch h.roundPhase {
		case phaseSubmitting:
			out["ends_at"] = h.roundSubmitEndsAt.UTC().Format(time.RFC3339)
			out["duration_seconds"] = int(h.roundSubmitDur.Seconds())
		case phaseVoting:
			if h.submissionsShown != nil {
				out["submissions_shown"] = json.RawMessage(h.submissionsShown)
			}
			out["voting_ends_at"] = h.roundVoteEndsAt.UTC().Format(time.RFC3339)
			out["voting_duration_seconds"] = int(h.roundVoteDur.Seconds())
		case phaseResults:
			if h.resultsShown != nil {
				out["results"] = json.RawMessage(h.resultsShown)
			}
			if h.resultsLeaderboard != nil {
				out["leaderboard"] = h.resultsLeaderboard
			}
		}
	}
	return out
}

func roundPhaseName(p hubRoundPhase) string {
	switch p {
	case phaseSubmitting:
		return "submitting"
	case phaseVoting:
		return "voting"
	case phaseResults:
		return "results"
	default:
		return "idle"
	}
}

func buildMessage(msgType string, data any) []byte {
	payload := map[string]any{"type": msgType}
	if data != nil {
		payload["data"] = data
	}
	b, _ := json.Marshal(payload)
	return b
}

func writeWS(conn *websocket.Conn, msg []byte) {
	conn.WriteMessage(websocket.TextMessage, msg) //nolint:errcheck
}
