import "../happydom";

import { beforeEach, describe, expect, test } from "bun:test";
import * as d3 from "d3";
import dayjs from "dayjs";
import isSameOrAfter from "dayjs/plugin/isSameOrAfter";
import isSameOrBefore from "dayjs/plugin/isSameOrBefore";
import { get, writable } from "svelte/store";

import { ROOT_EXPENSE_SCOPE } from "./expense";
import { renderCurrentExpensesBreakdown, renderMonthlyExpensesTimeline } from "./expense/monthly";
import type { Posting } from "./utils";

dayjs.extend(isSameOrBefore);
dayjs.extend(isSameOrAfter);

function posting(date: string, account: string, amount: number): Posting {
  return {
    id: `${date}-${account}-${amount}`,
    date: dayjs(date),
    payee: account,
    account,
    commodity: "EUR",
    quantity: amount,
    amount,
    status: "",
    tag_recurring: "",
    transaction_begin_line: 1,
    transaction_end_line: 1,
    file_name: "main.ledger",
    note: "",
    transaction_note: "",
    market_amount: amount,
    balance: amount
  };
}

function mountTimeline() {
  const wrapper = document.createElement("div");
  Object.defineProperty(wrapper, "clientWidth", {
    configurable: true,
    value: 900
  });

  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("id", "d3-monthly-expense-timeline");
  svg.setAttribute("width", "100%");
  svg.setAttribute("height", "400");

  wrapper.appendChild(svg);
  document.body.appendChild(wrapper);
}

function mountCurrentMonthBreakdown() {
  const wrapper = document.createElement("div");
  Object.defineProperty(wrapper, "clientWidth", {
    configurable: true,
    value: 900
  });

  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("id", "d3-current-month-breakdown");
  svg.setAttribute("width", "100%");

  wrapper.appendChild(svg);
  document.body.appendChild(wrapper);
}

describe("monthly expense timeline interactions", () => {
  const postings = [
    posting("2026-01-05", "Expenses:Food:Groceries", 20),
    posting("2026-01-12", "Expenses:Car:Fuel", 50),
    posting("2026-02-08", "Expenses:Food:Restaurants", 35),
    posting("2026-02-18", "Expenses:Car:Maintenance", 75)
  ];

  beforeEach(() => {
    document.body.innerHTML = "";
    mountTimeline();
  });

  test("timeline legend filtering stays independent from monthly detail scope", () => {
    const timelineGroup = writable<string | null>(null);
    const selectedMonth = writable("2099-01");
    const detailScope = writable(ROOT_EXPENSE_SCOPE);
    const dateRange = writable({
      from: dayjs("2026-01-01"),
      to: dayjs("2026-12-31")
    });

    const { legends, destroy } = renderMonthlyExpensesTimeline(
      postings,
      timelineGroup,
      selectedMonth,
      dateRange
    );

    const foodLegend = legends.find((legend) => String(legend.label).includes("Food"));
    expect(foodLegend).toBeDefined();

    foodLegend?.onClick?.(foodLegend);

    expect(get(timelineGroup)).toBe("Food");
    expect(get(detailScope)).toBe(ROOT_EXPENSE_SCOPE);

    const firstBar = document.querySelector("#d3-monthly-expense-timeline rect");
    expect(firstBar).toBeTruthy();

    firstBar?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(get(selectedMonth)).not.toBe("2099-01");
    expect(get(timelineGroup)).toBe("Food");
    expect(get(detailScope)).toBe(ROOT_EXPENSE_SCOPE);

    foodLegend?.onClick?.(foodLegend);
    expect(get(timelineGroup)).toBeNull();

    destroy();
  });
});

describe("monthly expense breakdown drilldown", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    mountCurrentMonthBreakdown();
  });

  test("only expandable top-level categories trigger drilldown", () => {
    const drilldowns: string[] = [];
    const renderer = renderCurrentExpensesBreakdown(
      d3.scaleOrdinal<string, string>().domain(["Food", "Insurance"]).range(["#0f0", "#f00"]),
      {
        onDrilldown: (scope) => drilldowns.push(scope)
      }
    );

    renderer(
      [
        posting("2026-01-05", "Expenses:Food:Groceries", 20),
        posting("2026-01-06", "Expenses:Food:Restaurants", 35),
        posting("2026-01-07", "Expenses:Insurance", 50)
      ],
      ROOT_EXPENSE_SCOPE
    );

    const bars = Array.from(document.querySelectorAll("#d3-current-month-breakdown rect"));
    expect(bars).toHaveLength(2);
    expect(bars.map((bar) => bar.getAttribute("class"))).toEqual(["zoomable", null]);

    bars[0]?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    bars[1]?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(drilldowns).toEqual(["Expenses:Food"]);
  });

  test("subcategory detail bars cannot drill deeper", () => {
    const drilldowns: string[] = [];
    const renderer = renderCurrentExpensesBreakdown(
      d3
        .scaleOrdinal<string, string>()
        .domain(["Groceries", "Restaurants"])
        .range(["#0f0", "#f0f"]),
      {
        onDrilldown: (scope) => drilldowns.push(scope)
      }
    );

    renderer(
      [
        posting("2026-01-05", "Expenses:Food:Groceries", 20),
        posting("2026-01-06", "Expenses:Food:Restaurants:Lunch", 35)
      ],
      "Expenses:Food"
    );

    const bars = Array.from(document.querySelectorAll("#d3-current-month-breakdown rect"));
    expect(bars).toHaveLength(2);
    expect(bars.map((bar) => bar.getAttribute("class"))).toEqual([null, null]);

    bars[0]?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    bars[1]?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(drilldowns).toEqual([]);
  });
});
