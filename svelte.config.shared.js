import adapter from "@sveltejs/adapter-static";
import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

/**
 * Shared Svelte/SvelteKit configuration.
 *
 * SvelteKit 3 removed `svelte.config.js` and takes its configuration through
 * the `sveltekit(...)` Vite plugin. This module keeps the same object available
 * to the ESLint Svelte parser without re-introducing the removed file.
 */
const svelteConfig = {
  alias: { $lib: "src/lib" },
  preprocess: vitePreprocess(),

  onwarn: (warning, handler) => {
    if (warning.code.startsWith("a11y-")) return;
    handler(warning);
  },

  adapter: adapter({
    pages: "web/static",
    assets: "web/static",
    out: "web/static",
    fallback: "index.html"
  })
};

export default svelteConfig;
