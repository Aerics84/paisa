import { derived, writable, type Readable, type Writable } from "svelte/store";
import { de } from "./messages/de";
import { en } from "./messages/en";

/**
 * Typed i18n core.
 *
 * The locale is a plain Svelte writable store; translations are exposed as a
 * derived store so components can use `$t("key")` and re-render on locale
 * changes. No i18n framework or runtime dependency is involved.
 */
export type Locale = "en" | "de";
export type MessageKey = keyof typeof en;
export type MessageParams = Record<string, string | number>;

const dictionaries: Record<Locale, Record<MessageKey, string>> = { en, de };

/** Active UI locale. Starts in English until configuration is applied. */
export const locale: Writable<Locale> = writable<Locale>("en");

/**
 * Map a config locale (for example `de-DE` or `en-IN`) to a supported UI
 * locale. Derivation intentionally uses only the locale string: regional
 * profile and tax regime must never pick the UI language.
 */
export function resolveLocale(configLocale: string): Locale {
  if (typeof configLocale !== "string") {
    return "en";
  }
  return configLocale.trim().toLowerCase().startsWith("de") ? "de" : "en";
}

/** Normalize and apply a locale, returning the locale that was set. */
export function setLocale(value: string): Locale {
  const next = resolveLocale(value);
  locale.set(next);
  return next;
}

/**
 * Replace `{name}` placeholders with matching params.
 *
 * A placeholder whose param is missing (or explicitly `undefined`) is left in
 * place, so the untranslated fragment stays visible in the UI instead of
 * silently rendering as `undefined`.
 */
function interpolate(template: string, params?: MessageParams): string {
  if (!params) {
    return template;
  }
  return template.replace(/\{(\w+)\}/g, (placeholder, name: string) => {
    const value = params[name];
    return value === undefined ? placeholder : String(value);
  });
}

/** Look up a key in the active catalog, falling back to English. */
function lookup($locale: Locale, key: string): string | undefined {
  return dictionaries[$locale][key as MessageKey] ?? dictionaries.en[key as MessageKey];
}

/**
 * Translate `key` for the active locale.
 *
 * When `params.count` is set, the plural category is selected with
 * `Intl.PluralRules($locale)` and `${key}_${category}` is looked up first,
 * then `${key}_other`, then the base key. Unknown keys resolve to the key
 * itself so missing translations are visible and never crash the UI.
 */
export const t: Readable<(key: MessageKey, params?: MessageParams) => string> = derived(
  locale,
  ($locale) => {
    return (key: MessageKey, params?: MessageParams): string => {
      const hasCount = params !== undefined && params.count !== undefined && params.count !== null;
      const candidates: string[] = [];

      if (hasCount) {
        const plural = new Intl.PluralRules($locale).select(Number(params.count));
        candidates.push(`${key}_${plural}`, `${key}_other`);
      }
      candidates.push(key);

      for (const candidate of candidates) {
        const template = lookup($locale, candidate);
        if (template !== undefined) {
          return interpolate(template, params);
        }
      }
      return key;
    };
  }
);
