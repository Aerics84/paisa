<script lang="ts">
  import "../tailwind.css";
  import { onMount } from "svelte";
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import { getOnboardingStatus } from "$lib/api/onboarding";

  onMount(async () => {
    try {
      const status = await getOnboardingStatus();
      if (status.needed && page.url.pathname !== "/onboarding") {
        await goto("/onboarding");
      }
    } catch {
      // Onboarding is optional for the layout: a failed status request must not
      // break the app, the login redirect or the read-only view.
    }
  });
</script>

<slot />
