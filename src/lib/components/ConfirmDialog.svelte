<script context="module" lang="ts">
  let nextDialogId = 0;
</script>

<script lang="ts">
  import { onDestroy, tick } from "svelte";
  import Modal from "$lib/components/Modal.svelte";

  export let open = false;
  export let title: string;
  export let description: string | undefined = undefined;
  export let confirmLabel = "Confirm";
  export let cancelLabel = "Cancel";
  export let variant: "default" | "danger" = "default";
  export let onconfirm: () => void = () => {};
  export let oncancel: (() => void) | undefined = undefined;

  const dialogId = ++nextDialogId;
  const titleId = `confirm-dialog-title-${dialogId}`;
  const descriptionId = `confirm-dialog-description-${dialogId}`;
  const focusableSelector =
    'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

  let dialogElement: HTMLElement | null = null;
  let confirmButton: HTMLButtonElement | undefined = undefined;
  let cancelButton: HTMLButtonElement | undefined = undefined;
  let previousFocus: HTMLElement | null = null;
  let confirming = false;
  let wasOpen = false;
  let destroyed = false;

  function focusableElements(): HTMLElement[] {
    if (!dialogElement) return [];
    return Array.from(dialogElement.querySelectorAll<HTMLElement>(focusableSelector)).filter(
      (element) => !element.hasAttribute("disabled") && element.tabIndex >= 0
    );
  }

  async function focusDialog() {
    await tick();
    if (destroyed || !open) return;

    const active = document.activeElement;
    previousFocus = active instanceof HTMLElement ? active : null;

    // Destructive dialogs start on the safe action.
    const target = variant === "danger" ? cancelButton : confirmButton;
    (target ?? dialogElement)?.focus();
  }

  function restoreFocusNow() {
    const target = previousFocus;
    previousFocus = null;
    if (target && document.contains(target)) {
      target.focus();
    }
  }

  async function restoreFocus() {
    await tick();
    if (destroyed) return;
    restoreFocusNow();
  }

  onDestroy(() => {
    destroyed = true;
    restoreFocusNow();
  });

  // Modal closes itself when the background or the close button is used.
  // Treat every close that did not come from the confirm button as a cancel.
  $: {
    if (open && !wasOpen) {
      confirming = false;
      void focusDialog();
    } else if (!open && wasOpen) {
      if (!confirming) oncancel?.();
      void restoreFocus();
    }
    wasOpen = open;
  }

  function confirm(close: (event: unknown) => void) {
    confirming = true;
    close(null);
    onconfirm();
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!open) return;

    if (event.key === "Escape") {
      event.preventDefault();
      open = false;
      return;
    }

    if (event.key !== "Tab") return;

    const focusables = focusableElements();
    if (focusables.length === 0) {
      event.preventDefault();
      dialogElement?.focus();
      return;
    }

    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    const active = document.activeElement;

    if (!dialogElement?.contains(active as Node)) {
      event.preventDefault();
      (event.shiftKey ? last : first).focus();
      return;
    }

    if (event.shiftKey && active === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && active === last) {
      event.preventDefault();
      first.focus();
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<Modal
  bind:active={open}
  bind:dialogElement
  role="dialog"
  ariaModal="true"
  ariaLabelledby={titleId}
  ariaDescribedby={description ? descriptionId : undefined}
  width="min(440px, calc(100vw - 32px))"
>
  <svelte:fragment slot="head" let:close>
    <p class="modal-card-title" id={titleId}>{title}</p>
    <button class="delete" aria-label="close" type="button" on:click={close}></button>
  </svelte:fragment>
  <svelte:fragment slot="body">
    {#if description}
      <p id={descriptionId}>{description}</p>
    {/if}
  </svelte:fragment>
  <svelte:fragment slot="foot" let:close>
    <button class="button" type="button" bind:this={cancelButton} on:click={close}
      >{cancelLabel}</button
    >
    <button
      class="button {variant === 'danger' ? 'is-danger' : 'is-primary'}"
      type="button"
      bind:this={confirmButton}
      on:click={() => confirm(close)}>{confirmLabel}</button
    >
  </svelte:fragment>
</Modal>
