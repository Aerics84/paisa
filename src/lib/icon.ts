import _ from "lodash";
import fa6brands from "../../fonts/fa6-brands-info.json";
import fa6regular from "../../fonts/fa6-regular-info.json";
import fa6solid from "../../fonts/fa6-solid-info.json";
import { stemmer } from "stemmer";

export type IconFont =
  "fa6-solid" | "fa6-regular" | "fa6-brands" | "arcticons" | "mdi" | "fluent-emoji-high-contrast";

type IconCodepoints = Record<string, number>;

interface IconMetadata {
  codepoints: IconCodepoints;
}

const REPLACEMENT_GLYPH = 65533;

// Only the small Font Awesome metadata ships in the initial bundle. The large
// icon sets are imported on demand by `loadIconFont` so that the icon picker
// and configured icons do not cost every page load.
const codepoints: Partial<Record<IconFont, IconCodepoints>> = {
  "fa6-brands": fa6brands["codepoints"],
  "fa6-regular": fa6regular["codepoints"],
  "fa6-solid": fa6solid["codepoints"]
};

const ICON_FONTS: IconFont[] = [
  "arcticons",
  "fa6-brands",
  "fa6-regular",
  "fa6-solid",
  "fluent-emoji-high-contrast",
  "mdi"
];

function unwrapMetadata(module: unknown): IconMetadata {
  const namespace = module as { default?: IconMetadata } & Partial<IconMetadata>;
  return namespace.default ?? (namespace as IconMetadata);
}

const fontLoaders: Partial<Record<IconFont, () => Promise<IconMetadata>>> = {
  arcticons: async () => unwrapMetadata(await import("../../fonts/arcticons-info.json")),
  "fluent-emoji-high-contrast": async () =>
    unwrapMetadata(await import("../../fonts/fluent-emoji-high-contrast-info.json")),
  mdi: async () => unwrapMetadata(await import("../../fonts/mdi-info.json"))
};

const pendingLoads = new Map<IconFont, Promise<void>>();

/**
 * Loads the metadata of an icon font on demand. Resolves immediately for the
 * built-in Font Awesome sets and for fonts that are already cached.
 */
export function loadIconFont(font: IconFont): Promise<void> {
  if (codepoints[font]) {
    return Promise.resolve();
  }

  const loader = fontLoaders[font];
  if (!loader) {
    return Promise.resolve();
  }

  let pending = pendingLoads.get(font);
  if (!pending) {
    pending = loader()
      .then((metadata) => {
        codepoints[font] = metadata["codepoints"];
      })
      .catch((error) => {
        // A failed chunk load must not break rendering; keep the replacement
        // glyph and allow a later attempt to retry.
        pendingLoads.delete(font);
        console.warn(`Could not load icon font "${font}"`, error);
      });
    pendingLoads.set(font, pending);
  }

  return pending;
}

let icons: string[] | null = null;

/**
 * Loads every icon font and returns the flat `font:name` list used by the icon
 * picker. The result is cached after the first call.
 */
export async function loadIcons(): Promise<string[]> {
  await Promise.all(ICON_FONTS.map((font) => loadIconFont(font)));

  if (!icons) {
    icons = _.flatMap(ICON_FONTS, (font) => {
      return _.map(codepoints[font] || {}, (_code, name) => `${font}:${name}`);
    });
  }

  return icons;
}

export function iconGlyph(symbol: string): string {
  if (!symbol) {
    return String.fromCodePoint(REPLACEMENT_GLYPH);
  }
  const [font, name] = symbol.split(":");
  const code = codepoints[font as IconFont]?.[name] || REPLACEMENT_GLYPH;
  return String.fromCodePoint(code);
}

interface IconLookup {
  symbol: string;
  words: string[];
}

const ICON_LOOKUP: IconLookup[] = [
  { symbol: "fa6-solid:house-user", words: ["rent"] },
  { symbol: "fa6-solid:motorcycle", words: ["bike", "motorcycl"] },
  { symbol: "fa6-solid:car", words: ["car", "vehicl"] },
  { symbol: "fa6-solid:piggy-bank", words: ["check", "bank"] },
  { symbol: "fa6-solid:sack-dollar", words: ["asset"] },
  { symbol: "fa6-solid:money-check-dollar", words: ["equiti"] },
  { symbol: "fa6-solid:money-check-dollar", words: ["debt"] },
  { symbol: "fa6-solid:ring", words: ["gold"] },
  { symbol: "fa6-solid:house", words: ["realest", "hous", "homeloan"] },
  { symbol: "fa6-solid:credit-card", words: ["liabil"] },
  { symbol: "fa6-solid:wallet", words: ["expens"] },
  { symbol: "fa6-solid:building", words: ["incom"] },
  { symbol: "fa6-solid:gift", words: ["gift", "reward"] },
  { symbol: "fa6-solid:shirt", words: ["cloth"] },
  { symbol: "fa6-solid:graduation-cap", words: ["educ"] },
  { symbol: "fa6-solid:burger", words: ["food"] },
  { symbol: "fa6-solid:film", words: ["entertain"] },
  { symbol: "fa6-solid:car-crash", words: ["insur"] },
  { symbol: "fa6-solid:hand-holding-dollar", words: ["interest"] },
  { symbol: "fa6-solid:cart-shopping", words: ["shop"] },
  { symbol: "fa6-solid:utensils", words: ["restaur"] },
  { symbol: "fa6-solid:layer-group", words: ["misc"] },
  { symbol: "fa6-solid:plug-circle-bolt", words: ["util"] },
  { symbol: "fa6-solid:carrot", words: ["veget"] },
  { symbol: "fa6-solid:taxi", words: ["transport"] },
  { symbol: "fa6-solid:money-bill", words: ["cash"] }
];

const ICONS: Record<string, string> = _.chain(ICON_LOOKUP)
  .flatMap((icon) => _.map(icon.words, (word) => [word, iconGlyph(icon.symbol)]))
  .fromPairs()
  .value();

export function iconText(account: string): string {
  if (account == "") {
    return "";
  }

  const accounts =
    typeof USER_CONFIG !== "undefined" && USER_CONFIG ? USER_CONFIG.accounts || [] : [];
  const accountConfig = accounts.find((a) => a.name == account);
  if (!_.isEmpty(accountConfig?.icon)) {
    return iconGlyph(accountConfig.icon);
  }

  const parts = account.split(":");
  const part = stemmer(_.last(parts).toLowerCase());
  return ICONS[part] || iconText(_.dropRight(parts, 1).join(":"));
}

export function iconify(account: string, options?: { group?: string; suffix?: boolean }) {
  const icon = options?.group ? iconText(options.group + ":" + account) : iconText(account);
  if (icon == "") {
    return account;
  } else {
    if (options?.suffix) {
      return account + " " + icon;
    } else {
      return icon + " " + account;
    }
  }
}
