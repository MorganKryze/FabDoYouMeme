// backend/internal/game/hub.go
package game

import (
	"context"
	"encoding/json"
	"log/slog"
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
	HubFinished HubState = "finished" // game ended; awaiting rematch or window expiry
)

// RematchWindowDuration is how long a finished room remains resurrectable.
// Picked to be long enough for a host to re-read the end screen and decide,
// but short enough that zombie hubs don't pile up on a live server. The
// plan's B2 layer specifies 5 minutes.
const RematchWindowDuration = 5 * time.Minute

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
)

// roundCtrlMsg is the interface for all round lifecycle control messages.
type roundCtrlMsg interface{ roundCtrl() }

// roundCtrlAdvance signals runRounds to move to the next round (host-triggered).
type roundCtrlAdvance struct{}

func (roundCtrlAdvance) roundCtrl() {}

// roundCtrlStartRound signals the hub to broadcast round_started.
type roundCtrlStartRound struct {
	roundID  uuid.UUID
	itemID   uuid.UUID
	payload  json.RawMessage
	mediaURL string
	endsAt   time.Time
	duration time.Duration
}

func (roundCtrlStartRound) roundCtrl() {}

// roundCtrlCloseSubmissions signals the hub to broadcast submissions_closed.
type roundCtrlCloseSubmissions struct {
	votingEndsAt time.Time
	duration     time.Duration
}

func (roundCtrlCloseSubmissions) roundCtrl() {}

// roundCtrlCloseVoting signals the hub to tally votes and broadcast vote_results.
type roundCtrlCloseVoting struct{}

func (roundCtrlCloseVoting) roundCtrl() {}

// roundCtrlEndGame signals the hub to finish the room and broadcast game_ended.
type roundCtrlEndGame struct{}

func (roundCtrlEndGame) roundCtrl() {}

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

	// Round state — only accessed from Run() goroutine
	roundPhase       hubRoundPhase
	roundNum         int
	currentRound     *db.Round
	roundSubmissions map[string]Submission // userID → submission
	roundVotes       map[string]Vote       // userID → vote

	// roundsCancel aborts the runRounds goroutine spawned in startGame.
	// It is set by startGame and cleared by finishRoom. Accessed only from
	// the Run() goroutine, so no mutex is needed despite the function type.
	roundsCancel context.CancelFunc

	// rematchWindowExpires marks the deadline after which rematch_request is
	// rejected. Set by finishRoom, cleared by the rematch handler. Zero value
	// means the hub is not in a rematchable window. Accessed only from the
	// Run() goroutine — no mutex needed.
	rematchWindowExpires time.Time
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
			h.handleRegister(p)

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

func (h *Hub) handleRegister(p *connectedPlayer) {
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
		h.sendTo(p, buildMessage("room_state", h.buildRoomState()))
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
	// HubFinished: new joins are permitted so the host can reconnect to
	// request a rematch and players who bounced can return before the
	// rematch window closes. If no rematch happens, these connections
	// simply drain when the hub eventually shuts down.

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
	if _, ok := h.players[p.userID]; !ok {
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
	if msg.userID == h.hostUserID && h.state == HubPlaying {
		h.finishRoom(ctx, "host_disconnected", nil)
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
			h.safeSend(msg.player, buildMessage("room_state", h.buildRoomState()))
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

	case "rematch_request":
		h.handleRematchRequest(ctx, msg)

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
		// host-triggered advance — currently a no-op since runRounds owns timing
		_ = c

	case roundCtrlStartRound:
		h.roundPhase = phaseSubmitting
		h.roundNum++
		h.roundSubmissions = make(map[string]Submission)
		h.roundVotes = make(map[string]Vote)
		h.currentRound = &db.Round{ID: c.roundID}
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
		h.roundPhase = phaseVoting
		submissions := submissionsMapToSlice(h.roundSubmissions)
		handler, ok := h.registry.Get(h.gameTypeSlug)
		if ok {
			shown, err := handler.BuildSubmissionsShownPayload(submissions)
			if err == nil {
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
		h.roundPhase = phaseIdle
		handler, ok := h.registry.Get(h.gameTypeSlug)
		if ok {
			submissions := submissionsMapToSlice(h.roundSubmissions)
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
				h.broadcast(buildMessage("vote_results", map[string]any{
					"results":     json.RawMessage(resultsPayload),
					"leaderboard": leaderboard,
				}))
				return
			}
		}
		h.broadcast(buildMessage("vote_results", nil))

	case roundCtrlEndGame:
		leaderboard, _ := h.db.GetRoomLeaderboard(ctx, h.roomID)
		h.finishRoom(ctx, "game_complete", map[string]any{
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

func (h *Hub) runRounds(ctx context.Context) {
	room, err := h.db.GetRoomByID(ctx, h.roomID)
	if err != nil {
		if h.log != nil {
			h.log.Error("runRounds: get room", "error", err)
		}
		h.sendRoundCtrl(ctx, roundCtrlEndGame{})
		return
	}

	var cfg struct {
		RoundCount            int `json:"round_count"`
		RoundDurationSeconds  int `json:"round_duration_seconds"`
		VotingDurationSeconds int `json:"voting_duration_seconds"`
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
			h.sendRoundCtrl(ctx, roundCtrlEndGame{})
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
			h.sendRoundCtrl(ctx, roundCtrlEndGame{})
			return
		}

		mediaURL := ""
		if item.MediaKey != nil {
			mediaURL = *item.MediaKey
		}
		endsAt := h.clock.Now().Add(roundDuration)
		if !h.sendRoundCtrl(ctx, roundCtrlStartRound{
			roundID:  dbRound.ID,
			itemID:   item.ID,
			payload:  item.Payload,
			mediaURL: mediaURL,
			endsAt:   endsAt,
			duration: roundDuration,
		}) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-h.clock.After(roundDuration):
		}

		votingEndsAt := h.clock.Now().Add(votingDuration)
		if !h.sendRoundCtrl(ctx, roundCtrlCloseSubmissions{
			votingEndsAt: votingEndsAt,
			duration:     votingDuration,
		}) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-h.clock.After(votingDuration):
		}

		if !h.sendRoundCtrl(ctx, roundCtrlCloseVoting{}) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-h.clock.After(3 * time.Second):
		}
	}

	h.sendRoundCtrl(ctx, roundCtrlEndGame{})
}

// finishRoom transitions the hub and the persisted room row to "finished",
// stamps a rematch window, and broadcasts game_ended. The extra map is
// merged into the broadcast payload — callers use it to attach the final
// leaderboard when they have one handy.
//
// The hub itself is NOT torn down. Its Run() goroutine keeps draining
// channels so that host players can still send rematch_request during the
// window. Zombie hubs that nobody rematches simply stay idle until the
// server restarts; this matches pre-B2 behaviour and is acceptable given
// the single-machine topology.
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
	h.rematchWindowExpires = h.clock.Now().Add(RematchWindowDuration)
	rematchExpiresPG := pgtype.Timestamptz{Time: h.rematchWindowExpires, Valid: true}

	if _, err := h.db.SetRoomState(ctx, db.SetRoomStateParams{
		ID: h.roomID, State: "finished",
	}); err != nil {
		h.log.Error("hub: set room state finished", "error", err)
	}
	if _, err := h.db.SetRematchWindow(ctx, db.SetRematchWindowParams{
		ID:                     h.roomID,
		RematchWindowExpiresAt: rematchExpiresPG,
	}); err != nil {
		h.log.Error("hub: set rematch window", "error", err)
	}

	data := map[string]any{
		"reason":                    reason,
		"rematch_window_expires_at": h.rematchWindowExpires.UTC().Format(time.RFC3339),
	}
	for k, v := range extra {
		data[k] = v
	}
	h.broadcast(buildMessage("game_ended", data))
}

// endRoomInternal is the Run-goroutine half of EndRoom: broadcast
// room_closed, tear down every player connection, and move the hub to
// HubFinished. The rematch window is explicitly NOT set — a killed room
// is terminal.
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
	h.rematchWindowExpires = time.Time{}

	msg := buildMessage("room_closed", map[string]string{"reason": reason})
	for _, p := range h.players {
		h.safeSend(p, msg)
	}
	for _, p := range h.players {
		close(p.send)
	}
	h.players = map[string]*connectedPlayer{}
}

// handleRematchRequest runs inside the Run goroutine when the host asks to
// resurrect the room. The DB gate in ResurrectRoom is the authoritative
// check (state=finished, within window, host match); the in-memory pre-checks
// here exist to produce friendlier error codes to the client.
func (h *Hub) handleRematchRequest(ctx context.Context, msg playerMessage) {
	if msg.player.userID != h.hostUserID {
		h.safeSend(msg.player, buildMessage("error", map[string]string{
			"code": "not_host", "message": "Only the host can request a rematch",
		}))
		return
	}
	if h.state != HubFinished {
		h.safeSend(msg.player, buildMessage("error", map[string]string{
			"code": "rematch_not_available", "message": "Rematch is only available after the game ends",
		}))
		return
	}
	if h.rematchWindowExpires.IsZero() || !h.clock.Now().Before(h.rematchWindowExpires) {
		h.safeSend(msg.player, buildMessage("error", map[string]string{
			"code": "rematch_window_expired", "message": "Rematch window has closed",
		}))
		return
	}

	hostUUID, err := uuid.Parse(h.hostUserID)
	if err != nil {
		h.safeSend(msg.player, buildMessage("error", map[string]string{
			"code": "internal_error", "message": "Host identity unparseable",
		}))
		return
	}
	if _, err := h.db.ResurrectRoom(ctx, db.ResurrectRoomParams{
		ID:     h.roomID,
		HostID: pgtype.UUID{Bytes: hostUUID, Valid: true},
	}); err != nil {
		// No rows returned means the DB-level gate failed (window expired
		// between the memory check and this query, host_id mismatch, etc.).
		// Treat all DB failures here as "window expired" — the client's
		// next move is the same: abandon rematch and return to home.
		h.safeSend(msg.player, buildMessage("error", map[string]string{
			"code": "rematch_window_expired", "message": "Rematch window has closed",
		}))
		return
	}
	if err := h.db.ResetRoomPlayerScores(ctx, h.roomID); err != nil && h.log != nil {
		h.log.Error("hub: reset player scores for rematch", "error", err)
	}

	// Reset in-memory round state. Players map is preserved — everyone who
	// is still connected stays connected. The meme_caption handler itself
	// is stateless, so there is no handler-level reset to perform.
	h.state = HubLobby
	h.roundPhase = phaseIdle
	h.roundNum = 0
	h.currentRound = nil
	h.roundSubmissions = make(map[string]Submission)
	h.roundVotes = make(map[string]Vote)
	h.rematchWindowExpires = time.Time{}

	h.broadcast(buildMessage("rematch_started", map[string]any{
		"room_state": h.buildRoomState(),
	}))
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
		h.safeSend(msg.player, buildMessage("submission_accepted", nil))

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
// every player and close every connection. Unlike finishRoom, there is no
// rematch window — a killed room is terminal. Safe to call from outside the
// Run goroutine; routes through h.incoming so all state mutations happen in
// Run(). Same context-bounded send pattern as KickPlayer (finding 4.B).
// Returns ctx.Err() when the context fires; a nil return means the
// termination is in flight.
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

func (h *Hub) buildRoomState() map[string]any {
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
	return map[string]any{
		"state":   string(h.state),
		"players": players,
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
