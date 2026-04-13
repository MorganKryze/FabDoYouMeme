/**
 * Svelte action: button hover effects.
 *
 * Attach with `use:hoverEffect={'gradient'}` (or 'swap', 'glow', 'bounce',
 * 'rainbow'). Each variant adds a distinctive hover animation appropriate
 * for different action contexts per the brand spec.
 *
 * - gradient: pastel gradient fades in behind text (primary CTAs)
 * - swap: white → dark color inversion (standard actions)
 * - glow: gradient halo breathes behind button (highlighted actions)
 * - bounce: scale(1.06) with elastic spring (playful secondary)
 * - rainbow: animated rainbow border ring (reserved for one button — Next Round)
 */
type HoverStyle = 'gradient' | 'swap' | 'glow' | 'bounce' | 'rainbow';

export function hoverEffect(node: HTMLElement, style: HoverStyle = 'swap') {
  const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (prefersReduced) return {};

  // Store original inline styles to restore on leave
  let origBackground = '';
  let origColor = '';
  let origTransform = '';
  let origLetterSpacing = '';
  let origBorderColor = '';
  let origBoxShadow = '';

  function saveOriginals() {
    origBackground = node.style.background;
    origColor = node.style.color;
    origTransform = node.style.transform;
    origLetterSpacing = node.style.letterSpacing;
    origBorderColor = node.style.borderColor;
    origBoxShadow = node.style.boxShadow;
  }

  function restoreOriginals() {
    node.style.background = origBackground;
    node.style.color = origColor;
    node.style.transform = origTransform;
    node.style.letterSpacing = origLetterSpacing;
    node.style.borderColor = origBorderColor;
    node.style.boxShadow = origBoxShadow;
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
        node.style.transition = 'background 0.4s ease, color 0.3s ease, transform 0.1s, box-shadow 0.1s';
        // Reuse the live time-of-day palette rather than hardcoding colors —
        // otherwise the hover state stays evening-pink regardless of the
        // active theme. Text is forced to --brand-text so dark cards using
        // `text-brand-white` (which inverts in night mode) don't become
        // dark-on-dark once the gradient replaces their background.
        node.style.background =
          'linear-gradient(135deg, var(--brand-grad-2), var(--brand-grad-3), var(--brand-grad-4))';
        node.style.color = 'var(--brand-text)';
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

      case 'rainbow': {
        // Use a layered gradient background clipped to the border box plus
        // a wide multi-colored box-shadow halo. The shadow runs the same
        // gradientFlow keyframe so the ring feels animated.
        node.style.transition = 'box-shadow 0.4s ease, transform 0.1s, background 0.4s ease';
        node.style.background = 'linear-gradient(135deg, #F9B4AB, #FDDCB5, #B5E2D0, #A8D8EA, #D4A5C9)';
        node.style.backgroundSize = '300% 300%';
        node.style.animation = 'gradientFlow 3s ease-in-out infinite';
        node.style.borderColor = 'transparent';
        node.style.boxShadow =
          '0 0 0 2px #F9B4AB, 0 0 0 4px #FDDCB5, 0 0 0 6px #B5E2D0, 0 0 0 8px #A8D8EA, 0 10px 24px -4px rgba(0,0,0,0.18)';
        break;
      }
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
