<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import favicon from '$lib/assets/favicon.svg';
	import TimeBackground from '$lib/components/TimeBackground.svelte';
	import BackgroundMusic from '$lib/components/room/BackgroundMusic.svelte';
	import { installPageTransitions } from '$lib/motion/navigation';
	import { theme } from '$lib/state/theme.svelte';

	let { children } = $props();

	let clockInterval: ReturnType<typeof setInterval> | null = null;

	onMount(() => {
		installPageTransitions();
		theme.tickTimeOfDay();
		clockInterval = setInterval(() => theme.tickTimeOfDay(), 5 * 60 * 1000);
	});

	onDestroy(() => {
		if (clockInterval !== null) clearInterval(clockInterval);
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<TimeBackground />
<BackgroundMusic />
{@render children()}
