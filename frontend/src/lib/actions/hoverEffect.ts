/**
 * Svelte action: button hover effects.
 *
 * Attach with `use:hoverEffect={'gradient'}` (or 'swap', 'glow', 'bounce').
 * Each variant adds a distinctive hover animation appropriate for different
 * action contexts per the brand spec.
 *
 * - gradient: pastel gradient fades in behind text (primary CTAs)
 * - swap: white → dark color inversion (standard actions)
 * - glow: gradient halo breathes behind button (highlighted actions)
 * - bounce: scale(1.06) with elastic spring (playful secondary)
 */
type HoverStyle = 'gradient' | 'swap' | 'glow' | 'bounce';

export function hoverEffect(node: HTMLElement, style: HoverStyle = 'swap') {
  const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (prefersReduced) return {};

  // Store original inline styles to restore on leave
  let origBackground = '';
  let origColor = '';
  let origTransform = '';
  let origLetterSpacing = '';

  function saveOriginals() {
    origBackground = node.style.background;
    origColor = node.style.color;
    origTransform = node.style.transform;
    origLetterSpacing = node.style.letterSpacing;
  }

  function restoreOriginals() {
    node.style.background = origBackground;
    node.style.color = origColor;
    node.style.transform = origTransform;
    node.style.letterSpacing = origLetterSpacing;
    node.style.backgroundSize = '';
    node.style.animation = '';
  }

  function onEnter() {
    saveOriginals();

    switch (style) {
      case 'swap':
        node.style.transition = 'background 0.3s ease, color 0.3s ease, transform 0.1s, box-shadow 0.1s';
        node.style.background = 'var(--brand-text)';
        node.style.color = 'var(--brand-white)';
        break;

      case 'gradient':
        node.style.transition = 'background 0.4s ease, transform 0.1s, box-shadow 0.1s';
        node.style.background = 'linear-gradient(135deg, #F9B4AB, #D4A5C9, #A8D8EA)';
        node.style.backgroundSize = '200% 200%';
        node.style.animation = 'gradientFlow 4s ease-in-out infinite';
        break;

      case 'glow': {
        // Add a pseudo-glow by using box-shadow with a gradient-like spread
        node.style.transition = 'box-shadow 0.4s ease, transform 0.1s';
        const currentShadow = getComputedStyle(node).boxShadow;
        node.style.boxShadow = `${currentShadow}, 0 0 20px 4px rgba(212,165,201,0.4), 0 0 40px 8px rgba(168,216,234,0.2)`;
        break;
      }

      case 'bounce':
        node.style.transition = 'transform 0.4s cubic-bezier(0.34, 1.56, 0.64, 1), letter-spacing 0.3s ease, box-shadow 0.1s';
        node.style.transform = 'scale(1.06)';
        node.style.letterSpacing = '0.04em';
        break;
    }
  }

  function onLeave() {
    restoreOriginals();
    node.style.transition = 'background 0.3s ease, color 0.3s ease, transform 0.1s, box-shadow 0.1s, letter-spacing 0.3s ease';
  }

  node.addEventListener('mouseenter', onEnter);
  node.addEventListener('mouseleave', onLeave);

  return {
    destroy() {
      node.removeEventListener('mouseenter', onEnter);
      node.removeEventListener('mouseleave', onLeave);
    },
  };
}
