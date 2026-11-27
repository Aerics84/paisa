<script lang="ts">
  import { setColorPreference, themePreference, type ThemePreference } from "$lib/theme";
  import { t } from "$lib/i18n";
  import { refresh } from "../../store";

  const nextPreference: Record<ThemePreference, ThemePreference> = {
    system: "light",
    light: "dark",
    dark: "system"
  };
  const themeKeys = {
    system: "theme.system",
    light: "theme.light",
    dark: "theme.dark"
  } as const;

  const next = $derived(nextPreference[$themePreference]);
  const nextLabel = $derived($t(themeKeys[next]));

  const toggle = async () => {
    if (await refresh()) {
      setColorPreference(next);
    }
  };
</script>

<button
  type="button"
  onclick={toggle}
  class="theme-toggle p-1"
  title={$t("theme.switchTo", { mode: nextLabel })}
  aria-label={$t("theme.switchTo", { mode: nextLabel })}
>
  {#if $themePreference === "system"}
    <svg
      aria-hidden="true"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
    >
      <rect x="3" y="3" width="18" height="13" rx="2" />
      <path d="M12 16v5m-4 0h8" />
    </svg>
  {:else}
    <svg class="sun-and-moon" aria-hidden="true" width="24" height="24" viewBox="0 0 24 24">
      <mask class="moon" id="moon-mask">
        <rect x="0" y="0" width="100%" height="100%" fill="white" />
        <circle cx="24" cy="10" r="6" fill="black" />
      </mask>
      <circle class="sun" cx="12" cy="12" r="6" mask="url(#moon-mask)" fill="currentColor" />
      <g class="sun-beams" stroke="currentColor">
        <line x1="12" y1="1" x2="12" y2="3" />
        <line x1="12" y1="21" x2="12" y2="23" />
        <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
        <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
        <line x1="1" y1="12" x2="3" y2="12" />
        <line x1="21" y1="12" x2="23" y2="12" />
        <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
        <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
      </g>
    </svg>
  {/if}
</button>
