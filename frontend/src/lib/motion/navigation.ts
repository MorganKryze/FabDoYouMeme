import { onNavigate } from '$app/navigation';

/**
 * Installs page-to-page transitions using the View Transitions API
 * where available, falling back to a manual `onNavigate` handler that
 * animates a `.page-exiting` class on <html>.
 *
 * Respects `prefers-reduced-motion: reduce` — when set, navigation is
 * instant and no classes are toggled.
 *
 * Call once from the root layout's `onMount`.
 */
export function installPageTransitions(): void {
  onNavigate((navigation) => {
    if (!window.matchMedia('(prefers-reduced-motion: no-preference)').matches) {
      return;
    }

    const doc = document as Document & {
      startViewTransition?: (cb: () => void | Promise<void>) => { finished: Promise<void> };
    };

    if (typeof doc.startViewTransition !== 'function') {
      return manualFallback(navigation);
    }

    return new Promise<void>((resolve) => {
      doc.startViewTransition!(async () => {
        resolve();
        await navigation.complete;
      });
    });
  });
}

async function manualFallback(navigation: { complete: Promise<void> }): Promise<void> {
  const root = document.documentElement;
  root.classList.add('page-exiting');
  await new Promise((r) => setTimeout(r, 200));
  root.classList.remove('page-exiting');
  await navigation.complete;
}
