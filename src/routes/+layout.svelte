<script lang="ts">
  import "../tailwind.css";
  import { onMount } from "svelte";
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import { getOnboardingStatus, type OnboardingStatus } from "$lib/api/onboarding";
  import { t } from "$lib/i18n";
  import { registerServiceWorker } from "$lib/pwa";
  import { initializeTheme } from "$lib/theme";
  import { theme } from "../store";

  let { children } = $props();
  let status = $state<OnboardingStatus | null>(null);
  // A remote first-run client cannot POST to the loopback-only endpoint. Show
  // the escape hatch instead of redirecting it into a dead end.
  let showRemoteNotice = $derived(
    status?.needed === true && status.can_setup === false && page.url.pathname === "/"
  );

  async function loadOnboarding() {
    try {
      status = await getOnboardingStatus();
      if (status.needed && status.can_setup && page.url.pathname !== "/onboarding") {
        await goto("/onboarding");
      }
    } catch {
      // Onboarding is optional for the layout: a failed status request must not
      // break the app, the login redirect or the read-only view.
    }
  }

  onMount(() => {
    registerServiceWorker();
    const disposeTheme = initializeTheme(theme.set);
    void loadOnboarding();

    return disposeTheme;
  });
</script>

{#if showRemoteNotice}
  <div class="notification is-warning m-3" role="status" data-testid="onboarding-remote-notice">
    {$t("onboarding.remoteLocked")}
  </div>
{/if}

{@render children()}
