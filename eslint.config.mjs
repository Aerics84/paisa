import js from "@eslint/js";
import prettier from "eslint-config-prettier";
import globals from "globals";
import tsParser from "@typescript-eslint/parser";
import tsPlugin from "@typescript-eslint/eslint-plugin";
import svelte from "eslint-plugin-svelte";
import svelteConfig from "./svelte.config.js";

export default [
  {
    ignores: [
      ".DS_Store",
      "node_modules",
      "build",
      ".svelte-kit",
      "package",
      ".cache",
      "desktop/build",
      "web/static",
      ".env",
      ".env.*",
      "pnpm-lock.yaml",
      "package-lock.json",
      "yarn.lock"
    ]
  },
  js.configs.recommended,
  ...tsPlugin.configs["flat/recommended"],
  ...svelte.configs["flat/recommended"],
  prettier,
  ...svelte.configs["flat/prettier"],
  {
    languageOptions: {
      ecmaVersion: 2020,
      sourceType: "module",
      globals: {
        ...globals.browser,
        ...globals.es2021,
        ...globals.node
      }
    }
  },
  {
    files: ["**/*.svelte", "**/*.svelte.js", "**/*.svelte.ts"],
    languageOptions: {
      parserOptions: {
        parser: tsParser,
        extraFileExtensions: [".svelte"],
        svelteConfig
      }
    }
  },
  {
    rules: {
      "no-undef": "off",
      "@typescript-eslint/no-empty-object-type": "off",
      "svelte/valid-compile": ["error", { ignoreWarnings: true }],
      "@typescript-eslint/no-empty-function": "off",
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/no-unsafe-function-type": "off",
      "@typescript-eslint/no-unused-vars": [
        "warn",
        {
          argsIgnorePattern: "^_",
          varsIgnorePattern: "^_"
        }
      ],
      "@typescript-eslint/no-unused-expressions": "off",
      "svelte/no-immutable-reactive-statements": "off",
      "svelte/no-navigation-without-resolve": "off",
      "svelte/no-reactive-reassign": "off",
      "svelte/prefer-svelte-reactivity": "off",
      "svelte/require-each-key": "off"
    }
  }
];
