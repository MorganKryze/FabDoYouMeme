<script lang="ts">
  import { Sparkles, Shield, X } from '$lib/icons';
  import { physCard } from '$lib/actions/physCard';
  import type { Medal } from '$lib/medals';
  import { formatMakerSince } from '$lib/medals';
  import * as m from '$lib/paraglide/messages';

  interface Props {
    user: {
      id: string;
      username: string;
      role: 'player' | 'admin';
      created_at: string;
    };
    medals: Medal[];
    onClose?: () => void;
  }

  let { user, medals, onClose }: Props = $props();

  const initialLetter = $derived((user.username?.[0] ?? '?').toUpperCase());
  const serial = $derived(
    ((user.id ?? '').replace(/-/g, '').slice(0, 6).toUpperCase() || 'XXXXXX')
      .replace(/^(.{3})(.{3})$/, '$1-$2')
  );
  const makerSince = $derived(formatMakerSince(user.created_at));
</script>

<div
  use:physCard
  class="relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-5 pt-9 pb-5 flex flex-col gap-4"
  style="box-shadow: 0 6px 0 rgba(0,0,0,0.1);"
>
  {#if onClose}
    <button
      type="button"
      onclick={(e) => { e.stopPropagation(); onClose?.(); }}
      aria-label={m.home_maker_hide_aria()}
      class="absolute top-2 left-2 inline-flex items-center justify-center h-7 w-7 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white cursor-pointer focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
      style="box-shadow: 0 2px 0 rgba(0,0,0,0.1);"
    >
      <X size={12} strokeWidth={3} />
    </button>
  {/if}

  <!-- Corner stamp -->
  <div class="absolute top-3 right-3 inline-flex items-center gap-1.5">
    <Sparkles size={11} strokeWidth={2.75} />
    <span class="text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
      {m.home_maker_title()}
    </span>
  </div>

  <!-- Identity row -->
  <div class="flex items-center gap-4">
    <div
      class="shrink-0 w-16 h-16 rounded-[16px] border-[2.5px] border-brand-border-heavy bg-brand-accent text-brand-text flex items-center justify-center text-3xl font-extrabold select-none"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.14); transform: rotate(-3deg);"
      aria-hidden="true"
    >
      {initialLetter}
    </div>
    <div class="flex flex-col min-w-0 flex-1 pt-2">
      <p class="text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
        {m.home_maker_username_label()}
      </p>
      <p
        class="text-xl font-extrabold leading-none truncate mt-0.5 text-brand-accent"
        style="letter-spacing: -0.02em;"
      >
        {user.username}
      </p>
      <div class="flex items-center gap-2 mt-2 flex-wrap">
        <span
          class="inline-flex items-center gap-1 rounded-full border-[2px] border-brand-border-heavy bg-brand-white px-2 py-0.5 text-xs font-bold uppercase tracking-[0.15em]"
          style="box-shadow: 0 1.5px 0 rgba(0,0,0,0.08);"
        >
          {#if user.role === 'admin'}
            <Shield size={9} strokeWidth={3} />
            {m.home_maker_label_admin()}
          {:else}
            <Sparkles size={9} strokeWidth={3} />
            {m.home_maker_label_maker()}
          {/if}
        </span>
        <span class="font-mono text-xs font-bold text-brand-text-mid tracking-[0.1em] tabular-nums">
          {m.home_maker_id({ serial })}
        </span>
      </div>
    </div>
  </div>

  <!-- Dashed divider -->
  <div class="border-t border-dashed border-brand-border-heavy opacity-40"></div>

  <!-- Medals -->
  <div class="flex flex-col gap-2">
    <p class="text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
      {m.home_maker_medals()}
    </p>
    <div class="flex items-center gap-2">
      {#each medals as medal (medal.id)}
        <div
          class="medal-target relative w-9 h-9 rounded-[10px] border-[2px] border-brand-border-heavy flex items-center justify-center text-lg transition-opacity"
          class:bg-brand-accent={medal.earned}
          class:bg-brand-white={!medal.earned}
          class:opacity-35={!medal.earned}
          style="box-shadow: 0 2px 0 rgba(0,0,0,0.1);"
          data-tip={medal.earned ? medal.description : m.home_maker_medal_locked_tip({ description: medal.description })}
          aria-label={medal.earned ? m.home_maker_medal_aria_earned({ name: medal.name }) : m.home_maker_medal_aria_locked({ name: medal.name })}
        >
          {medal.icon}
        </div>
      {/each}
    </div>
  </div>

  <!-- Maker since footer -->
  <p class="text-xs font-bold uppercase tracking-[0.15em] text-brand-text-mid">
    {m.home_maker_since({ date: makerSince })}
  </p>
</div>

<style>
  /* Styled hover tooltip for each medal — appears above the medal
     square with a short fade + lift animation. Uses `::after` with
     `attr(data-tip)` so there is zero JavaScript or shared state:
     the browser reads the attribute when the element is hovered. */
  .medal-target::after {
    content: attr(data-tip);
    position: absolute;
    left: 50%;
    bottom: calc(100% + 8px);
    transform: translateX(-50%) translateY(4px);
    padding: 0.4rem 0.7rem;
    border-radius: 999px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-text);
    color: var(--brand-white);
    font-size: 0.6rem;
    font-weight: 700;
    letter-spacing: 0.02em;
    line-height: 1.3;
    white-space: nowrap;
    text-transform: none;
    pointer-events: none;
    opacity: 0;
    box-shadow: 0 3px 0 rgba(0, 0, 0, 0.2);
    transition: opacity 120ms ease, transform 120ms ease;
    z-index: 50;
  }
  .medal-target:hover::after,
  .medal-target:focus-visible::after {
    opacity: 1;
    transform: translateX(-50%) translateY(0);
  }
</style>
