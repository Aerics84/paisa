<script lang="ts">
  import { type AssetBreakdown, buildTree, isMobile, lastName } from "$lib/utils";
  import _ from "lodash";
  import Table from "./Table.svelte";
  import type { ColumnDefinition } from "tabulator-tables";
  import {
    accountName,
    formatCurrencyChange,
    indendedAssetAccountName,
    nonZeroCurrency,
    nonZeroFloatChange,
    nonZeroPercentageChange
  } from "$lib/table_formatters";
  import { iconText } from "$lib/icon";

  export let breakdowns: Record<string, AssetBreakdown>;
  export let indent = true;

  const mobile = typeof window !== "undefined" && isMobile();

  function mobileAccountName(account: string, shorten: boolean) {
    const name = shorten ? lastName(account) : account;
    return `<span class="truncate is-inline-block" style="max-width: 100%; vertical-align: bottom;" title="${_.escape(account)}"><span class="has-text-grey custom-icon">${iconText(account)}</span> <a href="/assets/gain/${account}">${_.escape(name)}</a></span>`;
  }

  function accountColumn(): ColumnDefinition {
    return {
      title: "Account",
      field: "group",
      formatter: mobile
        ? (cell) => mobileAccountName(cell.getValue(), indent)
        : indent
          ? indendedAssetAccountName
          : accountName,
      width: mobile ? 140 : undefined,
      minWidth: mobile ? 140 : undefined,
      maxWidth: mobile ? 155 : undefined
      // Do not freeze this column: Tabulator renders frozen columns in a
      // separate table, which drifts out of sync with the header when the
      // table is horizontally scrollable.
    };
  }

  const columns: ColumnDefinition[] = [
    accountColumn(),
    {
      title: "Investment Amount",
      field: "investmentAmount",
      hozAlign: "right",
      vertAlign: "middle",
      formatter: nonZeroCurrency
    },
    {
      title: "Withdrawal Amount",
      field: "withdrawalAmount",
      hozAlign: "right",
      formatter: nonZeroCurrency
    },
    {
      title: "Balance Units",
      field: "balanceUnits",
      hozAlign: "right",
      formatter: nonZeroCurrency
    },
    { title: "Market Value", field: "marketAmount", hozAlign: "right", formatter: nonZeroCurrency },
    { title: "Change", field: "gainAmount", hozAlign: "right", formatter: formatCurrencyChange },
    { title: "XIRR", field: "xirr", hozAlign: "right", formatter: nonZeroFloatChange },
    {
      title: "Absolute Return",
      field: "absoluteReturn",
      hozAlign: "right",
      formatter: nonZeroPercentageChange
    }
  ];

  // Keep the most useful figures next to the account name on narrow screens.
  const mobileColumns: ColumnDefinition[] = [
    columns[0],
    columns[4],
    columns[5],
    columns[6],
    columns[7],
    columns[1],
    columns[2],
    columns[3]
  ];

  let tree: AssetBreakdown[] = [];
  $: if (breakdowns) {
    tree = buildTree(Object.values(breakdowns), (i) => i.group);
  }
</script>

{#if indent}
  <Table data={tree} tree columns={mobile ? mobileColumns : columns} />
{:else}
  <Table data={Object.values(breakdowns)} columns={mobile ? mobileColumns : columns} />
{/if}
