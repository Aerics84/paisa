import _ from "lodash";

type ConfiguredCommodity = {
  name?: string;
  display_name?: string;
  price?: {
    code?: string;
  };
};

function configuredCommodities(): ConfiguredCommodity[] {
  if (typeof USER_CONFIG === "undefined" || !USER_CONFIG) {
    return [];
  }
  return (USER_CONFIG.commodities || []) as ConfiguredCommodity[];
}

export function findCommodityInfo(commodity?: string): {
  displayName: string;
  code: string;
} {
  if (!commodity) {
    return { displayName: "", code: "" };
  }

  const configured = configuredCommodities().find((c) => c.name === commodity);
  return {
    displayName: configured?.display_name || "",
    code: configured?.price?.code || ""
  };
}

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

export function commodityTooltip(commodity?: string) {
  const { displayName, code } = findCommodityInfo(commodity);
  if (!displayName && !code) {
    return "";
  }
  return commodityTooltipHtml(commodity, displayName, code);
}
