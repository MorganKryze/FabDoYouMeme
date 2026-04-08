import type { GameType, Player, LeaderboardEntry, Submission, Round, WsMessage } from '$lib/api/types';

type RoomPhase = 'idle' | 'countdown' | 'submitting' | 'voting' | 'results';
type RoomStatus = 'lobby' | 'playing' | 'finished';

class RoomState {
  code = $state<string | null>(null);
  gameType = $state<GameType | null>(null);
  state = $state<RoomStatus>('lobby');
  players = $state<Player[]>([]);
  currentRound = $state<Round | null>(null);
  phase = $state<RoomPhase>('idle');
  submissions = $state<Submission[]>([]);
  leaderboard = $state<LeaderboardEntry[]>([]);
  endReason = $state<string | null>(null);

  hasSubmitted = $state(false);
  hasVoted = $state(false);

  init(data: { code: string; game_type: GameType; state: string; players: Player[] }): void {
    this.code = data.code;
    this.gameType = data.game_type;
    this.state = data.state as RoomStatus;
    this.players = data.players;
  }

  handleMessage(msg: WsMessage) {
    switch (msg.type) {
      case 'player_joined': {
        const d = msg.data as Player;
        if (!this.players.find(p => p.user_id === d.user_id)) {
          this.players = [...this.players, d];
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
        const d = msg.data as { state: RoomStatus; players: Player[] };
        this.state = d.state;
        this.players = d.players;
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
    this.hasSubmitted = false;
    this.hasVoted = false;
  }
}

export const room = new RoomState();
