import '@testing-library/jest-dom/vitest';
// Registers an afterEach hook that unmounts components rendered via
// @testing-library/svelte. Required because we run with globals:false,
// which disables the library's default auto-cleanup wiring.
import '@testing-library/svelte/vitest';
