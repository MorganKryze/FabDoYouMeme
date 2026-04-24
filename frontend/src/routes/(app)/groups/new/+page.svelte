<script lang="ts">
  import { goto } from '$app/navigation';
  import { groupsState } from '$lib/state/groups.svelte';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { Save, XCircle } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';
  import type { GroupClassification, GroupLanguage } from '$lib/api/groups';

  let name = $state('');
  let description = $state('');
  let language = $state<GroupLanguage>('en');
  let classification = $state<GroupClassification>('sfw');
  let submitting = $state(false);
  let error = $state<string | null>(null);

  async function onSubmit(e: Event) {
    e.preventDefault();
    submitting = true;
    error = null;
    try {
      const g = await groupsState.create({
        name: name.trim(),
        description: description.trim(),
        language,
        classification,
      });
      goto(`/groups/${g.id}`);
    } catch (err) {
      error = (err as Error).message;
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head>
  <title>{m.groups_new_page_title()}</title>
</svelte:head>

<div class="w-full max-w-lg mx-auto p-6 flex flex-col gap-6" use:reveal>
  <h1 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
    {m.groups_new_heading()}
  </h1>

  <section
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <form class="flex flex-col gap-5" onsubmit={onSubmit}>
      <div class="flex flex-col gap-2">
        <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
          {m.groups_field_name()}
        </p>
        <input
          bind:value={name}
          maxlength={80}
          required
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
          required
          class="rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors resize-none"
        ></textarea>
        <p class="text-xs font-semibold text-brand-text-muted">{description.length} / 500</p>
      </div>

      <div class="flex flex-col gap-2">
        <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
          {m.groups_field_language()}
        </p>
        <select
          bind:value={language}
          class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
        >
          <option value="en">{m.groups_language_en()}</option>
          <option value="fr">{m.groups_language_fr()}</option>
          <option value="multi">{m.groups_language_multi()}</option>
        </select>
      </div>

      <div class="flex flex-col gap-2">
        <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
          {m.groups_field_classification()}
        </p>
        <select
          bind:value={classification}
          class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
        >
          <option value="sfw">{m.groups_classification_sfw()}</option>
          <option value="nsfw">{m.groups_classification_nsfw()}</option>
        </select>
      </div>

      {#if error}
        <p class="text-sm font-bold text-red-600">{error}</p>
      {/if}

      <div class="flex justify-end gap-2">
        <button
          type="button"
          use:pressPhysics={'ghost'}
          use:hoverEffect={'swap'}
          onclick={() => goto('/groups')}
          class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-transparent text-sm font-bold cursor-pointer inline-flex items-center gap-2"
        >
          <XCircle size={16} strokeWidth={2.5} />
          {m.common_cancel()}
        </button>
        <button
          type="submit"
          use:pressPhysics={'dark'}
          use:hoverEffect={'swap'}
          disabled={submitting}
          class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <Save size={16} strokeWidth={2.5} />
          {m.groups_create_submit()}
        </button>
      </div>
    </form>
  </section>
</div>
