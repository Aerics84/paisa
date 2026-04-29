import { sveltekit } from "@sveltejs/kit/vite";

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
  plugins: [sveltekit()],
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
