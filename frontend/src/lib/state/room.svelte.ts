import type { GameType, Player, LeaderboardEntry, Submission, Round, WsMessage } from '$lib/api/types';
import { toast } from './toast.svelte';

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

  hasSubmitted = $state(false);
  hasVoted = $state(false);

  init(data: { code: string; game_type: GameType; state: string; players: Player[]; host_id?: string }): void {
    this.code = data.code;
    this.gameType = data.game_type;
    this.state = data.state as RoomStatus;
    this.hostUserId = data.host_id ?? null;
    this.players = (data.players ?? []).map(p => ({
      ...p,
      is_host: p.user_id === this.hostUserId
    }));
  }

  handleMessage(msg: WsMessage) {
    switch (msg.type) {
      case 'player_joined': {
        const d = msg.data as Player;
        if (!this.players.find(p => p.user_id === d.user_id)) {
          this.players = [...this.players, { ...d, is_host: d.user_id === this.hostUserId }];
        }
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
        break;
      case 'round_started':
        this.currentRound = msg.data as Round;
        this.phase = 'submitting';
        this.submissions = [];
        this.hasSubmitted = false;
        this.hasVoted = false;
        break;
      case 'submissions_closed':
        this.phase = 'voting';
        break;
      case 'vote_results': {
        const d = msg.data as { submissions: Submission[]; leaderboard: LeaderboardEntry[] };
        this.submissions = d.submissions ?? [];
        this.leaderboard = d.leaderboard ?? [];
        this.phase = 'results';
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
        const d = msg.data as { state: RoomStatus; players: Player[]; host_id?: string };
        this.state = d.state;
        if (d.host_id) this.hostUserId = d.host_id;
        this.players = (d.players ?? []).map(p => ({
          ...p,
          is_host: p.user_id === this.hostUserId
        }));
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
    }
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
    this.hasSubmitted = false;
    this.hasVoted = false;
  }
}

export const room = new RoomState();
