<script lang="ts">
  import { goto } from '$app/navigation';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import RoomCodeInput from '$lib/components/RoomCodeInput.svelte';
  import {
    Lock,
    Server,
    Shield,
    Gamepad2,
    Users,
    Home,
    Sparkles,
    PartyPopper,
    Play,
    Hash,
  } from '$lib/icons';

  let joinCode = $state('');

  function join(next: string) {
    if (next.length !== 4) return;
    goto(`/join/${next}`);
  }

  const steps = [
    {
      n: 1,
      title: 'Host a room',
      body: 'Pick a game, spin up a 4-letter code, share it with your people.',
      Icon: Home,
    },
    {
      n: 2,
      title: 'Friends pile in',
      body: 'They type the code on their phone. No account needed to join.',
      Icon: Users,
    },
    {
      n: 3,
      title: 'Caption, vote, laugh',
      body: 'Round by round. Scoring, timers, and reconnects are on us.',
      Icon: PartyPopper,
    },
  ] as const;

  const advantages = [
    {
      title: 'Self-hosted',
      body: 'Runs on your own server via Docker Compose. One machine is enough.',
      Icon: Server,
    },
    {
      title: 'Invite-only',
      body: 'No strangers signing up. Admins issue invite tokens — nobody else gets in.',
      Icon: Lock,
    },
    {
      title: 'GDPR-first',
      body: 'Explicit consent, magic-link auth (no passwords), 2-year retention, full data export. Deletion wipes your trace.',
      Icon: Shield,
    },
    {
      title: 'Multi-game',
      body: 'One engine, many games. Adding a new game type is a handler, not a migration.',
      Icon: Gamepad2,
    },
  ] as const;

  const audiences = [
    {
      title: 'Friend groups',
      body: 'Your 10-person Discord deserves a private playground. Nothing leaves your circle.',
    },
    {
      title: 'Families',
      body: 'Game night without ads or random content. Curate your own meme packs, kid-safe if you want.',
    },
    {
      title: 'Remote teams',
      body: 'A 20-minute icebreaker where the data never leaves your infrastructure.',
    },
    {
      title: 'Communities & clubs',
      body: 'Guilds, book clubs, improv troupes — bring your own content, your own rules.',
    },
  ] as const;
</script>

<svelte:head>
  <title>FabDoYouMeme — a private, self-hosted party meme game</title>
  <meta
    name="description"
    content="A party meme game for you and your people. Self-hosted, invite-only, GDPR-first. Host a room, share the code, caption, vote, laugh."
  />
</svelte:head>

<div class="flex-1 flex flex-col items-center px-6">
  <!-- ─── Hero ────────────────────────────────────────────────── -->
  <section use:reveal class="w-full max-w-4xl flex flex-col items-center text-center gap-6 pt-20 pb-24">
    <h1 class="hero">FabDoYouMeme</h1>
    <p class="hero-sub max-w-2xl">
      A party meme game for you and your people. Host a room, share the code,
      caption, vote, laugh.
    </p>
    <p class="text-sm font-semibold text-brand-text-muted max-w-xl">
      Self-hosted, invite-only, GDPR-first. Built to stay inside your circle.
    </p>
  </section>

  <!-- ─── How it works ────────────────────────────────────────── -->
  <section class="w-full max-w-5xl pb-24">
    <div use:reveal class="text-center mb-12">
      <div class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted mb-2">
        How it works
      </div>
      <h2 class="text-4xl font-bold">Three steps. One code. Go.</h2>
    </div>

    <ol class="grid grid-cols-1 md:grid-cols-3 gap-5">
      {#each steps as step, i}
        <li
          use:reveal={{ delay: i + 1 }}
          use:physCard
          class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-6 flex flex-col gap-3"
          style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
        >
          <div class="flex items-center justify-between">
            <span
              class="inline-flex h-10 w-10 items-center justify-center rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white font-bold"
              style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
            >
              {step.n}
            </span>
            <step.Icon size={22} strokeWidth={2.5} />
          </div>
          <h3 class="text-xl font-bold mt-2">{step.title}</h3>
          <p class="text-sm font-semibold text-brand-text-muted">{step.body}</p>
        </li>
      {/each}
    </ol>
  </section>

  <!-- ─── Advantages ─────────────────────────────────────────── -->
  <section class="w-full max-w-5xl pb-24">
    <div use:reveal class="text-center mb-12">
      <div class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted mb-2">
        Why this exists
      </div>
      <h2 class="text-4xl font-bold">Yours. Not theirs.</h2>
      <p class="text-sm font-semibold text-brand-text-muted mt-2 max-w-xl mx-auto">
        Most party game platforms turn your laughter into someone else's
        training data. This one doesn't.
      </p>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
      {#each advantages as adv, i}
        <div
          use:reveal={{ delay: i + 1 }}
          use:physCard
          class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-6 flex gap-4"
          style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
        >
          <div
            class="shrink-0 inline-flex h-12 w-12 items-center justify-center rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white"
            style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
          >
            <adv.Icon size={22} strokeWidth={2.5} />
          </div>
          <div class="flex flex-col gap-1">
            <h3 class="text-xl font-bold">{adv.title}</h3>
            <p class="text-sm font-semibold text-brand-text-muted">{adv.body}</p>
          </div>
        </div>
      {/each}
    </div>
  </section>

  <!-- ─── Audiences ──────────────────────────────────────────── -->
  <section class="w-full max-w-5xl pb-24">
    <div use:reveal class="text-center mb-12">
      <div class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted mb-2">
        Who it's for
      </div>
      <h2 class="text-4xl font-bold">Bring your circle.</h2>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
      {#each audiences as aud, i}
        <div
          use:reveal={{ delay: i + 1 }}
          use:physCard
          class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-2"
          style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
        >
          <div class="inline-flex items-center gap-2 text-sm font-bold">
            <Sparkles size={16} strokeWidth={2.5} />
            {aud.title}
          </div>
          <p class="text-xs font-semibold text-brand-text-muted leading-relaxed">{aud.body}</p>
        </div>
      {/each}
    </div>
  </section>

  <!-- ─── Final CTA ──────────────────────────────────────────── -->
  <section use:reveal class="w-full max-w-3xl pb-20">
    <div
      class="rounded-[28px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-8 sm:p-10 flex flex-col items-center gap-8 text-center"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.08);"
    >
      <div class="flex flex-col items-center gap-2">
        <h2 class="text-3xl sm:text-4xl font-bold">Ready to play?</h2>
        <p class="text-sm font-semibold text-brand-text-muted max-w-md">
          Pick your lane. No wrong answer.
        </p>
      </div>

      <div class="relative w-full grid grid-cols-1 md:grid-cols-2 gap-6 md:gap-0">
        <!-- Join lane -->
        <div class="flex flex-col items-center gap-4 md:pr-8">
          <div class="inline-flex items-center gap-2 text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
            <Hash size={14} strokeWidth={2.5} />
            Got a code?
          </div>
          <p class="text-sm font-semibold text-brand-text max-w-[16rem]">
            Jump straight in. No account needed.
          </p>
          <div class="w-full max-w-[14rem]">
            <RoomCodeInput bind:value={joinCode} onenter={join} />
          </div>
          <button
            type="button"
            use:pressPhysics={'ghost'}
            use:hoverEffect={'swap'}
            disabled={joinCode.length !== 4}
            onclick={() => join(joinCode)}
            class="inline-flex items-center justify-center gap-2 px-6 h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white font-bold disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
            style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
          >
            <Play size={16} strokeWidth={2.5} />
            Join the room
          </button>
        </div>

        <!-- Divider -->
        <div
          class="md:absolute md:inset-y-0 md:left-1/2 md:-translate-x-1/2 flex md:flex-col items-center justify-center gap-3"
          aria-hidden="true"
        >
          <span class="flex-1 md:w-[2.5px] md:flex-1 h-[2.5px] md:h-auto bg-brand-border-heavy rounded-full"></span>
          <span
            class="inline-flex items-center justify-center h-8 min-w-8 px-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-[0.65rem] font-bold uppercase tracking-[0.15em]"
            style="box-shadow: 0 2px 0 rgba(0,0,0,0.06);"
          >
            or
          </span>
          <span class="flex-1 md:w-[2.5px] md:flex-1 h-[2.5px] md:h-auto bg-brand-border-heavy rounded-full"></span>
        </div>

        <!-- Host lane -->
        <div class="flex flex-col items-center gap-4 md:pl-8">
          <div class="inline-flex items-center gap-2 text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
            <Home size={14} strokeWidth={2.5} />
            Spinning one up?
          </div>
          <p class="text-sm font-semibold text-brand-text max-w-[16rem]">
            Host your own room. One account, unlimited games.
          </p>
          <div class="w-full max-w-[14rem] h-[52px]" aria-hidden="true"></div>
          <a
            href="/auth/magic-link"
            use:pressPhysics={'dark'}
            use:hoverEffect={'gradient'}
            class="inline-flex items-center justify-center gap-2 px-6 h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold no-underline"
            style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
          >
            <Play size={16} strokeWidth={2.5} />
            Sign in to host
          </a>
        </div>
      </div>

      <p class="text-xs text-brand-text-muted">
        Hosting requires an account. Joining with a code does not.
      </p>
    </div>
  </section>
</div>
