<script lang="ts">
  import {
    createEditor,
    editorState,
    focus,
    moveToEnd,
    moveToLine,
    updateContent
  } from "$lib/editor";
  import { askConfirm } from "$lib/confirm";
  import { shouldDeferNavigation } from "$lib/navigation_guard";
  import { insertTab } from "@codemirror/commands";
  import { buildDirectoryTree, type LedgerFile } from "$lib/utils";
  import {
    deleteEditorBackups,
    getEditorFile,
    getEditorFiles,
    saveEditorFile
  } from "$lib/api/editor";
  import { redo, undo } from "@codemirror/commands";
  import type { KeyBinding } from "@codemirror/view";
  import * as toast from "bulma-toast";
  import type { EditorView } from "codemirror";
  import { format } from "$lib/journal";
  import _ from "lodash";
  import { onMount, untrack } from "svelte";
  import { beforeNavigate, goto, afterNavigate } from "$app/navigation";
  import type { PageData } from "./$types";
  import FileTree from "$lib/components/FileTree.svelte";
  import FileModal from "$lib/components/FileModal.svelte";
  import { readonly } from "../../../../../persisted_store";
  import { page } from "$app/state";
  import { t } from "$lib/i18n";

  let { data }: { data: PageData } = $props();
  let editorDom = $state<Element>();
  let editor = $state<EditorView>();
  let editorFileName = $state("");
  let filesMap = $state<Record<string, LedgerFile>>({});
  let selectedFile = $state<LedgerFile>(null);
  let accounts = $state<string[]>([]);
  let commodities = $state<string[]>([]);
  let payees = $state<string[]>([]);
  let selectedVersion = $state<string>(null);
  let lineNumber = $state(0);

  function command(fn: Function) {
    return () => {
      fn();
      return true;
    };
  }

  const keybindings: readonly KeyBinding[] = [
    { key: "Tab", run: insertTab },
    {
      key: "Ctrl-s",
      run: command(save),
      preventDefault: true
    },
    {
      key: "Ctrl-I",
      run: command(pretty),
      preventDefault: true
    }
  ];

  let cancelled = false;

  function askLeaveUnsavedChanges(): Promise<boolean> {
    return askConfirm({
      title: $t("ledger.unsavedTitle"),
      description: $t("ledger.unsavedDescription"),
      confirmLabel: $t("ledger.leave"),
      cancelLabel: $t("ledger.stay"),
      variant: "danger"
    });
  }

  beforeNavigate(({ cancel, to, willUnload }) => {
    if (!shouldDeferNavigation({ dirty: $editorState.hasUnsavedChanges })) return;

    // Navigations initiated by the editor itself ask before calling goto().
    // Everything else (links, back/forward) is cancelled here and repeated
    // once the user chooses to leave.
    cancel();
    cancelled = true;
    if (willUnload || !to) return;

    const target = to.url;
    void askLeaveUnsavedChanges().then((confirmed) => {
      if (shouldDeferNavigation({ dirty: true, confirmed })) {
        cancelled = false;
        return;
      }
      cancelled = false;
      $editorState = _.assign({}, $editorState, { hasUnsavedChanges: false });
      void goto(target, { reset: false });
    });
  });

  async function navigate(url: string) {
    const dirty = $editorState.hasUnsavedChanges;
    cancelled = false;

    // Ask before goto() so a confirmed navigation can update selectedFile.
    const confirmed = dirty ? await askLeaveUnsavedChanges() : false;
    if (shouldDeferNavigation({ dirty, confirmed, cancelled })) {
      cancelled = false;
      return false;
    }

    if (dirty) {
      $editorState = _.assign({}, $editorState, { hasUnsavedChanges: false });
    }

    await goto(url, { reset: false });
    if (cancelled) {
      cancelled = false;
      return false;
    }
    return true;
  }

  // A navigation not started by navigate() (for example an external link) is
  // cancelled in beforeNavigate and repeated after confirmation. Pick the new
  // file from the route once that deferred navigation lands.
  afterNavigate(() => {
    const file = filesMap[page.params.slug];
    if (file && selectedFile?.name !== file.name) {
      selectedFile = file;
    }
  });

  onMount(async () => {
    loadFiles(data.name);
    const line = _.toNumber(page.url.hash.substring(1));
    if (_.isNumber(line)) {
      lineNumber = line;
    }
  });

  async function loadFiles(selectedFileName: string) {
    let files;
    ({ files, accounts, commodities, payees } = await getEditorFiles());
    filesMap = _.fromPairs(_.map(files, (f) => [f.name, f]));
    if (!_.isEmpty(files)) {
      selectedFile = _.find(files, (f) => f.name == selectedFileName) || files[0];
    }
  }

  async function selectFile(file: LedgerFile) {
    const success = await navigate(`/ledger/editor/${encodeURIComponent(file.name)}`);
    if (success) {
      selectedFile = file;
    }
  }

  async function revert(version: string) {
    const { file } = await getEditorFile(version, { background: true });

    updateContent(editor, file.content);
  }

  async function pretty() {
    const formatted = format(editor.state.doc.toString());
    if (formatted != editor.state.doc.toString()) {
      updateContent(editor, formatted);
    }
  }

  async function deleteBackups() {
    const { file } = await deleteEditorBackups(selectedFile.name, { background: true });

    selectedFile.versions = file.versions;
  }

  async function save() {
    if ($readonly) return;
    const doc = editor.state.doc;
    const { errors, saved, file, message } = await saveEditorFile(
      { name: selectedFile.name, content: doc.toString() },
      { background: true }
    );

    if (!saved) {
      toast.toast({
        message: $t("common.saveFailed", { name: selectedFile.name, reason: message }),
        type: "is-danger",
        duration: 10000
      });
      if (!_.isEmpty(errors)) {
        moveToLine(editor, errors[0].line_from);
      }
    } else {
      toast.toast({
        message: $t("common.saved", { name: selectedFile.name }),
        type: "is-success"
      });
      filesMap[file.name] = file;
      selectedFile = file;
      selectedVersion = null;
      $editorState = _.assign({}, $editorState, { hasUnsavedChanges: false });
    }
  }

  $effect(() => {
    const file = selectedFile;
    const dom = editorDom;
    const autocompletions = {
      string: accounts,
      strong: payees,
      unit: commodities
    };
    if (!file) return;

    // Legacy reactive statements did not reschedule themselves on their own
    // writes; untrack mirrors that and keeps the editor creation from looping.
    untrack(() => {
      if (!editor || editorFileName !== file.name || editor.state.doc.toString() != file.content) {
        if (editor) {
          editor.destroy();
        }

        editor = createEditor(file.content, dom, {
          name: file.name,
          keybindings,
          readonly: $readonly,
          autocompletions
        });
        editorFileName = file.name;
        if (lineNumber > 0) {
          moveToLine(editor, lineNumber, true);
          focus(editor);
          lineNumber = 0;
        } else {
          moveToEnd(editor);
        }
      }
    });
  });

  let modalOpen = $state(false);
  function openCreateModal() {
    modalOpen = true;
  }

  async function createFile(destinationFile: string) {
    const { saved, message } = await saveEditorFile(
      { name: destinationFile, content: "", operation: "create" },
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

      const success = await navigate(`/ledger/editor/${encodeURIComponent(destinationFile)}`);
      if (success) {
        await loadFiles(destinationFile);
      }
    } else {
      toast.toast({
        message: $t("common.createFailed", { name: destinationFile, reason: message }),
        type: "is-danger",
        duration: 10000
      });
    }
  }
</script>

<FileModal
  bind:open={modalOpen}
  onsave={(detail) => createFile(detail)}
  label={$t("common.create")}
  help=""
/>

<section class="section tab-editor max-h-screen" style="padding-bottom: 0 !important">
  <div class="container is-fluid">
    <div class="columuns">
      <div class="column is-12 px-0 pt-0 mb-2">
        <div class="box p-3 is-flex is-align-items-center overflow-x-auto" style="width: 100%">
          <div class="field has-addons mb-0">
            <p class="control">
              <button
                class="button is-small is-link invertable is-light"
                disabled={$editorState.hasUnsavedChanges}
                onclick={(_e) => openCreateModal()}
              >
                <span class="icon is-small">
                  <i class="fas fa-file-circle-plus"></i>
                </span>
                <span>{$t("common.create")}</span>
              </button>
            </p>
          </div>

          <div class="field has-addons ml-5 mb-0">
            <p class="control">
              <button
                class="button is-small"
                disabled={$readonly || $editorState.hasUnsavedChanges == false}
                onclick={(_e) => save()}
              >
                <span class="icon is-small">
                  <i class="fas fa-floppy-disk"></i>
                </span>
                <span>{$t("common.save")}</span>
              </button>
            </p>
            <p class="control">
              <button
                class="button is-small"
                disabled={$editorState.undoDepth == 0}
                onclick={(_e) => undo(editor)}
              >
                <span class="icon is-small">
                  <i class="fas fa-arrow-left"></i>
                </span>
                <span>{$t("common.undo")}</span>
              </button>
            </p>
            <p class="control">
              <button
                class="button is-small"
                disabled={$editorState.redoDepth == 0}
                onclick={(_e) => redo(editor)}
              >
                <span>{$t("common.redo")}</span>
                <span class="icon is-small">
                  <i class="fas fa-arrow-right"></i>
                </span>
              </button>
            </p>
            <p class="control">
              <button class="button is-small" onclick={(_e) => pretty()} type="button">
                <span class="icon is-small">
                  <i class="fas fa-code"></i>
                </span>
                <span>{$t("ledger.prettify")}</span>
              </button>
            </p>
          </div>

          {#if !_.isEmpty(selectedFile?.versions)}
            <div class="field has-addons ml-5 mb-0">
              <p class="control">
                <button
                  class="button is-small"
                  disabled={!selectedVersion}
                  onclick={(_e) => revert(selectedVersion)}
                >
                  <span class="icon is-small">
                    <i class="fas fa-clock-rotate-left"></i>
                  </span>
                  <span>{$t("common.revert")}</span>
                </button>
              </p>

              <div class="control">
                <div class="select is-small">
                  <select bind:value={selectedVersion}>
                    {#each selectedFile.versions as version}
                      <option>{version}</option>
                    {/each}
                  </select>
                </div>
              </div>

              <p class="control">
                <button
                  class="button is-small"
                  onclick={(_e) => deleteBackups()}
                  type="button"
                  aria-label={$t("ledger.deleteBackups")}
                >
                  <span class="icon is-small">
                    <i class="fas fa-trash-can"></i>
                  </span>
                </button>
              </p>
            </div>
          {/if}

          {#if $editorState.errors.length > 0}
            <div class="control ml-5">
              <button
                class="button-reset"
                onclick={(_e) => moveToLine(editor, $editorState.errors[0].line_from)}
                type="button"
              >
                <span class="ml-1 tag invertable is-danger is-light"
                  >{$t("ledger.errorsFound", { count: $editorState.errors.length })}</span
                >
              </button>
            </div>
          {/if}
        </div>
      </div>
    </div>
    <div class="columns">
      <div class="column is-3-widescreen is-2-fullhd is-4">
        <div class="box px-2 full-height overflow-y-auto">
          <aside class="menu">
            <FileTree
              path=""
              onselect={(file) => selectFile(file)}
              files={buildDirectoryTree(_.values(filesMap))}
              selectedFileName={selectedFile?.name}
              hasUnsavedChanges={$editorState.hasUnsavedChanges}
            />
          </aside>
        </div>
      </div>
      <div class="column is-6-widescreen is-6-fullhd is-8">
        <div class="box py-0">
          <div class="editor" bind:this={editorDom}></div>
        </div>
      </div>
      <div class="column is-3-widescreen is-4-fullhd is-hidden-touch is-hidden-desktop-only">
        {#if !_.isEmpty($editorState.output)}
          <!-- WCAG 2.1.1: scrollable output needs keyboard access. -->
          <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
          <pre
            class="box px-3 full-height"
            tabindex="0"
            role="region"
            aria-label={$t("ledger.editorOutput")}>{$editorState.output}</pre>
        {/if}
      </div>
    </div>
  </div>
</section>
