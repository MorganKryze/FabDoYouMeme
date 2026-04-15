<script lang="ts">
  import '../../app.css';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import JoinCodePopover from '$lib/components/marketing/JoinCodePopover.svelte';
  import Wordmark from '$lib/components/Wordmark.svelte';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();
</script>

<div class="relative z-[2] min-h-screen flex flex-col text-brand-text">
  <!-- Skip link: first tab stop, visually hidden until focused -->
  <a
    href="#main"
    class="sr-only focus:not-sr-only focus:absolute focus:top-3 focus:left-3 focus:z-[100] focus:px-4 focus:py-2 focus:rounded-full focus:bg-brand-text focus:text-brand-white focus:font-bold focus:no-underline focus:outline-none focus:ring-4 focus:ring-brand-accent/60"
  >
    Skip to main content
  </a>

  <!-- Marketing nav: wordmark left, join + auth right — fixed so it stays on scroll -->
  <header class="fixed top-0 inset-x-0 z-50 flex items-center justify-between gap-4 px-6 py-3">
    <Wordmark href="/" />

    <div class="flex items-center gap-3">
      <JoinCodePopover />

      {#if data.user}
        <!-- Already signed in (reached via ?preview=1) — offer a fast exit. -->
        <a
          href="/home"
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          class="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-text text-brand-white border-[2.5px] border-brand-border-heavy no-underline transition-colors focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
          style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
        >
          Dashboard
        </a>
      {:else}
        <a
          href="/auth/magic-link"
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          class="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-text text-brand-white border-[2.5px] border-brand-border-heavy no-underline transition-colors focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
          style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
        >
          Sign in
        </a>
      {/if}
    </div>
  </header>

  <main id="main" class="flex-1 flex flex-col pt-[3.5rem]">
    {@render children()}
  </main>

  <footer class="mt-24 border-t-[2.5px] border-brand-border-heavy">
    <div class="max-w-5xl mx-auto px-6 pt-12 pb-8 grid grid-cols-1 sm:grid-cols-[2fr_1fr_1fr] gap-10">
      <!-- Brand block -->
      <div class="flex flex-col gap-2">
        <span class="text-xl font-bold tracking-tight">FabDoYouMeme</span>
        <p class="text-sm font-semibold text-brand-text-mid max-w-xs leading-relaxed">
          A private, self-hosted party meme game. Built for circles that prefer to keep things inside.
        </p>
      </div>

      <!-- Project links -->
      <div class="flex flex-col gap-3">
        <div class="text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
          Project
        </div>
        <ul class="flex flex-col gap-2 text-sm font-bold">
          <li>
            <a
              href="https://github.com/MorganKryze/FabDoYouMeme"
              class="hover:text-brand-text-mid transition-colors no-underline rounded-sm focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
            >
              Source
            </a>
          </li>
          <li>
            <a
              href="https://github.com/MorganKryze/FabDoYouMeme/issues"
              class="hover:text-brand-text-mid transition-colors no-underline rounded-sm focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
            >
              Issues
            </a>
          </li>
          <li>
            <a
              href="https://github.com/MorganKryze/FabDoYouMeme/blob/main/LICENSE"
              class="hover:text-brand-text-mid transition-colors no-underline rounded-sm focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
            >
              GPLv3
            </a>
          </li>
        </ul>
      </div>

      <!-- Legal links -->
      <div class="flex flex-col gap-3">
        <div class="text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
          Legal
        </div>
        <ul class="flex flex-col gap-2 text-sm font-bold">
          <li>
            <a href="/privacy" class="hover:text-brand-text-mid transition-colors no-underline rounded-sm focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60">
              Privacy policy
            </a>
          </li>
          <li>
            <a href="/auth/register" class="hover:text-brand-text-mid transition-colors no-underline rounded-sm focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60">
              Register with invite
            </a>
          </li>
        </ul>
      </div>
    </div>

    <div class="max-w-5xl mx-auto px-6 pb-10 pt-6 border-t border-brand-border flex flex-col sm:flex-row items-center justify-between gap-3 text-xs font-semibold text-brand-text-mid">
      <p>© {new Date().getFullYear()} FabDoYouMeme — self-hosted, invite-only.</p>
      <p>Built with <span class="font-bold">SvelteKit</span> + <span class="font-bold">Go</span> + <span class="font-bold">Postgres</span>.</p>
    </div>
  </footer>
</div>
