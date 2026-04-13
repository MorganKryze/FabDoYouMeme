<script lang="ts">
  interface Props {
    value?: string;
    name?: string;
    autofocus?: boolean;
    onenter?: (code: string) => void;
  }

  let {
    value = $bindable(''),
    name = 'code',
    autofocus = false,
    onenter
  }: Props = $props();

  let inputEl = $state<HTMLInputElement | null>(null);

  $effect(() => {
    if (autofocus && inputEl) inputEl.focus();
  });

  function onInput(e: Event) {
    const raw = (e.target as HTMLInputElement).value.toUpperCase().replace(/[^A-Z0-9]/g, '');
    value = raw.slice(0, 4);
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && onenter) {
      e.preventDefault();
      onenter(value);
    }
  }
</script>

<input
  bind:this={inputEl}
  {name}
  value={value}
  oninput={onInput}
  onkeydown={onKeydown}
  type="text"
  inputmode="text"
  autocapitalize="characters"
  maxlength={4}
  placeholder="WXYZ"
  aria-label="Room code"
  class="h-16 w-full rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-6 text-center text-2xl font-mono font-bold tracking-widest uppercase focus:outline-none focus:border-brand-text transition-colors"
  style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
/>
