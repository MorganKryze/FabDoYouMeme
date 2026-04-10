// backend/internal/game/hub.go
package game

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

// HubState tracks in-memory room lifecycle.
type HubState string

const (
	HubLobby   HubState = "lobby"
	HubPlaying HubState = "playing"
)

// graceExpiredMsg is sent to the Run goroutine when a reconnect grace window expires.
type graceExpiredMsg struct {
	userID   string
	username string
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
	players map[string]*connectedPlayer // userID → player

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
}

// connectedPlayer is hub-internal player state.
type connectedPlayer struct {
	userID       string
	username     string
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

// Join is called from the HTTP handler (outside Run goroutine) to add a new WS connection.
// It blocks until the player is registered or the context is cancelled.
func (h *Hub) Join(ctx context.Context, userID, username string, conn *websocket.Conn) {
	p := &connectedPlayer{
		userID:   userID,
		username: username,
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
		p.reconnecting = false
		h.sendTo(p, buildMessage("room_state", h.buildRoomState()))
		h.broadcast(buildMessage("player_joined", map[string]string{
			"user_id": p.userID, "username": p.username,
		}))
		go h.readPump(p)
		go h.writePump(p)
		return
	}

	if h.state != HubLobby {
		writeWS(p.conn, buildMessage("error", map[string]string{
			"code": "game_already_started", "message": "Game is already in progress",
		}))
		p.conn.Close()
		return
	}

	h.players[p.userID] = p
	h.broadcast(buildMessage("player_joined", map[string]string{
		"user_id": p.userID, "username": p.username,
	}))
	go h.readPump(p)
	go h.writePump(p)
}

func (h *Hub) handleUnregister(p *connectedPlayer) {
	if _, ok := h.players[p.userID]; !ok {
		return
	}
	p.reconnecting = true
	h.broadcast(buildMessage("reconnecting", map[string]string{
		"user_id": p.userID, "username": p.username,
	}))
	// AfterFunc avoids a goroutine leak: the fake/real timer fires once and
	// exits. All state changes still happen inside Run() because the send
	// is into a buffered channel read by Run().
	userID, username := p.userID, p.username
	grace := h.cfg.ReconnectGraceWindow
	h.clock.AfterFunc(grace, func() {
		h.graceExpired <- graceExpiredMsg{userID: userID, username: username}
	})
}

func (h *Hub) handleGraceExpired(ctx context.Context, msg graceExpiredMsg) {
	cp, ok := h.players[msg.userID]
	if !ok || !cp.reconnecting {
		return // player reconnected in time — ignore
	}
	delete(h.players, msg.userID)
	h.broadcast(buildMessage("player_left", map[string]string{
		"user_id": msg.userID, "username": msg.username,
	}))
	if msg.userID == h.hostUserID && h.state == HubPlaying {
		h.finishRoom(ctx, "host_disconnected")
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

	case "ping":
		h.sendTo(msg.player, buildMessage("pong", nil))

	case "system:kick":
		var d struct {
			TargetUserID string `json:"target_user_id"`
		}
		json.Unmarshal(msg.data, &d) //nolint:errcheck
		if p, ok := h.players[d.TargetUserID]; ok {
			h.safeSend(p, buildMessage("player_kicked", map[string]string{"user_id": d.TargetUserID}))
			delete(h.players, d.TargetUserID)
			h.broadcast(buildMessage("player_kicked", map[string]string{"user_id": d.TargetUserID}))
		}

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
				if _, err := h.db.UpdatePlayerScore(ctx, db.UpdatePlayerScoreParams{
					RoomID: h.roomID,
					UserID: playerID,
					Score:  int32(pts),
				}); err != nil && h.log != nil {
					h.log.Error("hub: update player score", "error", err)
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
		h.finishRoom(ctx, "game_complete")
		h.broadcast(buildMessage("game_ended", map[string]any{
			"reason":      "game_complete",
			"leaderboard": leaderboard,
		}))
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
		json.Unmarshal(room.Config, &cfg) //nolint:errcheck
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

func (h *Hub) finishRoom(ctx context.Context, reason string) {
	// Cancel the round loop before touching anything else. This is called
	// from three paths — host grace expiry, normal game completion, and
	// error branches inside handleRoundCtrl — so the cancel must be the
	// single point of truth. runRounds' next <-ctx.Done() check returns
	// immediately, preventing ghost roundCtrl messages after game_ended.
	if h.roundsCancel != nil {
		h.roundsCancel()
		h.roundsCancel = nil
	}
	if _, err := h.db.SetRoomState(ctx, db.SetRoomStateParams{
		ID: h.roomID, State: "finished",
	}); err != nil {
		h.log.Error("hub: set room state finished", "error", err)
	}
	h.broadcast(buildMessage("game_ended", map[string]string{"reason": reason}))
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
		sub, err := h.db.CreateSubmission(ctx, db.CreateSubmissionParams{
			RoundID: h.currentRound.ID,
			UserID:  uid,
			Payload: msg.data,
		})
		if err != nil && h.log != nil {
			h.log.Error("hub: create submission", "error", err)
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
		_, err := h.db.CreateVote(ctx, db.CreateVoteParams{
			SubmissionID: submissionID,
			VoterID:      voterID,
			Value:        json.RawMessage(`{"points":1}`),
		})
		if err != nil && h.log != nil {
			h.log.Error("hub: create vote", "error", err)
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
		// Slow consumer: drop message and log (A2-M1)
		if h.log != nil {
			h.log.Warn("hub: dropped message for slow consumer", "user_id", p.userID,
				"msg_type", extractType(msg))
		}
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
// It is safe to call from outside the Run goroutine (sends via the incoming channel).
func (h *Hub) KickPlayer(userID string) {
	data, _ := json.Marshal(map[string]string{"target_user_id": userID})
	h.incoming <- playerMessage{
		player:  &connectedPlayer{userID: "system"},
		msgType: "system:kick",
		data:    data,
	}
}

func (h *Hub) buildRoomState() map[string]any {
	players := make([]map[string]string, 0, len(h.players))
	for _, p := range h.players {
		players = append(players, map[string]string{
			"user_id": p.userID, "username": p.username,
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
