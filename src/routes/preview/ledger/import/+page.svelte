<script lang="ts">
  import Select from "svelte-select";
  import {
    createEditor as createTemplateEditor,
    editorState as templateEditorState
  } from "$lib/template_editor";
  import {
    createEditor as createPreviewEditor,
    updateContent as updatePreviewContent
  } from "$lib/editor";
  import Dropzone from "svelte-file-dropzone";
  import _ from "lodash";
  import type { EditorView } from "codemirror";
  import { onMount, untrack } from "svelte";
  import { ajax } from "$lib/utils";
  import { saveEditorFile } from "$lib/api/editor";
  import {
    deleteTemplate,
    getTemplates,
    upsertTemplate,
    type ImportTemplate
  } from "$lib/api/templates";
  import { accountTfIdf } from "../../../../store";
  import { askConfirm } from "$lib/confirm";
  import * as toast from "bulma-toast";
  import FileModal from "$lib/components/FileModal.svelte";
  import Modal from "$lib/components/Modal.svelte";
  import StatementImport from "$lib/components/StatementImport.svelte";
  import { t } from "$lib/i18n";

  // The template/preset flow is the default; statement staging is additive.
  let importMode = $state<"template" | "staging">("template");

  let templates = $state<ImportTemplate[]>([]);
  let selectedTemplate = $state<ImportTemplate>();
  let saveAsName = $state<string>();
  let lastTemplate: any;
  let lastData: any;
  let preview = $state("");
  let parseErrorMessage = $state<string>(null);
  let columnCount = $state<number>();
  let data = $state<any[][]>([]);
  let rows = $state<Array<Record<string, any>>>([]);
  let lastOptions: any;
  let options = $state<{ reverse: boolean; trim: boolean }>({ reverse: false, trim: true });

  let templateEditorDom = $state<Element>();
  let templateEditor = $state<EditorView>();

  let previewEditorDom = $state<Element>();
  let previewEditor = $state<EditorView>();
  let spreadsheetModule = $state<typeof import("$lib/spreadsheet") | null>(null);

  async function getSpreadsheetModule() {
    spreadsheetModule ||= await import("$lib/spreadsheet");
    return spreadsheetModule;
  }

  onMount(async () => {
    accountTfIdf.set(await ajax("/api/account/tf_idf"));
    ({ templates } = await getTemplates());
    selectedTemplate = templates[0];
    saveAsName = selectedTemplate.name;
    templateEditor = createTemplateEditor(selectedTemplate.content, templateEditorDom);
    previewEditor = createPreviewEditor(preview, previewEditorDom, { readonly: true });
    void getSpreadsheetModule();
  });

  let saveAsNameDuplicate = $derived(
    !!_.find(templates, { name: saveAsName, template_type: "custom" })
  );

  async function save() {
    const { template, saved, message } = await upsertTemplate(
      {
        name: saveAsName,
        content: templateEditor.state.doc.toString()
      },
      { background: true }
    );

    if (!saved) {
      toast.toast({
        message: $t("common.saveFailed", { name: saveAsName, reason: message }),
        type: "is-danger",
        duration: 10000
      });
      return;
    }

    ({ templates } = await getTemplates({ background: true }));
    selectedTemplate = _.find(templates, { id: template.id });
    saveAsName = selectedTemplate.name;
    toast.toast({
      message: $t("common.saved", { name: saveAsName }),
      type: "is-success"
    });

    $templateEditorState = _.assign({}, $templateEditorState, { hasUnsavedChanges: false });
  }

  async function remove() {
    const oldName = selectedTemplate.name;
    const confirmed = await askConfirm({
      title: $t("ledger.import.deleteTemplateTitle"),
      description: $t("ledger.import.deleteTemplateDescription", { name: oldName }),
      confirmLabel: $t("common.delete"),
      cancelLabel: $t("common.cancel"),
      variant: "danger"
    });
    if (!confirmed) {
      return;
    }
    const { success, message } = await deleteTemplate(selectedTemplate.name, {
      background: true
    });

    if (!success) {
      toast.toast({
        message: $t("ledger.import.removeFailed", { name: oldName, reason: message }),
        type: "is-danger",
        duration: 10000
      });
      return;
    }

    ({ templates } = await getTemplates({ background: true }));
    selectedTemplate = templates[0];
    saveAsName = selectedTemplate.name;
    toast.toast({
      message: $t("ledger.import.removed", { name: oldName }),
      type: "is-success"
    });

    $templateEditorState = _.assign({}, $templateEditorState, { hasUnsavedChanges: false });
  }

  let input: any;

  $effect(() => {
    if (!_.isEmpty(data) && $templateEditorState.template && spreadsheetModule) {
      if (
        lastTemplate != $templateEditorState.template ||
        lastData != data ||
        lastOptions != options
      ) {
        try {
          const nextPreview = spreadsheetModule.render(rows, $templateEditorState.template, {
            reverse: options.reverse,
            trim: options.trim
          });
          preview = nextPreview;
          updatePreviewContent(previewEditor, nextPreview);
          lastTemplate = $templateEditorState.template;
          lastData = data;
          lastOptions = _.clone(options);
        } catch (e) {
          console.log(e);
        }
      }
    }
  });

  $effect(() => {
    const template = selectedTemplate;
    const dom = templateEditorDom;
    if (!template) return;

    // Legacy reactive statements did not reschedule themselves on their own
    // writes; untrack mirrors that and keeps the editor creation from looping.
    untrack(() => {
      if (templateEditor && templateEditor.state.doc.toString() != template.content) {
        templateEditor.destroy();
        templateEditor = createTemplateEditor(template.content, dom);
      }
    });
  });

  async function handleFilesSelect(e: { detail: { acceptedFiles: File[] } }) {
    const { acceptedFiles } = e.detail;

    const { parse, asRows } = await getSpreadsheetModule();
    const results = await parse(acceptedFiles[0]);
    if (results.error) {
      parseErrorMessage = results.error;
    } else {
      parseErrorMessage = null;
      data = results.data;
      rows = asRows(results);

      columnCount = _.maxBy(data, (row) => row.length).length;
      _.each(data, (row) => {
        row.length = columnCount;
      });
    }
  }

  async function copyToClipboard() {
    try {
      await navigator.clipboard.writeText(preview);
      toast.toast({
        message: $t("common.copied"),
        type: "is-success"
      });
    } catch (e) {
      console.log(e);
      toast.toast({
        message: $t("common.copyFailed"),
        type: "is-danger"
      });
    }
  }

  let modalOpen = $state(false);
  function openSaveModal() {
    modalOpen = true;
  }

  async function saveToFile(destinationFile: string) {
    const { saved, message } = await saveEditorFile(
      { name: destinationFile, content: preview, operation: "overwrite" },
      { background: true }
    );

    if (saved) {
      toast.toast({
        message: $t("common.createdFile", {
          url: `/ledger/editor/${encodeURIComponent(destinationFile)}`,
          name: _.escape(destinationFile)
        }),
        type: "is-success",
        duration: 5000
      });
    } else {
      toast.toast({
        message: $t("common.saveFailed", { name: destinationFile, reason: message }),
        type: "is-danger",
        duration: 10000
      });
    }
  }

  let saveTooltip = $derived(
    $templateEditorState.hasUnsavedChanges == false
      ? $t("ledger.import.noUnsavedChanges")
      : selectedTemplate?.template_type == "builtin"
        ? $t("ledger.import.builtinSaveNotAllowed")
        : $t("common.save")
  );
  let deleteTooltip = $derived(
    selectedTemplate?.template_type == "builtin"
      ? $t("ledger.import.builtinDeleteNotAllowed")
      : $t("common.delete")
  );

  let templateCreateModalOpen = $state(false);
  function openTemplateCreateModal() {
    templateCreateModalOpen = true;
  }
</script>

<Modal bind:active={templateCreateModalOpen}>
  <svelte:fragment slot="head" let:close>
    <p class="modal-card-title">{$t("ledger.import.createTemplate")}</p>
    <button class="delete" aria-label={$t("common.close")} onclick={(e) => close(e)}></button>
  </svelte:fragment>
  <div class="field" slot="body">
    <label class="label" for="save-filename">{$t("ledger.import.templateName")}</label>
    <div class="control" id="save-filename">
      <input class="input" type="text" bind:value={saveAsName} />
      {#if saveAsNameDuplicate}
        <p class="help is-danger">{$t("ledger.import.templateExists")}</p>
      {/if}
    </div>
  </div>
  <svelte:fragment slot="foot" let:close>
    <button
      class="button is-success"
      disabled={_.isEmpty(saveAsName) || saveAsNameDuplicate}
      onclick={(e) => save() && close(e)}>{$t("common.create")}</button
    >
    <button class="button" onclick={(e) => close(e)}>{$t("common.cancel")}</button>
  </svelte:fragment>
</Modal>

<FileModal bind:open={modalOpen} onsave={(detail) => saveToFile(detail)} />

<section class="section tab-import" style="padding-bottom: 0 !important">
  <div class="container is-fluid">
    <div class="tabs is-toggle is-toggle-rounded is-small mb-4">
      <ul>
        <li class:is-active={importMode === "template"}>
          <a
            href="#template-import"
            onclick={(event) => {
              event.preventDefault();
              importMode = "template";
            }}
          >
            <span class="icon is-small"><i class="fas fa-code"></i></span>
            <span>{$t("ledger.import.templateTab")}</span>
          </a>
        </li>
        <li class:is-active={importMode === "staging"}>
          <a
            href="#statement-staging"
            onclick={(event) => {
              event.preventDefault();
              importMode = "staging";
            }}
          >
            <span class="icon is-small"><i class="fas fa-wand-magic-sparkles"></i></span>
            <span>{$t("ledger.import.stagingTab")}</span>
          </a>
        </li>
      </ul>
    </div>

    <div class:is-hidden={importMode !== "staging"}>
      <StatementImport />
    </div>
    <div class="columns mb-0" class:is-hidden={importMode !== "template"}>
      <div class="column is-5 py-0">
        <div class="box p-3 mb-3 overflow-x-auto">
          <div class="field is-grouped mb-0">
            <p class="control">
              <span data-tippy-content={$t("common.create")} data-tippy-followCursor="false">
                <button
                  class="button"
                  onclick={(_e) => openTemplateCreateModal()}
                  aria-label={$t("ledger.import.createTemplateAria")}
                >
                  <span class="icon">
                    <i class="fas fa-file-circle-plus"></i>
                  </span>
                </button>
              </span>

              <span class="ml-4" data-tippy-followCursor="false" data-tippy-content={saveTooltip}>
                <button
                  class="button"
                  onclick={(_e) => save()}
                  aria-label={$t("ledger.import.saveTemplateAria")}
                  disabled={$templateEditorState.hasUnsavedChanges == false ||
                    selectedTemplate?.template_type == "builtin"}
                >
                  <span class="icon">
                    <i class="fas fa-floppy-disk"></i>
                  </span>
                </button>
              </span>

              <span data-tippy-followCursor="false" data-tippy-content={deleteTooltip}>
                <button
                  class="button"
                  onclick={(_e) => remove()}
                  aria-label={$t("ledger.import.deleteTemplateAria")}
                  disabled={selectedTemplate?.template_type == "builtin"}
                >
                  <span class="icon">
                    <i class="fas fa-trash-can"></i>
                  </span>
                </button>
              </span>
            </p>

            <p class="control is-expanded">
              <Select
                bind:value={selectedTemplate}
                showChevron={true}
                items={templates}
                label="name"
                itemId="id"
                searchable={true}
                clearable={false}
                floatingConfig={{ strategy: "fixed" }}
                onchange={() => {
                  saveAsName = selectedTemplate.name;
                }}
              >
                {#snippet selection({ selection })}
                  <div>
                    {selection.name}
                    <span class="tag is-small is-link invertable is-light"
                      >{selection.template_type}</span
                    >
                  </div>
                {/snippet}
                {#snippet item({ item })}
                  <div>
                    <span class="name">{item.name}</span>
                    <span class="tag is-small is-link invertable is-light"
                      >{item.template_type}</span
                    >
                  </div>
                {/snippet}
              </Select>
            </p>
          </div>
        </div>
        <div class="box py-0">
          <div class="field">
            <div class="control">
              <div class="template-editor" bind:this={templateEditorDom}></div>
            </div>
          </div>
        </div>
        <div class="box py-0">
          <div class="field">
            <div class="control">
              <button
                data-tippy-followCursor="false"
                data-tippy-content={$t("common.copyToClipboard")}
                class="button clipboard"
                aria-label={$t("ledger.import.copyPreviewAria")}
                disabled={_.isEmpty(preview)}
                onclick={copyToClipboard}
              >
                <span class="icon">
                  <i class="fas fa-copy"></i>
                </span>
              </button>
              <button
                data-tippy-followCursor="false"
                data-tippy-content={$t("common.save")}
                class="button save"
                aria-label={$t("ledger.import.savePreviewAria")}
                disabled={_.isEmpty(preview)}
                onclick={openSaveModal}
              >
                <span class="icon">
                  <i class="fas fa-floppy-disk"></i>
                </span>
              </button>
              <div class="preview-editor" bind:this={previewEditorDom}></div>
            </div>
          </div>
        </div>
      </div>
      <div class="column is-7 py-0">
        <div class="box p-3 mb-3">
          <Dropzone
            multiple={false}
            inputElement={input}
            accept=".csv,.txt,.sta,.mt940,.xlsx,.pdf,.xml,.CSV,.TXT,.STA,.MT940,.XLSX,.PDF,.XML"
            on:drop={handleFilesSelect}
          >
            {$t("ledger.import.dropzone")}
          </Dropzone>
        </div>
        <div class="is-flex justify-end mb-3 gap-4">
          <div class="field color-switch">
            <input
              id="import-reverse"
              type="checkbox"
              bind:checked={options.reverse}
              class="switch is-rounded is-small"
            />
            <label for="import-reverse">{$t("ledger.import.reverse")}</label>
          </div>
          <div class="field color-switch">
            <input
              id="trim-reverse"
              type="checkbox"
              bind:checked={options.trim}
              class="switch is-rounded is-small"
            />
            <label for="trim-reverse">{$t("ledger.import.trim")}</label>
          </div>
        </div>
        {#if parseErrorMessage}
          <div class="message invertable is-danger">
            <div class="message-header">{$t("ledger.import.parseFailed")}</div>
            <div class="message-body">{parseErrorMessage}</div>
          </div>
        {/if}
        {#if !_.isEmpty(data)}
          <div class="table-wrapper">
            <table
              class="mt-0 table is-bordered is-size-7 is-narrow has-sticky-header has-sticky-column"
            >
              <thead>
                <tr>
                  <th></th>
                  {#each _.range(0, columnCount) as ci}
                    <th class="has-background-light">{String.fromCharCode(65 + ci)}</th>
                  {/each}
                </tr>
              </thead>
              <tbody>
                {#each data as row, ri}
                  <tr>
                    <th class="has-background-light"><b>{ri}</b></th>
                    {#each row as cell}
                      <td>{cell || ""}</td>
                    {/each}
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    </div>
    <div></div>
  </div>
</section>

<style lang="scss">
  $import-full-height: calc(100vh - 205px);

  .clipboard {
    float: right;
    position: absolute;
    right: 0;
    z-index: 1;
  }

  .save {
    float: right;
    position: absolute !important;
    right: 40px;
    z-index: 1;
  }

  .table-wrapper {
    overflow-x: auto;
    overflow-y: auto;
    max-height: $import-full-height;
  }

  .color-switch {
    .switch[type="checkbox"]:checked + label::before,
    .switch[type="checkbox"]:checked + label:before {
      background: var(--bulma-link, #485fc7);
    }
  }
</style>
