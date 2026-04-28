<script lang="ts">
  import GermanyTaxCard from "$lib/components/GermanyTaxCard.svelte";
  import { ajax, type GermanyTaxYear } from "$lib/utils";
  import _ from "lodash";
  import { onMount } from "svelte";

  let taxYears: GermanyTaxYear[] = [];

  onMount(async () => {
    const { tax_years: taxYearsByYear } = await ajax("/api/capital_income_tax");
    taxYears = _.chain(taxYearsByYear).values().sortBy("tax_year").reverse().value();
  });
</script>

<section class="section tab-capital-income-tax">
  <div class="container is-fluid">
    <div class="columns is-flex-wrap-wrap">
      {#each taxYears as taxYear}
        <GermanyTaxCard {taxYear} />
      {/each}
    </div>
  </div>
</section>
