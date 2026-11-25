<script lang="ts">
  import { isMobile, type LedgerFile, type Transaction as T } from "$lib/utils";
  import { getBalancedPostings, getTransactions } from "$lib/api/ledger";
  import { getEditorFiles, saveEditorFile } from "$lib/api/editor";
  import _ from "lodash";
  import { onDestroy, onMount } from "svelte";
  import VirtualList from "svelte-tiny-virtual-list";
  import Transaction from "$lib/components/Transaction.svelte";
  import BulkEditForm from "$lib/components/BulkEditForm.svelte";
  import { slide } from "svelte/transition";
  import * as bulkEdit from "$lib/bulk_edit";
  import * as toast from "bulma-toast";
  import DiffViewModal from "$lib/components/DiffViewModal.svelte";
  import SearchQuery from "$lib/components/SearchQuery.svelte";
  import { editorState } from "$lib/search_query_editor";
  import { get } from "svelte/store";
  import { download } from "$lib/export";
  import { t } from "$lib/i18n";

  let buldEditOpen = $state(false);
  let transactions = $state<T[]>(null);
  let filtered = $state<T[]>([]);
  let files = $state<LedgerFile[]>([]);
  let newFiles = $state<LedgerFile[]>([]);
  let updatedTransactionsCount = $state(0);
  let openPreviewModal = $state(false);
  let accounts = $state<string[]>([]);
  let commodities = $state<string[]>([]);

  const debits = (t: T) => {
    return _.filter(t.postings, (p) => p.amount < 0);
  };

  const credits = (t: T) => {
    return _.filter(t.postings, (p) => p.amount >= 0);
  };

  function handleInputRaw(predicate: (t: T) => boolean) {
    filtered = _.filter(transactions, predicate);
  }

  const handleInput = _.debounce(handleInputRaw, 100);

  const unsubscribe = editorState.subscribe((state) => {
    handleInput(state.predicate);
  });

  onDestroy(async () => {
    unsubscribe();
  });

  const mobile = isMobile();

  const itemSize = (i: number) => {
    const t = filtered[i];
    const count = mobile ? t.postings.length : Math.max(credits(t).length, debits(t).length);
    return 8 + count * 22 + (mobile ? 25 : 0);
  };

  async function loadTransactions() {
    ({ files, accounts, commodities } = await getEditorFiles());
    ({ transactions } = await getTransactions());
    handleInputRaw(get(editorState).predicate);

    newFiles = files;
  }

  async function downloadTransactions() {
    const { balancedPostings } = await getBalancedPostings();
    download(balancedPostings);
  }

  function showPreview(detail: any) {
    ({ newFiles, updatedTransactionsCount } = bulkEdit.applyChanges(
      files,
      filtered,
      detail.operation,
      detail.args
    ));
    openPreviewModal = true;
  }

  async function saveAll(newFiles: LedgerFile[]) {
    for (const newFile of newFiles) {
      const { saved, message } = await saveEditorFile(
        { name: newFile.name, content: newFile.content },
        { background: true }
      );

      if (!saved) {
        toast.toast({
          message: $t("common.saveFailed", { name: newFile.name, reason: message }),
          type: "is-danger",
          duration: 10000
        });
      } else {
        toast.toast({
          message: $t("common.saved", { name: newFile.name }),
          type: "is-success"
        });
      }
    }
    await loadTransactions();
  }

  onMount(async () => {
    await loadTransactions();
  });
</script>

<DiffViewModal
  onsave={(files) => saveAll(files)}
  bind:open={openPreviewModal}
  oldFiles={files}
  {newFiles}
  {updatedTransactionsCount}
/>

{#if transactions}
  <section class="section tab-journal">
    <div class="container is-fluid">
      <div class="columns">
        <div class="column is-12">
          <nav class="level">
            <div class="level-left">
              <div class="level-item">
                <div class="field">
                  <div class="control">
                    <SearchQuery
                      autocomplete={{
                        account: accounts,
                        commodity: commodities,
                        filename: files.map((f) => f.name)
                      }}
                    />
                  </div>
                </div>
              </div>
              <div class="level-item">
                <div class="field">
                  <div class="control">
                    <button
                      class="button is-link is-light invertable"
                      onclick={(_e) => (buldEditOpen = !buldEditOpen)}
                    >
                      <span>{$t("ledger.bulkEdit")}</span>
                      <span class="icon is-small">
                        <i class="fas {buldEditOpen ? 'fa-angle-up' : 'fa-angle-down'}"></i>
                      </span>
                    </button>
                  </div>
                </div>
              </div>
            </div>
            <div class="level-right">
              <div class="level-item">
                <p class="is-6">{$t("ledger.transactionCount", { count: filtered.length })}</p>
              </div>
              <div class="level-item">
                <button
                  class="button is-text p-0"
                  type="button"
                  onclick={(_e) => downloadTransactions()}
                >
                  <span class="icon is-small">
                    <i class="fa-solid fa-file-arrow-down"></i>
                  </span>
                  {$t("ledger.download")}
                </button>
              </div>
            </div>
          </nav>
        </div>
      </div>

      {#if buldEditOpen}
        <div class="columns">
          <div class="column is-12" transition:slide>
            <BulkEditForm {accounts} onpreview={(detail) => showPreview(detail)} />
          </div>
        </div>
      {/if}

      <div class="columns">
        <div class="column is-12">
          <div class="box">
            <VirtualList
              scrollDirection="vertical"
              width="100%"
              height={window.innerHeight - 150}
              itemCount={filtered.length}
              {itemSize}
            >
              {#snippet item({ index, style })}
                {@const t = filtered[index]}
                <div {style}>
                  <Transaction {t} />
                </div>
              {/snippet}
            </VirtualList>
          </div>
        </div>
      </div>
    </div>
  </section>
{/if}
