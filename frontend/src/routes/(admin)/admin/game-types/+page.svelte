<script lang="ts">
  import { reveal } from '$lib/actions/reveal';
  import * as m from '$lib/paraglide/messages';
  import type { PageData } from './$types';
  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>{m.admin_game_types_page_title()}</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4" use:reveal>
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">{m.admin_game_types_heading()}</h1>
    <p class="text-xs text-brand-text-muted">{m.admin_game_types_readonly_hint()}</p>
  </div>

  <div class="rounded-xl border border-brand-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-brand-border bg-muted/40 text-xs font-medium text-brand-text-muted">
          <th class="text-left px-4 py-3">{m.admin_game_types_col_slug()}</th>
          <th class="text-left px-4 py-3">{m.admin_game_types_col_name()}</th>
          <th class="text-left px-4 py-3">{m.admin_game_types_col_description()}</th>
          <th class="text-left px-4 py-3">{m.admin_game_types_col_required_packs()}</th>
          <th class="text-left px-4 py-3">{m.admin_game_types_col_supports_solo()}</th>
        </tr>
      </thead>
      <tbody>
        {#each data.gameTypes as gt}
          <tr class="border-b border-brand-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3 font-mono text-xs">{gt.slug}</td>
            <td class="px-4 py-3 font-medium">{gt.name}</td>
            <td class="px-4 py-3 text-brand-text-muted">{gt.description}</td>
            <td class="px-4 py-3 text-brand-text-muted font-mono text-xs">
              {#if (gt.required_packs ?? []).length === 0}
                —
              {:else}
                {(gt.required_packs ?? [])
                  .map((p) => `${p.role}:[${(p.payload_versions ?? []).join(',')}]`)
                  .join(' · ')}
              {/if}
            </td>
            <td class="px-4 py-3">
              <span class="text-xs px-2 py-0.5 rounded-full {gt.supports_solo ? 'bg-green-100 text-green-800' : 'bg-muted text-brand-text-muted'}">
                {gt.supports_solo ? m.admin_game_types_yes() : m.admin_game_types_no()}
              </span>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
