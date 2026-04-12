<script lang="ts">
  import { onMount } from 'svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Save } from '$lib/icons';

  let { src, onSave }: { src: string | null; onSave: (blob: Blob) => void } = $props();

  let canvas: HTMLCanvasElement;
  let ctx: CanvasRenderingContext2D | null = null;
  let drawing = $state(false);
  let tool = $state<'draw' | 'crop'>('draw');
  let strokeColor = $state('#ff0000');
  let strokeWidth = $state(3);

  let lastX = 0;
  let lastY = 0;

  onMount(() => {
    ctx = canvas.getContext('2d');
  });

  $effect(() => {
    if (src && ctx) loadImage(src);
  });

  function loadImage(url: string) {
    const img = new Image();
    img.crossOrigin = 'anonymous';
    img.onload = () => {
      if (!ctx) return;
      canvas.width = img.naturalWidth;
      canvas.height = img.naturalHeight;
      ctx.drawImage(img, 0, 0);
    };
    img.src = url;
  }

  function getCanvasPos(e: MouseEvent | TouchEvent): { x: number; y: number } {
    const rect = canvas.getBoundingClientRect();
    const scaleX = canvas.width / rect.width;
    const scaleY = canvas.height / rect.height;
    const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX;
    const clientY = 'touches' in e ? e.touches[0].clientY : e.clientY;
    return {
      x: (clientX - rect.left) * scaleX,
      y: (clientY - rect.top) * scaleY,
    };
  }

  function startDraw(e: MouseEvent | TouchEvent) {
    if (tool !== 'draw' || !ctx) return;
    drawing = true;
    const { x, y } = getCanvasPos(e);
    lastX = x; lastY = y;
  }

  function doDraw(e: MouseEvent | TouchEvent) {
    if (!drawing || tool !== 'draw' || !ctx) return;
    const { x, y } = getCanvasPos(e);
    ctx.beginPath();
    ctx.moveTo(lastX, lastY);
    ctx.lineTo(x, y);
    ctx.strokeStyle = strokeColor;
    ctx.lineWidth = strokeWidth;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.stroke();
    lastX = x; lastY = y;
  }

  function stopDraw() {
    drawing = false;
  }

  function save() {
    canvas.toBlob((blob) => {
      if (blob) onSave(blob);
    }, 'image/png');
  }
</script>

<div class="flex flex-col gap-3">
  <!-- Toolbar -->
  <div class="flex items-center gap-3 flex-wrap">
    <div class="flex gap-1 rounded-md border border-brand-border overflow-hidden">
      <button type="button" onclick={() => tool = 'draw'}
        class="px-3 py-1 text-xs font-medium transition-colors {tool === 'draw' ? 'bg-primary text-primary-foreground' : 'hover:bg-muted'}">
        Draw
      </button>
    </div>
    <label class="flex items-center gap-1 text-xs text-brand-text-muted">
      Color
      <input type="color" bind:value={strokeColor} class="h-6 w-8 rounded border-none cursor-pointer" />
    </label>
    <label class="flex items-center gap-1 text-xs text-brand-text-muted">
      Size
      <input type="range" min={1} max={20} bind:value={strokeWidth} class="w-16 accent-primary" />
    </label>
  </div>

  <!-- Canvas -->
  <div class="relative overflow-hidden rounded-lg border border-brand-border bg-muted/20">
    <canvas
      bind:this={canvas}
      class="w-full cursor-crosshair"
      onmousedown={startDraw}
      onmousemove={doDraw}
      onmouseup={stopDraw}
      onmouseleave={stopDraw}
      ontouchstart={(e) => { e.preventDefault(); startDraw(e); }}
      ontouchmove={(e) => { e.preventDefault(); doDraw(e); }}
      ontouchend={stopDraw}
    ></canvas>
  </div>

  <button
    type="button"
    onclick={save}
    use:pressPhysics={'dark'}
    class="h-10 rounded-lg bg-primary text-primary-foreground text-sm font-medium inline-flex items-center justify-center gap-1.5"
  >
    <Save size={14} strokeWidth={2.5} />
    Save as new version
  </button>
</div>
