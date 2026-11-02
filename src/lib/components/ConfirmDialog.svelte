<script lang="ts">
  import Modal from "$lib/components/Modal.svelte";

  export let open = false;
  export let title: string;
  export let description: string | undefined = undefined;
  export let confirmLabel = "Confirm";
  export let cancelLabel = "Cancel";
  export let variant: "default" | "danger" = "default";
  export let onconfirm: () => void = () => {};
  export let oncancel: (() => void) | undefined = undefined;

  let confirming = false;
  let wasOpen = false;

  // Modal closes itself when the background or the close button is used.
  // Treat every close that did not come from the confirm button as a cancel.
  $: {
    if (open && !wasOpen) {
      confirming = false;
    } else if (!open && wasOpen && !confirming) {
      oncancel?.();
    }
    wasOpen = open;
  }

  function confirm(close: (event: unknown) => void) {
    confirming = true;
    close(null);
    onconfirm();
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!open || event.key !== "Escape") return;
    event.preventDefault();
    open = false;
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<Modal bind:active={open} width="min(440px, calc(100vw - 32px))">
  <svelte:fragment slot="head" let:close>
    <p class="modal-card-title">{title}</p>
    <button class="delete" aria-label="close" type="button" on:click={close}></button>
  </svelte:fragment>
  <svelte:fragment slot="body">
    {#if description}
      <p>{description}</p>
    {/if}
  </svelte:fragment>
  <svelte:fragment slot="foot" let:close>
    <button class="button" type="button" on:click={close}>{cancelLabel}</button>
    <button
      class="button {variant === 'danger' ? 'is-danger' : 'is-primary'}"
      type="button"
      on:click={() => confirm(close)}>{confirmLabel}</button
    >
  </svelte:fragment>
</Modal>
