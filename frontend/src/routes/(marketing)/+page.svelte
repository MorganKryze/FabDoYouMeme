<script lang="ts">
  import { goto } from '$app/navigation';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';
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

  const heroCards = $derived([
    { caption: m.marketing_hero_card1_caption(), who: m.marketing_hero_card1_who(), votes: 1 },
    { caption: m.marketing_hero_card2_caption(), who: m.marketing_hero_card2_who(), votes: 2 },
    { caption: m.marketing_hero_card3_caption(), who: m.marketing_hero_card3_who(), votes: 3, winner: true },
    { caption: m.marketing_hero_card4_caption(), who: m.marketing_hero_card4_who(), votes: 1 },
  ] as { caption: string; who: string; votes: number; winner?: boolean }[]);

  const tableHand = $derived([
    { label: m.marketing_table_card1_label(), caption: m.marketing_table_card1_caption(), tilt: -2, votes: 2 },
    { label: m.marketing_table_card2_label(), caption: m.marketing_table_card2_caption(), tilt: 1.4, votes: 1 },
    { label: m.marketing_table_card3_label(), caption: m.marketing_table_card3_caption(), tilt: -0.8, votes: 0 },
    { label: m.marketing_table_card4_label(), caption: m.marketing_table_card4_caption(), tilt: 2, votes: 3 },
  ]);

  const steps = $derived([
    { suit: '♠', label: m.marketing_step1_label(), n: 1, title: m.marketing_step1_title(), body: m.marketing_step1_body(), tilt: -1.5 },
    { suit: '♥', label: m.marketing_step2_label(), n: 2, title: m.marketing_step2_title(), body: m.marketing_step2_body(), tilt: 0.8 },
    { suit: '♦', label: m.marketing_step3_label(), n: 3, title: m.marketing_step3_title(), body: m.marketing_step3_body(), tilt: -0.6 },
  ]);

  const packs = $derived([
    { art: m.marketing_pack1_art(), artClass: 'pack-art-a1', title: m.marketing_pack1_title(), body: m.marketing_pack1_body(), meta: m.marketing_pack1_meta(), count: m.marketing_pack1_count() },
    { art: m.marketing_pack2_art(), artClass: 'pack-art-a2', title: m.marketing_pack2_title(), body: m.marketing_pack2_body(), meta: m.marketing_pack2_meta(), count: m.marketing_pack2_count() },
    { art: m.marketing_pack3_art(), artClass: 'pack-art-a3', title: m.marketing_pack3_title(), body: m.marketing_pack3_body(), meta: m.marketing_pack3_meta(), count: m.marketing_pack3_count() },
  ]);

  const advantages = $derived([
    { glyph: '♠', title: m.marketing_adv1_title(), body: m.marketing_adv1_body(), Icon: Server },
    { glyph: '♥', title: m.marketing_adv2_title(), body: m.marketing_adv2_body(), Icon: Lock },
    { glyph: '♦', title: m.marketing_adv3_title(), body: m.marketing_adv3_body(), Icon: Sparkles },
    { glyph: '♣', title: m.marketing_adv4_title(), body: m.marketing_adv4_body(), Icon: Shield },
    { glyph: '★', title: m.marketing_adv5_title(), body: m.marketing_adv5_body(), Icon: Gamepad2 },
    { glyph: null, title: m.marketing_adv6_title(), body: m.marketing_adv6_body(), Icon: Code2 },
  ]);

  function onFanMove(e: MouseEvent) {
    if (!fan) return;
    if (!matchMedia('(hover: hover) and (pointer: fine)').matches) return;
    // Handler is now on .fan-wrap (which has a real hit-rect); .fan itself
    // is 0×0 and would never receive mousemove on its empty top region.
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const x = (e.clientX - r.left) / r.width - 0.5;
    const y = (e.clientY - r.top) / r.height - 0.5;
    // Preserve the baseline translateX set in CSS.
    const shift = getComputedStyle(fan).getPropertyValue('--fan-shift') || '0px';
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
  <title>{m.marketing_page_title()}</title>
  <meta name="description" content={m.marketing_meta_description()} />
  <meta property="og:type" content="website" />
  <meta property="og:title" content={m.marketing_og_title()} />
  <meta property="og:description" content={m.marketing_og_description()} />
  <meta name="twitter:card" content="summary" />
  <meta name="twitter:title" content={m.marketing_og_title()} />
  <meta name="twitter:description" content={m.marketing_og_description()} />
</svelte:head>

<!-- ─── HERO ─────────────────────────────────────────────────── -->
<section class="hero-grid mx-auto w-full max-w-[1180px] px-6 pt-14 pb-24 md:pb-32 overflow-x-clip">
  <div class="hero-text" use:reveal>
    <span class="hero-mark">
      <span class="hero-mark-suit">♠</span>
      {m.marketing_hero_mark()}
    </span>
    <h1 class="hero-title mt-12">
      {m.marketing_hero_title_line1()}<em>{m.marketing_hero_title_line1_em()}</em>
      <span class="line2">{m.marketing_hero_title_line2()}</span>
    </h1>
    <p class="hero-sub max-w-[520px] mt-12">
      {m.marketing_hero_sub()}
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
          {m.marketing_hero_cta_dashboard()}
        </a>
      {:else}
        <a
          href="#join"
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          class="btn btn-lg btn-dark"
        >
          <span aria-hidden="true" class="text-lg leading-none">♠</span>
          {m.marketing_hero_cta_deal_in()}
        </a>
      {/if}
      <a
        href="#round"
        use:pressPhysics={'ghost'}
        use:hoverEffect={'swap'}
        class="btn btn-lg btn-ghost"
      >
        {m.marketing_hero_cta_see_round()}
      </a>
    </div>

    <div class="mt-10 flex flex-wrap gap-4 text-[13px] font-bold">
      <span class="chip"><span class="glyph">01</span> {m.marketing_hero_chip_auth()}</span>
      <span class="chip"><span class="glyph">02</span> {m.marketing_hero_chip_gdpr()}</span>
      <span class="chip"><span class="glyph">03</span> {m.marketing_hero_chip_license()}</span>
    </div>
  </div>

  <div
    class="fan-wrap"
    use:reveal
    onmousemove={onFanMove}
    onmouseleave={onFanLeave}
    role="presentation"
  >
    <div class="float f1" aria-hidden="true"></div>
    <div bind:this={fan} class="fan">
      {#each heroCards as c, i (i)}
        <article class="card fan-card" class:winner={c.winner} data-pos={i + 1} data-winner-label={m.marketing_hero_winner_badge()}>
          <div class="card-stripe">{m.marketing_hero_stripe_meme()}</div>
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
    <span class="label"><span class="label-dot"></span> {m.marketing_round_label()}</span>
    <h2 class="section-h">{m.marketing_round_heading()}</h2>
    <p class="section-p">{m.marketing_round_body()}</p>
  </div>

  <div class="table" use:reveal>
    <div class="table-top">
      <div class="timer"><span class="timer-dot"></span>00:24</div>
      <div class="room-code" aria-label={m.marketing_round_code_aria()}>
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
          <div class="card-stripe">{m.marketing_round_stripe_prompt()}</div>
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
    <span class="label">{m.marketing_how_label()}</span>
    <h2 class="section-h">{m.marketing_how_heading()}</h2>
    <p class="section-p">{m.marketing_how_body()}</p>
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
    <span class="label">{m.marketing_packs_label()}</span>
    <h2 class="section-h">{m.marketing_packs_heading()}</h2>
    <p class="section-p">{m.marketing_packs_body()}</p>
  </div>
  <div class="packs">
    {#each packs as p, i (i)}
      <article class="pack" use:reveal>
        <div class="pack-art {p.artClass}">{m.marketing_pack_art_prefix()}{p.art}{m.marketing_pack_art_suffix()}</div>
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
    <span class="label">{m.marketing_why_label()}</span>
    <h2 class="section-h">{m.marketing_why_heading()}</h2>
    <p class="section-p">{m.marketing_why_body()}</p>
  </div>
  <div class="why">
    {#each advantages as a, i (i)}
      <div class="why-row" use:reveal>
        <div class="why-icon" aria-hidden="true">
          {#if a.glyph}{a.glyph}{:else}<a.Icon size={22} strokeWidth={2.25} />{/if}
        </div>
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
    <span class="label"><span class="label-dot"></span> {m.marketing_final_label()}</span>
    <h2 class="section-h mt-1">{m.marketing_final_heading()}</h2>
    <p class="section-p">{m.marketing_final_body()}</p>

    <div class="final-lanes">
      <div class="lane">
        <div class="label">{m.marketing_final_lane1_label()}</div>
        <h3 class="lane-title">{m.marketing_final_lane1_title()}</h3>
        <div class="code-input" role="group" aria-label={m.marketing_final_lane1_code_aria()}>
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
              aria-label={m.marketing_final_lane1_letter_aria({ n: i + 1 })}
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
          {m.marketing_final_lane1_cta()}
        </button>
      </div>

      <div class="lane-divider" aria-hidden="true">
        <span class="lane-bar"></span>
        <span class="lane-or">{m.marketing_final_or()}</span>
        <span class="lane-bar"></span>
      </div>

      <div class="lane">
        <div class="label">{m.marketing_final_lane2_label()}</div>
        <h3 class="lane-title">{m.marketing_final_lane2_title()}</h3>
        <p class="lane-body">
          {data.user ? m.marketing_final_lane2_body_user({ name: data.user.username }) : m.marketing_final_lane2_body_guest()}
        </p>
        {#if data.user}
          <a
            href="/home"
            use:pressPhysics={'dark'}
            use:hoverEffect={'gradient'}
            class="btn btn-dark"
          >
            <Home size={16} strokeWidth={2.5} />
            {m.marketing_final_lane2_cta_dashboard()}
          </a>
        {:else}
          <a
            href="/auth/magic-link"
            use:pressPhysics={'dark'}
            use:hoverEffect={'gradient'}
            class="btn btn-dark"
          >
            <Sparkles size={16} strokeWidth={2.5} />
            {m.marketing_final_lane2_cta_signin()}
          </a>
        {/if}
      </div>
    </div>

    <p class="text-[12px] font-semibold text-brand-text-muted">
      {m.marketing_final_footer()}
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
      gap: 16px;
      padding-bottom: 0;
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
    width: 244px; height: 340px;
    padding: 18px;
    border-radius: 22px;
    gap: 10px;
    transition: transform 0.4s cubic-bezier(0.22, 1, 0.36, 1);
  }
  .fan-card .card-stripe { height: 134px; border-radius: 14px; }
  .fan-card .card-caption { font-size: 15px; }
  .fan-card .card-foot { font-size: 11px; }
  .fan-card .vote-pip { width: 10px; height: 10px; }
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
    --fan-shift: 40px;
    transform: translateX(var(--fan-shift));
    transition: transform 0.2s ease;
  }
  @media (max-width: 960px) {
    .fan { --fan-shift: 0px; }
  }
  /* Cards positioned with top-left translates so the visual bounding box
     spans roughly ±200px around the fan reference — fits inside the right
     column at viewport 1180 (col2 ≈ 533px) and any narrower viewport. */
  .fan .fan-card[data-pos='1'] { transform: translate(-160px, 14px) rotate(-12deg); }
  .fan .fan-card[data-pos='2'] { transform: translate(-130px, -4px) rotate(-3deg); z-index: 2; }
  .fan .fan-card[data-pos='3'] { transform: translate(-100px, 0px) rotate(5deg); z-index: 3; box-shadow: 0 8px 0 rgba(0, 0, 0, 0.14); }
  .fan .fan-card[data-pos='4'] { transform: translate(-68px, 18px) rotate(13deg); z-index: 1; }

  .fan-wrap:hover .fan-card[data-pos='1'] { transform: translate(-184px, 12px) rotate(-16deg); }
  .fan-wrap:hover .fan-card[data-pos='2'] { transform: translate(-142px, -12px) rotate(-6deg); }
  .fan-wrap:hover .fan-card[data-pos='3'] { transform: translate(-94px, -6px) rotate(8deg); }
  .fan-wrap:hover .fan-card[data-pos='4'] { transform: translate(-50px, 24px) rotate(16deg); }

  @media (max-width: 1180px) {
    .fan-wrap { padding-top: 0; height: 360px; }
  }
  /* Mobile: hide the entire fan + ambient floats — text-only hero. */
  @media (max-width: 720px) {
    .fan-wrap { display: none; }
  }

  .winner { position: relative; }
  .winner::after {
    content: attr(data-winner-label);
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
  .float.f1 { top: 40px; left: 30px; transform: rotate(-14deg); animation-delay: -2s; }
  .float.f2 { bottom: -20px; right: 30px; transform: rotate(18deg); animation-delay: -5s; width: 140px; height: 200px; }
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
  @media (max-width: 560px) {
    .hand { grid-template-columns: 1fr; }
    .hand-card { min-height: 240px; }
  }

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
    /* Stacked layout — flip the divider from vertical bars to a single
       horizontal rule with the OR pill centered. The vertical version
       just produced a long empty gap between the two lanes. */
    .lane-divider { flex-direction: row; width: 100%; gap: 12px; }
    .lane-bar { width: auto; height: 2.5px; flex: 1; }
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
