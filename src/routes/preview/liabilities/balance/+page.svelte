<script lang="ts">
  import Table from "$lib/components/Table.svelte";
  import {
    indendedLiabilityAccountName,
    nonZeroCurrency,
    nonZeroFloatChange
  } from "$lib/table_formatters";
  import { getLiabilityBalance } from "$lib/api/liabilities";
  import { buildTree, type LiabilityBreakdown } from "$lib/utils";
  import _ from "lodash";
  import { onMount } from "svelte";
  import type { ColumnDefinition } from "tabulator-tables";
  import { t } from "$lib/i18n";

  let breakdowns = $state<Record<string, LiabilityBreakdown>>({});
  let isEmpty = $state(false);

  onMount(async () => {
    ({ liability_breakdowns: breakdowns } = await getLiabilityBalance());

    if (_.isEmpty(breakdowns)) {
      isEmpty = true;
    }
  });

  let columns = $derived<ColumnDefinition[]>([
    {
      title: $t("liabilities.account"),
      field: "group",
      formatter: indendedLiabilityAccountName,
      frozen: true
    },
    {
      title: $t("liabilities.drawnAmount"),
      field: "drawn_amount",
      hozAlign: "right",
      vertAlign: "middle",
      formatter: nonZeroCurrency
    },
    {
      title: $t("liabilities.repaidAmount"),
      field: "repaid_amount",
      hozAlign: "right",
      formatter: nonZeroCurrency
    },
    {
      title: $t("liabilities.balanceAmount"),
      field: "balance_amount",
      hozAlign: "right",
      formatter: nonZeroCurrency
    },
    {
      title: $t("liabilities.interestAmount"),
      field: "interest_amount",
      hozAlign: "right",
      formatter: nonZeroCurrency
    },
    { title: $t("liabilities.apr"), field: "apr", hozAlign: "right", formatter: nonZeroFloatChange }
  ]);

  let tree = $derived(buildTree(Object.values(breakdowns), (i) => i.group));
</script>

<section class="section" class:is-hidden={!isEmpty}>
  <div class="container is-fluid">
    <div class="columns is-centered">
      <div class="column is-4 has-text-centered">
        <article class="message">
          <div class="message-body">
            <strong>{$t("common.hurray")}</strong>
            {$t("liabilities.noLiabilities")}
          </div>
        </article>
      </div>
    </div>
  </div>
</section>

<section class="section pb-0" class:is-hidden={isEmpty}>
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12 pb-0">
        <Table data={tree} tree {columns} />
      </div>
    </div>
  </div>
</section>
