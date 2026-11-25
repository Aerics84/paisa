<script lang="ts">
  import { goto } from "$app/navigation";
  import { onMount } from "svelte";
  import Logo from "$lib/components/Logo.svelte";
  import { apiErrorMessage } from "$lib/api/client";
  import {
    runOnboarding,
    type OnboardingMode,
    type OnboardingProfileId
  } from "$lib/api/onboarding";
  import { setLocale, t } from "$lib/i18n";
  import { authStatus } from "$lib/utils";

  let profile = $state<OnboardingProfileId>("germany-eu");
  let mode = $state<OnboardingMode>("empty");
  let running = $state(false);
  let errorMessage = $state("");

  onMount(async () => {
    try {
      const status = await authStatus();
      if (status.locale) {
        setLocale(status.locale);
      }
    } catch {
      // The fallback locale keeps the page usable when status is unavailable.
    }
  });

  async function submit() {
    if (running) return;

    running = true;
    errorMessage = "";
    try {
      await runOnboarding(profile, mode);
      await goto("/");
    } catch (error) {
      errorMessage = $t("onboarding.error", { error: apiErrorMessage(error) });
    } finally {
      running = false;
    }
  }

  const selectedClass = "border-sky-500 bg-sky-50 dark:bg-sky-950/40";
  const unselectedClass = "border-transparent hover:border-slate-300";
</script>

<section class="section">
  <div class="container">
    <div class="columns is-centered">
      <div class="column is-8">
        <div class="flex items-center gap-2 mb-2">
          <Logo size={32} />
          <span class="is-size-4">Paisa</span>
        </div>

        <h1 class="title is-3">{$t("onboarding.title")}</h1>
        <p class="subtitle">{$t("onboarding.intro")}</p>

        <h2 class="title is-5 mt-5">{$t("onboarding.profileTitle")}</h2>
        <div class="columns">
          <div class="column">
            <button
              type="button"
              class="box w-full text-left border-2 {profile === 'germany-eu'
                ? selectedClass
                : unselectedClass}"
              aria-pressed={profile === "germany-eu"}
              onclick={() => (profile = "germany-eu")}
            >
              <p class="title is-6 mb-1">{$t("onboarding.profile.germanyEu")}</p>
              <p class="is-size-7">{$t("onboarding.profile.germanyEuHint")}</p>
            </button>
          </div>
          <div class="column">
            <button
              type="button"
              class="box w-full text-left border-2 {profile === 'india'
                ? selectedClass
                : unselectedClass}"
              aria-pressed={profile === "india"}
              onclick={() => (profile = "india")}
            >
              <p class="title is-6 mb-1">{$t("onboarding.profile.india")}</p>
              <p class="is-size-7">{$t("onboarding.profile.indiaHint")}</p>
            </button>
          </div>
        </div>

        <h2 class="title is-5 mt-5">{$t("onboarding.modeTitle")}</h2>
        <div class="columns">
          <div class="column">
            <button
              type="button"
              class="box w-full text-left border-2 {mode === 'empty'
                ? selectedClass
                : unselectedClass}"
              aria-pressed={mode === "empty"}
              onclick={() => (mode = "empty")}
            >
              <p class="title is-6 mb-1">{$t("onboarding.mode.empty")}</p>
              <p class="is-size-7">{$t("onboarding.mode.emptyHint")}</p>
            </button>
          </div>
          <div class="column">
            <button
              type="button"
              class="box w-full text-left border-2 {mode === 'demo'
                ? selectedClass
                : unselectedClass}"
              aria-pressed={mode === "demo"}
              onclick={() => (mode = "demo")}
            >
              <p class="title is-6 mb-1">{$t("onboarding.mode.demo")}</p>
              <p class="is-size-7">{$t("onboarding.mode.demoHint")}</p>
            </button>
          </div>
        </div>

        {#if errorMessage}
          <div class="notification is-danger mt-5">{errorMessage}</div>
        {/if}

        <div class="field is-grouped is-grouped-right mt-5">
          <div class="control">
            <button class="button is-link" disabled={running} onclick={submit}>
              {running ? $t("onboarding.running") : $t("onboarding.submit")}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</section>
