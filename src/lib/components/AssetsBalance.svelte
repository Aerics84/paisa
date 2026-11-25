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
  import { t } from "$lib/i18n";
  import { get } from "svelte/store";

  interface Props {
    breakdowns: Record<string, AssetBreakdown>;
    indent?: boolean;
  }

  let { breakdowns, indent = true }: Props = $props();

  const mobile = typeof window !== "undefined" && isMobile();
  const i18n = get(t);

  function mobileAccountName(account: string, shorten: boolean) {
    const name = shorten ? lastName(account) : account;
    return `<span class="truncate is-inline-block" style="max-width: 100%; vertical-align: bottom;" title="${_.escape(account)}"><span class="has-text-grey custom-icon">${iconText(account)}</span> <a href="/assets/gain/${account}">${_.escape(name)}</a></span>`;
  }

  function accountColumn(): ColumnDefinition {
    return {
      title: i18n("assets.account"),
      field: "group",
      formatter: mobile
        ? (cell) => mobileAccountName(cell.getValue(), indent)
        : indent
          ? indendedAssetAccountName
          : accountName,
      // A fixed width keeps the header and body columns aligned even when
      // fitDataTable only measures a subset of rows. Long names are truncated
      // with the full account in the title tooltip.
      width: mobile ? 140 : 300,
      minWidth: mobile ? 140 : 240,
      maxWidth: mobile ? 155 : 360
    };
  }

  const columns: ColumnDefinition[] = [
    accountColumn(),
    {
      title: i18n("assets.investmentAmount"),
      field: "investmentAmount",
      hozAlign: "right",
      vertAlign: "middle",
      formatter: nonZeroCurrency
    },
    {
      title: i18n("assets.withdrawalAmount"),
      field: "withdrawalAmount",
      hozAlign: "right",
      formatter: nonZeroCurrency
    },
    {
      title: i18n("assets.balanceUnits"),
      field: "balanceUnits",
      hozAlign: "right",
      formatter: nonZeroCurrency
    },
    {
      title: i18n("assets.marketValue"),
      field: "marketAmount",
      hozAlign: "right",
      formatter: nonZeroCurrency
    },
    {
      title: i18n("assets.cash"),
      field: "cashAmount",
      headerTooltip: i18n("assets.cashTooltip"),
      hozAlign: "right",
      formatter: nonZeroCurrency
    },
    {
      title: i18n("assets.investmentChange"),
      field: "gainAmount",
      headerTooltip: i18n("assets.investmentChangeTooltip"),
      hozAlign: "right",
      formatter: formatCurrencyChange
    },
    { title: "XIRR", field: "xirr", hozAlign: "right", formatter: nonZeroFloatChange },
    {
      title: i18n("assets.absoluteReturnColumn"),
      field: "absoluteReturn",
      hozAlign: "right",
      formatter: nonZeroPercentageChange
    }
  ];

  // Keep the most useful figures next to the account name on narrow screens.
  const mobileColumns: ColumnDefinition[] = [
    columns[0],
    columns[4],
    columns[6],
    columns[7],
    columns[8],
    columns[1],
    columns[2],
    columns[3]
  ];

  let tree = $derived(
    breakdowns ? buildTree(Object.values(breakdowns), (i) => i.group) : ([] as AssetBreakdown[])
  );
</script>

{#if indent}
  <Table data={tree} tree columns={mobile ? mobileColumns : columns} />
{:else}
  <Table data={Object.values(breakdowns)} columns={mobile ? mobileColumns : columns} />
{/if}
