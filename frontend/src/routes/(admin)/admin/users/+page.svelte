<script lang="ts">
  import { enhance } from '$app/forms';
  import { goto } from '$app/navigation';
  import { toast } from '$lib/state/toast.svelte';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let searchTerm = $state(data.q ?? '');
  let searchTimeout: ReturnType<typeof setTimeout>;
  let confirmDeleteId = $state<string | null>(null);

  function onSearchInput() {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      goto(`?q=${encodeURIComponent(searchTerm)}`, { replaceState: true });
    }, 300);
  }

  $effect(() => {
    if (form?.error) toast.show(form.error, 'error');
    if (form?.success) toast.show('User updated.', 'success');
    if (form?.deleted) toast.show('User deleted.', 'success');
  });
</script>

<svelte:head>
  <title>Users — Admin</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4">
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">Users</h1>
    <input
      type="search"
      placeholder="Search users…"
      bind:value={searchTerm}
      oninput={onSearchInput}
      class="h-9 w-56 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
    />
  </div>

  <div class="rounded-xl border border-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-border bg-muted/40 text-xs font-medium text-muted-foreground">
          <th class="text-left px-4 py-3">Username</th>
          <th class="text-left px-4 py-3">Email</th>
          <th class="text-left px-4 py-3">Role</th>
          <th class="text-left px-4 py-3">Active</th>
          <th class="text-left px-4 py-3">Joined</th>
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each data.users as u}
          <tr class="border-b border-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3">
              <form method="POST" action="?/updateUser" use:enhance class="flex gap-1">
                <input type="hidden" name="user_id" value={u.id} />
                <input
                  name="username"
                  type="text"
                  value={u.username}
                  onblur={(e) => {
                    if ((e.target as HTMLInputElement).value !== u.username)
                      (e.target as HTMLInputElement).closest('form')?.requestSubmit();
                  }}
                  class="h-7 w-28 rounded border border-transparent hover:border-border px-1 text-sm bg-transparent focus:outline-none focus:border-ring"
                />
              </form>
            </td>
            <td class="px-4 py-3">
              <form method="POST" action="?/updateUser" use:enhance class="flex gap-1">
                <input type="hidden" name="user_id" value={u.id} />
                <input
                  name="email"
                  type="email"
                  value={u.email}
                  onblur={(e) => {
                    if ((e.target as HTMLInputElement).value !== u.email)
                      (e.target as HTMLInputElement).closest('form')?.requestSubmit();
                  }}
                  class="h-7 w-40 rounded border border-transparent hover:border-border px-1 text-sm bg-transparent focus:outline-none focus:border-ring"
                />
              </form>
            </td>
            <td class="px-4 py-3">
              <form method="POST" action="?/updateUser" use:enhance>
                <input type="hidden" name="user_id" value={u.id} />
                <select
                  name="role"
                  value={u.role}
                  onchange={(e) => (e.target as HTMLSelectElement).closest('form')?.requestSubmit()}
                  class="h-7 rounded border border-transparent hover:border-border px-1 text-sm bg-transparent focus:outline-none focus:border-ring cursor-pointer"
                >
                  <option value="player">player</option>
                  <option value="admin">admin</option>
                </select>
              </form>
            </td>
            <td class="px-4 py-3">
              <form method="POST" action="?/updateUser" use:enhance>
                <input type="hidden" name="user_id" value={u.id} />
                <input
                  type="checkbox"
                  name="is_active"
                  checked={u.is_active}
                  value="true"
                  onchange={(e) => {
                    const input = e.target as HTMLInputElement;
                    if (!input.checked) {
                      const hidden = document.createElement('input');
                      hidden.type = 'hidden';
                      hidden.name = 'is_active';
                      hidden.value = 'false';
                      input.closest('form')?.appendChild(hidden);
                      input.remove();
                    }
                    input.closest('form')?.requestSubmit();
                  }}
                  class="h-4 w-4 cursor-pointer"
                />
              </form>
            </td>
            <td class="px-4 py-3 text-muted-foreground text-xs">
              {new Date(u.created_at).toLocaleDateString()}
            </td>
            <td class="px-4 py-3 text-right">
              {#if confirmDeleteId === u.id}
                <div class="flex gap-1 justify-end">
                  <form method="POST" action="?/deleteUser" use:enhance>
                    <input type="hidden" name="user_id" value={u.id} />
                    <button type="submit" class="text-xs text-red-600 underline hover:text-red-800">
                      Confirm Delete
                    </button>
                  </form>
                  <button type="button" onclick={() => confirmDeleteId = null}
                    class="text-xs text-muted-foreground underline">
                    Cancel
                  </button>
                </div>
              {:else}
                <button type="button" onclick={() => confirmDeleteId = u.id}
                  class="text-muted-foreground hover:text-red-600 transition-colors text-lg leading-none"
                  aria-label="Delete user">
                  ×
                </button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  {#if data.nextCursor}
    <a
      href="?{data.q ? `q=${encodeURIComponent(data.q)}&` : ''}cursor={data.nextCursor}"
      class="self-center text-sm text-muted-foreground underline hover:text-foreground"
    >
      Load more
    </a>
  {/if}
</div>
