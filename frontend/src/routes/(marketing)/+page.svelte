<script lang="ts">
  import { goto } from '$app/navigation';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import type { PageData } from './$types';
  import {
    Lock,
    Server,
    Shield,
    Gamepad2,
    Home,
    Sparkles,
    Play,
    Hash,
    Sliders,
    Code2,
  } from '$lib/icons';

  let { data }: { data: PageData } = $props();
  let codeChars = $state(['', '', '', '']);
  let codeRefs: (HTMLInputElement | null)[] = $state([null, null, null, null]);
  let fan: HTMLDivElement | undefined = $state();

  const joinCode = $derived(codeChars.join(''));

  function join(next: string) {
    if (next.length !== 4) return;
    goto(`/join/${next}`);
  }

  function onCodeInput(i: number, e: Event) {
    const target = e.target as HTMLInputElement;
    const cleaned = target.value.toUpperCase().replace(/[^A-Z0-9]/g, '');
    if (cleaned.length > 1) {
      // Paste: spread across remaining tiles
      const chars = cleaned.slice(0, 4 - i).split('');
      for (let k = 0; k < chars.length; k++) codeChars[i + k] = chars[k];
      target.value = codeChars[i];
      const next = Math.min(i + chars.length, 3);
      codeRefs[next]?.focus();
      if (codeChars.join('').length === 4) join(codeChars.join(''));
      return;
    }
    codeChars[i] = cleaned;
    target.value = cleaned;
    if (cleaned && i < 3) codeRefs[i + 1]?.focus();
    if (codeChars.join('').length === 4) join(codeChars.join(''));
  }

  function onCodeKey(i: number, e: KeyboardEvent) {
    if (e.key === 'Backspace' && !codeChars[i] && i > 0) {
      codeRefs[i - 1]?.focus();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      join(joinCode);
    } else if (e.key === 'ArrowLeft' && i > 0) {
      codeRefs[i - 1]?.focus();
    } else if (e.key === 'ArrowRight' && i < 3) {
      codeRefs[i + 1]?.focus();
    }
  }

  const heroCards: { caption: string; who: string; votes: number; winner?: boolean }[] = [
    { caption: '"my wifi after three people join the voice call"', who: 'R.03 · Bob', votes: 1 },
    { caption: '"me pretending I read the group chat before replying"', who: 'R.03 · Mia', votes: 2 },
    { caption: '"dev-ops trying to debug at 2am with one working brain cell"', who: 'R.03 · Leo', votes: 3, winner: true },
    { caption: '"my ISP when I ask for upload speed"', who: 'R.03 · Jun', votes: 1 },
  ];

  const tableHand = [
    { label: 'Caption A', caption: '"when the docker compose finally comes up"', tilt: -2, votes: 2 },
    { label: 'Caption B', caption: '"reading docs after breaking prod"', tilt: 1.4, votes: 1 },
    { label: 'Caption C', caption: '"me explaining self-hosting to my partner"', tilt: -0.8, votes: 0 },
    { label: 'Caption D', caption: '"the raspberry pi that hosts our whole weekend"', tilt: 2, votes: 3 },
  ] as const;

  const steps = [
    {
      suit: '♠',
      label: 'Step I',
      n: 1,
      title: 'Host a room',
      body: 'Pick a game, pick a pack, get a 4-letter room code. Everything runs on your box — no cloud in the loop.',
      tilt: -1.5,
    },
    {
      suit: '♥',
      label: 'Step II',
      n: 2,
      title: 'Friends pile in',
      body: "They tap the code on their phone. No download, no account, no tracking. Guests get a seat at the table.",
      tilt: 0.8,
    },
    {
      suit: '♦',
      label: 'Step III',
      n: 3,
      title: 'Caption & vote',
      body: 'Round by round. The best punchline wins the hand. Scoring, timers, and reconnects are on the house.',
      tilt: -0.6,
    },
  ] as const;

  const packs = [
    {
      art: 'SUNSET',
      artClass: 'pack-art-a1',
      title: 'House Warming',
      body: 'The default pack for new labs. Lightly spicy, universally legible, safe for in-laws.',
      meta: 'System · v1.0',
      count: '84 cards',
    },
    {
      art: 'DUSK',
      artClass: 'pack-art-a2',
      title: 'Dev & Deploy',
      body: 'For the group chat that Slacks in Markdown. Sprint retros welcome. CI failures encouraged.',
      meta: 'System · v1.0',
      count: '96 cards',
    },
    {
      art: 'MINT',
      artClass: 'pack-art-a3',
      title: 'Lab Original',
      body: 'Your pack. Upload images, write prompts, version it, ship it. The studio is a full-fledged tool.',
      meta: 'Custom',
      count: 'you decide',
    },
  ] as const;

  const advantages = [
    { glyph: '♠', title: 'Self-hosted, one command', body: 'One Docker Compose stack. Old laptop, home server, €5 VPS — it runs anywhere you run containers.', Icon: Server },
    { glyph: '♥', title: 'Invite-only by design', body: 'No public signup. Single-use invite tokens, optional email restriction. You decide who sits at your table.', Icon: Lock },
    { glyph: '♦', title: 'Magic-link auth', body: 'No passwords to leak or rotate. Tokens are SHA-256 hashed in the DB, single-use, expire in 15 minutes.', Icon: Sparkles },
    { glyph: '♣', title: 'GDPR-ready', body: 'Consent capture, one-click data export, admin-driven erasure with sentinel-UUID anonymisation. Retention windows built in.', Icon: Shield },
    { glyph: '★', title: 'Multi-game platform', body: 'Meme captioning is the first handler; trivia, drawing duels, pairs, quickfire slot in as plugins. No schema churn.', Icon: Gamepad2 },
    { glyph: '⚙', title: 'Read every line', body: 'GPLv3. Go backend, SvelteKit frontend. Fork it, rebrand it, rewire it — just don’t lock it back up.', Icon: Code2 },
  ] as const;

  function onFanMove(e: MouseEvent) {
    if (!fan) return;
    if (!matchMedia('(hover: hover) and (pointer: fine)').matches) return;
    // .fan collapses to 0x0, so use the wrapper for a real hit-rect.
    const wrap = fan.parentElement ?? fan;
    const r = wrap.getBoundingClientRect();
    const x = (e.clientX - r.left) / r.width - 0.5;
    const y = (e.clientY - r.top) / r.height - 0.5;
    // Preserve the baseline translateX set in CSS.
    const shift = getComputedStyle(fan).getPropertyValue('--fan-shift') || '-40px';
    fan.style.transform = `translateX(${shift}) perspective(900px) rotateX(${y * -4}deg) rotateY(${x * 6}deg)`;
  }
  function onFanLeave() {
    if (fan) fan.style.transform = '';
  }

  function votePips(n: number, total = 3) {
    return Array.from({ length: total }, (_, i) => i < n);
  }
</script>

<svelte:head>
  <title>FabDoYouMeme — Deal yourself in.</title>
  <meta
    name="description"
    content="A party meme game for you and your people. Self-hosted, invite-only, GDPR-first. Host a room, share the code, caption, vote, laugh."
  />
  <meta property="og:type" content="website" />
  <meta property="og:title" content="FabDoYouMeme: a private, self-hosted party meme game" />
  <meta property="og:description" content="A party meme game for you and your people. Self-hosted, invite-only, GDPR-first." />
  <meta name="twitter:card" content="summary" />
  <meta name="twitter:title" content="FabDoYouMeme: a private, self-hosted party meme game" />
  <meta name="twitter:description" content="A party meme game for you and your people. Self-hosted, invite-only, GDPR-first." />
</svelte:head>

<!-- ─── HERO ─────────────────────────────────────────────────── -->
<section class="hero-grid mx-auto w-full max-w-[1180px] px-6 pt-14 pb-24 md:pb-32">
  <div class="hero-text" use:reveal>
    <span class="hero-mark">
      <span class="hero-mark-suit">♠</span>
      A party game you host yourself.
    </span>
    <h1 class="hero-title mt-12">
      Deal yourself <em>in.</em>
      <span class="line2">Then deal your friends a punchline.</span>
    </h1>
    <p class="hero-sub max-w-[520px] mt-12">
      FabDoYouMeme is a self-hosted, invite-only party meme game. Spin up a room, share a 4-letter code, caption the deck, vote on the funniest. No accounts for players. No data leaves your lab.
    </p>

    <div class="mt-10 flex flex-wrap gap-3.5">
      {#if data.user}
        <a
          href="/home"
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          class="btn btn-lg btn-dark"
        >
          <Home size={18} strokeWidth={2.5} />
          Go to dashboard
        </a>
      {:else}
        <a
          href="/auth/magic-link"
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          class="btn btn-lg btn-dark"
        >
          <span aria-hidden="true" class="text-lg leading-none">♠</span>
          Deal me in
        </a>
      {/if}
      <a
        href="#round"
        use:pressPhysics={'ghost'}
        use:hoverEffect={'swap'}
        class="btn btn-lg btn-ghost"
      >
        See a round →
      </a>
    </div>

    <div class="mt-10 flex flex-wrap gap-4 text-[13px] font-bold">
      <span class="chip"><span class="glyph">01</span> Magic-link auth</span>
      <span class="chip"><span class="glyph">02</span> GDPR-first</span>
      <span class="chip"><span class="glyph">03</span> GPLv3, no ads</span>
    </div>
  </div>

  <div class="fan-wrap" use:reveal>
    <div class="float f1" aria-hidden="true"></div>
    <div
      bind:this={fan}
      class="fan"
      onmousemove={onFanMove}
      onmouseleave={onFanLeave}
      role="presentation"
    >
      {#each heroCards as c, i (i)}
        <article class="card fan-card" class:winner={c.winner} data-pos={i + 1}>
          <div class="card-stripe">[ MEME IMAGE ]</div>
          <div class="card-caption">{c.caption}</div>
          <div class="card-foot">
            <span>{c.who}</span>
            <span class="votes">
              {#each votePips(c.votes) as on, vi (vi)}
                <span class="vote-pip" class:off={!on}></span>
              {/each}
            </span>
          </div>
        </article>
      {/each}
    </div>
    <div class="float f2" aria-hidden="true"></div>
  </div>
</section>

<!-- ─── LIVE ROUND ────────────────────────────────────────────── -->
<section id="round" class="mx-auto w-full max-w-[1180px] px-6 py-24">
  <div use:reveal class="text-center mb-14">
    <span class="label"><span class="label-dot"></span> Round 3 of 5 · Voting phase</span>
    <h2 class="section-h">A table, not a feed.</h2>
    <p class="section-p">Every session plays like a real card table. A timer. A hand. Four captions. One winner. Then shuffle and deal again.</p>
  </div>

  <div class="table" use:reveal>
    <div class="table-top">
      <div class="timer"><span class="timer-dot"></span>00:24</div>
      <div class="room-code" aria-label="Room code">
        <span>L</span><span>A</span><span>B</span><span>7</span>
      </div>
      <div class="players">
        <span class="avatar">BK</span>
        <span class="avatar a2">MI</span>
        <span class="avatar a3">LE</span>
        <span class="avatar a4">JN</span>
        <span class="avatar a5">+2</span>
      </div>
    </div>
    <div class="hand">
      {#each tableHand as h, i (i)}
        <article class="card hand-card" style="transform: rotate({h.tilt}deg);" use:physCard>
          <div class="card-stripe">[ PROMPT IMAGE ]</div>
          <div class="card-caption">{h.caption}</div>
          <div class="card-foot">
            <span>{h.label}</span>
            <span class="votes">
              {#each votePips(h.votes) as on, vi (vi)}
                <span class="vote-pip" class:off={!on}></span>
              {/each}
            </span>
          </div>
        </article>
      {/each}
    </div>
  </div>
</section>

<!-- ─── HOW IT WORKS ──────────────────────────────────────────── -->
<section id="how" class="mx-auto w-full max-w-[1180px] px-6 py-24">
  <div use:reveal class="text-center mb-14">
    <span class="label">How to play</span>
    <h2 class="section-h">Shuffle. Deal. Laugh. Repeat.</h2>
    <p class="section-p">Three steps, one code, zero friction. Your friends don’t sign up — they just show up.</p>
  </div>
  <div class="steps">
    {#each steps as s, i (i)}
      <article class="step-card" style="transform: rotate({s.tilt}deg);" use:reveal use:physCard>
        <div class="step-suit">{s.suit} {s.label}</div>
        <div class="step-num">{s.n}</div>
        <h3 class="step-title">{s.title}</h3>
        <p class="step-body">{s.body}</p>
      </article>
    {/each}
  </div>
</section>

<!-- ─── PACKS ─────────────────────────────────────────────────── -->
<section id="packs" class="mx-auto w-full max-w-[1180px] px-6 py-24">
  <div use:reveal class="text-center mb-14">
    <span class="label">Decks shipped in v1</span>
    <h2 class="section-h">Open a fresh pack.</h2>
    <p class="section-p">Start with a system pack or forge your own in the Lab. Every maker can upload images, write prompts, and publish decks to their instance.</p>
  </div>
  <div class="packs">
    {#each packs as p, i (i)}
      <article class="pack" use:reveal>
        <div class="pack-art {p.artClass}">[ DECK ART · {p.art} ]</div>
        <h3 class="pack-title">{p.title}</h3>
        <p class="pack-body">{p.body}</p>
        <div class="pack-meta">
          <span>{p.meta}</span>
          <span class="pack-count">{p.count}</span>
        </div>
      </article>
    {/each}
  </div>
</section>

<!-- ─── WHY SELF-HOST ─────────────────────────────────────────── -->
<section id="why" class="mx-auto w-full max-w-[1180px] px-6 py-24">
  <div use:reveal class="text-center mb-14">
    <span class="label">The small print (but fun)</span>
    <h2 class="section-h">Yours. Not theirs.</h2>
    <p class="section-p">Most party platforms turn your laughter into somebody’s training data. This one doesn’t — it’s GPLv3, self-hosted, and boring where it counts.</p>
  </div>
  <div class="why">
    {#each advantages as a, i (i)}
      <div class="why-row" use:reveal>
        <div class="why-icon" aria-hidden="true">{a.glyph}</div>
        <div>
          <h4 class="why-title">{a.title}</h4>
          <p class="why-body">{a.body}</p>
        </div>
      </div>
    {/each}
  </div>
</section>

<!-- ─── FINAL CTA ─────────────────────────────────────────────── -->
<section id="join" class="mx-auto w-full max-w-[1180px] px-6 pt-10 pb-20 scroll-mt-20">
  <div class="final-card" use:reveal>
    <span class="label"><span class="label-dot"></span> Table’s open</span>
    <h2 class="section-h mt-1">Ready to play?</h2>
    <p class="section-p">Pick your lane. No wrong answer — and yes, the Lab Master always eats last.</p>

    <div class="final-lanes">
      <div class="lane">
        <div class="label">♠ Got a code?</div>
        <h3 class="lane-title">Join the room</h3>
        <div class="code-input" role="group" aria-label="4-letter room code">
          {#each codeChars as _ch, i (i)}
            <input
              bind:this={codeRefs[i]}
              value={codeChars[i]}
              oninput={(e) => onCodeInput(i, e)}
              onkeydown={(e) => onCodeKey(i, e)}
              type="text"
              inputmode="text"
              autocapitalize="characters"
              autocomplete="off"
              maxlength={i === 0 ? 4 : 1}
              aria-label={`Letter ${i + 1}`}
            />
          {/each}
        </div>
        <button
          type="button"
          use:pressPhysics={'ghost'}
          use:hoverEffect={'swap'}
          disabled={joinCode.length !== 4}
          onclick={() => join(joinCode)}
          class="btn disabled:opacity-40 disabled:cursor-not-allowed"
        >
          <Play size={16} strokeWidth={2.5} />
          Join the table
        </button>
      </div>

      <div class="lane-divider" aria-hidden="true">
        <span class="lane-bar"></span>
        <span class="lane-or">OR</span>
        <span class="lane-bar"></span>
      </div>

      <div class="lane">
        <div class="label">★ Spinning one up?</div>
        <h3 class="lane-title">Host your own lab</h3>
        <p class="lane-body">
          {data.user ? `Welcome back, ${data.user.username}. Jump straight in.` : 'One account, unlimited rooms. Packs, players, rounds — all yours.'}
        </p>
        {#if data.user}
          <a
            href="/home"
            use:pressPhysics={'dark'}
            use:hoverEffect={'gradient'}
            class="btn btn-dark"
          >
            <Home size={16} strokeWidth={2.5} />
            Go to dashboard
          </a>
        {:else}
          <a
            href="/auth/magic-link"
            use:pressPhysics={'dark'}
            use:hoverEffect={'gradient'}
            class="btn btn-dark"
          >
            <Sparkles size={16} strokeWidth={2.5} />
            Sign in with magic link
          </a>
        {/if}
      </div>
    </div>

    <p class="text-[12px] font-semibold text-brand-text-muted">
      Hosting needs an account. Joining with a code does not.
    </p>
  </div>
</section>

<style>
  /* ─── Buttons (page-local pill spec matches the design) ────── */
  .btn {
    display: inline-flex; align-items: center; justify-content: center; gap: 10px;
    font-family: inherit; font-weight: 700; font-size: 15px;
    padding: 0 22px; height: 48px;
    border-radius: 999px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white); color: var(--brand-text);
    text-decoration: none; cursor: pointer;
    box-shadow: 0 5px 0 rgba(0, 0, 0, 0.22);
    white-space: nowrap;
  }
  .btn-dark {
    background: var(--brand-text); color: var(--brand-white);
    box-shadow: 0 5px 0 rgba(0, 0, 0, 0.35);
  }
  .btn-ghost {
    background: transparent;
    box-shadow: 0 4px 0 rgba(0, 0, 0, 0.1);
  }
  .btn-lg { height: 56px; font-size: 16px; padding: 0 28px; }

  /* ─── Hero ─────────────────────────────────────────────────── */
  /* Design uses a 1.05:1 grid so the title column has ~560px at cap —
     enough to wrap the 6rem headline to 3-4 lines. Any narrower and it
     wraps to 6+ lines and blows up the hero vertically. */
  .hero-grid {
    display: grid;
    grid-template-columns: 1.05fr 1fr;
    gap: 40px;
    align-items: center;
  }
  .hero-grid > * { min-width: 0; }
  @media (max-width: 960px) {
    .hero-grid {
      grid-template-columns: 1fr;
      gap: 24px;
      padding-bottom: 20px;
    }
  }

  .hero-mark {
    display: inline-flex; align-items: center; gap: 12px;
    padding: 6px 14px 6px 8px;
    border-radius: 999px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white);
    box-shadow: 0 3px 0 rgba(0, 0, 0, 0.12);
    font-size: 13px; font-weight: 700; letter-spacing: 0.02em;
  }
  .hero-mark-suit {
    width: 26px; height: 26px; border-radius: 8px;
    background: var(--brand-text); color: var(--brand-white);
    display: grid; place-items: center;
    font-weight: 700; font-size: 14px;
    transform: rotate(-6deg);
  }

  .hero-title {
    font-size: clamp(3rem, 7.5vw, 6rem);
    font-weight: 700;
    letter-spacing: -0.04em;
    line-height: 0.95;
    text-wrap: balance;
  }
  .hero-title :global(em) {
    font-style: normal;
    background: linear-gradient(95deg, var(--brand-accent) 0%, #C89DBF 60%, #7CB5A1 100%);
    -webkit-background-clip: text;
    background-clip: text;
    color: transparent;
    padding-right: 4px;
  }
  .hero-title .line2 {
    display: block;
    font-style: italic;
    font-weight: 700;
    color: var(--brand-text-mid);
  }
  .hero-sub {
    font-size: 19px;
    color: var(--brand-text-mid);
    line-height: 1.45;
    font-weight: 600;
  }
  .chip {
    display: inline-flex; align-items: center; gap: 8px;
    padding: 8px 14px; border-radius: 999px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white);
    color: var(--brand-text);
    box-shadow: 0 2px 0 rgba(0, 0, 0, 0.08);
  }
  .chip :global(.glyph) {
    font-family: 'JetBrains Mono', ui-monospace, monospace;
    color: var(--brand-accent);
  }

  /* ─── Card fan ─────────────────────────────────────────────── */
  /* Fan is anchored to the top of the row (align-self: start) and uses
     `place-items: start center` so card positions don't drift with the
     title column's line-count. Padding-top sets the fan's anchor Y; card
     Y-translates fan out from there. */
  .fan-wrap {
    position: relative;
    align-self: start;
    height: 500px;
    padding-top: 80px;
    display: grid;
    place-items: start center;
  }
  @media (max-width: 960px) {
    .fan-wrap {
      align-self: center;
      height: 440px;
      padding-top: 0;
      place-items: center;
      margin-top: 10px;
    }
  }

  .card {
    background: var(--brand-white);
    border: 2.5px solid var(--brand-border-heavy);
    border-radius: 22px;
    box-shadow: 0 6px 0 rgba(0, 0, 0, 0.10);
    padding: 18px;
    display: flex; flex-direction: column; gap: 10px;
  }
  .fan-card {
    position: absolute;
    width: 240px; height: 340px;
    transition: transform 0.4s cubic-bezier(0.22, 1, 0.36, 1);
  }
  .card-stripe {
    height: 130px; border-radius: 14px;
    border: 2.5px solid var(--brand-border-heavy);
    background:
      repeating-linear-gradient(45deg, rgba(26, 26, 26, 0.06) 0 10px, transparent 10px 20px),
      linear-gradient(180deg, var(--brand-grad-1), var(--brand-grad-3));
    display: grid; place-items: center;
    font-family: 'JetBrains Mono', ui-monospace, monospace;
    font-size: 10px; color: var(--brand-text-mid);
    letter-spacing: 0.1em; text-transform: uppercase;
  }
  .card-caption {
    flex: 1;
    font-size: 15px; font-weight: 700; line-height: 1.25;
    color: var(--brand-text);
    display: flex; align-items: center;
    text-wrap: balance;
  }
  .card-foot {
    display: flex; align-items: center; justify-content: space-between;
    font-size: 11px; font-weight: 700; letter-spacing: 0.1em;
    text-transform: uppercase; color: var(--brand-text-muted);
  }
  .votes { display: inline-flex; gap: 4px; }
  .vote-pip {
    width: 10px; height: 10px; border-radius: 50%;
    border: 2px solid var(--brand-border-heavy);
    background: var(--brand-accent);
  }
  .vote-pip.off { background: transparent; }

  .fan {
    position: relative;
    width: 0;
    height: 0;
    --fan-shift: -90px;
    transform: translateX(var(--fan-shift));
    transition: transform 0.2s ease;
  }
  @media (max-width: 960px) {
    .fan { --fan-shift: 0px; }
  }
  .fan .fan-card[data-pos='1'] { transform: translate(-170px, 20px) rotate(-14deg); }
  .fan .fan-card[data-pos='2'] { transform: translate(-55px, -8px) rotate(-4deg); z-index: 2; }
  .fan .fan-card[data-pos='3'] { transform: translate(70px, 6px) rotate(6deg); z-index: 3; box-shadow: 0 8px 0 rgba(0, 0, 0, 0.14); }
  .fan .fan-card[data-pos='4'] { transform: translate(185px, 40px) rotate(16deg); z-index: 1; }

  .fan:hover .fan-card[data-pos='1'] { transform: translate(-210px, 20px) rotate(-20deg); }
  .fan:hover .fan-card[data-pos='2'] { transform: translate(-72px, -22px) rotate(-8deg); }
  .fan:hover .fan-card[data-pos='3'] { transform: translate(80px, -14px) rotate(10deg); }
  .fan:hover .fan-card[data-pos='4'] { transform: translate(230px, 50px) rotate(22deg); }

  .winner { position: relative; }
  .winner::after {
    content: '★ WINNER';
    position: absolute; top: -14px; right: -14px;
    background: var(--brand-text); color: var(--brand-white);
    font-size: 10px; letter-spacing: 0.18em; font-weight: 700;
    padding: 7px 10px; border-radius: 999px;
    border: 2.5px solid var(--brand-border-heavy);
    box-shadow: 0 3px 0 rgba(0, 0, 0, 0.3);
    transform: rotate(6deg);
  }

  /* Floating ambient cards — decorative backdrop, must sit behind the fan */
  .float {
    position: absolute; pointer-events: none; opacity: 0.7;
    width: 120px; height: 170px; border-radius: 16px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white);
    box-shadow: 0 6px 0 rgba(0, 0, 0, 0.10);
    animation: floaty 9s ease-in-out infinite;
    z-index: 0;
  }
  .fan { z-index: 1; }
  .float::after {
    content: ''; position: absolute; inset: 10px; border-radius: 10px;
    background:
      repeating-linear-gradient(45deg, rgba(26, 26, 26, 0.05) 0 8px, transparent 8px 16px),
      linear-gradient(180deg, var(--brand-grad-4), var(--brand-grad-1));
    border: 2px solid var(--brand-border);
  }
  .float.f1 { top: 40px; left: -60px; transform: rotate(-14deg); animation-delay: -2s; }
  .float.f2 { bottom: -40px; right: -40px; transform: rotate(18deg); animation-delay: -5s; width: 140px; height: 200px; }
  @keyframes floaty { 0%, 100% { translate: 0 0; } 50% { translate: 0 -14px; } }
  @media (prefers-reduced-motion: reduce) { .float { animation: none; } }

  /* ─── Section heads ────────────────────────────────────────── */
  .section-h {
    font-size: clamp(2.2rem, 4vw, 3.2rem); font-weight: 700;
    letter-spacing: -0.02em; margin: 8px 0 10px; text-wrap: balance;
  }
  .section-p {
    color: var(--brand-text-mid); font-size: 17px;
    max-width: 620px; margin: 0 auto; line-height: 1.45; font-weight: 600;
  }
  .label {
    display: inline-flex; align-items: center; gap: 8px;
    font-size: 11px; font-weight: 700; letter-spacing: 0.2em;
    text-transform: uppercase; color: var(--brand-text-mid);
  }
  .label-dot {
    width: 7px; height: 7px; border-radius: 50%;
    background: var(--brand-accent);
    box-shadow: 0 0 0 2px rgba(232, 147, 127, 0.25);
    animation: pulse 2s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { transform: scale(1); opacity: 1; }
    50% { transform: scale(1.3); opacity: 0.6; }
  }

  /* ─── Live-round table ─────────────────────────────────────── */
  .table {
    position: relative;
    background: radial-gradient(ellipse at 50% 40%, rgba(255, 255, 255, 0.7), rgba(255, 255, 255, 0.45));
    border: 2.5px solid var(--brand-border-heavy);
    border-radius: 28px;
    padding: 40px 28px 44px;
    box-shadow: 0 8px 0 rgba(0, 0, 0, 0.10);
  }
  .table::before {
    content: ''; position: absolute; inset: 10px; border-radius: 20px;
    border: 2px dashed var(--brand-border); pointer-events: none;
  }
  .table-top {
    display: flex; justify-content: space-between; align-items: center;
    margin-bottom: 28px; gap: 14px; flex-wrap: wrap;
  }
  .timer {
    display: inline-flex; align-items: center; gap: 10px;
    padding: 8px 16px; border-radius: 999px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white);
    font-size: 22px; font-weight: 700; font-variant-numeric: tabular-nums;
    box-shadow: 0 3px 0 rgba(0, 0, 0, 0.12);
  }
  .timer-dot {
    width: 10px; height: 10px; border-radius: 50%;
    background: var(--brand-accent);
    animation: pulse 1.4s infinite;
  }
  .room-code {
    display: inline-flex; gap: 6px;
    font-family: 'JetBrains Mono', ui-monospace, monospace;
    font-weight: 700; font-size: 18px;
  }
  .room-code span {
    display: grid; place-items: center;
    width: 36px; height: 44px;
    background: var(--brand-white);
    border: 2.5px solid var(--brand-border-heavy);
    border-radius: 10px;
    box-shadow: 0 3px 0 rgba(0, 0, 0, 0.15);
  }
  .players { display: flex; align-items: center; }
  .avatar {
    width: 36px; height: 36px; border-radius: 50%;
    border: 2.5px solid var(--brand-border-heavy);
    background: #7CB5A1; color: var(--brand-white);
    display: grid; place-items: center;
    font-size: 12px; font-weight: 700;
    margin-left: -8px;
    box-shadow: 0 2px 0 rgba(0, 0, 0, 0.2);
  }
  .avatar:first-child { margin-left: 0; }
  .avatar.a2 { background: var(--brand-accent); }
  .avatar.a3 { background: #C89DBF; }
  .avatar.a4 { background: var(--brand-text); }
  .avatar.a5 { background: var(--brand-grad-4); color: var(--brand-text); }

  .hand {
    display: grid; grid-template-columns: repeat(4, 1fr); gap: 18px;
  }
  .hand-card { position: relative; min-height: 320px; }
  @media (max-width: 1060px) { .hand { grid-template-columns: repeat(2, 1fr); } }
  @media (max-width: 560px) { .hand { grid-template-columns: 1fr; } }

  /* ─── Steps ────────────────────────────────────────────────── */
  .steps {
    display: grid; grid-template-columns: repeat(3, 1fr); gap: 20px;
  }
  @media (max-width: 880px) { .steps { grid-template-columns: 1fr; } }
  .step-card {
    background: var(--brand-surface);
    border: 2.5px solid var(--brand-border-heavy);
    border-radius: 22px;
    padding: 26px;
    box-shadow: 0 6px 0 rgba(0, 0, 0, 0.10);
    position: relative;
  }
  .step-num {
    width: 44px; height: 44px; border-radius: 999px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-text); color: var(--brand-white);
    display: grid; place-items: center;
    font-weight: 700; font-size: 18px;
    box-shadow: 0 3px 0 rgba(0, 0, 0, 0.25);
    margin-bottom: 14px;
  }
  .step-title { font-size: 22px; margin: 0 0 8px; letter-spacing: -0.01em; font-weight: 700; }
  .step-body { color: var(--brand-text-mid); font-size: 15px; line-height: 1.45; margin: 0; font-weight: 600; }
  .step-suit {
    position: absolute; top: 20px; right: 20px;
    font-family: 'JetBrains Mono', ui-monospace, monospace;
    font-size: 11px; letter-spacing: 0.2em;
    color: var(--brand-text-muted);
    text-transform: uppercase;
  }

  /* ─── Packs ────────────────────────────────────────────────── */
  .packs {
    display: grid; grid-template-columns: repeat(3, 1fr); gap: 22px;
  }
  @media (max-width: 880px) { .packs { grid-template-columns: 1fr; } }
  .pack {
    position: relative; padding: 28px;
    background: var(--brand-white);
    border: 2.5px solid var(--brand-border-heavy);
    border-radius: 22px;
    box-shadow: 0 6px 0 rgba(0, 0, 0, 0.10);
    transition: transform 0.3s cubic-bezier(0.22, 1, 0.36, 1);
  }
  .pack:hover { transform: translateY(-6px); }
  .pack::before,
  .pack::after {
    content: ''; position: absolute; inset: 0;
    background: var(--brand-white);
    border: 2.5px solid var(--brand-border-heavy);
    border-radius: 22px;
    z-index: -1;
  }
  .pack::before { transform: translate(6px, 8px) rotate(3deg); opacity: 0.85; }
  .pack::after { transform: translate(-6px, 10px) rotate(-4deg); opacity: 0.65; }
  .pack-art {
    height: 180px; border-radius: 14px;
    border: 2.5px solid var(--brand-border-heavy);
    margin-bottom: 18px;
    display: grid; place-items: center;
    font-family: 'JetBrains Mono', ui-monospace, monospace;
    font-size: 11px; color: var(--brand-text-mid);
    letter-spacing: 0.2em; text-transform: uppercase;
  }
  .pack-art-a1 { background: linear-gradient(150deg, var(--brand-grad-1), var(--brand-accent)); }
  .pack-art-a2 { background: linear-gradient(150deg, var(--brand-grad-3), #C89DBF); }
  .pack-art-a3 { background: linear-gradient(150deg, var(--brand-grad-4), #7CB5A1); }
  .pack-title { margin: 0 0 6px; font-size: 22px; letter-spacing: -0.01em; font-weight: 700; }
  .pack-body { margin: 0; font-size: 14px; color: var(--brand-text-mid); line-height: 1.45; font-weight: 600; }
  .pack-meta {
    display: flex; justify-content: space-between; align-items: center;
    margin-top: 12px; font-size: 12px; font-weight: 700;
    color: var(--brand-text-mid);
    letter-spacing: 0.1em; text-transform: uppercase;
  }
  .pack-count {
    padding: 4px 10px; border-radius: 999px;
    border: 2px solid var(--brand-border-heavy);
    background: var(--brand-white);
    color: var(--brand-text);
  }

  /* ─── Why self-host ────────────────────────────────────────── */
  .why {
    display: grid; grid-template-columns: repeat(2, 1fr); gap: 16px;
  }
  @media (max-width: 760px) { .why { grid-template-columns: 1fr; } }
  .why-row {
    display: flex; align-items: flex-start; gap: 16px;
    padding: 20px 22px; border-radius: 22px;
    background: var(--brand-surface);
    border: 2.5px solid var(--brand-border-heavy);
    box-shadow: 0 6px 0 rgba(0, 0, 0, 0.10);
  }
  .why-icon {
    flex-shrink: 0; width: 46px; height: 46px; border-radius: 999px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white);
    display: grid; place-items: center;
    box-shadow: 0 3px 0 rgba(0, 0, 0, 0.12);
    font-size: 20px;
  }
  .why-title { margin: 0 0 4px; font-size: 18px; letter-spacing: -0.01em; font-weight: 700; }
  .why-body { margin: 0; font-size: 14px; color: var(--brand-text-mid); line-height: 1.45; font-weight: 600; }

  /* ─── Final CTA ────────────────────────────────────────────── */
  .final-card {
    background: var(--brand-white);
    border: 2.5px solid var(--brand-border-heavy);
    border-radius: 28px;
    padding: 44px 32px;
    box-shadow: 0 8px 0 rgba(0, 0, 0, 0.12);
    text-align: center;
    display: flex; flex-direction: column; align-items: center; gap: 24px;
  }
  .final-lanes {
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    gap: 20px; align-items: center;
    width: 100%; max-width: 780px; margin-top: 10px;
  }
  @media (max-width: 760px) {
    .final-lanes { grid-template-columns: 1fr; }
    .lane-divider { display: none; }
  }
  .lane {
    display: flex; flex-direction: column; align-items: center;
    gap: 14px; padding: 16px;
  }
  .lane-title { margin: 0; font-size: 18px; letter-spacing: -0.01em; font-weight: 700; }
  .lane-body {
    font-size: 14px; color: var(--brand-text-mid);
    margin: 0; max-width: 220px; line-height: 1.45; font-weight: 600;
  }
  .code-input {
    display: flex; gap: 8px;
  }
  .code-input input {
    width: 46px; height: 56px; text-align: center;
    font-family: 'JetBrains Mono', ui-monospace, monospace;
    font-weight: 700; font-size: 26px;
    border-radius: 12px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white);
    color: var(--brand-text);
    box-shadow: 0 3px 0 rgba(0, 0, 0, 0.15);
    transition: transform 0.1s, box-shadow 0.2s;
    text-transform: uppercase;
  }
  .code-input input:focus {
    outline: none;
    transform: translateY(-2px);
    box-shadow: 0 5px 0 rgba(0, 0, 0, 0.22),
                0 0 0 3px rgba(232, 147, 127, 0.3);
  }

  .lane-divider {
    display: flex; flex-direction: column; align-items: center; gap: 10px;
  }
  .lane-bar { width: 2.5px; height: 60px; background: var(--brand-border-heavy); border-radius: 2px; }
  .lane-or {
    padding: 4px 12px; border-radius: 999px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white);
    font-size: 12px; font-weight: 700; letter-spacing: 0.2em;
    box-shadow: 0 2px 0 rgba(0, 0, 0, 0.08);
  }
</style>
