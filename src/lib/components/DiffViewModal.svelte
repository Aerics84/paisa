<script lang="ts">
  import Modal from "./Modal.svelte";
  import type { MergeView } from "@codemirror/merge";
  import { createDiffEditor } from "$lib/editor";
  import { t } from "$lib/i18n";
  import type { LedgerFile } from "$lib/utils";

  interface Props {
    oldFiles?: LedgerFile[];
    newFiles?: LedgerFile[];
    updatedTransactionsCount?: number;
    open?: boolean;
    onsave?: (files: LedgerFile[]) => void;
  }

  let {
    oldFiles = [],
    newFiles = [],
    updatedTransactionsCount = 0,
    open = $bindable(false),
    onsave
  }: Props = $props();

  let editorDom = $state<Element>();
  let editor: MergeView;
  let changedOldFiles = $derived.by(() => {
    const changed: LedgerFile[] = [];
    for (let i = 0; i < newFiles.length; i++) {
      if (oldFiles[i].content !== newFiles[i].content) {
        changed.push(oldFiles[i]);
      }
    }
    return changed;
  });
  let changedNewFiles = $derived.by(() => {
    const changed: LedgerFile[] = [];
    for (let i = 0; i < newFiles.length; i++) {
      if (oldFiles[i].content !== newFiles[i].content) {
        changed.push(newFiles[i]);
      }
    }
    return changed;
  });
  let selectedFileIndex = $state(0);

  $effect(() => {
    if (open) {
      if (editor) {
        editor.destroy();
      }

      if (changedOldFiles.length > 0) {
        editor = createDiffEditor(
          changedOldFiles[selectedFileIndex].content,
          changedNewFiles[selectedFileIndex].content,
          editorDom,
          changedNewFiles[selectedFileIndex].name
        );
      }
    }
  });
</script>

<Modal
  bind:active={open}
  width="min(1300px, 100vw)"
  bodyClass="p-0 min-h-[500px]"
  headerClass="pt-1 pb-1"
  footerClass="is-justify-content-right"
>
  <svelte:fragment slot="head" let:close>
    <p class="modal-card-title">
      {#if changedOldFiles.length > 0}
        {changedOldFiles[selectedFileIndex]?.name}
        [{selectedFileIndex + 1}/{changedNewFiles.length}]
      {:else}
        {$t("components.diffView.noChanges")}
      {/if}
    </p>
    <div class="field has-addons mt-3 mr-3">
      {#if changedOldFiles.length > 0}
        <div class="mr-3 mt-2">
          <!-- eslint-disable-next-line svelte/no-at-html-tags -- static catalog sentence with a numeric count -->
          {@html $t("components.diffView.transactionsChanged", {
            count: updatedTransactionsCount
          })}
        </div>
      {/if}
      <p class="control">
        <button
          class="button"
          disabled={selectedFileIndex <= 0}
          onclick={() => selectedFileIndex--}
        >
          <span class="icon is-small">
            <i class="fas fa-chevron-left"></i>
          </span>
          <span>{$t("components.diffView.prev")}</span>
        </button>
      </p>
      <p class="control">
        <button
          class="button"
          disabled={selectedFileIndex >= changedNewFiles.length - 1}
          onclick={() => selectedFileIndex++}
        >
          <span>{$t("components.diffView.next")}</span>
          <span class="icon is-small">
            <i class="fas fa-chevron-right"></i>
          </span>
        </button>
      </p>
    </div>
    <button class="delete" aria-label={$t("dialog.closeModal")} onclick={(e) => close(e)}></button>
  </svelte:fragment>
  <div class="field" slot="body">
    <div class="box py-0">
      <div class="diff-editor" bind:this={editorDom}></div>
      {#if changedOldFiles.length === 0}
        <div class="has-text-centered mt-6">
          <!-- eslint-disable-next-line svelte/no-at-html-tags -- static catalog sentence with translated emphasis -->
          {@html $t("components.diffView.noChangesHint", { oops: $t("common.oops") })}
        </div>
      {/if}
    </div>
  </div>
  <svelte:fragment slot="foot" let:close>
    <button class="button" onclick={(e) => close(e)}>{$t("common.cancel")}</button>
    {#if changedOldFiles.length > 0}
      <button
        class="button is-success"
        onclick={(e) => {
          onsave?.(changedNewFiles);
          close(e);
        }}>{$t("components.diffView.saveAll")}</button
      >
    {/if}
  </svelte:fragment>
</Modal>
