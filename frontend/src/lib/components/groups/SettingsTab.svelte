<script lang="ts">
  // Phase 1 of the groups paradigm. Two cards: editable identity (name +
  // description, admin-only) and a danger zone (delete = admin-only,
  // leave = anyone). Both danger actions confirm before acting and route
  // back to /groups on success.
  import { goto } from '$app/navigation';
  import { groupDetailState } from '$lib/state/groups.svelte';
  import { groupsApi } from '$lib/api/groups';
  import { user } from '$lib/state/user.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Save, Trash2, LogOut } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';

  let { gid }: { gid: string } = $props();

  let name = $state('');
  let description = $state('');
  let saving = $state(false);
  let deleting = $state(false);
  let leaving = $state(false);

  const selfIsAdmin = $derived(
    groupDetailState.members.some(
      (mem) => mem.user_id === user.id && mem.role === 'admin'
    )
  );

  // Mirror the loaded group into the form fields. Re-runs when the
  // detail state hydrates so the inputs hold the canonical values, not
  // the empty initial state.
  $effect(() => {
    if (groupDetailState.group) {
      name = groupDetailState.group.name;
      description = groupDetailState.group.description;
    }
  });

  const dirty = $derived(
    !!groupDetailState.group &&
      (name !== groupDetailState.group.name || description !== groupDetailState.group.description)
  );

  async function save() {
    if (!dirty || saving) return;
    saving = true;
    try {
      await groupsApi.update(gid, { name: name.trim(), description: description.trim() });
      await groupDetailState.load(gid);
      toast.show(m.groups_settings_saved(), 'success');
    } catch (e) {
      toast.show((e as Error).message, 'error');
    } finally {
      saving = false;
    }
  }

  async function del() {
    if (!confirm(m.groups_delete_confirm())) return;
    deleting = true;
    try {
      await groupsApi.delete(gid);
      goto('/groups');
    } catch (e) {
      toast.show((e as Error).message, 'error');
      deleting = false;
    }
  }

  async function leave() {
    if (!confirm(m.groups_leave_confirm())) return;
    leaving = true;
    try {
      await groupsApi.leave(gid);
      goto('/groups');
    } catch (e) {
      toast.show((e as Error).message, 'error');
      leaving = false;
    }
  }
</script>

<div class="flex flex-col gap-6">
  {#if selfIsAdmin}
    <section
      class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-5"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
    >
      <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
        {m.groups_settings_identity()}
      </h2>

      <div class="flex flex-col gap-2">
        <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
          {m.groups_field_name()}
        </p>
        <input
          bind:value={name}
          maxlength={80}
          class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
        />
      </div>

      <div class="flex flex-col gap-2">
        <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
          {m.groups_field_description()}
        </p>
        <textarea
          bind:value={description}
          maxlength={500}
          rows={3}
          class="rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors resize-none"
        ></textarea>
        <p class="text-xs font-semibold text-brand-text-muted">{description.length} / 500</p>
      </div>

      <div class="flex justify-end">
        <button
          type="button"
          use:pressPhysics={'dark'}
          use:hoverEffect={'swap'}
          disabled={!dirty || saving}
          onclick={save}
          class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <Save size={16} strokeWidth={2.5} />
          {m.common_save()}
        </button>
      </div>
    </section>
  {/if}

  <section
    class="rounded-[22px] border-[2.5px] border-red-300 bg-brand-surface p-5 flex flex-col gap-3"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.06);"
  >
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-red-600 m-0">
      {m.groups_danger_heading()}
    </h2>
    <div class="flex flex-wrap gap-2">
      {#if selfIsAdmin}
        <button
          type="button"
          disabled={deleting}
          onclick={del}
          class="h-11 px-5 rounded-full border-[2.5px] border-red-400 bg-red-50 text-red-700 text-sm font-bold cursor-pointer inline-flex items-center gap-2 disabled:opacity-50"
        >
          <Trash2 size={16} strokeWidth={2.5} />
          {m.groups_delete_cta()}
        </button>
      {/if}
      <button
        type="button"
        disabled={leaving}
        onclick={leave}
        class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2 disabled:opacity-50"
      >
        <LogOut size={16} strokeWidth={2.5} />
        {m.groups_leave_cta()}
      </button>
    </div>
  </section>
</div>
