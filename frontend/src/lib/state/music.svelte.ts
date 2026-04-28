// Shared singleton for the background-music feature so multiple views
// can drive the same audio element without remounting it. The audio
// itself + the play/pause/fade implementation still live in
// BackgroundMusic.svelte (mounted once at the root layout); this state
// just exposes the reactive surface and a stable command interface.

class MusicState {
  playing = $state(true);
  muted = $state(false);
  level = $state(1);
  /** True once BackgroundMusic.svelte has mounted and bound handlers.
   *  Consumers (e.g. RoomHeader) gate their UI on this flag so they
   *  don't render a button before audio control is available. */
  available = $state(false);

  /** Handlers wired by BackgroundMusic on mount; cleared on unmount.
   *  Public methods below proxy to these. */
  toggleHandler: (() => void) | null = null;
  setLevelHandler: ((n: number) => void) | null = null;

  toggle(): void {
    this.toggleHandler?.();
  }

  setLevel(n: number): void {
    this.setLevelHandler?.(n);
  }
}

export const music = new MusicState();
export const MUSIC_LEVELS = 5;
