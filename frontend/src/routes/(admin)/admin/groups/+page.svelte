<script lang="ts">
  // Phase 5 — platform-admin overview of every group on the instance. The
  // row-level action is an inline "Edit" modal that covers quota + member
  // cap; reads come from the /api/admin/groups load, writes go through the
  // two server actions.
  import { enhance } from '$app/forms';
  import { reveal } from '$lib/actions/reveal';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Save, Users, Package } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let editing = $state<string | null>(null); // group_id currently being edited
  let quotaInput = $state<Record<string, number>>({});
  let capInput = $state<Record<string, number>>({});

  function beginEdit(group: (typeof data.groups)[number]) {
    editing = group.id;
    quotaInput[group.id] = group.quota_bytes;
    capInput[group.id] = group.member_cap;
  }
</script>

<svelte:head>
  <title>{m.admin_groups_page_title()}</title>
</svelte:head>

<div class="w-full max-w-4xl mx-auto p-6 flex flex-col gap-5" use:reveal>
  <header class="flex items-center justify-between gap-3">
    <h1 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
      {m.admin_groups_heading()}
    </h1>
  </header>

  {#if form?.error}
    <div class="rounded-[18px] border-[2.5px] border-red-300 bg-red-50 p-4 text-sm font-semibold text-red-700">
      {form.error}
    </div>
  {:else if form?.ok}
    <div class="rounded-[18px] border-[2.5px] border-green-300 bg-green-50 p-4 text-sm font-semibold text-green-700">
      {m.admin_groups_saved()}
    </div>
  {/if}

  {#if data.groups.length === 0}
    <p class="text-sm font-semibold text-brand-text-muted">{m.admin_groups_empty()}</p>
  {:else}
    <ul class="flex flex-col gap-3 list-none p-0 m-0">
      {#each data.groups as g (g.id)}
        <li
          class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-3"
          class:opacity-70={g.deleted_at}
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <h2 class="text-base font-bold m-0 truncate">{g.name}</h2>
                <span class="shrink-0 text-[0.6rem] font-bold uppercase tracking-[0.15em] rounded-full px-2 py-1 border-[2px] border-brand-border-heavy">
                  {g.classification}
                </span>
                <span class="shrink-0 text-[0.6rem] font-bold uppercase tracking-[0.15em] rounded-full px-2 py-1 border-[2px] border-brand-border-heavy">
                  {g.language}
                </span>
                {#if g.deleted_at}
                  <span class="shrink-0 text-[0.6rem] font-bold uppercase tracking-[0.15em] rounded-full px-2 py-1 border-[2px] border-red-300 text-red-700">
                    {m.admin_groups_deleted_badge()}
                  </span>
                {/if}
              </div>
              {#if g.description}
                <p class="text-xs text-brand-text-muted mt-1 line-clamp-2 m-0">{g.description}</p>
              {/if}
            </div>
            <button
              type="button"
              use:hoverEffect={'swap'}
              onclick={() => (editing === g.id ? (editing = null) : beginEdit(g))}
              class="h-9 px-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-xs font-bold cursor-pointer"
            >
              {editing === g.id ? m.common_cancel() : m.admin_groups_edit()}
            </button>
          </div>

          <div class="flex items-center gap-4 text-xs font-semibold text-brand-text-muted">
            <span class="inline-flex items-center gap-1.5">
              <Users size={12} strokeWidth={2.5} />
              {m.admin_groups_members_count({ count: String(g.member_count), cap: String(g.member_cap) })}
            </span>
            <span class="inline-flex items-center gap-1.5">
              <Package size={12} strokeWidth={2.5} />
              {m.admin_groups_quota({ bytes: String(g.quota_bytes) })}
            </span>
          </div>

          {#if editing === g.id}
            <div class="rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-white p-4 flex flex-col gap-3">
              <form method="POST" action="?/setQuota" use:enhance class="flex flex-col gap-2">
                <input type="hidden" name="group_id" value={g.id} />
                <label class="flex flex-col gap-1">
                  <span class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
                    {m.admin_groups_quota_label()}
                  </span>
                  <input
                    type="number"
                    name="quota_bytes"
                    bind:value={quotaInput[g.id]}
                    min={0}
                    class="h-10 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text"
                  />
                </label>
                <button
                  type="submit"
                  use:pressPhysics={'dark'}
                  class="self-end h-9 px-4 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-xs font-bold cursor-pointer inline-flex items-center gap-1.5"
                >
                  <Save size={12} strokeWidth={2.5} />
                  {m.admin_groups_set_quota()}
                </button>
              </form>
              <form method="POST" action="?/setMemberCap" use:enhance class="flex flex-col gap-2">
                <input type="hidden" name="group_id" value={g.id} />
                <label class="flex flex-col gap-1">
                  <span class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
                    {m.admin_groups_cap_label()}
                  </span>
                  <input
                    type="number"
                    name="member_cap"
                    bind:value={capInput[g.id]}
                    min={1}
                    class="h-10 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text"
                  />
                </label>
                <button
                  type="submit"
                  use:pressPhysics={'dark'}
                  class="self-end h-9 px-4 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-xs font-bold cursor-pointer inline-flex items-center gap-1.5"
                >
                  <Save size={12} strokeWidth={2.5} />
                  {m.admin_groups_set_cap()}
                </button>
              </form>
            </div>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

