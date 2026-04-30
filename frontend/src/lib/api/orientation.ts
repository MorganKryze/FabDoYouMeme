// Maps the orientation bucket stored in `game_item_versions.payload.orientation`
// (one of five fixed values, set server-side at upload time) to a CSS class.
// Renderers wrap the <img> in a container with this class so the frame matches
// the image's bucket — `object-cover` inside then crops minimally on the
// off-axis dimension. Missing or unknown values fall back to landscape 4:3,
// which is the historical default for the bundled image pack.

export type Orientation =
  | 'landscape_4_3'
  | 'landscape_16_9'
  | 'square'
  | 'portrait_3_4'
  | 'portrait_9_16';

const CLASS_BY_ORIENTATION: Record<Orientation, string> = {
  landscape_4_3: 'is-landscape-4-3',
  landscape_16_9: 'is-landscape-16-9',
  square: 'is-square',
  portrait_3_4: 'is-portrait-3-4',
  portrait_9_16: 'is-portrait-9-16'
};

const FALLBACK: Orientation = 'landscape_4_3';

export function orientationOf(payload: unknown): Orientation {
  if (payload && typeof payload === 'object') {
    const o = (payload as { orientation?: unknown }).orientation;
    if (typeof o === 'string' && o in CLASS_BY_ORIENTATION) {
      return o as Orientation;
    }
  }
  return FALLBACK;
}

export function orientationClass(payload: unknown): string {
  return CLASS_BY_ORIENTATION[orientationOf(payload)];
}
