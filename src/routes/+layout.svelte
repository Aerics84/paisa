<script lang="ts">
  import "../tailwind.css";
  import { loadIconFont, type IconFont } from "$lib/icon";

  // Font Awesome metadata is built into the entry bundle; only the large sets
  // need a preload for icons that are already referenced by the configuration.
  const LAZY_ICON_FONTS: IconFont[] = ["arcticons", "mdi", "fluent-emoji-high-contrast"];

  function configuredIconFonts(config: UserConfig | undefined): IconFont[] {
    if (!config) {
      return [];
    }

    const symbols = [
      ...(config.accounts || []).map((account) => account?.icon),
      ...Object.values(config.goals || {}).flatMap((goals) =>
        (goals || []).map((goal) => goal?.icon)
      )
    ];
    const fonts = symbols
      .filter((symbol): symbol is string => typeof symbol === "string" && symbol.includes(":"))
      .map((symbol) => symbol.split(":")[0] as IconFont)
      .filter((font) => LAZY_ICON_FONTS.includes(font));

    return [...new Set(fonts)];
  }

  // Load the fonts before the routed content renders so configured icons are
  // drawn with their real glyph on the first paint. Remounting the app after
  // mount (`willRefresh`) would destroy pages that are still awaiting data.
  const iconsReady = Promise.all(
    configuredIconFonts(typeof USER_CONFIG === "undefined" ? undefined : USER_CONFIG).map((font) =>
      loadIconFont(font)
    )
  );
</script>

{#await iconsReady}
  <div aria-busy="true"></div>
{:then}
  <slot />
{/await}
