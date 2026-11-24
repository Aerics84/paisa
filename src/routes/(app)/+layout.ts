export const trailingSlash = "never";

import type { LayoutLoad } from "./$types";
import { ajax, configUpdated, setNow } from "$lib/utils";
import { preloadConfiguredIconFonts } from "$lib/icon";

export const load = (async () => {
  const { config, now } = await ajax("/api/config");
  if (now) {
    setNow(now);
  }
  globalThis.USER_CONFIG = config;
  configUpdated();
  await preloadConfiguredIconFonts(config);
  return {};
}) satisfies LayoutLoad;
