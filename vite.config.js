import { sveltekit } from "@sveltejs/kit/vite";
import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";
import adapter from "@sveltejs/adapter-static";

/** @type {import('vite').UserConfig} */
const config = {
  build: {
    target: "es2021",
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            {
              name: "spreadsheet-vendor",
              test: /node_modules[\\/](xlsx|xlsx-populate|cfb|jszip|sax)/,
              priority: 30
            },
            {
              name: "pdf-vendor",
              test: /node_modules[\\/]pdfjs-dist/,
              priority: 25
            },
            {
              name: "editor-vendor",
              test: /node_modules[\\/](@codemirror|codemirror|@lezer)/,
              priority: 20
            },
            {
              name: "viz-vendor",
              test: /node_modules[\\/](d3|d3-|textures)/,
              priority: 15
            }
          ]
        }
      }
    }
  },
  css: {
    preprocessorOptions: {
      scss: {
        quietDeps: true,
        silenceDeprecations: ["import", "global-builtin", "if-function"]
      }
    }
  },
  plugins: [
    sveltekit({
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
    })
  ],
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:7500"
      }
    },
    fs: {
      allow: ["./fonts"]
    }
  }
};

export default config;
