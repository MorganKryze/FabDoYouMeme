<script lang="ts">
  import { onMount } from 'svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Save, RotateCcw } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';

  let { src, onSave, readOnly = false }: {
    src: string | null;
    onSave: (blob: Blob) => void;
    readOnly?: boolean;
  } = $props();

  // Bucket names match the five values produced by the backend's
  // storage.DetectOrientation so the user-picked crop and the auto-detected
  // orientation share the same vocabulary end-to-end.
  type Orientation =
    | 'landscape_4_3'
    | 'landscape_16_9'
    | 'square'
    | 'portrait_3_4'
    | 'portrait_9_16';
  const DIMENSIONS: Record<Orientation, { w: number; h: number }> = {
    landscape_4_3:  { w: 1200, h: 900 },
    landscape_16_9: { w: 1280, h: 720 },
    square:         { w: 1200, h: 1200 },
    portrait_3_4:   { w: 900, h: 1200 },
    portrait_9_16:  { w: 720, h: 1280 },
  };
  const FRAME_WIDTH_PX: Record<Orientation, number> = {
    landscape_4_3:  600,
    landscape_16_9: 640,
    square:         520,
    portrait_3_4:   450,
    portrait_9_16:  405,
  };

  let canvas: HTMLCanvasElement;
  let ctx: CanvasRenderingContext2D | null = null;
  let img: HTMLImageElement | null = null;

  let orientation = $state<Orientation>('landscape_4_3');

  // Same nearest-neighbour rule as backend storage.DetectOrientation, so the
  // editor pre-selects the same bucket the server would have picked from the
  // raw upload. The user can still override by clicking another pill.
  const ORIENTATION_TARGETS: { value: Orientation; ratio: number }[] = [
    { value: 'landscape_16_9', ratio: 16 / 9 },
    { value: 'landscape_4_3', ratio: 4 / 3 },
    { value: 'square', ratio: 1 },
    { value: 'portrait_3_4', ratio: 3 / 4 },
    { value: 'portrait_9_16', ratio: 9 / 16 },
  ];

  function bucketForRatio(w: number, h: number): Orientation {
    if (w <= 0 || h <= 0) return 'landscape_4_3';
    const r = w / h;
    let best = ORIENTATION_TARGETS[0];
    let bestDist = Math.abs(r - best.ratio);
    for (const t of ORIENTATION_TARGETS.slice(1)) {
      const d = Math.abs(r - t.ratio);
      if (d < bestDist) {
        best = t;
        bestDist = d;
      }
    }
    return best.value;
  }
  let x = $state(0);
  let y = $state(0);
  let fitScale = $state(1);
  let zoom = $state(1);

  let dragging = $state(false);
  let dragOrigin = { px: 0, py: 0, ix: 0, iy: 0 };

  const dims = $derived(DIMENSIONS[orientation]);

  onMount(() => {
    ctx = canvas.getContext('2d');
  });

  $effect(() => {
    if (src) loadImage(src);
  });

  $effect(() => {
    void orientation; void zoom; void x; void y;
    if (img && ctx) render();
  });

  function loadImage(url: string) {
    const el = new Image();
    el.crossOrigin = 'anonymous';
    el.onload = () => {
      img = el;
      // Pre-select the bucket whose aspect ratio is closest to the source.
      // Mirrors backend storage.DetectOrientation so what the user sees
      // selected matches what the backend would have picked unattended.
      orientation = bucketForRatio(el.naturalWidth, el.naturalHeight);
      fitToFrame();
      // Force a paint even if fitToFrame left every reactive value at its
      // previous number (happens when a new version of the same image is
      // loaded — same dimensions → identical fitScale/x/y → no render effect
      // would fire on its own).
      render();
    };
    el.src = url;
  }

  function fitToFrame() {
    if (!img) return;
    const sx = dims.w / img.naturalWidth;
    const sy = dims.h / img.naturalHeight;
    fitScale = Math.max(sx, sy);
    zoom = 1;
    x = dims.w / 2;
    y = dims.h / 2;
  }

  function clampPosition() {
    if (!img) return;
    const s = fitScale * zoom;
    const w = img.naturalWidth * s;
    const h = img.naturalHeight * s;
    x = Math.min(w / 2, Math.max(dims.w - w / 2, x));
    y = Math.min(h / 2, Math.max(dims.h - h / 2, y));
  }

  function render() {
    if (!ctx || !img) return;
    if (canvas.width !== dims.w || canvas.height !== dims.h) {
      canvas.width = dims.w;
      canvas.height = dims.h;
    }
    clampPosition();
    ctx.clearRect(0, 0, dims.w, dims.h);
    const s = fitScale * zoom;
    const w = img.naturalWidth * s;
    const h = img.naturalHeight * s;
    ctx.drawImage(img, x - w / 2, y - h / 2, w, h);
  }

  function getCanvasPos(e: MouseEvent | TouchEvent): { x: number; y: number } {
    const rect = canvas.getBoundingClientRect();
    const sx = canvas.width / rect.width;
    const sy = canvas.height / rect.height;
    const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX;
    const clientY = 'touches' in e ? e.touches[0].clientY : e.clientY;
    return { x: (clientX - rect.left) * sx, y: (clientY - rect.top) * sy };
  }

  function startDrag(e: MouseEvent | TouchEvent) {
    const p = getCanvasPos(e);
    dragging = true;
    dragOrigin = { px: p.x, py: p.y, ix: x, iy: y };
  }

  function doDrag(e: MouseEvent | TouchEvent) {
    if (!dragging) return;
    const p = getCanvasPos(e);
    x = dragOrigin.ix + (p.x - dragOrigin.px);
    y = dragOrigin.iy + (p.y - dragOrigin.py);
  }

  function stopDrag() {
    dragging = false;
  }

  function onWheel(e: WheelEvent) {
    e.preventDefault();
    const next = zoom + (-e.deltaY * 0.0015);
    zoom = Math.max(1, Math.min(5, next));
  }

  function setOrientation(next: Orientation) {
    if (orientation === next) return;
    orientation = next;
    fitToFrame();
  }

  function save() {
    // JPEG at q=0.9 keeps the blob well under SvelteKit's default 512 KiB
    // body-size limit for a 1200×1200 canvas — PNG was regularly blowing past
    // it and the proxy rejected the upload before it reached the backend.
    canvas.toBlob(
      (blob) => {
        if (blob) onSave(blob);
      },
      'image/jpeg',
      0.9
    );
  }

  // Buttons sit in a tight pill row; long localised labels wrap to two lines
  // and overflow the column on narrow studio panels. The compact ratio
  // (`16:9`, `4:3`, …) carries the orientation visually, with the full
  // localised name behind a tooltip for screen readers and pointer hover.
  const orientationOptions = $derived<{ value: Orientation; label: string; ratio: string }[]>([
    { value: 'landscape_16_9', label: m.studio_image_orientation_landscape_16_9(), ratio: '16:9' },
    { value: 'landscape_4_3', label: m.studio_image_orientation_landscape_4_3(), ratio: '4:3' },
    { value: 'square', label: m.studio_image_orientation_square(), ratio: '1:1' },
    { value: 'portrait_3_4', label: m.studio_image_orientation_portrait_3_4(), ratio: '3:4' },
    { value: 'portrait_9_16', label: m.studio_image_orientation_portrait_9_16(), ratio: '9:16' },
  ]);
</script>

<div class="flex flex-col gap-3">
  <!-- Toolbar -->
  <div class="flex items-center gap-3 flex-wrap">
    <div class="inline-flex rounded-lg border border-brand-border bg-background p-1 shadow-sm">
      {#each orientationOptions as opt (opt.value)}
        <button
          type="button"
          onclick={() => setOrientation(opt.value)}
          title={opt.label}
          aria-label={opt.label}
          class="px-2 py-1.5 text-xs font-semibold rounded-md transition-colors whitespace-nowrap tabular-nums {orientation === opt.value ? 'bg-primary text-primary-foreground shadow' : 'text-brand-text-muted hover:text-foreground hover:bg-muted'}"
        >
          {opt.ratio}
        </button>
      {/each}
    </div>
    <label class="flex items-center gap-2 text-xs text-brand-text-muted">
      {m.studio_image_zoom_label()}
      <input
        type="range"
        min="1"
        max="5"
        step="0.01"
        bind:value={zoom}
        class="w-32 accent-primary"
      />
    </label>
    <button
      type="button"
      onclick={fitToFrame}
      class="px-3 py-1.5 text-xs font-semibold rounded-md border border-brand-border bg-background hover:bg-muted inline-flex items-center gap-1 shadow-sm"
    >
      <RotateCcw size={12} strokeWidth={2.5} />
      {m.studio_image_reset()}
    </button>
  </div>

  <!-- Canvas frame (enforces the chosen aspect ratio visually) -->
  <div class="flex justify-center">
    <div
      class="relative overflow-hidden rounded-lg border border-brand-border bg-muted/20"
      style="aspect-ratio: {dims.w} / {dims.h}; width: min(100%, {FRAME_WIDTH_PX[orientation]}px);"
    >
      <canvas
        bind:this={canvas}
        class="w-full h-full touch-none {dragging ? 'cursor-grabbing' : 'cursor-grab'}"
        onmousedown={startDrag}
        onmousemove={doDrag}
        onmouseup={stopDrag}
        onmouseleave={stopDrag}
        onwheel={onWheel}
        ontouchstart={(e) => { e.preventDefault(); startDrag(e); }}
        ontouchmove={(e) => { e.preventDefault(); doDrag(e); }}
        ontouchend={stopDrag}
      ></canvas>
    </div>
  </div>

  {#if !readOnly}
    <button
      type="button"
      onclick={save}
      use:pressPhysics={'dark'}
      class="h-10 rounded-lg bg-primary text-primary-foreground text-sm font-medium inline-flex items-center justify-center gap-1.5"
    >
      <Save size={14} strokeWidth={2.5} />
      {m.studio_image_save_version()}
    </button>
  {/if}
</div>
