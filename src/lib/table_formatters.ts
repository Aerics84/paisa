import { type CellComponent } from "tabulator-tables";
import _ from "lodash";
import { formatCurrency, formatFloat, formatPercentage, isZero, lastName } from "./utils";
import { iconText } from "./icon";

type TreeRow = {
  _children?: unknown[];
  commodity?: string;
  commodity_name?: string;
  commodity_code?: string;
};

export function commodityTooltipHtml(commodity?: string, name?: string, code?: string) {
  const rows: string[] = [];
  const displayName = name || commodity || "";
  if (displayName) rows.push(`<p><strong>${_.escape(displayName)}</strong></p>`);
  if (code) rows.push(`<p>Symbol: ${_.escape(code)}</p>`);
  if (commodity && commodity !== displayName && commodity !== code) {
    rows.push(`<p>Commodity: ${_.escape(commodity)}</p>`);
  }
  return rows.join("");
}

function commodityAttributes(data: TreeRow) {
  const tooltip = commodityTooltipHtml(data.commodity, data.commodity_name, data.commodity_code);
  if (!tooltip) return "";
  const name = data.commodity_name || data.commodity || "";
  const plain =
    name && data.commodity_code ? `${name} (${data.commodity_code})` : name || data.commodity_code;
  return ` data-tippy-content="${_.escape(tooltip)}" title="${_.escape(plain)}" aria-label="${_.escape(plain)}"`;
}

export function indendedAssetAccountName(cell: CellComponent) {
  const account = cell.getValue();
  let children = "";
  const data = cell.getData() as TreeRow;
  if ((data._children?.length || 0) > 0) {
    children = `(${data._children?.length})`;
  }
  return `
<span class="whitespace-nowrap" style="max-width: max(15rem, 33.33vw); overflow: hidden;">
  <span class="has-text-grey custom-icon">${iconText(account)}</span>
  <a href="/assets/gain/${account}"${commodityAttributes(data)}>${lastName(account)}</a>
  <span class="has-text-grey-light is-size-7">${children}</span>
</span>
`;
}

export function indendedLiabilityAccountName(cell: CellComponent) {
  const account = cell.getValue();
  let children = "";
  const data = cell.getData() as TreeRow;
  if ((data._children?.length || 0) > 0) {
    children = `(${data._children?.length})`;
  }
  return `
<span class="whitespace-nowrap" style="max-width: max(15rem, 33.33vw); overflow: hidden;">
  <span class="has-text-grey custom-icon">${iconText(account)}</span>
  <span>${lastName(account)}</span>
  <span class="has-text-grey-light is-size-7">${children}</span>
</span>
`;
}

export function accountName(cell: CellComponent) {
  const account = cell.getValue();
  const data = cell.getData() as TreeRow;
  return `
<span class="whitespace-nowrap" style="max-width: max(15rem, 33.33vw); overflow: hidden;">
  <span class="has-text-grey custom-icon">${iconText(account)}</span>
  <a href="/assets/gain/${account}"${commodityAttributes(data)}>${account}</a>
</span>
`;
}

function calculateChangeClass(gain: number) {
  let changeClass = "";
  if (gain > 0) {
    changeClass = "has-text-success";
  } else if (gain < 0) {
    changeClass = "has-text-danger";
  }
  return changeClass;
}

export function nonZeroCurrency(cell: CellComponent) {
  const value = cell.getValue();
  return isZero(value) ? "" : formatCurrency(value);
}

export function nonZeroFloatChange(cell: CellComponent) {
  const value = cell.getValue();
  return isZero(value)
    ? ""
    : `<span class="${calculateChangeClass(value)}">${formatFloat(value)}</span>`;
}

export function nonZeroPercentageChange(cell: CellComponent) {
  const value = cell.getValue();
  return isZero(value)
    ? ""
    : `<span class="${calculateChangeClass(value)}">${formatPercentage(value, 2)}</span>`;
}

export function formatCurrencyChange(cell: CellComponent) {
  const value = cell.getValue();
  return isZero(value)
    ? ""
    : `<span class="${calculateChangeClass(value)}">${formatCurrency(value)}</span>`;
}
