import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig, type UserConfig } from 'vite';

// The `test` block is read by vitest at runtime. vitest ships its own vite
// typings (a different major than the app's vite), so the config is asserted as
// a plain vite UserConfig.
export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	resolve: {
		conditions: ['browser']
	},
	server: {
		proxy: {
			'/graphql': { target: 'http://localhost:8090', ws: true },
			'/llms.txt': { target: 'http://localhost:8090' }
		}
	},
	test: {
		environment: 'jsdom',
		setupFiles: ['./src/test/setup.ts'],
		include: ['src/**/*.{test,spec}.{js,ts}']
	}
} as UserConfig);
