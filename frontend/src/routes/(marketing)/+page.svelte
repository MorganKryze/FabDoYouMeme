<script lang="ts">
  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import { goto } from '$app/navigation';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import { dealCard } from '$lib/actions/dealCard';
  import RoomCodeInput from '$lib/components/RoomCodeInput.svelte';
  import type { PageData } from './$types';
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
    Plus,
    Sliders,
    Code2,
    ChevronDown,
  } from '$lib/icons';

  let { data }: { data: PageData } = $props();
  let joinCode = $state('');
  let hasScrolled = $state(false);

  function join(next: string) {
    if (next.length !== 4) return;
    goto(`/join/${next}`);
  }

  function revealRest() {
    hasScrolled = true;
    // Give Svelte a tick to render the section before scrolling to it.
    requestAnimationFrame(() => {
      window.scrollTo({ top: window.innerHeight * 0.9, behavior: 'smooth' });
    });
  }

  onMount(() => {
    // If the browser restores a non-zero scroll position (e.g. back-nav), skip the gate.
    if (window.scrollY > 24) {
      hasScrolled = true;
      return;
    }
    const onScroll = () => {
      if (window.scrollY > 24) {
        hasScrolled = true;
        window.removeEventListener('scroll', onScroll);
      }
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  });

  const steps = [
    {
      n: 1,
      title: 'Host a room',
      body: 'Pick a game, spin up a 4-letter code, share it with your people.',
      Icon: Plus,
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

  // Per-card tilt for the dealt-from-deck animation. Alternating signs +
  // uneven magnitudes read as "hand-dealt" rather than mechanical.
  const stepTilts = [-8, 3, -5] as const;
  const advantageTilts = [-7, 5, -4, 6, -9, 3] as const;

  const advantages = [
    {
      title: 'Open source',
      body: 'Free forever, fully open. Read the code, run your own copy, change whatever you want. Nothing locked away.',
      Icon: Code2,
    },
    {
      title: 'Ships in one command',
      body: 'Runs on any small server or old computer. One setup script brings the whole app to life. No cloud bills, no vendor lock-in.',
      Icon: Server,
    },
    {
      title: 'Invite-only',
      body: 'No strangers signing up. You decide who gets in by handing out invite codes.',
      Icon: Lock,
    },
    {
      title: 'GDPR-first',
      body: 'No passwords, no ads, no tracking. You can export your data anytime, or wipe every trace of it in one click.',
      Icon: Shield,
    },
    {
      title: 'Widely configurable',
      body: 'Change round length, player limits, how many rounds, and more. Tweak a setting, restart the app, you are good to go.',
      Icon: Sliders,
    },
    {
      title: 'Built for many games',
      body: 'More games are on the way. The platform is built so new ones drop in without breaking the ones you already love.',
      Icon: Gamepad2,
    },
  ] as const;

</script>

<svelte:head>
  <title>FabDoYouMeme: a private, self-hosted party meme game</title>
  <meta
    name="description"
    content="A party meme game for you and your people. Self-hosted, invite-only, GDPR-first. Host a room, share the code, caption, vote, laugh."
  />

  <!-- Open Graph / link unfurl (Discord, Slack, iMessage) -->
  <meta property="og:type" content="website" />
  <meta property="og:title" content="FabDoYouMeme: a private, self-hosted party meme game" />
  <meta
    property="og:description"
    content="A party meme game for you and your people. Self-hosted, invite-only, GDPR-first."
  />
  <!-- TODO: drop a 1200×630 og-card.png into frontend/static/ and uncomment:
  <meta property="og:image" content="/og-card.png" />
  <meta name="twitter:card" content="summary_large_image" /> -->
  <meta name="twitter:card" content="summary" />
  <meta name="twitter:title" content="FabDoYouMeme: a private, self-hosted party meme game" />
  <meta
    name="twitter:description"
    content="A party meme game for you and your people. Self-hosted, invite-only, GDPR-first."
  />
</svelte:head>

<!-- min-h keeps the page at least one viewport tall so the layout's footer stays below the fold until content (or scrolling) pushes it down. -->
<div class="flex-1 flex flex-col items-center px-6 min-h-[calc(100vh-3.5rem)]">
  <!-- ─── Hero ────────────────────────────────────────────────── -->
  <section use:reveal class="w-full max-w-4xl flex flex-col items-center text-center gap-6 pt-20 pb-24">
    <h1 class="hero">FabDoYouMeme</h1>
    <p class="hero-sub max-w-2xl">
      A party meme game for you and your people. Host a room, share the code,
      caption, vote, laugh.
    </p>
    <p class="text-sm font-semibold text-brand-text-mid max-w-xl">
      Self-hosted, invite-only, GDPR-first. Built to stay inside your circle.
    </p>

    <!-- Above-the-fold CTA — gives cold visitors an immediate path without scrolling -->
    <div class="mt-4 flex flex-col sm:flex-row items-center justify-center gap-3">
      {#if data.user}
        <a
          href="/home"
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          class="inline-flex items-center justify-center gap-2 px-6 h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold no-underline focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
        >
          <Home size={16} strokeWidth={2.5} />
          Go to dashboard
        </a>
      {:else}
        <a
          href="/auth/magic-link"
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          class="inline-flex items-center justify-center gap-2 px-6 h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold no-underline focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
        >
          <Play size={16} strokeWidth={2.5} />
          Sign in to host
        </a>
      {/if}
      <a
        href="#join"
        onclick={(e) => {
          if (!hasScrolled) {
            // Target is gated behind hasScrolled — reveal it, then scroll once Svelte has rendered it.
            e.preventDefault();
            hasScrolled = true;
            requestAnimationFrame(() => {
              document.getElementById('join')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
            });
          }
        }}
        use:pressPhysics={'ghost'}
        use:hoverEffect={'swap'}
        class="inline-flex items-center justify-center gap-2 px-6 h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white font-bold no-underline focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
        style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
      >
        <Hash size={16} strokeWidth={2.5} />
        Join with a code
      </a>
    </div>
  </section>

  <!-- ─── How it works ────────────────────────────────────────── -->
  <section class="w-full max-w-5xl pb-24">
    <div use:reveal class="text-center mb-12">
      <div class="text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid mb-2">
        How it works
      </div>
      <h2 class="text-4xl font-bold">Three steps. One code. Go.</h2>
    </div>

    <ol class="grid grid-cols-1 md:grid-cols-3 gap-5">
      {#each steps as step, i}
        <li
          use:dealCard={{ delay: 120 + i * 160, rotate: stepTilts[i] }}
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
          <p class="text-sm font-semibold text-brand-text-mid">{step.body}</p>
        </li>
      {/each}
    </ol>
  </section>

  <!-- ─── Below-the-fold (gated behind first scroll) ──────────── -->
  {#if hasScrolled}
    <section in:fade={{ duration: 400 }} class="w-full max-w-5xl pb-24">
      <div use:reveal class="text-center mb-12">
        <div class="text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid mb-2">
          Why this exists
        </div>
        <h2 class="text-4xl font-bold">Yours. Not theirs.</h2>
        <p class="text-sm font-semibold text-brand-text-mid mt-2 max-w-xl mx-auto">
          Most party game platforms turn your laughter into someone else's
          training data. This one doesn't.
        </p>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
        {#each advantages as adv, i}
          <div
            use:dealCard={{ delay: 100 + i * 110, rotate: advantageTilts[i], smooth: true }}
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
              <p class="text-sm font-semibold text-brand-text-mid">{adv.body}</p>
            </div>
          </div>
        {/each}
      </div>
    </section>
  {:else}
    <!--
      Scroll-invite pill: small, pushed to the bottom of the initial viewport.
      flex-1 eats remaining vertical space so the button sits low rather than
      hugging the steps section above.
    -->
    <section
      out:fade={{ duration: 200 }}
      class="w-full flex-1 flex items-end justify-center pb-10"
    >
      <button
        type="button"
        onclick={revealRest}
        use:pressPhysics={'ghost'}
        class="scroll-pill group inline-flex items-center gap-2.5 px-5 py-2.5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-surface cursor-pointer transition-transform duration-200 ease-out hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
        style="box-shadow: 0 2px 0 rgba(0,0,0,0.06);"
        aria-label="Scroll to see the rest of the page"
      >
        <span class="text-xs font-semibold tracking-tight text-brand-text-mid transition-colors duration-200 group-hover:text-brand-text">
          Scroll to see why
        </span>
        <span class="scroll-chevron inline-flex items-center text-brand-text-mid transition-colors duration-200 group-hover:text-brand-text">
          <ChevronDown size={14} strokeWidth={2} />
        </span>
      </button>
    </section>
  {/if}

  <!-- ─── Final CTA (also gated so initial view stays minimal) ── -->
  {#if hasScrolled}
  <section id="join" in:fade={{ duration: 400 }} use:reveal class="w-full max-w-3xl pb-20 scroll-mt-24">
    <div
      class="rounded-[28px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-8 sm:p-10 flex flex-col items-center gap-8 text-center"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.08);"
    >
      <div class="flex flex-col items-center gap-2">
        <h2 class="text-3xl sm:text-4xl font-bold">Ready to play?</h2>
        <p class="text-sm font-semibold text-brand-text-mid max-w-md">
          Pick your lane. No wrong answer.
        </p>
      </div>

      <div class="relative w-full grid grid-cols-1 md:grid-cols-2 gap-6 md:gap-0">
        <!-- Join lane -->
        <div class="flex flex-col items-center gap-4 md:pr-8">
          <div class="inline-flex items-center gap-2 text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
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
            class="inline-flex items-center justify-center gap-2 px-6 h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white font-bold disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
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
            class="inline-flex items-center justify-center h-8 min-w-8 px-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-[0.7rem] font-bold uppercase tracking-[0.15em]"
            style="box-shadow: 0 2px 0 rgba(0,0,0,0.06);"
          >
            or
          </span>
          <span class="flex-1 md:w-[2.5px] md:flex-1 h-[2.5px] md:h-auto bg-brand-border-heavy rounded-full"></span>
        </div>

        <!-- Host lane -->
        <div class="flex flex-col items-center gap-4 md:pl-8">
          <div class="inline-flex items-center gap-2 text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
            <Sparkles size={14} strokeWidth={2.5} />
            Spinning one up?
          </div>
          <p class="text-sm font-semibold text-brand-text max-w-[16rem]">
            {data.user ? `Welcome back, ${data.user.username}. Jump straight into your dashboard.` : 'Host your own room. One account, unlimited games.'}
          </p>
          {#if data.user}
            <a
              href="/home"
              use:pressPhysics={'dark'}
              use:hoverEffect={'gradient'}
              class="mt-4 inline-flex items-center justify-center gap-2 px-6 h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold no-underline focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
              style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
            >
              <Home size={16} strokeWidth={2.5} />
              Go to dashboard
            </a>
          {:else}
            <a
              href="/auth/magic-link"
              use:pressPhysics={'dark'}
              use:hoverEffect={'gradient'}
              class="mt-4 inline-flex items-center justify-center gap-2 px-6 h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold no-underline focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
              style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
            >
              <Play size={16} strokeWidth={2.5} />
              Sign in to host
            </a>
          {/if}
        </div>
      </div>

      <p class="text-xs font-semibold text-brand-text-mid">
        {data.user ? 'You are signed in. Host or join, your call.' : 'Hosting requires an account. Joining with a code does not.'}
      </p>
    </div>
  </section>
  {/if}
</div>

<style>
  /* Slow, smooth downward drift. A ~3px travel over 2.4s with standard-curve
     easing reads as "breathing" rather than the prior 4px/1.6s jitter. */
  @keyframes scroll-float {
    0%, 100% { transform: translateY(-1px); }
    50% { transform: translateY(2px); }
  }
  .scroll-chevron {
    animation: scroll-float 2.4s cubic-bezier(0.45, 0, 0.55, 1) infinite;
  }
  @media (prefers-reduced-motion: reduce) {
    .scroll-chevron { animation: none; }
  }
</style>
