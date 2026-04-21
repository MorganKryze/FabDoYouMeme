// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	namespace App {
		// interface Error {}
		interface Locals {
			user: {
				id: string;
				username: string;
				email: string;
				role: 'player' | 'admin';
				locale: 'en' | 'fr';
				created_at: string;
			} | null;
			nonce: string;
		}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}
}

export {};
