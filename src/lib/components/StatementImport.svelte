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
  import { defaultIncludedTransactions, mergeIngestedTransactions } from "$lib/import_staging";
  import { buildStatementRequest, type IngestionColumnMapping } from "$lib/import_provenance";
  import Papa from "papaparse";

  let accountsList: string[] = $state([]);
  let defaultCurrency: string = $state(globalThis.USER_CONFIG?.default_currency || "");

  let stagedTransactions: StagedTransaction[] = $state([]);
  let includedStaged: boolean[] = $state([]);
  const baseAccountStorageKey = "paisa:statement-base-account";
  let selectedAccount: string = $state(
    typeof localStorage === "undefined" ? "" : localStorage.getItem(baseAccountStorageKey) || ""
  );

  $effect(() => {
    if (typeof localStorage !== "undefined" && selectedAccount.trim()) {
      localStorage.setItem(baseAccountStorageKey, selectedAccount.trim());
    }
  });

  type MappingKey = Exclude<keyof IngestionColumnMapping, "header_row">;

  let mappingRows: string[][] = $state([]);
  let mappingHeaderRow: number = $state(0);
  let mappingEnabled: boolean = $state(false);
  let mappingError: string | null = $state(null);
  let columnMapping: IngestionColumnMapping = $state(emptyColumnMapping());

  const mappingFields: Array<{ key: MappingKey; label: string }> = [
    { key: "date", label: "Date *" },
    { key: "description", label: "Description / Payee" },
    { key: "amount", label: "Amount" },
    { key: "debit", label: "Debit" },
    { key: "credit", label: "Credit" },
    { key: "balance", label: "Balance" },
    { key: "currency", label: "Currency" },
    { key: "dr_cr", label: "Dr / Cr" }
  ];

  function emptyColumnMapping(): IngestionColumnMapping {
    return {
      header_row: 0,
      date: -1,
      description: -1,
      debit: -1,
      credit: -1,
      amount: -1,
      balance: -1,
      currency: -1,
      dr_cr: -1
    };
  }

  function mappingColumns(): string[] {
    return mappingRows[mappingHeaderRow] || [];
  }

  function mappingStorageKey(headers: string[]): string {
    const signature = headers
      .map((header) =>
        String(header ?? "")
          .trim()
          .toLowerCase()
      )
      .join("|");
    return `paisa:statement-column-mapping:${signature}`;
  }

  function findHeaderIndex(headers: string[], names: string[]): number {
    for (const name of names) {
      const index = headers.indexOf(name);
      if (index >= 0) return index;
    }
    for (const name of names) {
      if (name.length <= 2) continue;
      const index = headers.findIndex((header) => header.includes(name));
      if (index >= 0) return index;
    }
    return -1;
  }

  function suggestColumnMapping() {
    const headers = mappingColumns().map((header) =>
      String(header ?? "")
        .trim()
        .toLowerCase()
    );
    columnMapping = {
      header_row: mappingHeaderRow,
      date: findHeaderIndex(headers, [
        "transaction date",
        "txn date",
        "value date",
        "trans date",
        "date",
        "datum",
        "buchungstag",
        "buchung",
        "valuta",
        "valutadatum"
      ]),
      description: findHeaderIndex(headers, [
        "auftraggeber",
        "empfänger",
        "empfaenger",
        "payee",
        "name",
        "narration",
        "description",
        "buchungstext",
        "verwendungszweck"
      ]),
      amount: findHeaderIndex(headers, ["amount", "betrag", "umsatz"]),
      debit: findHeaderIndex(headers, ["debit amount", "withdrawal", "debit", "dr", "soll"]),
      credit: findHeaderIndex(headers, ["credit amount", "deposit", "credit", "cr", "haben"]),
      balance: findHeaderIndex(headers, [
        "balance",
        "closing balance",
        "running balance",
        "saldo",
        "kontostand"
      ]),
      currency: findHeaderIndex(headers, [
        "currency",
        "curr",
        "ccy",
        "iso currency",
        "währung",
        "waehrung"
      ]),
      dr_cr: findHeaderIndex(headers, [
        "dr / cr",
        "dr/cr",
        "dr cr",
        "d/c",
        "type",
        "txn type",
        "transaction type"
      ])
    };
  }

  function persistMapping() {
    const headers = mappingColumns();
    if (headers.length === 0 || typeof localStorage === "undefined") return;
    localStorage.setItem(mappingStorageKey(headers), JSON.stringify(columnMapping));
  }

  function restoreMapping(headers: string[]): boolean {
    if (typeof localStorage === "undefined") return false;
    const saved = localStorage.getItem(mappingStorageKey(headers));
    if (!saved) return false;
    try {
      columnMapping = {
        ...emptyColumnMapping(),
        ...JSON.parse(saved),
        header_row: mappingHeaderRow
      };
      mappingEnabled = true;
      return true;
    } catch {
      return false;
    }
  }

  function prepareMappingPreview() {
    mappingError = null;
    if (detectedFormat === "ofx" || detectedFormat === "qfx" || !rawFileContent.trim()) {
      mappingRows = [];
      mappingEnabled = false;
      return;
    }

    try {
      const parsed = Papa.parse<string[]>(rawFileContent, {
        delimiter: delimiter && delimiter !== "," ? delimiter : undefined,
        skipEmptyLines: false
      });
      mappingRows = (parsed.data as string[][]).slice(0, 50);
      if (mappingRows.length === 0) {
        mappingEnabled = false;
        return;
      }

      let bestRow = 0;
      let bestCount = -1;
      for (let index = 0; index < Math.min(mappingRows.length, 20); index++) {
        const count = mappingRows[index].filter((cell) => String(cell ?? "").trim() !== "").length;
        if (count > bestCount) {
          bestCount = count;
          bestRow = index;
        }
      }
      mappingHeaderRow = bestRow;
      if (restoreMapping(mappingRows[bestRow])) {
        return;
      }
      suggestColumnMapping();
      mappingEnabled = false;
    } catch (e) {
      mappingError = errorMessage(e);
      mappingRows = [];
      mappingEnabled = false;
    }
  }

  function changeMappingHeaderRow(value: number) {
    mappingHeaderRow = value;
    columnMapping = { ...columnMapping, header_row: value };
    if (!restoreMapping(mappingRows[value] || [])) {
      suggestColumnMapping();
    }
  }

  function updateMapping(key: MappingKey, value: number) {
    columnMapping = { ...columnMapping, [key]: value };
    persistMapping();
  }
  let delimiter: string = $state(",");
  let detectedFormat: IngestionFormat = $state("csv");
  let rawFileContent: string = $state("");
  let pasteContent: string = $state("");
  let parseErrorMessage: string | null = $state(null);
  let alreadyImportedMessage: string | null = $state(null);
  let uploadedFileName: string | null = $state(null);
  let pastedStatement: boolean = $state(false);
  let currentImportID: string | null = $state(null);
  let currentSourceType: string = $state("");
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
    alreadyImportedMessage = null;
    uploadedFileName = file.name;
    pastedStatement = false;

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
      prepareMappingPreview();
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
    alreadyImportedMessage = null;
    rawFileContent = pasteContent;
    detectedFormat = detectIngestionFormat("txt", pasteContent);
    uploadedFileName = null;
    pastedStatement = true;
    prepareMappingPreview();
    await runIngest();
  }

  async function runIngest(mode: "replace" | "merge" = "replace") {
    if (!rawFileContent.trim()) {
      toast.toast({ message: "No statement content to analyze", type: "is-warning" });
      return;
    }

    ingestLoading = true;
    try {
      const request = buildStatementRequest({
        format: detectedFormat,
        content: rawFileContent,
        delimiter,
        baseAccount: selectedAccount,
        fileName: uploadedFileName || undefined,
        pasted: pastedStatement,
        locale: globalThis.USER_CONFIG?.locale || "en-IN",
        columnMapping: mappingEnabled ? columnMapping : undefined
      });

      const response = await ajax<IngestionResult & { error?: string }>("/api/import/statement", {
        method: "POST",
        body: JSON.stringify(request),
        background: true
      });

      if (response.error) {
        parseErrorMessage = response.error;
        if (
          /column mapping|could not detect valid transaction table header/i.test(response.error)
        ) {
          mappingEnabled = true;
        }
        stagedTransactions = [];
        includedStaged = [];
        currentImportID = null;
        currentSourceType = "";
        return;
      }

      currentImportID = response.import_id || null;
      currentSourceType = request.source_type;

      if (response.already_imported) {
        stagedTransactions = [];
        includedStaged = [];
        alreadyImportedMessage =
          "This statement was already imported and committed to the ledger. No rows were staged again.";
        toast.toast({
          message: "Statement already imported — nothing to stage.",
          type: "is-info",
          duration: 5000
        });
        return;
      }
      alreadyImportedMessage = null;

      const incoming = (response.transactions || []).map((tx) => ({
        ...tx,
        selected_account: tx.selected_account || ""
      }));

      if (mode === "merge" && stagedTransactions.length > 0) {
        const merged = mergeIngestedTransactions(stagedTransactions, includedStaged, incoming);
        stagedTransactions = merged.transactions;
        includedStaged = merged.included;
      } else {
        stagedTransactions = incoming;
        includedStaged = defaultIncludedTransactions(incoming);
      }

      toast.toast({
        message: `Parsed ${response.total_parsed} transactions (${response.auto_categorized} rule-matched, ${response.possible_duplicates} duplicates flagged)`,
        type: "is-info",
        duration: 4000
      });
    } catch (e) {
      parseErrorMessage = errorMessage(e);
      stagedTransactions = [];
      includedStaged = [];
      currentImportID = null;
      currentSourceType = "";
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
          import_id: currentImportID || undefined,
          source_type: currentSourceType || undefined,
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
        includedStaged = defaultIncludedTransactions(stagedTransactions);
        currentImportID = null;
        currentSourceType = "";
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
      runIngest("merge");
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

    {#if mappingRows.length > 0 && detectedFormat !== "ofx" && detectedFormat !== "qfx"}
      <div class="box p-3 mb-3">
        <div class="field is-grouped is-align-items-center mb-2">
          <label class="checkbox is-size-7">
            <input type="checkbox" bind:checked={mappingEnabled} />
            Use column mapping
          </label>
          <span class="help is-size-7 ml-3">
            Override the detected columns for any bank or language.
          </span>
        </div>

        {#if mappingError}
          <p class="help is-danger">{mappingError}</p>
        {/if}

        {#if mappingEnabled}
          <div class="field mb-2">
            <label class="label is-small" for="statement-header-row">Header row</label>
            <div class="select is-small is-fullwidth">
              <select
                id="statement-header-row"
                value={mappingHeaderRow}
                onchange={(e) => changeMappingHeaderRow(Number(e.currentTarget.value))}
              >
                {#each mappingRows.slice(0, Math.min(mappingRows.length, 20)) as row, index}
                  <option value={index}>
                    Row {index + 1}: {row
                      .filter((cell) => String(cell ?? "").trim())
                      .slice(0, 4)
                      .join(" | ") || "(empty)"}
                  </option>
                {/each}
              </select>
            </div>
          </div>

          <div class="columns is-multiline is-mobile mb-0">
            {#each mappingFields as field}
              <div class="column is-3">
                <label class="label is-small" for={`statement-map-${field.key}`}
                  >{field.label}</label
                >
                <div class="select is-small is-fullwidth">
                  <select
                    id={`statement-map-${field.key}`}
                    value={columnMapping[field.key]}
                    onchange={(e) => updateMapping(field.key, Number(e.currentTarget.value))}
                  >
                    <option value={-1}>—</option>
                    {#each mappingColumns() as header, index}
                      <option value={index}>{index + 1}: {header || "(empty)"}</option>
                    {/each}
                  </select>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}

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
        <p class="help is-size-7">
          The physical account whose statement you are importing. Remembered for the next import.
        </p>
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
            onchange={prepareMappingPreview}
          />
        </div>
        <button
          class="button is-small ml-auto"
          class:is-loading={ingestLoading}
          disabled={ingestLoading || !rawFileContent.trim()}
          onclick={() => runIngest()}
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

    {#if alreadyImportedMessage}
      <div class="notification is-info is-light mb-3">
        <p class="has-text-weight-semibold mb-1">Already imported</p>
        <p class="is-size-7">{alreadyImportedMessage}</p>
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
