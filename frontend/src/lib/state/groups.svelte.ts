// frontend/src/lib/state/groups.svelte.ts
//
// Phase 1 of the groups paradigm. Two singletons:
//   - groupsState: the user's group list (drives /groups)
//   - groupDetailState: the currently-open group + its members (drives
//     /groups/[gid]). Detail state is per-instance — re-load on navigation.
import { groupsApi, type Group, type GroupListItem, type GroupMember } from '$lib/api/groups';

class GroupsState {
  groups = $state<GroupListItem[]>([]);
  loading = $state(false);
  error = $state<string | null>(null);

  async load() {
    this.loading = true;
    this.error = null;
    try {
      this.groups = await groupsApi.list();
    } catch (e) {
      this.error = (e as Error).message;
    } finally {
      this.loading = false;
    }
  }

  async create(body: Parameters<typeof groupsApi.create>[0]): Promise<Group> {
    const g = await groupsApi.create(body);
    await this.load();
    return g;
  }

  reset() {
    this.groups = [];
    this.error = null;
    this.loading = false;
  }
}

export const groupsState = new GroupsState();

class GroupDetailState {
  group = $state<Group | null>(null);
  members = $state<GroupMember[]>([]);
  loading = $state(false);
  error = $state<string | null>(null);

  async load(id: string) {
    this.loading = true;
    this.error = null;
    try {
      const [g, m] = await Promise.all([groupsApi.get(id), groupsApi.listMembers(id)]);
      this.group = g;
      this.members = m;
    } catch (e) {
      this.error = (e as Error).message;
      this.group = null;
      this.members = [];
    } finally {
      this.loading = false;
    }
  }

  reset() {
    this.group = null;
    this.members = [];
    this.error = null;
    this.loading = false;
  }
}

export const groupDetailState = new GroupDetailState();
