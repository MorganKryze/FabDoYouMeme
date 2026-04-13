export interface User {
  id: string;
  username: string;
  email: string;
  role: 'player' | 'admin';
}

export interface GameType {
  id: string;
  slug: string;
  name: string;
  description: string | null;
  version: string;
  supports_solo: boolean;
  config: GameTypeConfig;
  supported_payload_versions: number[];
}

export interface GameTypeConfig {
  min_round_duration_seconds: number;
  max_round_duration_seconds: number;
  default_round_duration_seconds: number;
  min_voting_duration_seconds: number;
  max_voting_duration_seconds: number;
  default_voting_duration_seconds: number;
  min_players: number;
  max_players: number | null;
  min_round_count: number;
  max_round_count: number;
  default_round_count: number;
}

export interface Pack {
  id: string;
  name: string;
  description: string | null;
  owner_id: string | null;
  is_official: boolean;
  visibility: 'private' | 'public';
  status: 'active' | 'flagged' | 'banned';
  item_count?: number;
  created_at: string;
}

export interface Room {
  id: string;
  code: string;
  game_type_id: string;
  game_type_slug: string;
  pack_id: string;
  host_id: string;
  mode: 'multiplayer' | 'solo';
  state: 'lobby' | 'playing' | 'finished';
  config: RoomConfig;
  created_at: string;
  finished_at: string | null;
}

export interface RoomConfig {
  round_duration_seconds: number;
  voting_duration_seconds: number;
  round_count: number;
}

export interface Invite {
  id: string;
  token: string;
  label: string | null;
  restricted_email: string | null;
  max_uses: number;
  uses_count: number;
  expires_at: string | null;
  created_at: string;
}

export interface GameItem {
  id: string;
  name: string;
  type: 'image' | 'text';
  position: number;
  payload_version: number;
  current_version_id: string | null;
  media_key?: string | null;
  payload?: unknown;
  thumbnail_url?: string | null;
  version_number?: number | null;
}

export interface ItemVersion {
  id: string;
  item_id: string;
  version_number: number;
  media_key: string | null;
  media_url?: string | null;
  content?: string | null;
  payload: unknown;
  created_at: string;
  deleted_at: string | null;
}

export interface ApiError {
  error: string;
  code: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  next_cursor: string | null;
  total?: number;
}

// WebSocket message types
export type WsMessageType =
  | 'pong'
  | 'player_joined'
  | 'player_left'
  | 'player_kicked'
  | 'reconnecting'
  | 'game_started'
  | 'round_started'
  | 'submissions_closed'
  | 'vote_results'
  | 'game_ended'
  | 'room_state'
  | 'error'
  | `meme-caption:submissions_shown`
  | `meme-caption:vote_results`;

export interface WsMessage {
  type: WsMessageType | string;
  data?: unknown;
}

export interface Player {
  user_id: string;
  username: string;
  connected?: boolean;
  is_host?: boolean;
  is_guest?: boolean;
}

export interface LeaderboardEntry {
  user_id: string;
  username: string;
  total_score: number;
  rank: number;
}

export interface Round {
  round_number: number;
  ends_at: string;
  duration_seconds: number;
  item: {
    payload: unknown;
    media_url?: string | null;
  };
  text_prompt?: string;   // backward compat
  media_url?: string;     // backward compat
}

export interface Submission {
  id: string;
  user_id: string;
  username: string;
  caption: string;
  votes_received?: number;
  points_awarded?: number;
}
