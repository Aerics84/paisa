import * as d3 from "d3";
import dayjs from "dayjs";
import _, { round } from "lodash";
import { get } from "svelte/store";
import COLORS from "./colors";
import { t } from "./i18n";
import { formatCurrency, formatFloat, restName, tooltip, type Harvestable } from "./utils";

export function renderHarvestables(harvestables: Harvestable[]) {
  const id = "#d3-harvestables";
  const i18n = get(t);
  const root = d3.select(id);
  const currencyCode = USER_CONFIG.default_currency || "";

  const card = root
    .selectAll("div.column")
    .data(_.filter(harvestables, (harvestable) => harvestable.harvestable_units > 0))
    .enter()
    .append("div")
    .attr("class", "column is-12")
    .append("div")
    .attr("class", "card");

  const header = card.append("header").attr("class", "card-header");

  header
    .append("p")
    .attr("class", "card-header-title")
    .text((h) => restName(h.account));

  header
    .append("div")
    .attr("class", "card-header-icon")
    .style("flex-grow", "1")
    .style("cursor", "auto")
    .append("div")
    .each(function (h) {
      const self = d3.select(this);
      const [units, amount, taxableGain] = unitsRequiredFromGain(h, 100000);
      self.append("span").html(`${i18n("tax.harvest.redeemPrefix")}&nbsp;`);
      const unitsSpan = self.append("span").text(formatFloat(units));
      self.append("span").html(`&nbsp;${i18n("tax.harvest.redeemUnits")}&nbsp;`);
      const amountInput = self
        .append("input")
        .attr("class", "input is-small adjustable-input")
        .attr("type", "number")
        .attr("value", round(amount))
        .attr("step", "1000")
        .on("input", (event) => {
          const [units, amount, taxableGain] = unitsRequiredFromAmount(
            h,
            parseInt((event.srcElement as HTMLInputElement).value)
          );

          unitsSpan.text(formatFloat(units));
          (taxableGainInput.node() as HTMLInputElement).value = round(taxableGain).toString();
          (event.srcElement as HTMLInputElement).value = round(amount).toString();
        });

      if (currencyCode) {
        self.append("span").html(`&nbsp;${currencyCode}`);
      }

      self.append("span").html(`&nbsp; ${i18n("tax.harvest.redeemSuffix")}&nbsp;`);
      const taxableGainInput = self
        .append("input")
        .attr("class", "input is-small adjustable-input")
        .attr("type", "number")
        .attr("value", round(taxableGain))
        .attr("step", "1000")
        .on("input", (event) => {
          const [units, amount, taxableGain] = unitsRequiredFromGain(
            h,
            parseInt((event.srcElement as HTMLInputElement).value)
          );
          unitsSpan.text(formatFloat(units));
          (event.srcElement as HTMLInputElement).value = round(taxableGain).toString();
          (amountInput.node() as HTMLInputElement).value = round(amount).toString();
        });

      if (currencyCode) {
        self.append("span").html(`&nbsp;${currencyCode}`);
      }
    });

  header
    .append("span")
    .attr("class", "card-header-icon")
    .text((harvestable) =>
      i18n("tax.harvest.priceAsOn", {
        date: dayjs(harvestable.current_unit_date).format("DD MMM YYYY")
      })
    );

  const content = card
    .append("div")
    .attr("class", "card-content")
    .append("div")
    .attr("class", "content")
    .append("div")
    .attr("class", "columns");

  const summary = content.append("div").attr("class", "column is-4");

  summary.append("div").each(renderSingleBar);

  summary.append("div").html((h) => {
    return `
<table class="table is-narrow is-fullwidth is-hoverable">
  <tbody>
    <tr>
      <td>${i18n("assets.balanceUnits")}</td>
      <td class='has-text-right has-text-weight-bold'>${formatFloat(h.total_units)}</td>
    </tr>
    <tr>
      <td>${i18n("tax.harvest.harvestableUnits")}</td>
      <td class='has-text-right has-text-weight-bold has-text-success'>${formatFloat(
        h.harvestable_units
      )}</td>
    </tr>
    <tr>
      <td>${i18n("tax.taxCategory")}</td>
      <td class='has-text-right is-uppercase'>${h.tax_category}</td>
    </tr>
    <tr>
      <td>${i18n("tax.harvest.currentUnitPrice")}</td>
      <td class='has-text-right has-text-weight-bold'>${formatFloat(h.current_unit_price)}</td>
    </tr>
    <tr>
      <td>${i18n("tax.harvest.unrealizedGain")}</td>
      <td class='has-text-right has-text-weight-bold'>${formatCurrency(h.unrealized_gain)}</td>
    </tr>
    <tr>
      <td>${i18n("tax.harvest.taxableUnrealizedGain")}</td>
      <td class='has-text-right has-text-weight-bold'>${formatCurrency(
        h.taxable_unrealized_gain
      )}</td>
    </tr>
  </tbody>
</table>
`;
  });

  const table = content
    .append("div")
    .attr("class", "column is-8")
    .append("div")
    .attr("class", "table-container")
    .style("overflow-y", "auto")
    .style("max-height", "245px")
    .append("table")
    .attr("class", "table is-hoverable");

  table.append("thead").html(`
<tr>
  <th>${i18n("tax.purchaseDate")}</th>
  <th class='has-text-right'>${i18n("ledger.table.units")}</th>
  <th class='has-text-right'>${i18n("tax.purchasePrice")}</th>
  <th class='has-text-right'>${i18n("tax.purchaseUnitPrice")}</th>
  <th class='has-text-right'>${i18n("tax.harvest.currentPrice")}</th>
  <th class='has-text-right'>${i18n("tax.gain")}</th>
  <th class='has-text-right'>${i18n("tax.taxableGain")}</th>
  <th class='has-text-right'>${i18n("tax.shortTermTax")}</th>
  <th class='has-text-right'>${i18n("tax.longTermTax")}</th>
  <th class='has-text-right'>${i18n("tax.taxableAtSlabRate")}</th>
</tr>
`);

  table
    .append("tbody")
    .selectAll("tr")
    .data((harvestable) => {
      return harvestable.harvest_breakdown;
    })
    .enter()
    .append("tr")
    .html((breakdown) => {
      return `
<tr>
  <td style="white-space: nowrap">${dayjs(breakdown.purchase_date).format("DD MMM YYYY")}</td>
  <td class='has-text-right'>${formatFloat(breakdown.units)}</td>
  <td class='has-text-right'>${formatCurrency(breakdown.purchase_price)}</td>
  <td class='has-text-right'>${formatFloat(breakdown.purchase_unit_price)}</td>
  <td class='has-text-right'>${formatCurrency(breakdown.current_price)}</td>
  <td class='has-text-right has-text-weight-bold'>${formatCurrency(breakdown.tax.gain)}</td>
  <td class='has-text-right has-text-weight-bold'>${formatCurrency(breakdown.tax.taxable)}</td>
  <td class='has-text-right has-text-weight-bold'>${formatCurrency(breakdown.tax.short_term)}</td>
  <td class='has-text-right has-text-weight-bold'>${formatCurrency(breakdown.tax.long_term)}</td>
  <td class='has-text-right has-text-weight-bold'>${formatCurrency(breakdown.tax.slab)}</td>
</tr>
`;
    });
}

function unitsRequiredFromGain(
  harvestable: Harvestable,
  taxableGain: number
): [number, number, number] {
  let gain = 0;
  let amount = 0;
  let units = 0;
  const available = _.clone(harvestable.harvest_breakdown);
  while (taxableGain > gain && available.length > 0) {
    const breakdown = available.shift();
    if (breakdown.tax.taxable < taxableGain - gain) {
      gain += breakdown.tax.taxable;
      units += breakdown.units;
      amount += breakdown.current_price;
    } else {
      const u = ((taxableGain - gain) * breakdown.units) / breakdown.tax.taxable;
      units += u;
      amount += u * harvestable.current_unit_price;
      gain = taxableGain;
    }
  }
  return [units, amount, gain];
}

function unitsRequiredFromAmount(
  harvestable: Harvestable,
  expectedAmount: number
): [number, number, number] {
  let gain = 0;
  let amount = 0;
  let units = 0;
  const available = _.clone(harvestable.harvest_breakdown);
  while (expectedAmount > amount && available.length > 0) {
    const breakdown = available.shift();
    if (breakdown.current_price < expectedAmount - amount) {
      gain += breakdown.tax.taxable;
      units += breakdown.units;
      amount += breakdown.current_price;
    } else {
      const u = (expectedAmount - amount) / harvestable.current_unit_price;
      units += u;
      amount = expectedAmount;
      gain += u * (breakdown.tax.taxable / breakdown.units);
    }
  }
  return [units, amount, gain];
}

function renderSingleBar(harvestable: Harvestable) {
  const i18n = get(t);
  const selection = d3.select(this);
  const svg = selection.append("svg");

  const height = 20;
  const margin = { top: 20, right: 0, bottom: 20, left: 0 },
    width = selection.node().clientWidth - margin.left - margin.right,
    g = svg.append("g").attr("transform", "translate(" + margin.left + "," + margin.top + ")");

  svg
    .attr("width", width + margin.left + margin.right)
    .attr("height", height + margin.top + margin.bottom);

  const x = d3.scaleLinear().range([0, width]).domain([0, harvestable.total_units]);

  const non_harvestable_units = harvestable.total_units - harvestable.harvestable_units;

  g.attr("data-tippy-content", () => {
    return tooltip([
      [
        [i18n("tax.harvest.type"), "has-text-weight-bold"],
        [i18n("ledger.table.units"), "has-text-weight-bold has-text-right"],
        [i18n("assets.percent"), "has-text-weight-bold has-text-right"]
      ],
      [
        i18n("tax.harvest.harvestable"),
        [formatFloat(harvestable.harvestable_units), "has-text-right"],
        [
          formatFloat((harvestable.harvestable_units / harvestable.total_units) * 100),
          "has-text-right"
        ]
      ],
      [
        i18n("tax.harvest.nonHarvestable"),
        [formatFloat(non_harvestable_units), "has-text-right"],
        [formatFloat((non_harvestable_units / harvestable.total_units) * 100), "has-text-right"]
      ]
    ]);
  });

  g.selectAll("rect")
    .data([
      { start: 0, end: harvestable.harvestable_units, color: COLORS.gainText },
      {
        start: harvestable.harvestable_units,
        end: harvestable.total_units,
        color: COLORS.tertiary
      }
    ])
    .join("rect")
    .attr("fill", (d) => d.color)
    .attr("x", (d) => x(d.start))
    .attr("width", (d) => x(d.end) - x(d.start))
    .attr("y", 0)
    .attr("height", height);
}
