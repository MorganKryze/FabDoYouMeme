<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import TimeBackground from '$lib/components/TimeBackground.svelte';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<TimeBackground />
{@render children()}
