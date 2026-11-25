<script lang="ts">
  import Modal from "$lib/components/Modal.svelte";
  import { t } from "$lib/i18n";
  import _ from "lodash";

  interface Props {
    label?: string;
    help?: string;
    placeholder?: string;
    open?: boolean;
    onsave?: (destinationFile: string) => void;
  }

  let {
    label = $t("components.fileModal.title"),
    help = $t("components.fileModal.help"),
    placeholder = $t("components.fileModal.placeholder"),
    open = $bindable(false),
    onsave
  }: Props = $props();
  let destinationFile = $state("");
</script>

<Modal bind:active={open}>
  <svelte:fragment slot="head" let:close>
    <p class="modal-card-title">{label}</p>
    <button class="delete" aria-label={$t("dialog.closeModal")} onclick={(e) => close(e)}></button>
  </svelte:fragment>
  <div class="field" slot="body">
    <label class="label" for="save-filename">{$t("components.fileModal.fileName")}</label>
    <div class="control" id="save-filename">
      <input class="input" type="text" {placeholder} bind:value={destinationFile} />
      <p class="help">{help}</p>
    </div>
  </div>
  <svelte:fragment slot="foot" let:close>
    <button
      class="button is-success"
      disabled={_.isEmpty(destinationFile)}
      onclick={(e) => {
        onsave?.(destinationFile);
        close(e);
      }}>{label}</button
    >
    <button class="button" onclick={(e) => close(e)}>{$t("common.cancel")}</button>
  </svelte:fragment>
</Modal>
