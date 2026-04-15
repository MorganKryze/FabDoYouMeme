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
  class="inline-block text-lg font-bold tracking-tight no-underline rounded-full px-2 py-1 focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
  aria-label="FabDoYouMeme home"
>
  FabDoYouMeme
</a>
