/**
 * Svelte action: 3D cursor-tracked card tilt with physical weight.
 *
 * Attach with `use:physCard` on any card-shaped element to get:
 *   - perspective-correct tilt that follows the cursor,
 *   - a lift off the surface (translateY + translateZ) on hover,
 *   - a dynamic drop shadow that reinforces the elevation,
 *   - a short tracking transition so motion has weight (no frame-by-frame jitter),
 *   - press-flat / spring-release physics on click.
 *
 * Perspective is baked into the transform so the action is self-contained;
 * ancestor markup doesn't need `perspective` declared.
 *
 * Respects `prefers-reduced-motion` — becomes a no-op when enabled.
 *
 * Branches on `(hover: hover) and (pointer: fine)`:
 *   - Desktop/trackpad: full 3D cursor-tracked tilt + lift + press physics
 *   - Touch/coarse pointer: tap-scale-press with spring release only
 */
export function physCard(node: HTMLElement) {
  const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (prefersReduced) return {};

  const canHover = window.matchMedia('(hover: hover) and (pointer: fine)').matches;
  if (!canHover) return touchFallback(node);

  // Preserve the element's authored inline state so leave/up restores cleanly.
  // Without this, `style.boxShadow = ''` on leave would wipe any `style="box-shadow: ..."`
  // baked into the markup, dropping the card's resting shadow after the first hover.
  const originalShadow = node.style.boxShadow;
  const originalZIndex = node.style.zIndex;

  // Tilt/lift magnitudes. Moderate enough to work on small cards, pronounced
  // enough on large cards to read as "picked up off the table."
  const PERSPECTIVE = 1000; // px — larger = subtler 3D, smaller = stronger
  const MAX_ROT_Y = 9;      // ±deg horizontal tilt
  const MAX_ROT_X = 6;      // ±deg vertical tilt
  const HOVER_LIFT = 5;     // px translateY — card rises off the surface
  const HOVER_Z = 18;       // px translateZ — card advances toward viewer

  const idleTransition =
    'transform 0.35s cubic-bezier(0.22, 1, 0.36, 1), box-shadow 0.35s cubic-bezier(0.22, 1, 0.36, 1)';
  // 130ms on transform during tracking gives the card perceptible weight — it
  // trails the cursor by one tick rather than snapping frame-to-frame, which
  // is what reads as "jittery" at zero transition.
  const trackTransition =
    'transform 130ms cubic-bezier(0.22, 1, 0.36, 1), box-shadow 130ms cubic-bezier(0.22, 1, 0.36, 1)';
  const pressTransition = 'transform 0.08s ease, box-shadow 0.08s ease';
  const releaseTransition =
    'transform 0.4s cubic-bezier(0.22, 1, 0.36, 1), box-shadow 0.4s cubic-bezier(0.22, 1, 0.36, 1)';

  node.style.transformStyle = 'preserve-3d';
  node.style.willChange = 'transform';
  node.style.transition = idleTransition;

  let tracking = false;
  let pressed = false;

  function onMouseEnter() {
    tracking = true;
    node.style.transition = trackTransition;
    // Raise above siblings so lift/shadow overflow never gets clipped by
    // later-painted neighbors in a grid.
    node.style.zIndex = '10';
  }

  function onMouseMove(e: MouseEvent) {
    if (!tracking || pressed) return;

    const rect = node.getBoundingClientRect();
    const cx = rect.width / 2;
    const cy = rect.height / 2;
    const nx = (e.clientX - rect.left - cx) / cx; // -1..1
    const ny = (e.clientY - rect.top - cy) / cy;  // -1..1

    const rotateY = nx * MAX_ROT_Y;
    const rotateX = -ny * MAX_ROT_X;

    node.style.transform =
      `perspective(${PERSPECTIVE}px) ` +
      `translateY(-${HOVER_LIFT}px) translateZ(${HOVER_Z}px) ` +
      `rotateX(${rotateX}deg) rotateY(${rotateY}deg)`;

    // Shadow drifts opposite the horizontal tilt and deepens with the lift —
    // grounds the card instead of leaving it floating abstractly.
    const offX = -nx * 10;
    const offY = 16 + HOVER_LIFT - ny * 3;
    node.style.boxShadow =
      `${offX}px ${offY}px 30px rgba(0,0,0,0.18), ` +
      `0 6px 0 rgba(0,0,0,0.10)`;
  }

  function onMouseLeave() {
    tracking = false;
    pressed = false;
    node.style.transition = idleTransition;
    node.style.transform = '';
    node.style.boxShadow = originalShadow;
    node.style.zIndex = originalZIndex;
  }

  function onMouseDown() {
    pressed = true;
    node.style.transition = pressTransition;
    node.style.transform =
      `perspective(${PERSPECTIVE}px) translateY(1px) translateZ(0px) scale(0.98)`;
    node.style.boxShadow = '0 2px 4px -1px rgba(0,0,0,0.18), 0 2px 0 rgba(0,0,0,0.12)';
  }

  function onMouseUp() {
    pressed = false;
    node.style.transition = releaseTransition;
    // Clearing transform lets the next mousemove re-compute the lift from the
    // current cursor position — the press releases back into the hover state.
    node.style.transform = '';
    node.style.boxShadow = originalShadow;
  }

  node.addEventListener('mouseenter', onMouseEnter);
  node.addEventListener('mousemove', onMouseMove);
  node.addEventListener('mouseleave', onMouseLeave);
  node.addEventListener('mousedown', onMouseDown);
  node.addEventListener('mouseup', onMouseUp);

  return {
    destroy() {
      node.removeEventListener('mouseenter', onMouseEnter);
      node.removeEventListener('mousemove', onMouseMove);
      node.removeEventListener('mouseleave', onMouseLeave);
      node.removeEventListener('mousedown', onMouseDown);
      node.removeEventListener('mouseup', onMouseUp);
    },
  };
}

/**
 * Touch / coarse-pointer fallback: tap-scale-press with spring release.
 * No 3D tilt (no cursor to track), no DeviceOrientation (breaks "chill"
 * feel and requires a permission prompt on iOS).
 */
function touchFallback(node: HTMLElement) {
  const pressTransition = 'transform 0.08s ease';
  const releaseTransition = 'transform 0.4s cubic-bezier(0.22, 1, 0.36, 1)';

  node.style.willChange = 'transform';

  function onDown() {
    node.style.transition = pressTransition;
    node.style.transform = 'translateY(1px) scale(0.97)';
  }

  function onUp() {
    node.style.transition = releaseTransition;
    node.style.transform = '';
  }

  node.addEventListener('pointerdown', onDown);
  node.addEventListener('pointerup', onUp);
  node.addEventListener('pointercancel', onUp);
  node.addEventListener('pointerleave', onUp);

  return {
    destroy() {
      node.removeEventListener('pointerdown', onDown);
      node.removeEventListener('pointerup', onUp);
      node.removeEventListener('pointercancel', onUp);
      node.removeEventListener('pointerleave', onUp);
    },
  };
}
