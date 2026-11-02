import { describe, expect, it } from "bun:test";
import dayjs from "dayjs";
import isSameOrBefore from "dayjs/plugin/isSameOrBefore";
import { dateBounds } from "./date_range";
import { resolveFinancialYear } from "./financial_year";
import { financialYear, forEachFinancialYear } from "./utils";

dayjs.extend(isSameOrBefore);
// The default config uses a January start; use the Indian April start for the
// integration case below so the data ending in March belongs to 2023-24.
globalThis.USER_CONFIG = { financial_year_starting_month: 4 } as never;

describe("dateBounds", () => {
  it("returns the earliest and latest dates from unsorted data", () => {
    const bounds = dateBounds([dayjs("2022-02-01"), dayjs("2024-03-31"), dayjs("2019-04-05")]);

    expect(bounds?.dateMin.format("YYYY-MM-DD")).toBe("2019-04-05");
    expect(bounds?.dateMax.format("YYYY-MM-DD")).toBe("2024-03-31");
  });

  it("returns the same date for a single data point", () => {
    const bounds = dateBounds([dayjs("2024-03-31")]);

    expect(bounds?.dateMin.format("YYYY-MM-DD")).toBe("2024-03-31");
    expect(bounds?.dateMax.format("YYYY-MM-DD")).toBe("2024-03-31");
  });

  it("returns null when there are no dates", () => {
    expect(dateBounds([])).toBeNull();
  });
});

describe("financial year default from loaded data", () => {
  it("selects the latest year covered by the data", () => {
    const bounds = dateBounds([dayjs("2024-03-31"), dayjs("2019-04-05"), dayjs("2022-02-01")]);

    const years = forEachFinancialYear(bounds!.dateMin, bounds!.dateMax)
      .reverse()
      .map(financialYear);

    expect(years[0]).toBe("2023 - 24");
    expect(resolveFinancialYear("2026 - 27", years)).toBe("2023 - 24");
  });
});
