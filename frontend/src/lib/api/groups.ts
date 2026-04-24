// frontend/src/lib/api/groups.ts
//
// Phase 1 of the groups paradigm — see
// docs/superpowers/specs/2026-04-22-groups-paradigm-design.md.
import { api } from './client';

export type GroupClassification = 'sfw' | 'nsfw';
export type GroupLanguage = 'en' | 'fr' | 'multi';
export type GroupRole = 'admin' | 'member';

export interface Group {
  id: string;
  name: string;
  description: string;
  language: GroupLanguage;
  classification: GroupClassification;
  avatar_media_key: string | null;
  member_cap: number;
  quota_bytes: number;
  created_by: string | null;
  created_at: string;
  deleted_at: string | null;
}

export interface GroupListItem extends Group {
  member_role: GroupRole;
}

export interface GroupMember {
  group_id: string;
  user_id: string;
  role: GroupRole;
  joined_at: string;
  username: string;
  last_login_at: string | null;
}

export interface GroupBan {
  id: string;
  group_id: string;
  user_id: string;
  banned_by: string | null;
  banned_at: string;
  username: string;
}

export interface CreateGroupBody {
  name: string;
  description: string;
  language: GroupLanguage;
  classification: GroupClassification;
  avatar_media_key?: string;
}

export interface UpdateGroupBody {
  name?: string;
  description?: string;
  language?: GroupLanguage;
  classification?: GroupClassification;
  // avatar_set toggles whether avatar_media_key is read at all. Send
  // avatar_set=true with avatar_media_key=null to clear the avatar.
  avatar_set?: boolean;
  avatar_media_key?: string | null;
}

export const groupsApi = {
  list: () => api.get<GroupListItem[]>('/api/groups'),
  get: (id: string) => api.get<Group>(`/api/groups/${id}`),
  create: (body: CreateGroupBody) => api.post<Group>('/api/groups', body),
  update: (id: string, body: UpdateGroupBody) => api.patch<Group>(`/api/groups/${id}`, body),
  delete: (id: string) => api.delete<void>(`/api/groups/${id}`),
  restore: (id: string) => api.post<Group>(`/api/groups/${id}/restore`),

  listMembers: (id: string) => api.get<GroupMember[]>(`/api/groups/${id}/members`),
  kick: (id: string, userID: string) =>
    api.delete<void>(`/api/groups/${id}/members/${userID}`),
  promote: (id: string, userID: string) =>
    api.post<GroupMember>(`/api/groups/${id}/members/${userID}/promote`),
  selfDemote: (id: string) => api.post<GroupMember>(`/api/groups/${id}/members/self/demote`),
  leave: (id: string) => api.delete<void>(`/api/groups/${id}/members/self`),

  ban: (id: string, userID: string) =>
    api.post<void>(`/api/groups/${id}/bans`, { user_id: userID }),
  unban: (id: string, userID: string) => api.delete<void>(`/api/groups/${id}/bans/${userID}`),
  listBans: (id: string) => api.get<GroupBan[]>(`/api/groups/${id}/bans`),

  // Phase 2 — invites
  listInvites: (id: string) => api.get<GroupInvite[]>(`/api/groups/${id}/invites`),
  mintGroupJoin: (id: string, body: MintInviteBody) =>
    api.post<GroupInvite>(`/api/groups/${id}/invites`, body),
  mintPlatformPlus: (id: string, body: MintInviteBody) =>
    api.post<GroupInvite>(`/api/groups/${id}/invites/platform_plus`, body),
  revokeInvite: (id: string, inviteID: string) =>
    api.delete<void>(`/api/groups/${id}/invites/${inviteID}`),
  redeemInvite: (token: string, nsfwAgeAffirmation = false) =>
    api.post<{ group: Group }>(`/api/groups/invites/redeem`, {
      token,
      nsfw_age_affirmation: nsfwAgeAffirmation
    }),
  previewInvite: (token: string) =>
    api.get<InvitePreview>(`/api/groups/invites/preview?token=${encodeURIComponent(token)}`),

  // Phase 3 — packs + duplication queue
  listPacks: (id: string) => api.get<GroupPack[]>(`/api/groups/${id}/packs`),
  duplicatePack: (id: string, sourcePackID: string) =>
    api.post<GroupPack | DuplicatePendingResponse>(`/api/groups/${id}/packs/duplicate`, {
      source_pack_id: sourcePackID
    }),
  deletePack: (id: string, packID: string) =>
    api.delete<void>(`/api/groups/${id}/packs/${packID}`),
  evictPack: (id: string, packID: string) =>
    api.post<void>(`/api/groups/${id}/packs/${packID}/evict`),

  listPending: (id: string) =>
    api.get<PendingDuplication[]>(`/api/groups/${id}/duplication-queue`),
  acceptPending: (id: string, queueID: string) =>
    api.post<GroupPack>(`/api/groups/${id}/duplication-queue/${queueID}/accept`),
  rejectPending: (id: string, queueID: string) =>
    api.post<void>(`/api/groups/${id}/duplication-queue/${queueID}/reject`)
};

export interface GroupPack extends Group {
  // Everything on Group + the extras populated for group-owned packs.
  owner_id: string | null;
  is_official: boolean;
  visibility: 'private' | 'public';
  status: 'active' | 'flagged' | 'banned';
  is_system: boolean;
  duplicated_from_pack_id: string | null;
  duplicated_by_user_id: string | null;
}

export interface DuplicatePendingResponse {
  status: 'pending_admin_approval';
  pending: {
    id: string;
    group_id: string;
    source_pack_id: string;
    requested_by: string;
    requested_at: string;
  };
}

export interface PendingDuplication {
  id: string;
  group_id: string;
  source_pack_id: string;
  requested_by: string;
  requested_at: string;
  resolved_at: string | null;
  resolved_by: string | null;
  resolution: 'accepted' | 'rejected' | null;
  source_pack_name: string;
  source_classification: 'sfw' | 'nsfw';
  requested_by_username: string;
}

export interface GroupInvite {
  id: string;
  token: string;
  group_id: string;
  created_by: string | null;
  kind: 'group_join' | 'platform_plus_group';
  restricted_email: string | null;
  max_uses: number;
  uses_count: number;
  expires_at: string | null;
  revoked_at: string | null;
  created_at: string;
}

export interface MintInviteBody {
  max_uses?: number;
  ttl_seconds?: number;
  restricted_email?: string;
}

export interface InvitePreview {
  group: Group;
  invite_kind: 'group_join' | 'platform_plus_group';
  revoked: boolean;
  expired: boolean;
  exhausted: boolean;
}
