<script lang="ts">
  import { formatCurrency, formatFloat, type CapitalGain, type FYCapitalGain } from "$lib/utils";
  import _ from "lodash";
  import CapitalGainDetailCard from "./CapitalGainDetailCard.svelte";
  import Toggleable from "./Toggleable.svelte";
  import { t } from "$lib/i18n";

  interface Props {
    financialYear: string;
    capitalGains: CapitalGain[];
    totals?: Record<string, FYCapitalGain> | undefined;
  }

  let { financialYear, capitalGains, totals = undefined }: Props = $props();

  const fyGains: FYCapitalGain[] = $derived(
    _.compact(capitalGains.map((cg) => cg.fy[financialYear]))
  );
  const fyTotal = $derived(totals?.[financialYear]);

  const total = $derived(
    fyTotal
      ? {
          withdrawn: fyTotal.sell_price,
          gain: fyTotal.tax.gain,
          taxableGain: fyTotal.tax.taxable,
          shortTermTax: fyTotal.tax.short_term,
          longTermTax: fyTotal.tax.long_term,
          slab: fyTotal.tax.slab
        }
      : {
          withdrawn: _.sumBy(fyGains, (fy) => fy.sell_price),
          gain: _.sumBy(fyGains, (fy) => fy.tax.gain),
          taxableGain: _.sumBy(fyGains, (fy) => fy.tax.taxable),
          shortTermTax: _.sumBy(fyGains, (fy) => fy.tax.short_term),
          longTermTax: _.sumBy(fyGains, (fy) => fy.tax.long_term),
          slab: _.sumBy(fyGains, (fy) => fy.tax.slab)
        }
  );
</script>

<div class="column is-12">
  <div class="card">
    <header class="card-header">
      <p class="card-header-title">{financialYear}</p>
    </header>

    <div class="card-content">
      <div class="content">
        <div class="columns">
          <div class="column is-4">
            <table class="table is-narrow is-fullwidth is-hoverable">
              <tbody>
                <tr>
                  <td>{$t("tax.withdrawn")}</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["withdrawn"])}</td
                  >
                </tr>
                <tr>
                  <td>{$t("tax.gain")}</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["gain"])}</td
                  >
                </tr>
                <tr>
                  <td>{$t("tax.taxableGain")}</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["taxableGain"])}</td
                  >
                </tr>
                <tr>
                  <td>{$t("tax.shortTermTax")}</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["shortTermTax"])}</td
                  >
                </tr>
                <tr>
                  <td>{$t("tax.longTermTax")}</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["longTermTax"])}</td
                  >
                </tr>
                <tr>
                  <td>{$t("tax.taxableAtSlabRate")}</td>
                  <td class="has-text-right has-text-weight-bold"
                    >{formatCurrency(total["slab"])}</td
                  >
                </tr>
              </tbody>
            </table>
          </div>
          <div class="column is-8 overflow-x-auto">
            <table class="table is-narrow is-fullwidth is-hoverable">
              <thead>
                <tr>
                  <th></th>
                  <th>{$t("assets.account")}</th>
                  <th>{$t("tax.taxCategory")}</th>
                  <th class="has-text-right">{$t("tax.soldUnits")}</th>
                  <th class="has-text-right">{$t("tax.purchasePrice")}</th>
                  <th class="has-text-right">{$t("tax.averagePurchaseUnitPrice")}</th>
                  <th class="has-text-right">{$t("tax.sellPrice")}</th>
                  <th class="has-text-right">{$t("tax.averageSellUnitPrice")}</th>
                  <th class="has-text-right">{$t("tax.gain")}</th>
                  <th class="has-text-right">{$t("tax.taxableGain")}</th>
                  <th class="has-text-right">{$t("tax.shortTermTax")}</th>
                  <th class="has-text-right">{$t("tax.longTermTax")}</th>
                  <th class="has-text-right">{$t("tax.taxableAtSlabRate")}</th>
                </tr>
              </thead>
              <tbody>
                {#each capitalGains as cg}
                  {#if cg.fy[financialYear]}
                    {@const fy = cg.fy[financialYear]}
                    <Toggleable>
                      <tr
                        class={active ? "is-active has-background-white-ter" : ""}
                        style="cursor: pointer;"
                        slot="toggle"
                        let:active
                        let:onclick
                        onclick={(e) => onclick(e)}
                      >
                        <td>
                          <span class="icon has-text-link">
                            <i
                              class="fas {active ? 'fa-chevron-up' : 'fa-chevron-down'}"
                              aria-hidden="true"
                            ></i>
                          </span>
                        </td>
                        <td>{cg.account}</td>
                        <td>{cg.tax_category}</td>
                        <td class="has-text-right">{formatFloat(fy.units)}</td>
                        <td class="has-text-right">{formatCurrency(fy.purchase_price)}</td>
                        <td class="has-text-right"
                          >{formatCurrency(fy.purchase_price / fy.units, 4)}</td
                        >
                        <td class="has-text-right">{formatCurrency(fy.sell_price)}</td>
                        <td class="has-text-right">{formatCurrency(fy.sell_price / fy.units, 4)}</td
                        >
                        <td class="has-text-right has-text-weight-bold"
                          >{formatCurrency(fy.tax.gain)}</td
                        >
                        <td class="has-text-right has-text-weight-bold"
                          >{formatCurrency(fy.tax.taxable)}</td
                        >
                        <td class="has-text-right has-text-weight-bold"
                          >{formatCurrency(fy.tax.short_term)}</td
                        >
                        <td class="has-text-right has-text-weight-bold"
                          >{formatCurrency(fy.tax.long_term)}</td
                        >
                        <td class="has-text-right has-text-weight-bold"
                          >{formatCurrency(fy.tax.slab)}</td
                        >
                      </tr>
                      <tr slot="content">
                        <td colspan="13" class="p-0">
                          <CapitalGainDetailCard fyCapitalGain={fy} />
                        </td>
                      </tr>
                    </Toggleable>
                  {/if}
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>
</div>
