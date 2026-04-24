export interface User {
  id: string;
  username: string;
  email: string;
  role: 'player' | 'admin';
  locale: 'en' | 'fr';
  created_at: string; // ISO 8601 UTC timestamp
}

export interface RequiredPack {
  role: 'image' | 'text';
  payload_versions: number[];
}

export interface GameType {
  id: string;
  slug: string;
  name: string;
  description: string | null;
  version: string;
  supports_solo: boolean;
  config: GameTypeConfig;
  required_packs: RequiredPack[];
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
  /** Hand-size bounds are only present for game types that deal a per-player
   *  hand of items (e.g. meme-showdown). Omitted for game types that don't. */
  min_hand_size?: number;
  max_hand_size?: number;
  default_hand_size?: number;
}

export interface Pack {
  id: string;
  name: string;
  description: string | null;
  owner_id: string | null;
  is_official: boolean;
  visibility: 'private' | 'public';
  status: 'active' | 'flagged' | 'banned';
  is_system: boolean;
  language: 'en' | 'fr' | 'multi';
  item_count?: number;
  created_at: string;
  // Group-owned packs carry `group_id`. Phase 3 of the groups paradigm —
  // studio renders these in a per-group section of the navigator.
  group_id?: string | null;
}

export interface Room {
  id: string;
  code: string;
  game_type_id: string;
  game_type_slug: string;
  game_type?: GameType | null;
  pack_id: string;
  text_pack_id: string | null;
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
  /** When true, the host manually advances rounds via "Next Round". Default false (server auto-advances after 3s). */
  host_paced?: boolean;
  /** Per-player jokers available to skip their own submit turn (platform-level). Defaults to ceil(round_count/5). */
  joker_count: number;
  /** When true, players may abstain from voting in a round. Defaults to true. */
  allow_skip_vote: boolean;
  /** Per-player hand of items dealt each round — only present for game types
   *  whose manifest declares hand-size bounds (e.g. meme-showdown). */
  hand_size?: number;
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
  | 'player_submitted'
  | 'player_skipped_submit'
  | 'player_voted'
  | 'player_skipped_vote'
  | 'reconnecting'
  | 'game_started'
  | 'round_started'
  | 'submissions_closed'
  | 'vote_results'
  | 'game_ended'
  | 'room_state'
  | 'error';

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
  player_id: string;
  display_name: string;
  is_guest: boolean;
  score: number;
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
}

export interface Submission {
  id: string;
  user_id: string;
  username: string;
  // Either field is present depending on game type:
  //  - meme-freestyle → `caption`
  //  - meme-showdown    → `text`
  // Components under `lib/games/<slug>/` read whichever field their handler
  // emits. Keeping both optional here avoids a discriminated union that would
  // leak game-type concerns into every consumer of the shared type.
  caption?: string;
  text?: string;
  votes_received?: number;
  points_awarded?: number;
}

// ── Upload outcomes (studio) ──────────────────────────────────────────────
export interface UploadResult {
  ok: true;
  item: GameItem;
}
export interface UploadFailure {
  ok: false;
  error: string;
  filename: string;
}
export type UploadOutcome = UploadResult | UploadFailure;

export interface BulkUploadOutcome {
  succeeded: GameItem[];
  failed: { filename: string; reason: string }[];
}

// ── Admin dashboard ───────────────────────────────────────────────────────
export interface AdminStats {
  active_rooms: number;
  total_users: number;
  games_played: number;
  pending_invites: number;
}

export interface AdminStorageStats {
  packs_count: number;
  assets_count: number;
  total_bytes: number;
}

export interface AuditEntry {
  id: string;
  admin_id: string | null;
  admin_username: string;
  action: string;
  resource_type: string;
  resource_id: string;
  resource_label: string;
  changes: unknown;
  created_at: string;
}

// ── Destructive admin actions ("danger zone") ────────────────────────────
// Shape of the server response for every /api/admin/danger/* endpoint.
// Every count is absolute — a zero field means "nothing of that kind was
// touched by this action" (not "action failed"). `s3_error` only appears
// when the S3 purge step raised an error after the DB transaction
// committed; callers should surface it but treat the DB-side counts as
// authoritative.
export interface DangerReport {
  rooms_deleted: number;
  room_players_deleted: number;
  rounds_deleted: number;
  submissions_deleted: number;
  votes_deleted: number;
  packs_deleted: number;
  items_deleted: number;
  versions_deleted: number;
  invites_deleted: number;
  sessions_deleted: number;
  magic_tokens_deleted: number;
  notifications_deleted: number;
  users_deleted: number;
  s3_objects_deleted: number;
  s3_error?: string;
  excluded_self?: boolean;
}

// ── Deep health response (admin dashboard) ────────────────────────────────
export interface DeepHealthCheck {
  status: 'ok' | 'degraded' | 'skipped';
  latency_ms?: number;
  error?: string;
}
export interface DeepHealthResponse {
  status: 'ok' | 'degraded';
  checks: {
    postgres: DeepHealthCheck;
    rustfs: DeepHealthCheck;
    smtp: DeepHealthCheck;
  };
}

// ─── Replay (GET /api/rooms/:code/replay) ─────────────────────────────────

export interface ReplayAuthor {
  display_name: string;
  kind: 'user' | 'guest' | 'deleted';
}

export interface ReplaySubmission {
  id: string;
  author: ReplayAuthor;
  /**
   * Opaque game-type payload. For meme-freestyle: `{ caption: string }`.
   * For meme-showdown: `{ card_id: string; text: string }` (server-resolved).
   */
  payload: Record<string, unknown>;
  votes_received: number;
  points_awarded: number;
}

export interface ReplayPrompt {
  payload_version: number;
  media_key?: string;
  image_url?: string;
  prompt?: string;
  text?: string;
}

export interface ReplayRound {
  round_number: number;
  prompt: ReplayPrompt;
  submissions: ReplaySubmission[];
}

export interface ReplayLeaderboardRow {
  rank: number;
  display_name: string;
  score: number;
  kind: 'user' | 'guest' | 'deleted';
}

export interface ReplayRoomHeader {
  code: string;
  game_type_slug: string;
  pack_name: string;
  text_pack_name?: string;
  started_at: string;
  finished_at?: string;
  player_count: number;
  config: Record<string, unknown>;
}

export interface ReplayPayload {
  room: ReplayRoomHeader;
  rounds: ReplayRound[];
  leaderboard: ReplayLeaderboardRow[];
}
