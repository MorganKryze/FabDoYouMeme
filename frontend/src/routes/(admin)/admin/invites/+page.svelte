<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Plus, Copy, Trash2, XCircle } from '$lib/icons';
  import type { Invite } from '$lib/api/types';
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';
  import { getLocale } from '$lib/paraglide/runtime';

  let { data }: { data: PageData } = $props();
  let showCreateForm = $state(false);
  // Local mutable copy so create/revoke can append/filter without a full
  // `invalidateAll` round-trip. Seeded once from `data.invites` on mount.
  let invites = $state<Invite[]>(untrack(() => data.invites));
  let revealedTokens = $state<Set<string>>(new Set());

  function copyLink(token: string) {
    const url = `${window.location.origin}/auth/register?invite=${token}`;
    navigator.clipboard.writeText(url).then(() => toast.show(m.admin_invites_toast_copied(), 'success'));
  }

  function toggleReveal(id: string) {
    const next = new Set(revealedTokens);
    if (next.has(id)) next.delete(id); else next.add(id);
    revealedTokens = next;
  }
</script>

<svelte:head>
  <title>{m.admin_invites_page_title()}</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4" use:reveal>
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">{m.admin_invites_heading()}</h1>
    <button
      type="button"
      onclick={() => showCreateForm = !showCreateForm}
      use:pressPhysics={'dark'}
      use:hoverEffect={'swap'}
      class="h-9 px-4 rounded-lg border border-brand-border-heavy bg-brand-white text-brand-text text-sm font-medium inline-flex items-center gap-1.5">
      <Plus size={14} strokeWidth={2.5} />
      {m.admin_invites_create()}
    </button>
  </div>

  {#if showCreateForm}
    <form
      method="POST"
      action="?/createInvite"
      use:enhance={() => {
        return async ({ result, update }) => {
          await update({ reset: false });
          if (result.type === 'success') {
            const created = (result.data as { created?: Invite } | undefined)?.created;
            if (created) {
              invites = [...invites, created];
              showCreateForm = false;
              toast.show(m.admin_invites_toast_created(), 'success');
            } else {
              toast.show(m.admin_invites_toast_created_malformed(), 'warning');
            }
          } else if (result.type === 'failure') {
            const msg = (result.data as { createError?: string } | undefined)?.createError;
            toast.show(msg ?? m.admin_invites_toast_create_failed(), 'error');
          } else if (result.type === 'error') {
            toast.show(result.error?.message ?? m.admin_invites_toast_create_error(), 'error');
          }
        };
      }}
      class="rounded-xl border border-brand-border bg-brand-white p-4 flex flex-col gap-3">
      <h2 class="text-sm font-semibold">{m.admin_invites_new_heading()}</h2>
      <div class="grid grid-cols-2 gap-3">
        <div class="flex flex-col gap-1">
          <label for="label" class="text-xs font-medium">{m.admin_invites_field_label()}</label>
          <input id="label" name="label" type="text" placeholder={m.admin_invites_field_label_placeholder()}
            class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
        </div>
        <div class="flex flex-col gap-1">
          <label for="restricted_email" class="text-xs font-medium">{m.admin_invites_field_restricted_email()}</label>
          <input id="restricted_email" name="restricted_email" type="email" placeholder={m.admin_invites_field_restricted_email_placeholder()}
            class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
        </div>
        <div class="flex flex-col gap-1">
          <label for="max_uses" class="text-xs font-medium">{m.admin_invites_field_max_uses()}</label>
          <input id="max_uses" name="max_uses" type="number" min={0} value={0}
            class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
        </div>
        <div class="flex flex-col gap-1">
          <label for="expires_at" class="text-xs font-medium">{m.admin_invites_field_expires_at()}</label>
          <input id="expires_at" name="expires_at" type="datetime-local"
            class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
        </div>
        <div class="flex flex-col gap-1">
          <label for="invite_locale" class="text-xs font-medium">{m.admin_invites_field_locale()}</label>
          <select id="invite_locale" name="locale"
            value={getLocale()}
            class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring">
            <option value="en">{m.admin_invites_field_locale_en()}</option>
            <option value="fr">{m.admin_invites_field_locale_fr()}</option>
          </select>
        </div>
      </div>
      <div class="flex gap-2 justify-end">
        <button
          type="button"
          onclick={() => showCreateForm = false}
          use:pressPhysics={'ghost'}
          use:hoverEffect={'swap'}
          class="h-9 px-4 rounded border border-brand-border text-sm inline-flex items-center gap-1.5">
          <XCircle size={14} strokeWidth={2.5} />
          {m.admin_users_cancel()}
        </button>
        <button
          type="submit"
          use:pressPhysics={'dark'}
          use:hoverEffect={'swap'}
          class="h-9 px-4 rounded border border-brand-border-heavy bg-brand-white text-brand-text text-sm font-medium inline-flex items-center gap-1.5">
          <Plus size={14} strokeWidth={2.5} />
          {m.admin_create()}
        </button>
      </div>
    </form>
  {/if}

  <div class="rounded-xl border border-brand-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-brand-border bg-muted/40 text-xs font-medium text-brand-text-muted">
          <th class="text-left px-4 py-3">{m.admin_invites_col_label()}</th>
          <th class="text-left px-4 py-3">{m.admin_invites_col_token()}</th>
          <th class="text-left px-4 py-3">{m.admin_invites_col_restricted_email()}</th>
          <th class="text-left px-4 py-3">{m.admin_invites_col_uses()}</th>
          <th class="text-left px-4 py-3">{m.admin_invites_col_expires()}</th>
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each invites as inv}
          <tr class="border-b border-brand-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3">{inv.label ?? '—'}</td>
            <td class="px-4 py-3 font-mono">
              <button type="button" onclick={() => toggleReveal(inv.id)}
                class="text-brand-text-muted hover:text-brand-text transition-colors">
                {revealedTokens.has(inv.id) ? inv.token : `${inv.token.slice(0, 4)}…`}
              </button>
            </td>
            <td class="px-4 py-3 text-brand-text-muted">{inv.restricted_email ?? '—'}</td>
            <td class="px-4 py-3 text-brand-text-muted">
              {inv.uses_count}/{inv.max_uses === 0 ? '∞' : inv.max_uses}
            </td>
            <td class="px-4 py-3 text-brand-text-muted text-xs">
              {inv.expires_at ? new Date(inv.expires_at).toLocaleDateString() : m.admin_invites_expires_never()}
            </td>
            <td class="px-4 py-3">
              <div class="flex gap-2 justify-end">
                <button
                  type="button"
                  onclick={() => copyLink(inv.token)}
                  use:hoverEffect={'swap'}
                  class="inline-flex items-center gap-1 text-xs text-brand-text-muted underline hover:text-brand-text px-2 py-1 rounded-full">
                  <Copy size={12} strokeWidth={2.5} />
                  {m.admin_invites_copy_link()}
                </button>
                <form
                  method="POST"
                  action="?/revokeInvite"
                  use:enhance={() => {
                    return async ({ result, update }) => {
                      await update({ reset: false });
                      if (result.type === 'success') {
                        const revokedId = (result.data as { revoked?: string } | undefined)?.revoked;
                        if (revokedId) {
                          invites = invites.filter((i) => i.id !== revokedId);
                          toast.show(m.admin_invites_toast_revoked(), 'success');
                        }
                      } else if (result.type === 'failure') {
                        const msg = (result.data as { revokeError?: string } | undefined)?.revokeError;
                        toast.show(msg ?? m.admin_invites_toast_revoke_failed(), 'error');
                      } else if (result.type === 'error') {
                        toast.show(result.error?.message ?? m.admin_invites_toast_revoke_error(), 'error');
                      }
                    };
                  }}
                  onsubmit={(e) => !confirm(m.admin_invites_revoke_confirm()) && e.preventDefault()}>
                  <input type="hidden" name="invite_id" value={inv.id} />
                  <button
                    type="submit"
                    class="inline-flex items-center gap-1 text-xs text-red-600 underline hover:text-red-800">
                    <Trash2 size={12} strokeWidth={2.5} />
                    {m.admin_invites_revoke()}
                  </button>
                </form>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
