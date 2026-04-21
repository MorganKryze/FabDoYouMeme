<script lang="ts">
  import { reveal } from '$lib/actions/reveal';
  import * as m from '$lib/paraglide/messages';
</script>

<div
  use:reveal
  class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 text-sm text-brand-text-mid text-left max-w-sm"
  style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
>
  <p class="font-bold text-brand-text mb-1">{m.game_meme_freestyle_name()}</p>
  <p class="font-semibold">{m.game_meme_freestyle_description()}</p>
</div>
