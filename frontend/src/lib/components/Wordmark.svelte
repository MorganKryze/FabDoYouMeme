<script lang="ts">
  import { pressPhysics } from '$lib/actions/pressPhysics';

  interface Props {
    href?: string;
  }

  let { href = '/' }: Props = $props();
</script>

<a
  {href}
  use:pressPhysics={'ghost'}
  class="wordmark inline-block text-lg font-bold tracking-tight no-underline rounded-full px-5 py-1.5 focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
  aria-label="FabDoYouMeme home"
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
