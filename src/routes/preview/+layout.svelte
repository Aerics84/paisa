<script lang="ts">
  import { afterNavigate, beforeNavigate } from "$app/navigation";
  import { followCursor, delegate, hideAll } from "tippy.js";
  import _ from "lodash";
  import Spinner from "$lib/components/Spinner.svelte";
  import PreviewNavbar from "$lib/components/PreviewNavbar.svelte";
  import { willClearTippy, willRefresh } from "../../store";

  function clearTippy() {
    hideAll();
  }

  function setupTippy() {
    delegate("body", {
      target: "[data-tippy-content]",
      theme: "light",
      onShow: (instance) => {
        const content = instance.reference.getAttribute("data-tippy-content");
        if (!_.isEmpty(content)) {
          instance.setContent(content);
        } else {
          return false;
        }
      },
      maxWidth: "none",
      delay: 0,
      allowHTML: true,
      followCursor: true,
      popperOptions: {
        modifiers: [
          {
            name: "flip",
            options: {
              fallbackPlacements: ["auto"]
            }
          }
        ]
      },
      plugins: [followCursor]
    });
  }

  function prefixInternalLinks() {
    for (const anchor of document.querySelectorAll<HTMLAnchorElement>('a[href^="/"]')) {
      const href = anchor.getAttribute("href");
      if (
        !href ||
        href.startsWith("/preview") ||
        href.startsWith("/api/") ||
        href.startsWith("//") ||
        anchor.dataset.noPreviewPrefix === "true" ||
        anchor.target === "_blank"
      ) {
        continue;
      }

      anchor.setAttribute("href", `/preview${href}`);
    }
  }

  function refreshEnhancements() {
    setupTippy();
    prefixInternalLinks();
  }

  willClearTippy.subscribe(clearTippy);
  beforeNavigate(clearTippy);
  willRefresh.subscribe(() => {
    clearTippy();
    refreshEnhancements();
  });

  afterNavigate(() => {
    refreshEnhancements();
  });
</script>

{#key $willRefresh}
  <div class="paisa-app-frame">
    <PreviewNavbar />

    <div class="paisa-shell-content">
      <Spinner>
        <slot />
      </Spinner>
    </div>
  </div>
{/key}
