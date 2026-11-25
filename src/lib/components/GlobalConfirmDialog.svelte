<script lang="ts">
  import ConfirmDialog from "$lib/components/ConfirmDialog.svelte";
  import { t } from "$lib/i18n";
  import { confirmRequest, resolveConfirm } from "$lib/confirm";

  let open = $state(false);

  $effect(() => {
    open = $confirmRequest !== null;
  });
</script>

{#if $confirmRequest}
  {#key $confirmRequest.id}
    <ConfirmDialog
      bind:open
      title={$confirmRequest.title}
      description={$confirmRequest.description}
      confirmLabel={$confirmRequest.confirmLabel ?? $t("dialog.confirm")}
      cancelLabel={$confirmRequest.cancelLabel ?? $t("common.cancel")}
      variant={$confirmRequest.variant ?? "default"}
      onconfirm={() => resolveConfirm(true)}
      oncancel={() => resolveConfirm(false)}
    />
  {/key}
{/if}
