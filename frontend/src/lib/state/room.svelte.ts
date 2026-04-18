import { goto } from '$app/navigation';
import type { GameType, Player, LeaderboardEntry, Submission, Round, WsMessage } from '$lib/api/types';
import { toast } from './toast.svelte';
import { ws } from './ws.svelte';

type RoomPhase = 'idle' | 'countdown' | 'submitting' | 'voting' | 'results';
type RoomStatus = 'lobby' | 'playing' | 'finished';

export class RoomState {
  code = $state<string | null>(null);
  gameType = $state<GameType | null>(null);
  state = $state<RoomStatus>('lobby');
  players = $state<Player[]>([]);
  currentRound = $state<Round | null>(null);
  phase = $state<RoomPhase>('idle');
  submissions = $state<Submission[]>([]);
  leaderboard = $state<LeaderboardEntry[]>([]);
  endReason = $state<string | null>(null);
  hostUserId = $state<string | null>(null);
  votingEndsAt = $state<string | null>(null);
  votingDurationSeconds = $state<number | null>(null);
  // Results phase auto-advance deadline (server-paced only; null in host-paced
  // mode, where the host clicks "Next Round" instead of a timer expiring).
  resultsEndsAt = $state<string | null>(null);
  ownSubmissionId = $state<string | null>(null);
  roundPaused = $state(false);
  roundPausedSince = $state<number | null>(null); // timestamp (ms) when pause started

  hasSubmitted = $state(false);
  hasVoted = $state(false);
  // IDs of players (users or guests) who have submitted in the current round.
  // Cleared on round_started; populated by player_submitted broadcasts so the
  // player panel can show per-player progress during the submission phase.
  submittedPlayerIds = $state<Set<string>>(new Set());

  // Skip-turn (joker) state — jokersRemaining drops as the current player
  // consumes jokers; own* flip to true for the current player's own action
  // and are consumed by the SubmitForm / VoteForm to render the locked state.
  // skippedSubmitIds / skippedVoteIds are used by the player rail to flip
  // other players' "done" chip identically to submittedPlayerIds.
  jokersRemaining = $state<number | null>(null);
  ownSkippedSubmit = $state(false);
  ownSkippedVote = $state(false);
  skippedSubmitIds = $state<Set<string>>(new Set());
  skippedVoteIds = $state<Set<string>>(new Set());
  // The client's own user_id, populated from the room_state snapshot so the
  // WS handlers can check broadcasts against "is this me?" without reaching
  // into a separate session/page store.
  ownUserId = $state<string | null>(null);

  // Pre-round countdown shown as a full-screen overlay. Driven imperatively
  // from the 'game_started' handler rather than observed via a reactive
  // phase effect: `round_started` lands within a single Svelte flush of
  // `game_started`, so a rAF-scheduled effect would only ever see the
  // final 'submitting' phase and skip the overlay entirely.
  countdown = $state<number | null>(null);
  #countdownInterval: ReturnType<typeof setInterval> | null = null;

  init(data: {
    code: string;
    game_type?: GameType | null;
    state: string;
    players?: Player[];
    host_id?: string | null;
    own_user_id?: string | null;
  }): void {
    this.code = data.code;
    this.gameType = data.game_type ?? null;
    this.state = data.state as RoomStatus;
    this.hostUserId = data.host_id ?? null;
    this.ownUserId = data.own_user_id ?? null;
    this.players = (data.players ?? []).map(p => ({
      ...p,
      is_host: p.user_id === this.hostUserId
    }));
  }

  handleMessage(msg: WsMessage) {
    switch (msg.type) {
      case 'player_joined': {
        const d = msg.data as Player;
        const existing = this.players.find(p => p.user_id === d.user_id);
        if (existing) {
          // Reconnect: the server re-sends player_joined when a player returns
          // within the grace window. Flip their connected flag so the rail pill
          // returns to "Online" without waiting for the next full snapshot.
          this.players = this.players.map(p =>
            p.user_id === d.user_id ? { ...p, connected: true } : p
          );
        } else {
          this.players = [
            ...this.players,
            { ...d, connected: d.connected ?? true, is_host: d.user_id === this.hostUserId },
          ];
        }
        break;
      }
      case 'reconnecting': {
        const d = msg.data as Player;
        this.players = this.players.map(p =>
          p.user_id === d.user_id ? { ...p, connected: false } : p
        );
        break;
      }
      case 'player_left':
      case 'player_kicked': {
        const d = msg.data as Player;
        this.players = this.players.filter(p => p.user_id !== d.user_id);
        break;
      }
      case 'game_started':
        this.state = 'playing';
        this.phase = 'countdown';
        this.startCountdown();
        break;
      case 'round_started':
        this.currentRound = msg.data as Round;
        this.phase = 'submitting';
        this.submissions = [];
        this.hasSubmitted = false;
        this.hasVoted = false;
        this.submittedPlayerIds = new Set();
        this.votingEndsAt = null;
        this.votingDurationSeconds = null;
        this.resultsEndsAt = null;
        this.ownSubmissionId = null;
        this.roundPaused = false;
        this.roundPausedSince = null;
        this.ownSkippedSubmit = false;
        this.ownSkippedVote = false;
        this.skippedSubmitIds = new Set();
        this.skippedVoteIds = new Set();
        break;
      case 'submission_accepted': {
        const d = msg.data as { submission_id?: string } | null;
        this.ownSubmissionId = d?.submission_id ?? null;
        this.hasSubmitted = true;
        break;
      }
      case 'player_submitted': {
        const d = msg.data as { user_id?: string; player_id?: string };
        const id = d.player_id ?? d.user_id;
        if (id) {
          // Reassign so Svelte picks up the mutation — $state(Set) tracks
          // identity, not internal set members.
          this.submittedPlayerIds = new Set([...this.submittedPlayerIds, id]);
        }
        break;
      }
      case 'player_skipped_submit': {
        const d = msg.data as {
          user_id?: string;
          player_id?: string;
          jokers_remaining?: number;
        };
        const id = d.player_id ?? d.user_id;
        if (id) {
          this.skippedSubmitIds = new Set([...this.skippedSubmitIds, id]);
        }
        if (id && this.ownUserId && id === this.ownUserId) {
          this.ownSkippedSubmit = true;
          if (typeof d.jokers_remaining === 'number') {
            this.jokersRemaining = d.jokers_remaining;
          }
        }
        break;
      }
      case 'player_skipped_vote': {
        const d = msg.data as { user_id?: string; player_id?: string };
        const id = d.player_id ?? d.user_id;
        if (id) {
          this.skippedVoteIds = new Set([...this.skippedVoteIds, id]);
        }
        if (id && this.ownUserId && id === this.ownUserId) {
          this.ownSkippedVote = true;
        }
        break;
      }
      case 'submissions_closed': {
        const d = msg.data as {
          submissions_shown?: { submissions?: Submission[] };
          ends_at?: string;
          duration_seconds?: number;
        };
        this.submissions = d.submissions_shown?.submissions ?? [];
        this.votingEndsAt = d.ends_at ?? null;
        this.votingDurationSeconds = d.duration_seconds ?? null;
        this.phase = 'voting';
        break;
      }
      case 'vote_results': {
        const d = msg.data as {
          results?: { submissions?: Submission[] };
          leaderboard?: LeaderboardEntry[];
          next_round_at?: string;
        };
        this.submissions = d.results?.submissions ?? [];
        this.leaderboard = d.leaderboard ?? [];
        this.resultsEndsAt = d.next_round_at ?? null;
        this.phase = 'results';
        break;
      }
      case 'round_paused':
        this.roundPaused = true;
        this.roundPausedSince = Date.now();
        break;
      case 'round_resumed': {
        const d = msg.data as { ends_at?: string };
        this.roundPaused = false;
        this.roundPausedSince = null;
        // Update whichever deadline is currently active.
        if (this.phase === 'voting' && d.ends_at) {
          this.votingEndsAt = d.ends_at;
        } else if (this.phase === 'submitting' && d.ends_at && this.currentRound) {
          this.currentRound = { ...this.currentRound, ends_at: d.ends_at };
        }
        break;
      }
      case 'game_ended': {
        const d = msg.data as {
          reason: string;
          leaderboard: LeaderboardEntry[];
        };
        this.state = 'finished';
        this.phase = 'idle';
        this.endReason = d.reason;
        this.leaderboard = d.leaderboard ?? [];
        break;
      }
      case 'room_state': {
        const d = msg.data as {
          state: RoomStatus;
          players: Player[];
          host_id?: string;
          // Mid-round rehydration fields (present only when the hub is mid-game).
          phase?: RoomPhase;
          round_number?: number;
          round_paused?: boolean;
          item?: Round['item'];
          ends_at?: string;
          duration_seconds?: number;
          submissions_shown?: { submissions?: Submission[] };
          voting_ends_at?: string;
          voting_duration_seconds?: number;
          results_ends_at?: string;
          results?: { submissions?: Submission[] };
          leaderboard?: LeaderboardEntry[];
          own_submission_id?: string;
          my_jokers_remaining?: number;
          skipped_submit?: boolean;
          skipped_vote?: boolean;
          own_user_id?: string;
          submitted_player_ids?: string[];
          skipped_submit_ids?: string[];
          skipped_vote_ids?: string[];
        };
        this.state = d.state;
        if (d.host_id) this.hostUserId = d.host_id;
        this.players = (d.players ?? []).map(p => ({
          ...p,
          is_host: p.user_id === this.hostUserId
        }));
        // Rehydrate the in-flight round so a page refresh doesn't wipe the
        // stage. The snapshot arrives in a single message, so we apply all
        // the phase-specific fields at once instead of waiting for the next
        // round-lifecycle broadcast (which may be minutes away in results).
        if (d.phase && d.phase !== 'idle' && d.item) {
          this.phase = d.phase;
          this.currentRound = {
            round_number: d.round_number ?? 0,
            ends_at: d.ends_at ?? '',
            duration_seconds: d.duration_seconds ?? 0,
            item: d.item,
          };
          this.roundPaused = d.round_paused ?? false;
          this.roundPausedSince = this.roundPaused ? Date.now() : null;
          if (d.phase === 'voting' || d.phase === 'results') {
            this.submissions = d.submissions_shown?.submissions ?? this.submissions;
            this.votingEndsAt = d.voting_ends_at ?? null;
            this.votingDurationSeconds = d.voting_duration_seconds ?? null;
          }
          if (d.phase === 'results') {
            this.submissions = d.results?.submissions ?? this.submissions;
            this.leaderboard = d.leaderboard ?? [];
            this.resultsEndsAt = d.results_ends_at ?? null;
          }
          if (d.own_submission_id) {
            this.ownSubmissionId = d.own_submission_id;
            this.hasSubmitted = true;
          }
          if (typeof d.my_jokers_remaining === 'number') {
            this.jokersRemaining = d.my_jokers_remaining;
          }
          if (d.skipped_submit) this.ownSkippedSubmit = true;
          if (d.skipped_vote) this.ownSkippedVote = true;
          // Seed the per-player progress sets so the rail renders every peer's
          // chip after a refresh or late join, not just our own. Reassign the
          // $state Set so Svelte picks up the change.
          if (d.submitted_player_ids) {
            this.submittedPlayerIds = new Set(d.submitted_player_ids);
          }
          if (d.skipped_submit_ids) {
            this.skippedSubmitIds = new Set(d.skipped_submit_ids);
          }
          if (d.skipped_vote_ids) {
            this.skippedVoteIds = new Set(d.skipped_vote_ids);
          }
        }
        break;
      }
      case 'error': {
        const d = msg.data as { code: string; message?: string };
        toast.show(d.message ?? d.code ?? 'An error occurred', 'error');
        if (d.code === 'submission_closed' || d.code === 'already_submitted') {
          this.hasSubmitted = true;
        }
        if (d.code === 'vote_closed' || d.code === 'already_voted') {
          this.hasVoted = true;
        }
        break;
      }
      case 'server_restarting':
        toast.show('Server restarting — reconnecting…', 'warning');
        break;

      case 'room_closed': {
        const d = msg.data as { reason?: string };
        const reason = d?.reason ?? 'ended_by_host';
        const message =
          reason === 'ended_by_admin'
            ? 'An admin ended this room'
            : 'The host ended this room';
        ws.disconnect();
        toast.show(message, 'warning');
        this.reset();
        goto('/');
        break;
      }
    }
  }

  startCountdown() {
    if (this.#countdownInterval) {
      clearInterval(this.#countdownInterval);
      this.#countdownInterval = null;
    }
    this.countdown = 3;
    this.#countdownInterval = setInterval(() => {
      if (this.countdown === null) return;
      if (this.countdown <= 0) {
        if (this.#countdownInterval) {
          clearInterval(this.#countdownInterval);
          this.#countdownInterval = null;
        }
        this.countdown = null;
      } else {
        this.countdown -= 1;
      }
    }, 1000);
  }

  reset() {
    this.code = null;
    this.gameType = null;
    this.state = 'lobby';
    this.players = [];
    this.currentRound = null;
    this.phase = 'idle';
    this.submissions = [];
    this.leaderboard = [];
    this.endReason = null;
    this.hostUserId = null;
    this.votingEndsAt = null;
    this.votingDurationSeconds = null;
    this.resultsEndsAt = null;
    this.ownSubmissionId = null;
    this.roundPaused = false;
    this.roundPausedSince = null;
    this.hasSubmitted = false;
    this.hasVoted = false;
    this.submittedPlayerIds = new Set();
    this.jokersRemaining = null;
    this.ownSkippedSubmit = false;
    this.ownSkippedVote = false;
    this.skippedSubmitIds = new Set();
    this.skippedVoteIds = new Set();
    this.ownUserId = null;
    if (this.#countdownInterval) {
      clearInterval(this.#countdownInterval);
      this.#countdownInterval = null;
    }
    this.countdown = null;
  }
}

export const room = new RoomState();
