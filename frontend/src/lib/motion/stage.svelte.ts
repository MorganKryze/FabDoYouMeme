/**
 * Lagging-mirror state class: wraps a source phase value and exposes
 * a `displayPhase` that updates only after a fade-out window, so the
 * UI can animate through the transition.
 *
 * Use a single shared instance (`stage`) and call `stage.sync(room.phase)`
 * from an `$effect` in the room page. Consumers render against
 * `stage.displayPhase` and toggle visibility with `class:hidden={!stage.visible}`.
 *
 * The 450ms hide window is half the full 0.9s curve, matching the
 * spec's fade-out/fade-in split.
 */
export class StageChoreographer {
  displayPhase = $state<string>('idle');
  visible = $state(true);

  private pendingTimeout: ReturnType<typeof setTimeout> | null = null;

  constructor(initial: string = 'idle') {
    this.displayPhase = initial;
  }

  sync(nextPhase: string): void {
    if (nextPhase === this.displayPhase && this.visible) return;

    if (this.pendingTimeout !== null) {
      clearTimeout(this.pendingTimeout);
      this.pendingTimeout = null;
    }

    this.visible = false;
    this.pendingTimeout = setTimeout(() => {
      this.displayPhase = nextPhase;
      this.visible = true;
      this.pendingTimeout = null;
    }, 450);
  }
}

export const stage = new StageChoreographer();
