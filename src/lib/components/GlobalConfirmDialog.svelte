<script lang="ts">
  import ConfirmDialog from "$lib/components/ConfirmDialog.svelte";
  import { confirmRequest, resolveConfirm } from "$lib/confirm";

  let open = false;
  $: open = $confirmRequest !== null;
</script>

{#if $confirmRequest}
  {#key $confirmRequest.id}
    <ConfirmDialog
      bind:open
      title={$confirmRequest.title}
      description={$confirmRequest.description}
      confirmLabel={$confirmRequest.confirmLabel ?? "Confirm"}
      cancelLabel={$confirmRequest.cancelLabel ?? "Cancel"}
      variant={$confirmRequest.variant ?? "default"}
      onconfirm={() => resolveConfirm(true)}
      oncancel={() => resolveConfirm(false)}
    />
  {/key}
{/if}
