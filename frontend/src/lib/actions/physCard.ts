/**
 * Svelte action: 3D cursor-tracked card tilt.
 *
 * Attach with `use:physCard` on any element to get playing-card-like
 * hover tilt, dynamic shadow, and press-flat/spring-release physics.
 *
 * Respects `prefers-reduced-motion` — becomes a no-op when enabled.
 *
 * Branches on `(hover: hover) and (pointer: fine)`:
 *   - Desktop/trackpad: full 3D cursor-tracked tilt + press physics
 *   - Touch/coarse pointer: tap-scale-press with spring release only
 * Both paths share the same release cubic-bezier so the feel is
 * consistent across devices.
 */
export function physCard(node: HTMLElement) {
  const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (prefersReduced) return {};

  const canHover = window.matchMedia('(hover: hover) and (pointer: fine)').matches;
  if (!canHover) return touchFallback(node);

  // Resting transition for leave/release animations
  const restTransition =
    'transform 0.35s cubic-bezier(0.22, 1, 0.36, 1), box-shadow 0.35s cubic-bezier(0.22, 1, 0.36, 1)';
  const pressTransition = 'transform 0.08s ease, box-shadow 0.08s ease';
  const releaseTransition =
    'transform 0.4s cubic-bezier(0.22, 1, 0.36, 1), box-shadow 0.4s cubic-bezier(0.22, 1, 0.36, 1)';

  node.style.transformStyle = 'preserve-3d';
  node.style.willChange = 'transform';
  node.style.transition = restTransition;

  let tracking = false;
  let pressed = false;

  function onMouseEnter() {
    tracking = true;
    // Remove transition during tracking for immediate response
    node.style.transition = 'box-shadow 0.35s cubic-bezier(0.22, 1, 0.36, 1)';
  }

  function onMouseMove(e: MouseEvent) {
    if (!tracking || pressed) return;

    const rect = node.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    const centerX = rect.width / 2;
    const centerY = rect.height / 2;

    // Normalize to -1..1
    const nx = (x - centerX) / centerX;
    const ny = (y - centerY) / centerY;

    // Tilt: max ±6deg Y, ±4deg X
    const rotateY = nx * 6;
    const rotateX = -ny * 4;

    node.style.transform = `rotateX(${rotateX}deg) rotateY(${rotateY}deg)`;
  }

  function onMouseLeave() {
    tracking = false;
    pressed = false;
    node.style.transition = restTransition;
    node.style.transform = '';
    node.style.boxShadow = '';
  }

  function onMouseDown() {
    pressed = true;
    node.style.transition = pressTransition;
    node.style.transform = 'translateY(-1px) rotateX(0deg) rotateY(0deg) scale(0.98)';
    node.style.boxShadow = '0 2px 4px -1px rgba(0,0,0,0.18), 0 2px 0 rgba(0,0,0,0.12)';
  }

  function onMouseUp() {
    pressed = false;
    node.style.transition = releaseTransition;
    node.style.transform = '';
    node.style.boxShadow = '';
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
