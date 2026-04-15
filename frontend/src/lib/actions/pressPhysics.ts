/**
 * Svelte action: button press/hover physics.
 *
 * Attach with `use:pressPhysics` for the brand's tactile button feel:
 * hover lifts the button up, press pushes it down like a physical key.
 *
 * Shadow values adapt based on the variant parameter:
 *   'primary' — offset shadow 0.22 opacity (default)
 *   'dark'    — offset shadow 0.35 opacity
 *   'ghost'   — offset shadow 0.10 opacity
 */
type Variant = 'primary' | 'dark' | 'ghost';

const shadowOpacity: Record<Variant, number> = {
  primary: 0.22,
  dark: 0.35,
  ghost: 0.1,
};

export function pressPhysics(node: HTMLElement, variant: Variant = 'primary') {
  const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (prefersReduced) return {};

  const op = shadowOpacity[variant];
  const restShadow = `0 5px 0 rgba(0,0,0,${op})`;
  const hoverShadow = `0 7px 0 rgba(0,0,0,${op})`;
  const pressShadow = `0 1px 0 rgba(0,0,0,${op})`;

  node.style.transition = 'transform 0.1s, box-shadow 0.1s';
  node.style.boxShadow = restShadow;

  const isDisabled = () =>
    node.hasAttribute('disabled') || node.getAttribute('aria-disabled') === 'true';

  function onMouseEnter() {
    if (isDisabled()) return;
    node.style.transform = 'translateY(-2px)';
    node.style.boxShadow = hoverShadow;
  }

  function onMouseLeave() {
    if (isDisabled()) return;
    node.style.transform = '';
    node.style.boxShadow = restShadow;
  }

  function onMouseDown() {
    if (isDisabled()) return;
    node.style.transform = 'translateY(3px)';
    node.style.boxShadow = pressShadow;
  }

  function onMouseUp() {
    if (isDisabled()) return;
    node.style.transform = 'translateY(-2px)';
    node.style.boxShadow = hoverShadow;
  }

  node.addEventListener('mouseenter', onMouseEnter);
  node.addEventListener('mouseleave', onMouseLeave);
  node.addEventListener('mousedown', onMouseDown);
  node.addEventListener('mouseup', onMouseUp);

  return {
    destroy() {
      node.removeEventListener('mouseenter', onMouseEnter);
      node.removeEventListener('mouseleave', onMouseLeave);
      node.removeEventListener('mousedown', onMouseDown);
      node.removeEventListener('mouseup', onMouseUp);
    },
  };
}
