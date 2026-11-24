<script lang="ts">
  import Modal from "$lib/components/Modal.svelte";
  import _ from "lodash";

  interface Props {
    label?: string;
    help?: string;
    placeholder?: string;
    open?: boolean;
    onsave?: (destinationFile: string) => void;
  }

  let {
    label = "Save As",
    help = "Create or overwrite existing file",
    placeholder = "expense.ledger",
    open = $bindable(false),
    onsave
  }: Props = $props();
  let destinationFile = $state("");
</script>

<Modal bind:active={open}>
  <svelte:fragment slot="head" let:close>
    <p class="modal-card-title">{label}</p>
    <button class="delete" aria-label="close" onclick={(e) => close(e)}></button>
  </svelte:fragment>
  <div class="field" slot="body">
    <label class="label" for="save-filename">File Name</label>
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
    <button class="button" onclick={(e) => close(e)}>Cancel</button>
  </svelte:fragment>
</Modal>
