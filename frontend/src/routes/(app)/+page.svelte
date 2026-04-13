<script lang="ts">
  import { enhance } from '$app/forms';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import RoomCodeInput from '$lib/components/RoomCodeInput.svelte';
  import { Play, Sparkles } from '$lib/icons';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let code = $state('');
  let joinForm = $state<HTMLFormElement | null>(null);

  function submitJoin(next: string) {
    code = next;
    if (next.length === 4 && joinForm) joinForm.requestSubmit();
  }
</script>

<svelte:head>
  <title>FabDoYouMeme</title>
</svelte:head>

<div class="flex-1 flex items-start justify-center p-6 pt-10">
  <div class="w-full max-w-3xl flex flex-col gap-10">

    <!-- ─── Join pill (the dominant path) ─────────────────────── -->
    <section use:reveal class="flex flex-col gap-4">
      <div class="text-center">
        <h1 class="text-3xl font-bold">Got a code?</h1>
        <p class="text-sm font-semibold text-brand-text-muted mt-1">
          Drop it in and jump straight into the room.
        </p>
      </div>

      {#if form?.joinError}
        <div
          class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold text-center"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
        >
          {form.joinError}
        </div>
      {/if}

      <form
        bind:this={joinForm}
        method="POST"
        action="?/joinRoom"
        use:enhance
        class="grid grid-cols-[1fr_auto] gap-3 items-end"
      >
        <RoomCodeInput bind:value={code} autofocus onenter={submitJoin} />
        <button
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          type="submit"
          disabled={code.length !== 4}
          class="h-16 px-7 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-40 cursor-pointer inline-flex items-center justify-center gap-2"
        >
          <Play size={18} strokeWidth={2.5} />
          Play
        </button>
      </form>
    </section>

    <!-- ─── Divider ──────────────────────────────────────────── -->
    <div use:reveal={{ delay: 1 }} class="flex items-center gap-4 text-xs font-bold uppercase tracking-[0.2em] text-brand-text-muted">
      <span class="flex-1 h-px bg-brand-border-heavy/40"></span>
      <span>or host a new game</span>
      <span class="flex-1 h-px bg-brand-border-heavy/40"></span>
    </div>

    <!-- ─── Game tile grid ───────────────────────────────────── -->
    <section class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
      {#each data.gameTypes as gt, i}
        <a
          href={`/host?game_type=${gt.slug}`}
          use:reveal={{ delay: i + 2 }}
          use:physCard
          use:hoverEffect={'gradient'}
          class="group rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-6 flex flex-col gap-3 cursor-pointer"
          style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
        >
          <div class="inline-flex items-center gap-2 text-lg font-bold">
            <Sparkles size={18} strokeWidth={2.5} />
            {gt.name}
          </div>
          {#if gt.description}
            <p class="text-sm font-semibold text-brand-text-muted line-clamp-3">{gt.description}</p>
          {/if}
          <div class="mt-auto text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
            Host this →
          </div>
        </a>
      {/each}
    </section>
  </div>
</div>
