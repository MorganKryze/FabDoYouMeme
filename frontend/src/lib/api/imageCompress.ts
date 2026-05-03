// frontend/src/lib/api/imageCompress.ts
//
// Client-side image compression so bulk uploads fit through tight upstream
// proxy body caps (Pangolin / nginx-ingress / etc. typically default to
// 1 MiB and surface oversize requests as a generic "500 Internal Server
// Error" with no actionable detail). Compression runs in the browser via
// canvas — no extra dependency — and is best-effort: on any decode or
// encode failure we return the original file untouched and let the upload
// fail honestly with the file-too-large reason rather than silently
// stripping bytes the user didn't ask us to.

export interface CompressOptions {
  /** Skip compression when the file is already this size or smaller, in bytes. */
  maxBytes: number;
  /** Cap the longest edge of the output image, in CSS pixels. */
  maxDimension: number;
  /** Initial JPEG quality (0..1). The encoder retries with progressively lower
   *  quality until the output fits maxBytes or quality bottoms out. */
  startQuality?: number;
  /** Lowest quality tried before giving up. Defaults to 0.55 — below this
   *  artefacts become obvious for typical meme content. */
  minQuality?: number;
}

// loadImage decodes the file into an HTMLImageElement. The browser handles
// JPEG/PNG/WebP transparently; if the file is not an image (defensive guard)
// the promise rejects and the caller falls back to the original.
function loadImage(file: File): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      URL.revokeObjectURL(url);
      resolve(img);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error('image decode failed'));
    };
    img.src = url;
  });
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error('canvas.toBlob returned null'))),
      type,
      quality
    );
  });
}

// compressImage returns a possibly-smaller File. PNG with transparency is
// kept as PNG (single re-encode at native size, no quality loop — PNG is
// lossless); everything else is re-encoded as JPEG with quality stepping
// until the output fits `maxBytes` or quality bottoms out at `minQuality`.
//
// The returned File keeps the original basename so dedup-by-name continues
// to work; only the extension switches to .jpg when the encoder converts
// PNG/WebP to JPEG. The MIME type on the returned File is the actual
// encoded type so the backend's magic-byte validation passes.
export async function compressImage(file: File, options: CompressOptions): Promise<File> {
  if (file.size <= options.maxBytes) return file;
  if (!file.type.startsWith('image/')) return file;

  let img: HTMLImageElement;
  try {
    img = await loadImage(file);
  } catch {
    return file;
  }

  let { width, height } = img;
  if (width > options.maxDimension || height > options.maxDimension) {
    const scale = Math.min(options.maxDimension / width, options.maxDimension / height);
    width = Math.max(1, Math.round(width * scale));
    height = Math.max(1, Math.round(height * scale));
  }

  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext('2d');
  if (!ctx) return file;
  ctx.drawImage(img, 0, 0, width, height);

  const startQuality = options.startQuality ?? 0.85;
  const minQuality = options.minQuality ?? 0.55;
  const step = 0.1;

  // Always re-encode as JPEG, even for PNG inputs. Keeping PNGs lossless
  // here meant a high-res screenshot would round-trip at multi-MiB and
  // still trip the upstream proxy's body cap. Meme content rarely needs
  // alpha, and the visible quality drop at q=0.85 is invisible compared
  // to "import failed (500)". If alpha matters for a specific item the
  // single-image flow is the right tool — bulk imports prioritise getting
  // through the wire.

  let quality = startQuality;
  let best: Blob | null = null;
  while (quality >= minQuality - 1e-6) {
    try {
      const blob = await canvasToBlob(canvas, 'image/jpeg', quality);
      best = blob;
      if (blob.size <= options.maxBytes) break;
    } catch {
      return file;
    }
    quality -= step;
  }
  if (!best) return file;

  // Swap the extension to .jpg so downstream filename derivation (item
  // name = filename minus extension) remains tidy and the backend's MIME
  // validator agrees with the actual bytes.
  const newName = file.name.replace(/\.[^.]+$/, '') + '.jpg';
  return new File([best], newName, { type: 'image/jpeg' });
}
