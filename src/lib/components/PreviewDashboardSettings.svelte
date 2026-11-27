<script lang="ts">
  import { tick } from "svelte";
  import Modal from "./Modal.svelte";
  import { t } from "$lib/i18n";
  import {
    PREVIEW_PRESETS,
    PREVIEW_WIDGETS,
    defaultLayout,
    presetLayout,
    setWidgetVisible,
    type PreviewDashboardLayout
  } from "$lib/preview_dashboard";

  interface Props {
    layout: PreviewDashboardLayout;
    open?: boolean;
    onchange?: (layout: PreviewDashboardLayout) => void;
    returnFocusTo?: HTMLElement | null;
  }

  let {
    layout,
    open = $bindable(false),
    onchange = () => {},
    returnFocusTo = null
  }: Props = $props();

  let dialogElement = $state<HTMLElement | null>(null);
  let wasOpen = false;

  $effect(() => {
    const isOpen = open;
    if (isOpen && !wasOpen) {
      wasOpen = true;
      void tick().then(() => {
        dialogElement?.querySelector<HTMLElement>("button")?.focus();
      });
    } else if (!isOpen && wasOpen) {
      wasOpen = false;
      returnFocusTo?.focus();
    }
  });

  function choosePreset(next: PreviewDashboardLayout) {
    onchange(next);
  }

  function toggleWidget(id: (typeof PREVIEW_WIDGETS)[number]["id"], checked: boolean) {
    onchange(setWidgetVisible(layout, id, checked));
  }
</script>

<svelte:window onkeydown={(event) => open && event.key === "Escape" && (open = false)} />

<Modal
  bind:active={open}
  width="min(760px, 95vw)"
  ariaLabel={$t("dashboard.customizeTitle")}
  bind:dialogElement
>
  <svelte:fragment slot="head" let:close>
    <div class="is-flex is-align-items-center is-justify-content-space-between" style="width: 100%">
      <h3 class="modal-card-title">{$t("dashboard.customizeTitle")}</h3>
      <button
        class="delete"
        type="button"
        aria-label={$t("dialog.closeModal")}
        onclick={close}
      ></button>
    </div>
  </svelte:fragment>

  <svelte:fragment slot="body">
    <div
      data-testid="preview-settings"
      role="dialog"
      aria-modal="true"
      aria-label={$t("dashboard.customizeTitle")}
      class="paisa-dashboard-settings"
    >
      <p class="paisa-dashboard-settings__subtitle">{$t("dashboard.customizeSubtitle")}</p>

      <div class="paisa-dashboard-settings__group">
        <div class="paisa-dashboard-settings__heading">{$t("dashboard.presets")}</div>
        <div class="paisa-dashboard-settings__presets">
          {#each PREVIEW_PRESETS as preset (preset.id)}
            <button
              class="button paisa-dashboard-settings__preset"
              class:is-active={layout.preset === preset.id}
              type="button"
              data-testid={`preview-preset-${preset.id}`}
              onclick={() => choosePreset(presetLayout(preset.id))}
            >
              <strong>{$t(preset.labelKey)}</strong>
              <span>{$t(preset.descriptionKey)}</span>
            </button>
          {/each}
        </div>
      </div>

      <div class="paisa-dashboard-settings__group">
        <div class="paisa-dashboard-settings__heading">{$t("dashboard.customize")}</div>
        <div class="paisa-dashboard-settings__widgets">
          {#each PREVIEW_WIDGETS as widget (widget.id)}
            <label class="paisa-dashboard-settings__widget">
              <input
                type="checkbox"
                data-testid={`preview-widget-${widget.id}`}
                checked={layout.visible[widget.id]}
                onchange={(event) =>
                  toggleWidget(widget.id, (event.currentTarget as HTMLInputElement).checked)}
              />
              <span>
                <strong>{$t(widget.titleKey)}</strong>
                <small>{$t(widget.descriptionKey)}</small>
              </span>
            </label>
          {/each}
        </div>
      </div>
    </div>
  </svelte:fragment>

  <svelte:fragment slot="foot">
    <button
      class="button is-light"
      type="button"
      data-testid="preview-settings-reset"
      onclick={() => onchange(defaultLayout())}
    >
      {$t("dashboard.resetDefault")}
    </button>
    <button
      class="button is-primary"
      type="button"
      data-testid="preview-settings-done"
      onclick={() => (open = false)}
    >
      {$t("dashboard.done")}
    </button>
  </svelte:fragment>
</Modal>
