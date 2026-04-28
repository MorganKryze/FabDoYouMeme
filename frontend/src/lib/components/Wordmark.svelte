<script lang="ts">
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import * as m from '$lib/paraglide/messages';

  interface Props {
    href?: string;
  }

  let { href = '/' }: Props = $props();
</script>

<a
  {href}
  use:pressPhysics={'ghost'}
  class="wordmark inline-block text-sm sm:text-lg font-bold tracking-tight no-underline rounded-full px-3 sm:px-5 py-1.5 whitespace-nowrap focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
  aria-label={m.common_wordmark_home_aria()}
>
  FabDoYouMeme
</a>

<style>
  .wordmark {
    background: transparent;
    outline: 1px solid rgba(26, 26, 26, 0.3);
    outline-offset: -1px;
  }
  .wordmark:focus-visible {
    outline: none;
  }
</style>
