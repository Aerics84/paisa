<script lang="ts">
  import { formatCurrency, type FYCapitalGain } from "$lib/utils";
  import { t } from "$lib/i18n";
  const DATE_FORMAT = "DD MMM YYYY";

  interface Props {
    fyCapitalGain: FYCapitalGain;
  }

  let { fyCapitalGain }: Props = $props();
</script>

<div>
  <table class="table is-narrow is-fullwidth is-bordered">
    <thead>
      <tr>
        <th>{$t("tax.purchaseDate")}</th>
        <th class="has-text-right">{$t("tax.purchasePrice")}</th>
        <th>{$t("tax.sellDate")}</th>
        <th class="has-text-right">{$t("tax.sellPrice")}</th>
        <th class="has-text-right">{$t("tax.gain")}</th>
        <th class="has-text-right">{$t("tax.taxableGain")}</th>
        <th class="has-text-right">{$t("tax.shortTermTax")}</th>
        <th class="has-text-right">{$t("tax.longTermTax")}</th>
        <th class="has-text-right">{$t("tax.taxableAtSlabRate")}</th>
      </tr>
    </thead>
    <tbody>
      {#each fyCapitalGain.posting_pairs as pp}
        <tr class="is-size-7">
          <td>{pp.purchase.date.format(DATE_FORMAT)}</td>
          <td class="has-text-right">{formatCurrency(pp.purchase.amount)}</td>
          <td>{pp.sell.date.format(DATE_FORMAT)}</td>
          <td class="has-text-right">{formatCurrency(-pp.sell.amount)}</td>
          <td class="has-text-right has-text-weight-bold">{formatCurrency(pp.tax.gain)}</td>
          <td class="has-text-right has-text-weight-bold">{formatCurrency(pp.tax.taxable)}</td>
          <td class="has-text-right has-text-weight-bold">{formatCurrency(pp.tax.short_term)}</td>
          <td class="has-text-right has-text-weight-bold">{formatCurrency(pp.tax.long_term)}</td>
          <td class="has-text-right has-text-weight-bold">{formatCurrency(pp.tax.slab)}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>
