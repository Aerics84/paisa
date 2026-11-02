<script lang="ts">
  import Dropzone from "svelte-file-dropzone";
  import { onMount } from "svelte";
  import * as toast from "bulma-toast";
  import RuleStudioModal from "./RuleStudioModal.svelte";
  import TransactionStagingTable from "./TransactionStagingTable.svelte";
  import {
    ajax,
    type ImportRule,
    type IngestionFormat,
    type IngestionResult,
    type StagedTransaction
  } from "$lib/utils";

  let accountsList: string[] = $state([]);
  let defaultCurrency: string = $state(globalThis.USER_CONFIG?.default_currency || "");

  let stagedTransactions: StagedTransaction[] = $state([]);
  let includedStaged: boolean[] = $state([]);
  let selectedAccount: string = $state("");
  let delimiter: string = $state(",");
  let detectedFormat: IngestionFormat = $state("csv");
  let rawFileContent: string = $state("");
  let pasteContent: string = $state("");
  let parseErrorMessage: string | null = $state(null);
  let ingestLoading: boolean = $state(false);
  let importSaving: boolean = $state(false);
  let groupByDate: boolean = $state(false);

  let ruleStudioOpen: boolean = $state(false);
  let initialRuleForStudio: Partial<ImportRule> | null = $state(null);

  onMount(async () => {
    try {
      const res = await ajax("/api/config");
      if (res?.config?.default_currency) {
        defaultCurrency = res.config.default_currency;
      }
      if (Array.isArray(res?.accounts)) {
        accountsList = res.accounts;
      }
    } catch {
      // The staging flow still works offline; accounts default to free text.
    }
  });

  function errorMessage(e: unknown): string {
    return e instanceof Error ? e.message : String(e);
  }

  function detectIngestionFormat(ext: string, content: string): IngestionFormat {
    if (ext === "ofx" || ext === "qfx") {
      return ext;
    }
    if (content.includes("<OFX>") || content.includes("<STMTTRN>")) {
      return "ofx";
    }
    return "csv";
  }

  function toCSVContent(data: unknown[][]): string {
    return data
      .map((row) =>
        row
          .map((cell) => {
            const value = cell == null ? "" : String(cell);
            if (/[",\r\n]/.test(value)) {
              return `"${value.replaceAll('"', '""')}"`;
            }
            return value;
          })
          .join(",")
      )
      .join("\n");
  }

  async function handleDrop(e: { detail: { acceptedFiles: File[] } }) {
    const files = e.detail?.acceptedFiles;
    if (!files || files.length === 0) return;
    const file = files[0];
    const ext = (file.name.split(".").pop() || "").toLowerCase();
    parseErrorMessage = null;

    try {
      if (ext === "xls" || ext === "xlsx" || ext === "pdf") {
        const { parse } = await import("$lib/spreadsheet");
        const result = await parse(file);
        if (result.error) {
          parseErrorMessage = result.error;
          return;
        }
        detectedFormat = ext === "pdf" ? "pdf" : "excel";
        rawFileContent = toCSVContent(result.data);
      } else {
        rawFileContent = await file.text();
        detectedFormat = detectIngestionFormat(ext, rawFileContent);
      }
      await runIngest();
    } catch (e) {
      parseErrorMessage = errorMessage(e);
    }
  }

  async function analyzePastedContent() {
    if (!pasteContent.trim()) {
      toast.toast({ message: "Paste a statement first", type: "is-warning" });
      return;
    }
    parseErrorMessage = null;
    rawFileContent = pasteContent;
    detectedFormat = detectIngestionFormat("txt", pasteContent);
    await runIngest();
  }

  async function runIngest() {
    if (!rawFileContent.trim()) {
      toast.toast({ message: "No statement content to analyze", type: "is-warning" });
      return;
    }

    ingestLoading = true;
    try {
      const response = await ajax<IngestionResult & { error?: string }>("/api/import/statement", {
        method: "POST",
        body: JSON.stringify({
          format: detectedFormat,
          content: rawFileContent,
          delimiter,
          base_account: selectedAccount
        }),
        background: true
      });

      if (response.error) {
        parseErrorMessage = response.error;
        stagedTransactions = [];
        includedStaged = [];
        return;
      }

      stagedTransactions = (response.transactions || []).map((tx) => ({
        ...tx,
        selected_account: tx.selected_account || ""
      }));
      // Only include rows that the backend already classified. Suggested rows
      // stay unchecked until the user accepts them.
      includedStaged = stagedTransactions.map((tx) => !tx.is_duplicate && !!tx.selected_account);

      toast.toast({
        message: `Parsed ${response.total_parsed} transactions (${response.auto_categorized} rule-matched, ${response.possible_duplicates} duplicates flagged)`,
        type: "is-info",
        duration: 4000
      });
    } catch (e) {
      parseErrorMessage = errorMessage(e);
      stagedTransactions = [];
      includedStaged = [];
    } finally {
      ingestLoading = false;
    }
  }

  function shortNote(payee?: string, memo?: string): string {
    return (payee || memo || "").trim().slice(0, 64);
  }

  function handleCreateRuleFromTransaction(tx: StagedTransaction) {
    const note = shortNote(tx.payee, tx.memo);
    initialRuleForStudio = {
      name: `Auto: ${note || tx.payee}`,
      payee_pattern: note || tx.payee,
      payee_match_type: "contains",
      tx_type: tx.is_debit ? "debit" : "credit",
      target_account: tx.selected_account || accountsList[0] || "Expenses:Miscellaneous",
      priority: 100
    };
    ruleStudioOpen = true;
  }

  async function handleQuickSaveRule(tx: StagedTransaction) {
    if (!tx.selected_account) return;
    const note = shortNote(tx.payee, tx.memo) || tx.payee;

    try {
      const res = await ajax<{ saved?: boolean; rule?: ImportRule; error?: string }>("/api/rules", {
        method: "POST",
        body: JSON.stringify({
          name: `Rule: ${note}`,
          payee_pattern: note,
          payee_match_type: "contains",
          tx_type: tx.is_debit ? "debit" : "credit",
          target_account: tx.selected_account,
          priority: 100,
          enabled: true
        }),
        background: true
      });

      if (res?.saved) {
        toast.toast({
          message: `Saved rule for "${note}" -> ${tx.selected_account}`,
          type: "is-success",
          duration: 3500
        });

        const ruleName = res.rule?.name || `Rule: ${note}`;
        const needle = note.toLowerCase();
        stagedTransactions.forEach((t) => {
          if (!t.selected_account && (t.payee || "").toLowerCase().includes(needle)) {
            t.selected_account = tx.selected_account;
            t.matched_rule_name = ruleName;
            t.confidence = "rule";
          }
        });
      }
    } catch (e) {
      toast.toast({
        message: `Failed to save rule: ${errorMessage(e)}`,
        type: "is-danger",
        duration: 5000
      });
    }
  }

  function selectedCount(): number {
    return stagedTransactions.filter((_, idx) => includedStaged[idx]).length;
  }

  async function commitImport() {
    if (!selectedAccount.trim()) {
      toast.toast({
        message: "Please select your Source / Bank Account first.",
        type: "is-warning"
      });
      return;
    }

    const selected = stagedTransactions.filter((_, idx) => includedStaged[idx]);
    if (selected.length === 0) {
      toast.toast({ message: "No transactions selected to commit", type: "is-warning" });
      return;
    }

    importSaving = true;
    try {
      const res = await ajax<{
        success?: boolean;
        committed_count?: number;
        message?: string;
        error?: string;
      }>("/api/import/commit", {
        method: "POST",
        body: JSON.stringify({
          base_account: selectedAccount,
          commodity: defaultCurrency,
          group_by_date: groupByDate,
          raw_content: "",
          transactions: selected
        }),
        background: true
      });

      if (res.success && typeof res.committed_count === "number") {
        toast.toast({
          message: `Successfully appended ${res.committed_count} transactions to the ledger`,
          type: "is-success",
          duration: 6000
        });

        stagedTransactions = stagedTransactions.filter((_, idx) => !includedStaged[idx]);
        includedStaged = stagedTransactions.map(() => true);
      } else if (res.success) {
        toast.toast({
          message: res.message || "Import request was accepted",
          type: "is-warning",
          duration: 6000
        });
      } else {
        toast.toast({
          message: `Failed to commit transactions: ${res.error || res.message || "unknown error"}`,
          type: "is-danger",
          duration: 6000
        });
      }
    } catch (e) {
      toast.toast({
        message: `Failed to commit transactions: ${errorMessage(e)}`,
        type: "is-danger"
      });
    } finally {
      importSaving = false;
    }
  }
</script>

<RuleStudioModal
  bind:open={ruleStudioOpen}
  accounts={accountsList}
  initialRule={initialRuleForStudio}
  onruleschanged={() => {
    initialRuleForStudio = null;
    if (stagedTransactions.length > 0) {
      runIngest();
    }
  }}
/>

<div class="columns mb-0">
  <div class="column is-4 py-0">
    <div class="box p-3 mb-3">
      <h4 class="title is-6 mb-2">1. Upload or Paste Statement</h4>
      <Dropzone
        multiple={false}
        accept=".csv,.txt,.ofx,.qfx,.xls,.xlsx,.pdf,.CSV,.TXT,.OFX,.QFX,.XLS,.XLSX,.PDF"
        on:drop={handleDrop}
      >
        Drag 'n' drop CSV, OFX, QFX, Excel or PDF statement here
      </Dropzone>
      <div class="mt-2 is-size-7 has-text-grey">
        Detected format:
        <span class="tag is-small is-light uppercase">{detectedFormat}</span>
      </div>

      <div class="field mt-3 mb-0">
        <label class="label is-small" for="statement-paste">Or paste statement text</label>
        <div class="control">
          <textarea
            id="statement-paste"
            class="textarea is-small font-mono"
            rows="4"
            bind:value={pasteContent}
            placeholder="Paste CSV or OFX content here..."></textarea>
        </div>
        <button
          class="button is-small is-link is-light mt-2"
          disabled={ingestLoading || !pasteContent.trim()}
          onclick={analyzePastedContent}
        >
          Analyze Pasted Text
        </button>
      </div>
    </div>

    <div class="box p-3 mb-3">
      <h4 class="title is-6 mb-2">2. Statement Account</h4>
      <div class="field mb-3">
        <label class="label is-small" for="statement-base-account">Source / Bank Account</label>
        <div class="control">
          <input
            id="statement-base-account"
            class="input is-small {!selectedAccount ? 'is-warning' : ''}"
            type="text"
            list="statement-accounts"
            bind:value={selectedAccount}
            placeholder="e.g. Assets:Checking:MyBank"
          />
          <datalist id="statement-accounts">
            {#each accountsList as acc}
              <option value={acc}>{acc}</option>
            {/each}
          </datalist>
        </div>
        <p class="help is-size-7">The physical account whose statement you are importing.</p>
      </div>

      <div class="field is-grouped is-align-items-center mb-0">
        <label class="label is-small mb-0 mr-2" for="statement-delimiter">Delimiter</label>
        <div class="control">
          <input
            id="statement-delimiter"
            class="input is-small"
            style="width: 50px"
            maxlength="1"
            bind:value={delimiter}
          />
        </div>
        <button
          class="button is-small ml-auto"
          class:is-loading={ingestLoading}
          disabled={ingestLoading || !rawFileContent.trim()}
          onclick={runIngest}
        >
          Re-analyze
        </button>
      </div>
    </div>

    <div class="box p-3">
      <h4 class="title is-6 mb-2">3. Append to Ledger</h4>
      <p class="is-size-7 has-text-grey mb-2">
        Selected rows are written through the backend, which formats them for the configured ledger
        dialect.
      </p>
      <div class="field mb-3">
        <label class="checkbox is-size-7">
          <input type="checkbox" bind:checked={groupByDate} />
          Club multiple debit entries per date into a single transaction
        </label>
      </div>
      <button
        class="button is-success is-fullwidth"
        class:is-loading={importSaving}
        disabled={selectedCount() === 0 || importSaving}
        onclick={commitImport}
      >
        <span class="icon is-small"><i class="fas fa-check-double"></i></span>
        <span>Commit {selectedCount()} Transactions</span>
      </button>
    </div>
  </div>

  <div class="column is-8 py-0">
    <div class="is-flex is-justify-content-space-between is-align-items-center mb-3">
      <h4 class="title is-6 mb-0">Staged Transactions</h4>
      <button
        class="button is-primary is-light is-small"
        onclick={() => {
          initialRuleForStudio = null;
          ruleStudioOpen = true;
        }}
      >
        <span class="icon is-small"><i class="fas fa-sliders"></i></span>
        <span>Rule Studio</span>
      </button>
    </div>

    {#if parseErrorMessage}
      <div class="message invertable is-danger mb-3">
        <div class="message-header">Ingestion Warning</div>
        <div class="message-body">{parseErrorMessage}</div>
      </div>
    {/if}

    {#if ingestLoading}
      <div class="notification is-light has-text-centered py-6">
        <span class="icon is-large has-text-grey">
          <i class="fas fa-spinner fa-spin fa-2x"></i>
        </span>
        <p class="has-text-grey is-size-6 mt-2">Analyzing statement...</p>
      </div>
    {:else if stagedTransactions.length > 0}
      <TransactionStagingTable
        bind:transactions={stagedTransactions}
        bind:included={includedStaged}
        accounts={accountsList}
        {defaultCurrency}
        oneditrule={handleCreateRuleFromTransaction}
        onquicksave={handleQuickSaveRule}
      />
    {:else}
      <div class="notification is-light has-text-centered py-6">
        <span class="icon is-large has-text-grey-light mb-2">
          <i class="fas fa-file-invoice-dollar fa-3x"></i>
        </span>
        <p class="has-text-grey is-size-6 font-semibold">No Statement Uploaded Yet</p>
        <p class="is-size-7 has-text-grey">
          Drag 'n' drop a bank statement or paste CSV/OFX content on the left to start
          auto-categorizing.
        </p>
      </div>
    {/if}
  </div>
</div>
