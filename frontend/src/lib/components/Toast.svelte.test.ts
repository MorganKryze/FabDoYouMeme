import { describe, it, expect, beforeEach } from 'vitest';
import { render } from '@testing-library/svelte';
import { flushSync } from 'svelte';
import Toast from './Toast.svelte';
import { toast } from '$lib/state/toast.svelte';

describe('Toast.svelte', () => {
  beforeEach(() => {
    // Reset the module-level singleton between tests. We deliberately do
    // NOT use vi.useFakeTimers() here: @testing-library's `findByRole`
    // polls via real setTimeout, and auto-dismiss (3000ms for success)
    // will not fire within the sub-second test runtime.
    for (const item of [...toast.items]) {
      toast.dismiss(item.id);
    }
  });

  it('renders no alerts when the toast list is empty', () => {
    const { queryAllByRole } = render(Toast);

    expect(queryAllByRole('alert')).toHaveLength(0);
  });

  it('renders a message after toast.show() is called', () => {
    const { getByRole } = render(Toast);

    toast.show('hello world', 'success');
    flushSync();

    const alert = getByRole('alert');
    expect(alert).toHaveTextContent('hello world');
  });
});
