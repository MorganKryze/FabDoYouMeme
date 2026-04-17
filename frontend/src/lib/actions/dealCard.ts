/**
 * Svelte action: "dealt from the deck" card reveal.
 *
 * On intersection, the card travels from a virtual deck (bottom-center of the
 * viewport, just below the fold) to its natural layout position, with a tilt
 * and slight overshoot for a hand-dealt feel.
 *
 * Usage: use:dealCard={{ delay: 140, rotate: -8, smooth: true }}
 *   delay  — ms before this card's animation starts (stagger).
 *   rotate — degrees of initial tilt; final multiplier is applied in CSS.
 *   smooth — single-keyframe glide (no overshoot/settle beat). Use when
 *            many cards are dealt at once so the collective motion doesn't
 *            read as glitchy. Default is the two-stage "hand-dealt" feel.
 *
 * Respects `prefers-reduced-motion` — falls back to the site-wide `.reveal`
 * fade-up so the section still appears deliberately.
 *
 * Coexists with `use:physCard` on the same node: this action uses a CSS
 * animation (not transition) so `physCard`'s transition-based hover work
 * is unaffected. On animation end, all traces are removed and `physCard`
 * fully owns `transform`.
 */
export function dealCard(
  node: HTMLElement,
  options: { delay?: number; rotate?: number; smooth?: boolean } = {},
) {
  const delay = options.delay ?? 0;
  const rotate = options.rotate ?? 0;
  const playClass = options.smooth ? 'deal-card-play-smooth' : 'deal-card-play';

  const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  if (prefersReduced) {
    // Fall back to the site's standard scroll reveal.
    node.classList.add('reveal');
    const fallback = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            entry.target.classList.add('visible');
            fallback.unobserve(entry.target);
          }
        }
      },
      { threshold: 0.12, rootMargin: '0px 0px -30px 0px' },
    );
    fallback.observe(node);
    return {
      destroy() {
        fallback.disconnect();
      },
    };
  }

  // Hide until the deal plays. Stays out of document flow changes.
  node.classList.add('deal-card-pending');

  const observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          play();
          observer.unobserve(entry.target);
        }
      }
    },
    { threshold: 0.15, rootMargin: '0px 0px -40px 0px' },
  );
  observer.observe(node);

  function play() {
    const rect = node.getBoundingClientRect();
    const centerX = rect.left + rect.width / 2;
    const centerY = rect.top + rect.height / 2;
    // "Deck" sits just below the viewport bottom, dead-center horizontally.
    const deckX = window.innerWidth / 2;
    const deckY = window.innerHeight + 80;
    const dx = deckX - centerX;
    const dy = deckY - centerY;

    node.style.setProperty('--deal-dx', `${dx}px`);
    node.style.setProperty('--deal-dy', `${dy}px`);
    node.style.setProperty('--deal-rot', `${rotate}deg`);
    node.style.setProperty('--deal-delay', `${delay}ms`);
    node.classList.remove('deal-card-pending');
    node.classList.add(playClass);

    node.addEventListener(
      'animationend',
      () => {
        node.classList.remove(playClass);
        node.style.removeProperty('--deal-dx');
        node.style.removeProperty('--deal-dy');
        node.style.removeProperty('--deal-rot');
        node.style.removeProperty('--deal-delay');
      },
      { once: true },
    );
  }

  return {
    destroy() {
      observer.disconnect();
    },
  };
}
