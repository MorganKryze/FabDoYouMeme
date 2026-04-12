/**
 * Svelte action: scroll reveal via IntersectionObserver.
 *
 * Attach with `use:reveal` or `use:reveal={{ delay: 1 }}` where delay
 * is a stagger index (0-based). The element fades up into view when it
 * enters the viewport.
 *
 * Relies on `.reveal` and `.visible` classes defined in app.css.
 * Stagger delays use `.d1`, `.d2`, etc. classes.
 */
export function reveal(node: HTMLElement, options?: { delay?: number }) {
  node.classList.add('reveal');

  if (options?.delay && options.delay > 0) {
    node.classList.add(`d${options.delay}`);
  }

  const observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          entry.target.classList.add('visible');
          observer.unobserve(entry.target);
        }
      }
    },
    { threshold: 0.12, rootMargin: '0px 0px -30px 0px' },
  );

  observer.observe(node);

  return {
    destroy() {
      observer.disconnect();
    },
  };
}
