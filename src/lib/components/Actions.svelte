<script lang="ts">
  import { sync } from "$lib/sync";
  import { t } from "$lib/i18n";
  import { authStatus, isMobile, logout, supportsPortfolioAnalysis } from "$lib/utils";
  import { startPriceUpdate } from "$lib/price_update";
  import { refresh } from "../../store";
  import { obscure } from "../../persisted_store";
  import { goto } from "$app/navigation";
  import { onMount } from "svelte";

  async function syncWithLoader(request: Record<string, any>) {
    try {
      await sync(request);
    } finally {
      await refresh();
    }
  }

  const obscureId = "obscure";
  let last = $obscure;
  obscure.subscribe(() => {
    if ($obscure === last) return;

    void refresh();
  });

  async function doLogout() {
    await logout();
    goto("/login");
  }

  let showLogout = $state(false);

  onMount(async () => {
    const status = await authStatus();
    showLogout = status.authenticated;
  });
</script>

<div class="dropdown ml-2 is-hoverable {isMobile() ? 'is-left' : 'is-right'}">
  <div class="dropdown-trigger dropdown-icon">
    <button
      class="button is-large"
      aria-haspopup="true"
      aria-label={$t("actions.open")}
      type="button"
    >
      <span class="icon">
        <i class="fas fa-ellipsis-vertical"></i>
      </span>
    </button>
  </div>
  <div class="dropdown-menu" id="dropdown-menu4" role="menu">
    <div class="dropdown-content">
      <button
        onclick={() => syncWithLoader({ journal: true })}
        class="dropdown-item icon-text button-reset w-full has-text-left"
        type="button"
      >
        <span class="icon is-small">
          <i class="fa-regular fa-file-lines"></i>
        </span>
        <span>{$t("actions.syncJournal")}</span>
      </button>
      <button
        onclick={() => startPriceUpdate(true)}
        class="dropdown-item icon-text button-reset w-full has-text-left"
        type="button"
      >
        <span class="icon is-small">
          <i class="fas fa-dollar-sign"></i>
        </span>
        <span>{$t("actions.updatePrices")}</span>
      </button>
      {#if supportsPortfolioAnalysis(USER_CONFIG)}
        <button
          onclick={() => syncWithLoader({ portfolios: true })}
          class="dropdown-item icon-text button-reset w-full has-text-left"
          type="button"
        >
          <span class="icon is-small">
            <i class="fas fa-layer-group"></i>
          </span>
          <span>{$t("actions.updatePortfolios")}</span>
        </button>
      {/if}
      <hr class="dropdown-divider" />
      <div class="dropdown-item icon-text">
        <label for={obscureId} class="cursor-pointer w-full inline-block">
          <input bind:checked={$obscure} id={obscureId} type="checkbox" class="is-hidden" />
          <span class="ml-0 icon is-small">
            <i class="fas {$obscure ? 'fa-eye-slash' : 'fa-eye'}"></i>
          </span>
          <span>{$obscure ? $t("actions.showNumbers") : $t("actions.hideNumbers")}</span>
        </label>
      </div>
      {#if showLogout}
        <hr class="dropdown-divider" />
        <button
          onclick={() => doLogout()}
          class="dropdown-item icon-text button-reset w-full has-text-left"
          type="button"
        >
          <span class="icon is-small">
            <i class="fas fa-arrow-right-from-bracket"></i>
          </span>
          <span>{$t("actions.logout")}</span>
        </button>
      {/if}
    </div>
  </div>
</div>

<style lang="scss">
  .button-reset {
    background: none;
    border: none;
  }
</style>
