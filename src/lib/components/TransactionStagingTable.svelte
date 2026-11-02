<script lang="ts">
  import type { StagedTransaction } from "$lib/utils";

  let {
    transactions = $bindable([]),
    included = $bindable([]),
    accounts = [],
    defaultCurrency = "INR",
    oneditrule = (_tx: StagedTransaction) => {},
    onquicksave = (_tx: StagedTransaction) => {}
  }: {
    transactions: StagedTransaction[];
    included: boolean[];
    accounts?: string[];
    defaultCurrency?: string;
    oneditrule?: (tx: StagedTransaction) => void;
    onquicksave?: (tx: StagedTransaction) => void;
  } = $props();

  let allIncluded = $derived(
    transactions.length > 0 && transactions.every((_, i) => !!included[i])
  );

  let filterStatus: "all" | "new" | "duplicate" | "unassigned" = $state("all");

  // For debits (outflow) prefer expense accounts; for credits (inflow) prefer income.
  let debitAccounts = $derived.by(() => {
    const expenses = accounts.filter((a) => a.startsWith("Expenses:"));
    const liabilities = accounts.filter((a) => a.startsWith("Liabilities:"));
    const assets = accounts.filter((a) => a.startsWith("Assets:"));
    const others = accounts.filter(
      (a) => !a.startsWith("Expenses:") && !a.startsWith("Liabilities:") && !a.startsWith("Assets:")
    );
    return [...expenses, ...assets, ...liabilities, ...others];
  });

  let creditAccounts = $derived.by(() => {
    const income = accounts.filter((a) => a.startsWith("Income:"));
    const assets = accounts.filter((a) => a.startsWith("Assets:"));
    const liabilities = accounts.filter((a) => a.startsWith("Liabilities:"));
    const others = accounts.filter(
      (a) => !a.startsWith("Income:") && !a.startsWith("Assets:") && !a.startsWith("Liabilities:")
    );
    return [...income, ...assets, ...liabilities, ...others];
  });

  let editingRowIdx: number | null = $state(null);
  let groupByMerchant: boolean = $state(false);
  let bulkSelectedAccount: string = $state("");
  let collapsedClusters: Record<string, boolean> = $state({});

  // Group similar merchants by the first two meaningful tokens of the payee.
  function getMerchantCluster(tx: StagedTransaction): string {
    const cleaned = (tx.payee || tx.memo || "").trim();
    if (!cleaned) return "other";
    const tokens = cleaned
      .toLowerCase()
      .split(/\s+/)
      .filter(
        (t) =>
          t.length >= 2 &&
          !["dr", "cr", "in", "ind", "payment", "pay", "pvt", "ltd", "bank"].includes(t)
      );

    if (tokens.length === 0) return "other";
    return tokens.slice(0, 2).join(" ");
  }

  let filteredIndices = $derived.by(() => {
    return transactions
      .map((tx, idx) => ({ tx, idx }))
      .filter(({ tx, idx }) => {
        if (filterStatus === "duplicate") return tx.is_duplicate;
        if (filterStatus === "unassigned") return !tx.selected_account || editingRowIdx === idx;
        if (filterStatus === "new") return !tx.is_duplicate && !!tx.selected_account;
        return true;
      })
      .map(({ idx }) => idx);
  });

  let selectedFilteredCount = $derived.by(
    () => filteredIndices.filter((idx) => included[idx]).length
  );

  function applyBulkAccount(acc: string) {
    if (!acc) return;
    filteredIndices.forEach((idx) => {
      if (included[idx]) {
        transactions[idx].selected_account = acc;
        transactions[idx].confidence = "manual";
      }
    });
    bulkSelectedAccount = "";
  }

  function applyBulkRule(acc: string) {
    if (!acc) return;
    applyBulkAccount(acc);
    const firstSelectedIdx = filteredIndices.find((idx) => included[idx]);
    if (firstSelectedIdx !== undefined) {
      onquicksave(transactions[firstSelectedIdx]);
    }
  }

  type MerchantGroup = {
    clusterName: string;
    indices: number[];
    totalAmount: number;
    debitCount: number;
    creditCount: number;
    assignedCount: number;
  };

  let merchantGroups = $derived.by(() => {
    const map = new Map<string, number[]>();
    for (const idx of filteredIndices) {
      const tx = transactions[idx];
      const cluster = getMerchantCluster(tx);
      if (!map.has(cluster)) {
        map.set(cluster, []);
      }
      map.get(cluster)!.push(idx);
    }

    const groups: MerchantGroup[] = [];
    for (const [clusterName, indices] of map.entries()) {
      let totalAmount = 0;
      let debitCount = 0;
      let creditCount = 0;
      let assignedCount = 0;

      for (const idx of indices) {
        const tx = transactions[idx];
        totalAmount += tx.amount;
        if (tx.is_debit) debitCount++;
        else creditCount++;
        if (tx.selected_account) assignedCount++;
      }

      groups.push({
        clusterName,
        indices,
        totalAmount,
        debitCount,
        creditCount,
        assignedCount
      });
    }

    groups.sort((a, b) => b.indices.length - a.indices.length);
    return groups;
  });

  function toggleCluster(clusterName: string) {
    collapsedClusters[clusterName] = !collapsedClusters[clusterName];
  }

  function selectCluster(indices: number[], val: boolean) {
    indices.forEach((idx) => {
      included[idx] = val;
    });
  }

  function selectAllFiltered(val: boolean) {
    filteredIndices.forEach((idx) => {
      included[idx] = val;
    });
  }

  function acceptSuggestion(tx: StagedTransaction, idx: number) {
    tx.selected_account = tx.suggested_account || "";
    tx.confidence = "accepted";
    included[idx] = true;
  }
</script>

<div class="staging-container">
  <div
    class="is-flex is-justify-content-space-between is-align-items-center mb-3 is-flex-wrap-wrap"
  >
    <div class="buttons are-small mb-0">
      <button
        class="button is-small {filterStatus === 'all' ? 'is-info' : 'is-light'}"
        onclick={() => (filterStatus = "all")}
      >
        All ({transactions.length})
      </button>
      <button
        class="button is-small {filterStatus === 'new' ? 'is-success' : 'is-light'}"
        onclick={() => (filterStatus = "new")}
      >
        Ready ({transactions.filter((t) => !t.is_duplicate && !!t.selected_account).length})
      </button>
      <button
        class="button is-small {filterStatus === 'duplicate' ? 'is-warning' : 'is-light'}"
        onclick={() => (filterStatus = "duplicate")}
      >
        Duplicates ({transactions.filter((t) => t.is_duplicate).length})
      </button>
      <button
        class="button is-small {filterStatus === 'unassigned' ? 'is-danger' : 'is-light'}"
        onclick={() => (filterStatus = "unassigned")}
      >
        Uncategorized ({transactions.filter((t) => !t.selected_account).length})
      </button>
    </div>

    <div class="buttons are-small mb-0">
      <button
        class="button is-small {groupByMerchant ? 'is-info' : 'is-light'}"
        onclick={() => (groupByMerchant = !groupByMerchant)}
        title="Group transactions by merchant name"
      >
        <span class="icon is-small"><i class="fas fa-layer-group"></i></span>
        <span>Group by Merchant</span>
      </button>
      {#if transactions.some((t) => !t.is_duplicate && !t.selected_account && !!t.suggested_account)}
        <button
          class="button is-small is-link is-light"
          onclick={() => {
            transactions.forEach((tx, idx) => {
              if (!tx.is_duplicate && !tx.selected_account && tx.suggested_account) {
                tx.selected_account = tx.suggested_account;
                tx.confidence = "accepted";
                included[idx] = true;
              }
            });
          }}
        >
          <span class="icon is-small"><i class="fas fa-check-double"></i></span>
          <span>Accept All Suggestions</span>
        </button>
      {/if}
      <button class="button is-white is-small" onclick={() => selectAllFiltered(true)}>
        Select Filtered
      </button>
      <button class="button is-white is-small" onclick={() => selectAllFiltered(false)}>
        Deselect Filtered
      </button>
    </div>
  </div>

  {#if selectedFilteredCount > 0}
    <div
      class="batch-action-bar is-flex is-align-items-center is-justify-content-space-between p-2 mb-3 is-flex-wrap-wrap"
    >
      <div class="is-flex is-align-items-center is-flex-wrap-wrap" style="gap: 0.5rem">
        <span class="tag is-primary is-light is-medium">
          <strong>{selectedFilteredCount}</strong>&nbsp;selected
        </span>
        <div class="field has-addons mb-0">
          <div class="control" style="min-width: 240px">
            <input
              class="input is-small"
              type="text"
              list="staging-all-accounts"
              placeholder="Assign offset account..."
              bind:value={bulkSelectedAccount}
              onkeydown={(e) => {
                if (e.key === "Enter" && bulkSelectedAccount) {
                  applyBulkAccount(bulkSelectedAccount);
                }
              }}
            />
          </div>
          <div class="control">
            <button
              class="button is-small is-primary"
              disabled={!bulkSelectedAccount}
              onclick={() => applyBulkAccount(bulkSelectedAccount)}
            >
              <span class="icon is-small"><i class="fas fa-check"></i></span>
              <span>Apply</span>
            </button>
          </div>
          <div class="control">
            <button
              class="button is-small is-success"
              disabled={!bulkSelectedAccount}
              title="Apply account and save rule for future imports"
              onclick={() => applyBulkRule(bulkSelectedAccount)}
            >
              <span class="icon is-small"><i class="fas fa-bookmark"></i></span>
              <span>Apply &amp; Save Rule</span>
            </button>
          </div>
        </div>
      </div>
      <div>
        <button class="button is-small is-ghost" onclick={() => selectAllFiltered(false)}>
          Clear selection
        </button>
      </div>
    </div>
  {/if}

  <div class="table-wrapper">
    <table class="table is-bordered is-striped is-narrow is-hoverable is-fullwidth is-size-7">
      <thead>
        <tr>
          <th style="width: 40px; text-align: center">
            <input
              type="checkbox"
              checked={allIncluded}
              onchange={(e) => {
                const checked = (e.currentTarget as HTMLInputElement).checked;
                included = transactions.map(() => checked);
              }}
            />
          </th>
          <th style="width: 90px">Date</th>
          <th>Payee &amp; Narration</th>
          <th style="width: 115px; text-align: right">Amount</th>
          <th style="width: 220px">Target Offset Account</th>
          <th style="width: 140px">Status / Rule</th>
          <th style="width: 45px">Action</th>
        </tr>
      </thead>
      <tbody>
        {#if groupByMerchant}
          {#each merchantGroups as group (group.clusterName)}
            {@const allGroupSelected =
              group.indices.length > 0 && group.indices.every((idx) => included[idx])}
            {@const isCollapsed = !!collapsedClusters[group.clusterName]}
            <tr class="merchant-group-header">
              <td style="text-align: center">
                <input
                  type="checkbox"
                  checked={allGroupSelected}
                  onchange={(e) =>
                    selectCluster(group.indices, (e.currentTarget as HTMLInputElement).checked)}
                />
              </td>
              <td colspan="2">
                <div
                  class="is-flex is-align-items-center is-clickable py-1"
                  onclick={() => toggleCluster(group.clusterName)}
                  role="button"
                  tabindex="0"
                  onkeydown={(e) => {
                    if (e.key === "Enter" || e.key === " ") toggleCluster(group.clusterName);
                  }}
                >
                  <span class="icon is-small mr-2 has-text-grey">
                    <i class="fas {isCollapsed ? 'fa-chevron-right' : 'fa-chevron-down'}"></i>
                  </span>
                  <span class="has-text-weight-bold is-capitalized">{group.clusterName}</span>
                  <span class="tag is-rounded is-small ml-2">
                    {group.indices.length}
                    {group.indices.length === 1 ? "txn" : "txns"}
                  </span>
                  {#if group.assignedCount === group.indices.length}
                    <span class="tag is-success is-light is-rounded is-small ml-1">
                      <i class="fas fa-check mr-1"></i>All categorized
                    </span>
                  {:else if group.assignedCount === 0}
                    <span class="tag is-danger is-light is-rounded is-small ml-1">Unassigned</span>
                  {:else}
                    <span class="tag is-warning is-light is-rounded is-small ml-1">
                      {group.assignedCount}/{group.indices.length} categorized
                    </span>
                  {/if}
                </div>
              </td>
              <td style="text-align: right" class="font-mono">
                <span class="has-text-weight-bold">
                  {group.totalAmount.toFixed(2)}
                  {defaultCurrency}
                </span>
              </td>
              <td colspan="3">
                <div class="is-flex is-align-items-center">
                  <input
                    class="input is-small mr-2"
                    style="max-width: 220px"
                    type="text"
                    list="staging-all-accounts"
                    placeholder="Assign all in cluster..."
                    onchange={(e) => {
                      const val = (e.currentTarget as HTMLInputElement).value;
                      if (val) {
                        group.indices.forEach((idx) => {
                          transactions[idx].selected_account = val;
                          transactions[idx].confidence = "manual";
                          included[idx] = true;
                        });
                        (e.currentTarget as HTMLInputElement).value = "";
                      }
                    }}
                  />
                </div>
              </td>
            </tr>
            {#if !isCollapsed}
              {#each group.indices as idx (idx)}
                {@render transactionRow(idx)}
              {/each}
            {/if}
          {/each}
        {:else}
          {#each filteredIndices as idx (idx)}
            {@render transactionRow(idx)}
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>

{#snippet transactionRow(idx: number)}
  {@const tx = transactions[idx]}
  <tr
    class:row-duplicate={tx.is_duplicate}
    class:row-unassigned={!tx.selected_account && !tx.is_duplicate}
  >
    <td style="text-align: center">
      <input type="checkbox" bind:checked={included[idx]} />
    </td>
    <td class="font-mono">{tx.date}</td>
    <td>
      <div class="has-text-weight-semibold">{tx.payee}</div>
      {#if tx.memo && tx.memo !== tx.payee}
        <div class="is-size-7 has-text-grey truncate" style="max-width: 320px" title={tx.memo}>
          {tx.memo}
        </div>
      {/if}
      {#if tx.tags && tx.tags.length > 0}
        <div class="tags mt-1 mb-0">
          {#each tx.tags as tag}
            <span class="tag is-info is-light is-rounded is-small">{tag}</span>
          {/each}
        </div>
      {/if}
    </td>
    <td style="text-align: right" class="font-mono">
      <span class={tx.is_debit ? "has-text-danger" : "has-text-success"}>
        {tx.is_debit ? "-" : "+"}{tx.amount.toFixed(2)}
        {tx.currency || defaultCurrency}
      </span>
    </td>
    <td>
      <div class="control">
        <input
          class="input is-small {!tx.selected_account ? 'is-danger' : ''}"
          type="text"
          list={tx.is_debit ? "staging-debit-accounts" : "staging-credit-accounts"}
          value={tx.selected_account}
          placeholder={tx.suggested_account
            ? `Suggested: ${tx.suggested_account}`
            : "Select account..."}
          onfocus={() => {
            editingRowIdx = idx;
          }}
          onblur={() => {
            editingRowIdx = null;
          }}
          oninput={(e) => {
            const newVal = (e.currentTarget as HTMLInputElement).value;
            tx.selected_account = newVal;
            if (newVal) {
              if (tx.suggested_account && newVal === tx.suggested_account) {
                tx.confidence = "accepted";
              } else {
                tx.confidence = "manual";
              }
              included[idx] = true;
            } else {
              tx.confidence = undefined;
              included[idx] = false;
            }
          }}
        />
      </div>
      {#if tx.suggested_account && !tx.selected_account}
        <div class="is-flex is-align-items-center mt-1">
          <span class="is-size-7 has-text-grey mr-2 truncate" title={tx.suggested_account}>
            <i class="fas fa-lightbulb has-text-warning mr-1"></i>{tx.suggested_account}
          </span>
          <button
            class="button is-small is-link is-light py-0 px-2 ml-auto"
            style="height: 20px; font-size: 0.7rem"
            onclick={() => acceptSuggestion(tx, idx)}
          >
            Accept
          </button>
        </div>
      {/if}
    </td>
    <td>
      {#if tx.is_duplicate}
        <span class="tag is-warning is-rounded is-small" title={tx.duplicate_reason}>
          <i class="fas fa-copy mr-1"></i> Dup ({tx.duplicate_score}%)
        </span>
      {:else if tx.matched_rule_name}
        <span class="tag is-success is-light is-rounded is-small" title={tx.matched_rule_name}>
          <i class="fas fa-wand-magic-sparkles mr-1"></i>
          {tx.matched_rule_name}
        </span>
      {:else if tx.confidence === "historical"}
        <span
          class="tag is-link is-light is-rounded is-small"
          title="Matched from previous transactions in your ledger"
        >
          <i class="fas fa-history mr-1"></i> Past Pattern
        </span>
      {:else if tx.confidence === "taxonomy"}
        <span
          class="tag is-info is-light is-rounded is-small"
          title="Matched from built-in merchant dictionary"
        >
          <i class="fas fa-store mr-1"></i> Merchant
        </span>
      {:else if tx.confidence === "heuristic"}
        <span
          class="tag is-info is-light is-rounded is-small"
          title="Matched using amount & type heuristics"
        >
          <i class="fas fa-calculator mr-1"></i> Heuristic
        </span>
      {:else if tx.confidence === "accepted"}
        <span class="tag is-success is-light is-rounded is-small" title="Accepted recommendation">
          <i class="fas fa-check mr-1"></i> Accepted
        </span>
      {:else if tx.confidence === "manual"}
        <span class="tag is-info is-light is-rounded is-small" title="Manually entered account">
          <i class="fas fa-user-pen mr-1"></i> Manual
        </span>
      {:else if tx.selected_account}
        <span class="tag is-info is-light is-rounded is-small">Assigned</span>
      {:else}
        <span class="tag is-danger is-light is-rounded is-small">Needs Account</span>
      {/if}
      {#if tx.flag_for_review}
        <span class="tag is-danger is-light is-rounded is-small ml-1" title="Flagged for review">
          <i class="fas fa-flag mr-1"></i> Review
        </span>
      {/if}
    </td>
    <td style="text-align: center">
      <div class="is-flex is-justify-content-center">
        {#if tx.selected_account && !tx.matched_rule_name}
          <button
            class="button is-small is-ghost p-1 mr-1"
            title="Quick Save: Remember as rule for future imports"
            onclick={() => onquicksave(tx)}
          >
            <span class="icon is-small has-text-success">
              <i class="fas fa-bookmark"></i>
            </span>
          </button>
        {/if}
        <button
          class="button is-small is-ghost p-1"
          title="Customize rule in Rule Studio"
          onclick={() => oneditrule(tx)}
        >
          <span class="icon is-small has-text-primary">
            <i class="fas fa-wand-magic-sparkles"></i>
          </span>
        </button>
      </div>
    </td>
  </tr>
{/snippet}

<datalist id="staging-all-accounts">
  {#each accounts as acc}
    <option value={acc}>{acc}</option>
  {/each}
</datalist>

<datalist id="staging-debit-accounts">
  {#each debitAccounts as acc}
    <option value={acc}>{acc}</option>
  {/each}
</datalist>

<datalist id="staging-credit-accounts">
  {#each creditAccounts as acc}
    <option value={acc}>{acc}</option>
  {/each}
</datalist>

<style lang="scss">
  .staging-container {
    border: 1px solid rgba(127, 127, 127, 0.2);
    border-radius: 6px;
    padding: 0.75rem;
    background: var(--bulma-card-background-color, #fff);

    :global(html[data-theme="dark"]) & {
      background: hsl(215, 18%, 13%);
      border-color: rgba(255, 255, 255, 0.1);
    }
  }

  .batch-action-bar {
    background: rgba(32, 156, 238, 0.08);
    border: 1px solid rgba(32, 156, 238, 0.25);
    border-radius: 6px;

    :global(html[data-theme="dark"]) & {
      background: rgba(32, 156, 238, 0.15);
      border-color: rgba(32, 156, 238, 0.35);
    }
  }

  .table-wrapper {
    max-height: calc(100vh - 350px);
    overflow-y: auto;
  }

  tr.merchant-group-header {
    background-color: rgba(0, 0, 0, 0.03);

    :global(html[data-theme="dark"]) & {
      background-color: rgba(255, 255, 255, 0.06);
    }

    td {
      vertical-align: middle;
      border-bottom: 2px solid rgba(127, 127, 127, 0.2);
    }
  }

  tr.row-unassigned {
    background-color: rgba(241, 70, 104, 0.08) !important;

    :global(html[data-theme="dark"]) & {
      background-color: rgba(241, 70, 104, 0.14) !important;
    }
  }

  tr.row-duplicate {
    background-color: rgba(255, 221, 87, 0.12) !important;

    :global(html[data-theme="dark"]) & {
      background-color: rgba(255, 221, 87, 0.16) !important;
    }
  }
</style>
